package app

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/GoSeoTaxi/t1/internal/config"
	"github.com/GoSeoTaxi/t1/internal/models"
	"github.com/GoSeoTaxi/t1/internal/storage"
	"go.uber.org/zap"
	"net/http"
	"time"
)

type Worker struct {
	ctx    context.Context
	logger *zap.Logger
	db     storage.DBinterface
	cfg    *config.Config
}

func NewWorker(ctx context.Context, logger *zap.Logger, db storage.DBinterface, cfg *config.Config) Worker {
	return Worker{
		ctx:    ctx,
		logger: logger,
		db:     db,
		cfg:    cfg,
	}
}

// UpdateStatus acts as worker that can update status of an order
func (w *Worker) UpdateStatus(t <-chan time.Time) {

	client := &http.Client{}
	for {
		select {
		case <-t:
			w.logger.Info("starting bonus update")
			oin := make(chan []models.Order)
			oout := make(chan models.Order)
			go w.db.SelectOrdersForUpdate(w.ctx, w.cfg, oin, oout)
			go w.getAccrual(oin, oout, client)
		case <-w.ctx.Done():
			w.logger.Info("context canceled")
		}
	}
}

// getAccrual updates statatus for each order in the selected order list, status updates are the forwarded to a channel
// sending data furter to pg
func (w *Worker) getAccrual(oin chan []models.Order, oout chan models.Order, client *http.Client) {
	url := fmt.Sprintf("%s/api/orders/", w.cfg.AccrualSystem)
	orders := <-oin

	for _, order := range orders {
		var intermOrder models.AccrualOrder
		url += fmt.Sprint(order.ID)
		request, err := http.NewRequest(http.MethodGet, url, nil)

		if err != nil {
			w.logger.Fatal("request creation failed", zap.Error(err))
		}

		response, requestErr := w.requestWithRetry(client, request)

		if requestErr != nil {
			w.logger.Error(requestErr.Error())
		}
		defer response.Body.Close()

		decoder := json.NewDecoder(response.Body)
		err = decoder.Decode(&intermOrder)
		if err != nil {
			w.logger.Debug("Error processing response" + err.Error())
		}

		oout <- models.Order{ID: intermOrder.ID, Amount: intermOrder.Amount, Status: intermOrder.Status}

	}
	close(oout)
	w.logger.Info("bonus update finished")
}

// requestWithRetry sends requests several times if error or not 200 response happen
func (w *Worker) requestWithRetry(client *http.Client, request *http.Request) (*http.Response, error) {
	var response *http.Response
	var requestErr error
	for i := 0; i < 5; i++ {
		response, requestErr = client.Do(request)
		if requestErr != nil {
			w.logger.Info("Retrying: " + requestErr.Error())
		} else if response.StatusCode == http.StatusTooManyRequests {
			w.logger.Info("Too many requests")
			time.Sleep(30 * time.Second)
		} else if response.StatusCode == http.StatusOK {
			return response, nil
		}
		w.logger.Info("Retrying...")
		time.Sleep(time.Duration(i*10) * time.Second)
	}

	return response, requestErr
}
