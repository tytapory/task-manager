package config

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

var _ Config = (*ConfigEnv)(nil)

type AppConfig struct {
	Database DatabaseConfig
	Redis    RedisConfig
	Server   ServerConfig
	JWT      JWTConfig
}

type DatabaseConfig struct {
	Host     string `env:"DB_HOST,required"`
	Port     string `env:"DB_PORT,required"`
	User     string `env:"DB_USER,required"`
	Password string `env:"DB_PASSWORD,required"`
	Name     string `env:"DB_NAME,required"`
	Charset  string `env:"DB_CHARSET" envDefault:"utf8mb4"`
}

type RedisConfig struct {
	Host     string        `env:"REDIS_HOST,required"`
	Port     string        `env:"REDIS_PORT,required"`
	Password string        `env:"REDIS_PASSWORD"`
	DB       int           `env:"REDIS_DB" envDefault:"0"`
	TTL      time.Duration `env:"REDIS_TTL" envDefault:"24h"`
}

type ServerConfig struct {
	Host    string        `env:"SERVER_HOST" envDefault:"0.0.0.0"`
	Port    string        `env:"SERVER_PORT" envDefault:"8080"`
	Timeout time.Duration `env:"SERVER_TIMEOUT" envDefault:"30s"`
}

type JWTConfig struct {
	Secret string        `env:"JWT_SECRET,required"`
	TTL    time.Duration `env:"JWT_TTL" envDefault:"72h"`
}

type Config interface {
	GetAppConfig() AppConfig
}

type ConfigEnv struct {
	cfg AppConfig
}

func (c *ConfigEnv) GetAppConfig() AppConfig {
	return c.cfg
}

func LoadConfig() (*ConfigEnv, error) {
	if err := godotenv.Load(); err != nil {
		slog.Warn("failed to load .env file, reading variables directly from environment", "error", err)
	}

	var appCfg AppConfig
	if err := env.Parse(&appCfg); err != nil {
		return nil, fmt.Errorf("env parsing error: %w", err)
	}

	return &ConfigEnv{cfg: appCfg}, nil
}
