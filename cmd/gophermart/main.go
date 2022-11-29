package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/GoSeoTaxi/t1/internal/Logger"
	"github.com/GoSeoTaxi/t1/internal/app"
	"github.com/GoSeoTaxi/t1/internal/config"
	"github.com/GoSeoTaxi/t1/internal/handlers"
	"github.com/GoSeoTaxi/t1/internal/storage"
	"go.uber.org/zap"
)

func main() {
	fmt.Println("starting...")

	// init Config
	cfg := config.NewConfig()

	// init New logger
	logger := Logger.NewLogger(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// initialize db

	db, err := storage.InitDB(ctx, cfg, logger)
	if err != nil {
		logger.Fatal("Error initializing db", zap.Error(err))
	}

	// prepare handles
	r := handlers.BonusRouter(ctx, db, cfg.Key, logger)
	srv := &http.Server{Addr: cfg.Endpoint, Handler: r}

	// run update status periodically
	statusTicker := time.NewTicker(time.Duration(1) * time.Second)
	worker := app.NewWorker(ctx, logger, db, cfg)
	go worker.UpdateStatus(statusTicker.C)

	logger.Info("Start serving on", zap.String("endpoint name", cfg.Endpoint))
	go log.Fatal(srv.ListenAndServe())

	// handle service stop
	for {
		quit := make(chan os.Signal)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
		select {
		case sig := <-quit:
			logger.Info(fmt.Sprintf("caught sig: %+v", sig))
			logger.Info("Microservice stopped successful!")
			break
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}
