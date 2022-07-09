package storage

import (
	"context"
	"fmt"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.uber.org/zap"
	"log"
	"maffka123gophermarktBonus/internal/config"
	"maffka123gophermarktBonus/internal/models"
	"strings"
	"time"
)

type PGinterface interface {
	Begin(context.Context) (pgx.Tx, error)
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
	Close()
}

type DBinterface interface {
	CreateNewUser(context.Context, *models.User) (int, error)
	SelectPass(context.Context, *models.User) (*string, error)
	SelectBalance(context.Context, int64) (*models.Balance, error)
	SelectUserForOrder(context.Context, models.Order) (int64, error)
	InsertOrder(context.Context, models.Order) error
	SelectOrdersForUpdate(context.Context, *config.Config, chan []models.Order, chan models.Order)
	SelectAllOrders(context.Context, int64) ([]*models.Order, error)
	SelectAllWithdrawals(context.Context, int64) (*[]models.Withdrawal, error)
}

type PGDB struct {
	path string
	Conn PGinterface
	log  *zap.Logger
}

// InitDB initialized pg connection and creates tables
func InitDB(ctx context.Context, cfg *config.Config, logger *zap.Logger, bp string) (*PGDB, error) {
	db := PGDB{
		path: cfg.DBpath,
		log:  logger,
	}
	conn, err := pgxpool.Connect(ctx, cfg.DBpath)

	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %v", err)
	}
	db.Conn = conn

	db.log.Info("initializing db tables...")

	tx, err := db.Conn.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to db: %v", err)
	}
	defer tx.Rollback(ctx)

	for _, q := range strings.Split(CreateDB, ";") {
		q := strings.TrimSpace(q)
		if q == "" {
			continue
		}
		if _, err := tx.Exec(ctx, q); err != nil {
			return nil, fmt.Errorf("failed executing sql: %v", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit failed: %v", err)
	}
	db.log.Info("db initialized succesfully")

	return &db, nil
}

// CreateNewUser insertes new user, handles not unique users
func (db *PGDB) CreateNewUser(ctx context.Context, user *models.User) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := db.Conn.QueryRow(ctx, `INSERT INTO users (login, password) VALUES($1,$2) RETURNING id;`, user.Login, user.Password).Scan(&user.ID)

	if err != nil && strings.Contains(err.Error(), "violates") {
		return -1, fmt.Errorf("user already exists: %v", err)
	} else if err != nil {
		return 0, fmt.Errorf("insert new user failed: %v", err)
	}

	return 1, nil
}

// SelectPass gets hashed password for a particular user
func (db *PGDB) SelectPass(ctx context.Context, user *models.User) (*string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var val string
	row := db.Conn.QueryRow(ctx, "SELECT password, id FROM users WHERE login=$1", user.Login)
	err := row.Scan(&val, &user.ID)

	if err != nil {
		return nil, fmt.Errorf("select from users failed: %v", err)
	}
	// TODO user does not exist
	return &val, nil
}

// SelectBalance gets sum of all bonuses and sum of all withdrawals for a user
func (db *PGDB) SelectBalance(ctx context.Context, user int64) (*models.Balance, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var val models.Balance
	row := db.Conn.QueryRow(ctx, "SELECT COALESCE(SUM(change), 0), COALESCE(SUM(nullif(LEAST(change, 0),0)),0) FROM bonuses WHERE user_id=$1 AND status='PROCESSED'", user)
	err := row.Scan(&val.Current, &val.Withdrawn)

	if err != nil {
		return nil, fmt.Errorf("select balance from bonuses failed: %v", err)
	}

	return &val, nil
}

// SelectUserForOrder tries to find if some order id was already used and if yes by which user
func (db *PGDB) SelectUserForOrder(ctx context.Context, order models.Order) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var val int64
	row := db.Conn.QueryRow(ctx, "SELECT users.id FROM users JOIN bonuses ON users.id=bonuses.user_id WHERE order_id=$1", order.ID)
	err := row.Scan(&val)

	if err == pgx.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("select from users failed: %v", err)
	}

	return val, nil
}

// doAsTransaction allow run sql statements inside one transaction
func (db *PGDB) doAsTransaction(ctx context.Context, fu ...func(pgx.Tx) error) error {
	/*ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()*/
	tx, err := db.Conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting connection failed: %v", err)
	}
	defer tx.Rollback(ctx)

	for _, f := range fu {
		err := f(tx)
		if err != nil {
			return fmt.Errorf("transaction failed: %v", err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("transaction failed: %v", err)
	}

	return nil
}

// InsertOrder appends new order to existing bonuses
func (db *PGDB) InsertOrder(ctx context.Context, order models.Order) error {
	err := db.doAsTransaction(ctx,
		func(tx pgx.Tx) error {
			_, err := tx.Prepare(ctx, "insert bonuses", `INSERT INTO bonuses (user_id, order_id, change, type, status) VALUES($1,$2,$3,$4,$5);`)

			if err != nil {
				return fmt.Errorf("init bonuses insert failed: %v", err)
			}

			if _, err = tx.Exec(ctx, "insert bonuses", order.UserID, order.ID, order.Amount, order.Type, order.Status); err != nil {
				return fmt.Errorf("update bonuses failed: %v", err)
			}
			return nil
		},
		func(tx pgx.Tx) error {
			_, err := tx.Prepare(ctx, "update amount", `UPDATE users SET balance=balance+$1 where id=$2;`)

			if err != nil {
				return fmt.Errorf("init update users failed: %v", err)
			}

			if _, err = tx.Exec(ctx, "update amount", order.Amount, order.UserID); err != nil {
				return fmt.Errorf("update amount failed: %v", err)
			}
			return nil
		})

	if err != nil {
		return fmt.Errorf("do with transaction failed: %v", err)
	}

	return nil
}

// SelectAllOrders gets all orders for particular user
func (db *PGDB) SelectAllOrders(ctx context.Context, u int64) ([]*models.Order, error) {
	var listOrders []*models.Order

	row, err := db.Conn.Query(context.Background(), `SELECT order_id, status, change, change_date 
														FROM bonuses WHERE user_id=$1 ORDER BY change_date`, u)
	if err != nil {
		return nil, fmt.Errorf("init select from orders failed: %v", err)
	}
	defer row.Close()

	for row.Next() {
		var o models.Order
		err := row.Scan(&o.ID, &o.Status, &o.Amount, &o.Date)
		if err != nil {
			db.log.Error("select orders failed:", zap.Error(err))
		}
		listOrders = append(listOrders, &o)
	}

	return listOrders, nil
}

// SelectAllWithdrawals gets all withdrwals for particular user
func (db *PGDB) SelectAllWithdrawals(ctx context.Context, u int64) (*[]models.Withdrawal, error) {
	var listOrders []models.Withdrawal

	row, err := db.Conn.Query(context.Background(), `SELECT order_id, change, change_date 
														FROM bonuses WHERE user_id=$1 AND type='withdraw' ORDER BY change_date`, u)
	if err != nil {
		return nil, fmt.Errorf("init select from orders failed: %v", err)
	}
	defer row.Close()

	for row.Next() {
		var o models.Withdrawal
		err := row.Scan(&o.ID, &o.Amount, &o.Date)
		if err != nil {
			db.log.Error("select orders failed:", zap.Error(err))
		}
		listOrders = append(listOrders, o)
	}

	return &listOrders, nil
}

// SelectOrdersForUpdate gets orders which are not yet finished and leaves transaction open
// until updated order comes back
// oin channel sends all selected orders to update system
// oout channel recieves updated orders and allows update table as a stream
func (db *PGDB) SelectOrdersForUpdate(ctx context.Context, cfg *config.Config, oin chan []models.Order, oout chan models.Order) {
	var listOrders []models.Order
	err := db.doAsTransaction(ctx,
		func(tx pgx.Tx) error {

			row, err := tx.Query(context.Background(), `SELECT order_id, status FROM bonuses 
										WHERE status not in ('PROCESSED', 'INVALID') LIMIT $1 FOR UPDATE SKIP LOCKED`, cfg.RowsToUpdate)
			if err != nil {
				return fmt.Errorf("init select from bonuses failed: %v", err)
			}
			defer row.Close()

			for row.Next() {
				var o models.Order
				err := row.Scan(&o.ID, &o.Status)
				if err != nil {
					return fmt.Errorf("select bonuses for update failed: %v", err)
				}
				listOrders = append(listOrders, o)
			}
			oin <- listOrders
			return nil
		},
		func(tx pgx.Tx) error {
			_, err := tx.Prepare(ctx, "update bonuses", `UPDATE bonuses SET change=$1, status=$2 where order_id=$3;`)

			if err != nil {
				return fmt.Errorf("init update users failed: %v", err)
			}

			_, err = tx.Prepare(ctx, "update user amount", `UPDATE users SET balance=balance+$1 where id=$2;`)

			if err != nil {
				return fmt.Errorf("init update users failed: %v", err)
			}

			for {
			insertUpdates:
				select {
				case bonus, ok := <-oout:
					if !ok {
						break insertUpdates
					}
					if _, err = tx.Exec(ctx, "update user amount", bonus.Amount, bonus.UserID); err != nil {
						return fmt.Errorf("update user amount failed: %v", err)
					}

					if _, err = tx.Exec(ctx, "update bonuses", bonus.Amount, bonus.Status, bonus.ID); err != nil {
						return fmt.Errorf("update amount failed: %v", err)
					}

				case <-ctx.Done():
					db.log.Info("context canceled")
					return nil
				}
				return nil
			}
		})

	if err != nil {
		log.Fatalf("transaction select bonuses for update failed: %v", err)
	}
}
