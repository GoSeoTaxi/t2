package Logger

import (
	"log"

	"github.com/GoSeoTaxi/t1/internal/config"
	"go.uber.org/zap"
)

func NewLogger(cfg *config.Config) *zap.Logger {

	logger, err := initLogger(cfg.Debug, cfg.AppName)
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}

	logger.Info("initializing the service...")
	return logger

}

// InitLogger configures zap logger
func initLogger(debug bool, projectID string) (*zap.Logger, error) {
	zapConfig := zap.NewProductionConfig()
	zapConfig.EncoderConfig.LevelKey = "severity"
	zapConfig.EncoderConfig.MessageKey = "message"

	if debug {
		zapConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}

	logger, err := zapConfig.Build(zap.Fields(
		zap.String("projectID", projectID),
	))

	if err != nil {
		return nil, err
	}

	return logger, nil
}
