package config

import "github.com/caarlos0/env/v11"

type Config struct {
	HTTPPort    string `env:"HTTP_PORT"          envDefault:"8080"`
	DatabaseURL string `env:"DATABASE_URL,required"`
	LogLevel    string `env:"LOG_LEVEL"          envDefault:"info"`
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
