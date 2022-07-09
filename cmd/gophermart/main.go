package main

import (
	"context"
	"fmt"
	"github.com/GoSeoTaxi/t1/internal/app"
	"github.com/GoSeoTaxi/t1/internal/config"
	"github.com/GoSeoTaxi/t1/internal/handlers"
	"github.com/GoSeoTaxi/t1/internal/storage"
	"go.uber.org/zap"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	fmt.Print("starting...")
	cfg, err := config.InitConfig()
	if err != nil {
		log.Fatalf("can't load config: %v", err)
	}

	logger, err := config.InitLogger(cfg.Debug, cfg.AppName)
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}

	logger.Info("initializing the service...")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// initialize db
	bp := app.GetBasePath()
	db, err := storage.InitDB(ctx, cfg, logger, bp)
	if err != nil {
		logger.Fatal("Error initializing db", zap.Error(err))
	}

	// prepare handles
	r := handlers.BonusRouter(ctx, db, cfg.Key, logger)

	// handle service stop
	srv := &http.Server{Addr: cfg.Endpoint, Handler: r}
	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		sig := <-quit
		logger.Info(fmt.Sprintf("caught sig: %+v", sig))
		if err := srv.Shutdown(ctx); err != nil {
			// Error from closing listeners, or context timeout:
			logger.Error("HTTP server Shutdown:", zap.Error(err))
		}
	}()

	// run update status periodically
	statusTicker := time.NewTicker(time.Duration(115) * time.Second)
	worker := app.NewWorker(ctx, logger, db, cfg)
	go worker.UpdateStatus(statusTicker.C)

	logger.Info("Start serving on", zap.String("endpoint name", cfg.Endpoint))
	log.Fatal(srv.ListenAndServe())

}
