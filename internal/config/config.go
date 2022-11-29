package config

import (
	"flag"
	"log"

	"github.com/caarlos0/env/v6"
)

type Config struct {
	Endpoint      string `env:"RUN_ADDRESS"`
	AppName       string `env:"APP_NAME" envDefault:"BonusApp"`
	Debug         bool   `env:"BONUS_APP_SERVER_DEBUG"`
	DBpath        string `env:"DATABASE_URI"`
	AccrualSystem string `env:"ACCRUAL_SYSTEM_ADDRESS"`
	Key           string `env:"KEY"`
	RowsToUpdate  int64  `env:"ROWS_UPDATE" envDefault:"1"`
}

// Make config via initConfig
func NewConfig() *Config {
	//	var cfg *Config
	cfg := new(Config)
	cfg, err := newConfing(cfg)
	if err != nil {
		log.Fatalf("can't load config: %v", err)
	}
	return cfg
}

// InitConfig initialises config, first from flags, then from env, so that env overwrites flags
func newConfing(cfg *Config) (*Config, error) {

	flag.StringVar(&cfg.Endpoint, "a", "127.0.0.1:8081", "server address as host:port")
	flag.StringVar(&cfg.DBpath, "d", "postgres://postgres:pass@localhost:5431/bonuses?pool_max_conns=10", "path for connection with pg: postgres://postgres:pass@localhost:5431/bonuses?pool_max_conns=10")
	flag.BoolVar(&cfg.Debug, "debug", true, "key for hash function")
	flag.StringVar(&cfg.AccrualSystem, "r", "http://127.0.0.1:8080", "accrual system address as host:port")
	flag.StringVar(&cfg.Key, "k", "someweirdkey!", "key for hash function")

	flag.Parse()

	err := env.Parse(cfg)

	return cfg, err
}
