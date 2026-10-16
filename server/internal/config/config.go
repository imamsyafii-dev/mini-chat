package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	defaultHTTPAddr        = ":28080"
	defaultShutdownTimeout = 10 * time.Second
)

var (
	ErrHTTPAddrRequired = errors.New(
		"HTTP address is required",
	)
	ErrShutdownTimeoutInvalid = errors.New(
		"shutdown timeout is invalid",
	)
	ErrPostgresURLRequired = errors.New(
		"PostgreSQL URL is required",
	)
)

type Config struct {
	HTTPAddr        string
	ShutdownTimeout time.Duration
	PostgresURL     string
}

type lookupEnvFunc func(
	key string,
) (string, bool)

func Load() (Config, error) {
	return load(os.LookupEnv)
}

func load(
	lookupEnv lookupEnvFunc,
) (Config, error) {
	result := Config{
		HTTPAddr:        defaultHTTPAddr,
		ShutdownTimeout: defaultShutdownTimeout,
	}

	if value, exists :=
		lookupEnv("MINI_CHAT_HTTP_ADDR"); exists {
		value = strings.TrimSpace(value)
		if value == "" {
			return Config{},
				ErrHTTPAddrRequired
		}

		result.HTTPAddr = value
	}

	if value, exists :=
		lookupEnv(
			"MINI_CHAT_SHUTDOWN_TIMEOUT",
		); exists {
		value = strings.TrimSpace(value)
		if value == "" {
			return Config{},
				ErrShutdownTimeoutInvalid
		}

		timeout, err :=
			time.ParseDuration(value)
		if err != nil {
			return Config{},
				fmt.Errorf(
					"%w: %v",
					ErrShutdownTimeoutInvalid,
					err,
				)
		}

		if timeout <= 0 {
			return Config{},
				fmt.Errorf(
					"%w: must be positive",
					ErrShutdownTimeoutInvalid,
				)
		}

		result.ShutdownTimeout = timeout
	}

	postgresURL, exists :=
		lookupEnv("MINI_CHAT_POSTGRES_URL")
	if !exists {
		return Config{},
			ErrPostgresURLRequired
	}

	postgresURL = strings.TrimSpace(
		postgresURL,
	)
	if postgresURL == "" {
		return Config{},
			ErrPostgresURLRequired
	}

	result.PostgresURL = postgresURL

	return result, nil
}
