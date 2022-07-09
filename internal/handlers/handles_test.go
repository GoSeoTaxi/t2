package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"time"

	"github.com/GoSeoTaxi/t1/internal/config"
	"github.com/GoSeoTaxi/t1/internal/models"
	"github.com/GoSeoTaxi/t1/internal/storage"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestHandler_HandlerPostRegister(t *testing.T) {
	type want struct {
		statusCode int
	}
	type request struct {
		route string
		body  models.User
	}
	tests := []struct {
		name    string
		request request
		want    want
	}{
		{name: "register_success",
			request: request{route: "/api/user/register", body: models.User{Login: "test", Password: "pass"}},
			want:    want{statusCode: 200},
		},
		{name: "user_exists",
			request: request{route: "/api/user/register", body: models.User{Login: "error", Password: "pass"}},
			want:    want{statusCode: 409},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//init stuff
			logger, _ := zap.NewDevelopment()
			db := newFakeDB()
			r := BonusRouter(context.Background(), db, "test", logger)

			body, _ := json.Marshal(tt.request.body)

			request := httptest.NewRequest(http.MethodPost, tt.request.route, bytes.NewBuffer(body))
			request.Header.Add("Content-Type", "application/json")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, request)
			result := w.Result()
			defer result.Body.Close()

			assert.Equal(t, tt.want.statusCode, result.StatusCode)
		})
	}
}

func TestHandler_HandlerPostLogin(t *testing.T) {
	type want struct {
		statusCode int
		cookie     http.Cookie
	}
	type request struct {
		route string
		body  models.User
	}
	tests := []struct {
		name    string
		request request
		want    want
	}{
		{name: "login_success",
			request: request{route: "/api/user/login", body: models.User{Login: "test", Password: "pass"}},
			want:    want{statusCode: 200, cookie: http.Cookie{Name: "jwt", Value: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoxMX0.4s_93NLMD4-Rw_U4gdtmcBs_ZysBCfCi4ERuVwoOMig"}},
		},
		{name: "user_not_exist",
			request: request{route: "/api/user/login", body: models.User{Login: "error", Password: "pass"}},
			want:    want{statusCode: 401},
		},
		{name: "pass_wrong",
			request: request{route: "/api/user/login", body: models.User{Login: "test", Password: "pass1"}},
			want:    want{statusCode: 401},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//init stuff
			logger, _ := zap.NewDevelopment()
			db := newFakeDB()
			r := BonusRouter(context.Background(), db, "test", logger)

			body, _ := json.Marshal(tt.request.body)

			request := httptest.NewRequest(http.MethodPost, tt.request.route, bytes.NewBuffer(body))
			request.Header.Add("Content-Type", "application/json")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, request)
			result := w.Result()
			defer result.Body.Close()

			assert.Equal(t, tt.want.statusCode, result.StatusCode)
			if tt.want.statusCode == 200 {
				assert.Equal(t, tt.want.cookie.Value, result.Cookies()[0].Value)
				assert.Equal(t, tt.want.cookie.Name, result.Cookies()[0].Name)
			}

		})
	}
}

func TestHandler_HandlerPostOrders(t *testing.T) {
	type want struct {
		statusCode int
	}
	type request struct {
		route string
		body  int64
	}
	tests := []struct {
		name    string
		request request
		want    want
		db      fakeDB
	}{
		{name: "order_added",
			request: request{route: "/api/user/orders", body: 18},
			want:    want{statusCode: 202},
			db:      fakeDB{selectUserForOrder: 0},
		},
		{name: "order_exists",
			request: request{route: "/api/user/orders", body: 182},
			want:    want{statusCode: 200},
			db:      fakeDB{selectUserForOrder: 11},
		},
		{name: "order_exists_other_user",
			request: request{route: "/api/user/orders", body: 1826},
			want:    want{statusCode: 409},
			db:      fakeDB{selectUserForOrder: 2},
		},
		{name: "order_number_wrong",
			request: request{route: "/api/user/orders", body: 799273987131},
			want:    want{statusCode: 422},
			db:      fakeDB{selectUserForOrder: 0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//init stuff
			logger, _ := zap.NewDevelopment()
			r := BonusRouter(context.Background(), &tt.db, "test", logger)

			body, _ := json.Marshal(tt.request.body)

			request := httptest.NewRequest(http.MethodPost, tt.request.route, bytes.NewBuffer(body))
			request.AddCookie(&http.Cookie{Name: "jwt", Value: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoxMX0.4s_93NLMD4-Rw_U4gdtmcBs_ZysBCfCi4ERuVwoOMig"})
			request.Header.Add("Content-Type", "text/plain")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, request)
			result := w.Result()
			defer result.Body.Close()

			assert.Equal(t, tt.want.statusCode, result.StatusCode)

		})
	}
}

func TestHandler_HandlerGetOrders(t *testing.T) {
	type want struct {
		statusCode int
	}
	type request struct {
		route string
	}
	tests := []struct {
		name    string
		request request
		want    want
		db      fakeDB
	}{
		{name: "empty_orders",
			request: request{route: "/api/user/orders"},
			want:    want{statusCode: 204},
			db:      fakeDB{selectAllOrders: []*models.Order{}},
		},
		{name: "get_orders",
			request: request{route: "/api/user/orders"},
			want:    want{statusCode: 200},
			db: fakeDB{selectAllOrders: []*models.Order{{Amount: 50, ID: 18, Status: "PROCESSING", Date: time.Date(2021, time.Month(2), 21, 1, 10, 30, 0, time.UTC)},
				{Amount: 150, ID: 182, Status: "PROCESSED", Date: time.Date(2021, time.Month(2), 21, 1, 10, 30, 0, time.UTC)}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//init stuff
			logger, _ := zap.NewDevelopment()
			r := BonusRouter(context.Background(), &tt.db, "test", logger)

			request := httptest.NewRequest(http.MethodGet, tt.request.route, nil)
			request.AddCookie(&http.Cookie{Name: "jwt", Value: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoxMX0.4s_93NLMD4-Rw_U4gdtmcBs_ZysBCfCi4ERuVwoOMig"})
			w := httptest.NewRecorder()

			r.ServeHTTP(w, request)
			result := w.Result()
			defer result.Body.Close()

			assert.Equal(t, tt.want.statusCode, result.StatusCode)
			if tt.want.statusCode == 200 {
				decoder := json.NewDecoder(result.Body)
				var o []*models.Order
				decoder.Decode(&o)
				assert.Equal(t, o, tt.db.selectAllOrders)
			}

		})
	}
}

func TestHandler_HandlerGetBalance(t *testing.T) {
	type want struct {
		statusCode int
	}
	type request struct {
		route string
	}
	tests := []struct {
		name    string
		request request
		want    want
		db      fakeDB
	}{

		{name: "get_balance",
			request: request{route: "/api/user/balance"},
			want:    want{statusCode: 200},
			db:      fakeDB{selectBalance: models.Balance{Current: 12350, Withdrawn: 3470}},
		},

		{name: "get_balance_no_withdraw",
			request: request{route: "/api/user/balance"},
			want:    want{statusCode: 200},
			db:      fakeDB{selectBalance: models.Balance{Current: 12350}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//init stuff
			logger, _ := zap.NewDevelopment()
			r := BonusRouter(context.Background(), &tt.db, "test", logger)

			request := httptest.NewRequest(http.MethodGet, tt.request.route, nil)
			request.AddCookie(&http.Cookie{Name: "jwt", Value: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoxMX0.4s_93NLMD4-Rw_U4gdtmcBs_ZysBCfCi4ERuVwoOMig"})
			w := httptest.NewRecorder()

			r.ServeHTTP(w, request)
			result := w.Result()
			defer result.Body.Close()

			assert.Equal(t, tt.want.statusCode, result.StatusCode)
			if tt.want.statusCode == 200 {
				decoder := json.NewDecoder(result.Body)
				var o models.Balance
				decoder.Decode(&o)
				assert.Equal(t, o, tt.db.selectBalance)
			}

		})
	}
}

func TestHandler_HandlerPostWithdraw(t *testing.T) {
	type want struct {
		statusCode int
	}
	type request struct {
		route string
	}
	tests := []struct {
		name     string
		request  request
		want     want
		db       fakeDB
		withdraw models.Withdrawal
	}{
		{name: "not_enough",
			request: request{route: "/api/user/balance/withdraw"},
			want:    want{statusCode: 402},
			db: fakeDB{selectBalance: models.Balance{Current: 100},
				selectAllWithdrawals: []models.Withdrawal{{Amount: 50, ID: 18, Date: time.Date(2021, time.Month(2), 21, 1, 10, 30, 0, time.UTC)},
					{Amount: 500, ID: 182, Date: time.Date(2021, time.Month(2), 21, 1, 10, 30, 0, time.UTC)}}},
			withdraw: models.Withdrawal{ID: 18, Amount: 500},
		},
		{name: "withdraw_success",
			request:  request{route: "/api/user/balance/withdraw"},
			want:     want{statusCode: 200},
			db:       fakeDB{selectBalance: models.Balance{Current: 10000}},
			withdraw: models.Withdrawal{ID: 18, Amount: 50},
		},
		{name: "order_number_wrong",
			request:  request{route: "/api/user/balance/withdraw"},
			want:     want{statusCode: 422},
			db:       fakeDB{},
			withdraw: models.Withdrawal{ID: 799273987131, Amount: 50},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//init stuff
			logger, _ := zap.NewDevelopment()
			r := BonusRouter(context.Background(), &tt.db, "test", logger)

			body, _ := json.Marshal(&tt.withdraw)

			request := httptest.NewRequest(http.MethodPost, tt.request.route, bytes.NewBuffer(body))
			request.AddCookie(&http.Cookie{Name: "jwt", Value: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoxMX0.4s_93NLMD4-Rw_U4gdtmcBs_ZysBCfCi4ERuVwoOMig"})
			request.Header.Add("Content-Type", "text/plain")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, request)
			result := w.Result()
			defer result.Body.Close()

			assert.Equal(t, tt.want.statusCode, result.StatusCode)
			if tt.want.statusCode == 200 {
				decoder := json.NewDecoder(result.Body)
				var o []models.Withdrawal
				decoder.Decode(&o)
				assert.Equal(t, o, tt.db.selectAllWithdrawals)
			}

		})
	}
}

type fakeDB struct {
	selectUserForOrder   int64
	Conn                 storage.PGinterface
	selectAllOrders      []*models.Order
	selectBalance        models.Balance
	selectAllWithdrawals []models.Withdrawal
}

func newFakeDB() *fakeDB {
	return &fakeDB{}
}

func (db *fakeDB) CreateNewUser(ctx context.Context, user *models.User) (int, error) {
	if user.Login == "error" {
		return -1, fmt.Errorf("user already exists")
	}
	return 1, nil
}

func (db *fakeDB) SelectPass(ctx context.Context, user *models.User) (*string, error) {
	if user.Login == "error" {
		return nil, fmt.Errorf("user not found")
	}
	np := sha256.Sum256([]byte("pass"))
	npb := hex.EncodeToString(np[:])
	user.ID = 11
	return &npb, nil
}

func (db *fakeDB) SelectUserForOrder(ctx context.Context, o models.Order) (int64, error) {
	return db.selectUserForOrder, nil
}
func (db *fakeDB) InsertOrder(ctx context.Context, o models.Order) error {
	return nil
}

func (db *fakeDB) SelectAllOrders(ctx context.Context, u int64) ([]*models.Order, error) {
	return db.selectAllOrders, nil
}

func (db *fakeDB) SelectAllWithdrawals(ctx context.Context, u int64) (*[]models.Withdrawal, error) {
	return &db.selectAllWithdrawals, nil
}

func (db *fakeDB) SelectBalance(ctx context.Context, user int64) (*models.Balance, error) {
	return &db.selectBalance, nil
}

func (db *fakeDB) SelectOrdersForUpdate(ctx context.Context, cfg *config.Config, ch chan []models.Order, ch2 chan models.Order) {
}
