package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/GoSeoTaxi/t1/internal/app"

	"context"
	"strings"

	"github.com/GoSeoTaxi/t1/internal/models"
	"github.com/GoSeoTaxi/t1/internal/storage"
	"github.com/go-chi/jwtauth/v5"
	"go.uber.org/zap"
)

type Handler struct {
	db     storage.DBinterface
	logger *zap.Logger
	ctx    context.Context
}

func NewHandler(ctx context.Context, db storage.DBinterface, logger *zap.Logger) *Handler {
	return &Handler{
		db:     db,
		logger: logger,
		ctx:    ctx,
	}
}

// HandlerPostRegister creates new user if user with such login not yet exist
func (h *Handler) HandlerPostRegister(tokenAuth *jwtauth.JWTAuth) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var u models.User
		err := decoder.Decode(&u)
		h.logger.Debug("recieved new user: ", zap.String("login", u.Login))

		if err != nil {
			http.Error(w, fmt.Sprintf("400 - Register json cannot be decoded: %s", err), http.StatusBadRequest)
			return
		}

		exists, err := h.db.CreateNewUser(h.ctx, &u)
		if exists == -1 {
			http.Error(w, fmt.Sprintf("409 - Login is already taken: %s", err), http.StatusConflict)
			return
		} else if err != nil {
			http.Error(w, fmt.Sprintf("500 - Internal error: %s", err), http.StatusInternalServerError)
			return
		} else if exists == 1 {

			_, tokenString, _ := tokenAuth.Encode(map[string]interface{}{"user_id": u.ID})

			h.logger.Debug("logged in: ", zap.String("login", u.Login))
			http.SetCookie(w, &http.Cookie{
				Name:  "jwt",
				Value: tokenString,
			})
			w.Header().Set("application-type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok}`))
		}
	}
}

// HandlerPostLogin logins user if login and password are valid
func (h *Handler) HandlerPostLogin(tokenAuth *jwtauth.JWTAuth) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var u models.User
		err := decoder.Decode(&u)
		h.logger.Debug("user is trying to login: ", zap.String("login", u.Login))

		if err != nil {
			http.Error(w, fmt.Sprintf("400 - Register json cannot be decoded: %s", err), http.StatusBadRequest)
			return
		}

		pass, err := h.db.SelectPass(h.ctx, &u)

		if err != nil {
			http.Error(w, fmt.Sprintf("500 - Internal error: %s", err), http.StatusInternalServerError)
			return
		}
		if pass == nil || !app.ComparePass(*pass, u.Password) {
			http.Error(w, fmt.Sprintf("401 - user or password are wrong: %s", err), http.StatusUnauthorized)
			return
		}

		_, tokenString, _ := tokenAuth.Encode(map[string]interface{}{"user_id": u.ID})

		h.logger.Debug("logged in: ", zap.String("login", u.Login))
		http.SetCookie(w, &http.Cookie{
			Name:  "jwt",
			Value: tokenString,
		})
		w.Header().Set("application-type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok}`))

	}
}

// HandlerPostOrders adds new order if order number is valid and does not yet exist
func (h *Handler) HandlerPostOrders() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var valid bool
		order := models.Order{}
		orderID, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("400 - could not parse order id: %s", err), http.StatusBadRequest)
			return
		}
		valid, order.ID, err = app.PrepOrderNumber(h.ctx, orderID)
		if err != nil {
			http.Error(w, fmt.Sprintf("400 - could not parse order id: %s", err), http.StatusBadRequest)
			return
		} else if !valid {
			http.Error(w, fmt.Sprintf("422 - wrong format of the order number: %s", err), http.StatusUnprocessableEntity)
			return
		}

		h.logger.Debug("adding new order: ", zap.String("order", string(orderID)))

		currUser, err := app.UserIDFromContext(r.Context())
		if err != nil {
			h.logger.Debug(err.Error())
			http.Error(w, fmt.Sprintf("400 - could not parse user id from token: %s", err), http.StatusBadRequest)
			return
		}
		h.logger.Debug("found user: ", zap.String("login", fmt.Sprint(currUser)))

		expectedUser, err := h.db.SelectUserForOrder(h.ctx, order)
		h.logger.Debug("found other user: ", zap.String("login", fmt.Sprint(expectedUser)))

		if expectedUser != 0 && currUser == expectedUser {
			w.Header().Set("application-type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok}`))
			return
		} else if expectedUser != 0 && currUser != expectedUser {
			http.Error(w, fmt.Sprintf("409 - Order was used by diferent user: %s", err), http.StatusConflict)
			return
		} else if err != nil {
			h.logger.Debug(err.Error())
			http.Error(w, fmt.Sprintf("500 - Internal error: %s", err), http.StatusInternalServerError)
			return
		}

		order.UserID = currUser
		order.Status = "NEW"
		order.Type = "top_up"

		err = h.db.InsertOrder(h.ctx, order)
		if err != nil {
			h.logger.Debug(err.Error())
			http.Error(w, fmt.Sprintf("500 - Internal error: %s", err), http.StatusInternalServerError)
			return
		}

		h.logger.Debug("order accepted: ", zap.String("login", string(orderID)))
		w.Header().Set("application-type", "text/plain")
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"status":"ok}`))
	}
}

// HandlerGetOrders gets list of all orders
func (h *Handler) HandlerGetOrders() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		h.logger.Debug("getting list of orders")

		currUser, err := app.UserIDFromContext(r.Context())
		if err != nil {
			http.Error(w, fmt.Sprintf("401 - could not parse user id from token: %s", err), http.StatusUnauthorized)
			return
		}

		orders, err := h.db.SelectAllOrders(h.ctx, currUser)
		if err != nil {
			http.Error(w, fmt.Sprintf("500 - internal server error: %s", err), http.StatusBadRequest)
			return
		} else if len(orders) == 0 {
			w.WriteHeader(http.StatusNoContent)
			w.Write([]byte(`{"status":"ok}`))
			return
		}

		mJSON, err := json.Marshal(orders)
		if err != nil {
			http.Error(w, fmt.Sprintf("500 - could not prepare data for return: %s", err), http.StatusBadRequest)
			return
		}

		h.logger.Debug(string(mJSON))
		h.logger.Debug(fmt.Sprintf("list of orders for user: %d", currUser), zap.String("len", fmt.Sprint(len(orders))))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(mJSON)
	}
}

// HandlerGetBalance get current balance and sum all withdrawals
func (h *Handler) HandlerGetBalance() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		h.logger.Debug("getting balance")

		currUser, err := app.UserIDFromContext(r.Context())
		if err != nil {
			http.Error(w, fmt.Sprintf("401 - could not parse user id from token: %s", err), http.StatusUnauthorized)
			return
		}

		balance, err := h.db.SelectBalance(h.ctx, currUser)
		if err != nil {
			http.Error(w, fmt.Sprintf("500 - internal server error: %s", err), http.StatusBadRequest)
			return
		}

		mJSON, err := json.Marshal(balance)
		if err != nil {
			http.Error(w, fmt.Sprintf("500 - could not prepare data for return: %s", err), http.StatusBadRequest)
			return
		}

		h.logger.Debug("balance for user: ", zap.String("login", fmt.Sprint(currUser)))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mJSON))
	}
}

// HandlerPostWithdraw adds an order with minus balance
func (h *Handler) HandlerPostWithdraw() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		h.logger.Debug("withdraw")

		currUser, err := app.UserIDFromContext(r.Context())
		if err != nil {
			http.Error(w, fmt.Sprintf("401 - could not parse user id from token: %s", err), http.StatusUnauthorized)
			return
		}

		decoder := json.NewDecoder(r.Body)
		var o models.Withdrawal
		err = decoder.Decode(&o)

		if err != nil && strings.Contains(err.Error(), "valid") {
			http.Error(w, fmt.Sprintf("422 - internal server error: %s", err), http.StatusUnprocessableEntity)
			return

		} else if err != nil {
			http.Error(w, fmt.Sprintf("500 - internal server error: %s", err), http.StatusBadRequest)
			return
		}

		balance, err := h.db.SelectBalance(h.ctx, currUser)
		if err != nil {
			http.Error(w, fmt.Sprintf("500 - internal server error: %s", err), http.StatusBadRequest)
			return
		}

		if balance.Current < o.Amount {
			http.Error(w, fmt.Sprintf("402 - currenct balance is not enough: %s", err), http.StatusPaymentRequired)
			return
		}

		order := models.Order{ID: o.ID, Amount: -o.Amount, UserID: currUser, Status: "PROCESSED", Type: "withdraw"}
		err = h.db.InsertOrder(h.ctx, order)
		if err != nil {
			http.Error(w, fmt.Sprintf("500 - internal server error: %s", err), http.StatusBadRequest)
			return
		}

		h.logger.Debug("withdraw succesfull for user: ", zap.String("login", fmt.Sprint(currUser)))
		w.Header().Set("application-type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok}`))
	}
}

// HandlerGetWithdrawals returns a list of all withwdrawals
func (h *Handler) HandlerGetWithdrawals() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		h.logger.Debug("withdrawals")

		currUser, err := app.UserIDFromContext(r.Context())
		if err != nil {
			http.Error(w, fmt.Sprintf("401 - could not parse user id from token: %s", err), http.StatusUnauthorized)
			return
		}

		orders, err := h.db.SelectAllWithdrawals(h.ctx, currUser)
		if err != nil {
			http.Error(w, fmt.Sprintf("500 - internal server error: %s", err), http.StatusBadRequest)
			return
		} else if len(*orders) == 0 {
			w.WriteHeader(http.StatusNoContent)
			w.Write([]byte(`{"status":"ok}`))
			return
		}

		mJSON, err := json.Marshal(orders)
		if err != nil {
			http.Error(w, fmt.Sprintf("500 - could not prepare data for return: %s", err), http.StatusBadRequest)
			return
		}

		h.logger.Debug("list of orders for user: ", zap.String("login", fmt.Sprint(currUser)))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mJSON))
	}
}
