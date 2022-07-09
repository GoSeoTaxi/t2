package models

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/theplant/luhn"
)

type User struct {
	ID       int64   `json:"id,omitempty"`
	Login    string  `json:"login"`
	Balance  float64 `json:"balance"`
	Password string  `json:"password"`
}

type Order struct {
	ID     int64     `json:"number,omitempty"`
	Status string    `json:"status,omitempty"`
	Amount int64     `json:"accrual,omitempty"`
	Date   time.Time `json:"uploaded_at,omitempty"`
	Type   string    `json:"type,omitempty"`
	UserID int64     `json:"user_id,omitempty"`
}

type AccrualOrder struct {
	ID     int64  `json:"order,omitempty"`
	Status string `json:"status,omitempty"`
	Amount int64  `json:"accrual,omitempty"`
}

type Withdrawal struct {
	ID     int64     `json:"order,omitempty"`
	Amount int64     `json:"sum,omitempty"`
	Date   time.Time `json:"processed_at,omitempty"`
}

type Balance struct {
	Current   int64 `json:"current"`
	Withdrawn int64 `json:"withdrawn"`
}

func (b *Balance) MarshalJSON() ([]byte, error) {
	type newBalance struct {
		Current   float64 `json:"current"`
		Withdrawn float64 `json:"withdrawn"`
	}

	nb := newBalance{
		Current:   float64(b.Current) / 100,
		Withdrawn: math.Abs(float64(b.Withdrawn)) / 100,
	}

	return json.Marshal(nb)
}

func (b *Balance) UnmarshalJSON(data []byte) error {
	type newBalance struct {
		Current   float64 `json:"current"`
		Withdrawn float64 `json:"withdrawn"`
	}

	var nu newBalance

	if err := json.Unmarshal(data, &nu); err != nil {
		return err
	}

	b.Current = int64(nu.Current * 100)
	b.Withdrawn = int64(nu.Withdrawn * 100)

	return nil
}

func (u *User) UnmarshalJSON(data []byte) error {
	type newU User
	nu := (*newU)(u)

	if err := json.Unmarshal(data, &nu); err != nil {
		return err
	}

	np := sha256.Sum256([]byte(u.Password))
	u.Password = hex.EncodeToString((np[:]))

	return nil
}

func (w *Withdrawal) UnmarshalJSON(data []byte) error {
	type newU struct {
		ID     string  `json:"order,omitempty"`
		Amount float64 `json:"sum,omitempty"`
	}
	nu := newU{}

	if err := json.Unmarshal(data, &nu); err != nil {
		return err
	}

	s, err := strconv.Atoi(nu.ID)
	if err != nil {
		return fmt.Errorf("order amount is not valid")
	}

	if !luhn.Valid(int(s)) {
		return fmt.Errorf("order id is not valid")
	}

	w.ID = int64(s)
	w.Amount = int64(nu.Amount * 100)

	return nil
}

func (w *Withdrawal) MarshalJSON() ([]byte, error) {
	type newWithdrawal struct {
		ID     string  `json:"order,omitempty"`
		Amount float64 `json:"sum,omitempty"`
		Date   string  `json:"processed_at,omitempty"`
	}

	nb := newWithdrawal{
		Amount: math.Abs(float64(w.Amount)) / 100,
		Date:   w.Date.Format(time.RFC3339),
		ID:     fmt.Sprint(w.ID),
	}

	return json.Marshal(nb)
}

func (o *Order) MarshalJSON() ([]byte, error) {
	type newOrder struct {
		ID     string  `json:"number,omitempty"`
		Status string  `json:"status"`
		Amount float64 `json:"accrual"`
		Date   string  `json:"uploaded_at,omitempty"`
	}

	nb := newOrder{
		Amount: math.Abs(float64(o.Amount)) / 100,
		Date:   o.Date.Format(time.RFC3339),
		ID:     fmt.Sprint(o.ID),
		Status: o.Status,
	}

	return json.Marshal(nb)
}

func (o *Order) UnmarshalJSON(data []byte) error {
	type newU struct {
		ID     string    `json:"number,omitempty"`
		Status string    `json:"status"`
		Amount float64   `json:"accrual"`
		Date   time.Time `json:"uploaded_at,omitempty"`
	}
	nu := newU{}

	if err := json.Unmarshal(data, &nu); err != nil {
		return err
	}

	s, err := strconv.Atoi(nu.ID)
	if err != nil {
		return fmt.Errorf("order id is not valid")
	}

	if !luhn.Valid(int(s)) {
		return fmt.Errorf("order id is not valid")
	}

	o.ID = int64(s)
	o.Amount = int64(nu.Amount * 100)
	o.Date = nu.Date
	o.Status = nu.Status

	return nil
}

func (u *AccrualOrder) UnmarshalJSON(data []byte) error {
	type newU struct {
		ID     string  `json:"order"`
		Status string  `json:"status"`
		Amount float64 `json:"accrual"`
	}
	nu := newU{}

	if err := json.Unmarshal(data, &nu); err != nil {
		return err
	}

	s, err := strconv.Atoi(nu.ID)
	if err != nil {
		return fmt.Errorf("order id is not valid")
	}

	if !luhn.Valid(int(s)) {
		return fmt.Errorf("order id is not valid")
	}

	u.ID = int64(s)
	u.Amount = int64(nu.Amount * 100)
	u.Status = nu.Status

	return nil
}
