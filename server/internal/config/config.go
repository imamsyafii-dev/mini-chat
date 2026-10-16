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
	defaultAccessTokenTTL  = time.Hour
	minimumJWTSecretBytes  = 32
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
	ErrJWTSecretInvalid = errors.New(
		"JWT secret is invalid",
	)
	ErrAccessTokenTTLInvalid = errors.New(
		"access token TTL is invalid",
	)
)

type Config struct {
	HTTPAddr        string
	ShutdownTimeout time.Duration
	PostgresURL     string
	JWTSecret       string
	AccessTokenTTL  time.Duration
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
		AccessTokenTTL:  defaultAccessTokenTTL,
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

	jwtSecret, exists :=
		lookupEnv("MINI_CHAT_JWT_SECRET")
	if !exists ||
		len(strings.TrimSpace(jwtSecret)) == 0 ||
		len([]byte(jwtSecret)) < minimumJWTSecretBytes {
		return Config{}, ErrJWTSecretInvalid
	}

	result.JWTSecret = jwtSecret

	if value, exists := lookupEnv(
		"MINI_CHAT_ACCESS_TOKEN_TTL",
	); exists {
		value = strings.TrimSpace(value)
		if value == "" {
			return Config{},
				ErrAccessTokenTTLInvalid
		}

		ttl, err := time.ParseDuration(value)
		if err != nil {
			return Config{},
				fmt.Errorf(
					"%w: invalid duration",
					ErrAccessTokenTTLInvalid,
				)
		}

		if ttl <= 0 {
			return Config{},
				fmt.Errorf(
					"%w: must be positive",
					ErrAccessTokenTTLInvalid,
				)
		}

		result.AccessTokenTTL = ttl
	}

	return result, nil
}
