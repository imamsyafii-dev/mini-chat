package config

import (
	"errors"
	"testing"
	"time"
)

func TestLoadUsesDefaults(
	t *testing.T,
) {
	result, err := load(
		func(
			key string,
		) (string, bool) {
			switch key {
			case "MINI_CHAT_POSTGRES_URL":
				return " postgres://mini-chat-test ", true
			case "MINI_CHAT_JWT_SECRET":
				return "0123456789abcdef0123456789abcdef", true
			}

			return "", false
		},
	)
	if err != nil {
		t.Fatalf(
			"load() error = %v, want nil",
			err,
		)
	}

	if result.HTTPAddr != defaultHTTPAddr {
		t.Errorf(
			"HTTPAddr = %q, want %q",
			result.HTTPAddr,
			defaultHTTPAddr,
		)
	}

	if result.ShutdownTimeout !=
		defaultShutdownTimeout {
		t.Errorf(
			"ShutdownTimeout = %v, want %v",
			result.ShutdownTimeout,
			defaultShutdownTimeout,
		)
	}

	if result.PostgresURL !=
		"postgres://mini-chat-test" {
		t.Errorf(
			"PostgresURL = %q, want %q",
			result.PostgresURL,
			"postgres://mini-chat-test",
		)
	}

	if result.JWTSecret !=
		"0123456789abcdef0123456789abcdef" {
		t.Errorf(
			"JWTSecret was not preserved",
		)
	}

	if result.AccessTokenTTL !=
		defaultAccessTokenTTL {
		t.Errorf(
			"AccessTokenTTL = %v, want %v",
			result.AccessTokenTTL,
			defaultAccessTokenTTL,
		)
	}
}

func TestLoadUsesEnvironmentOverrides(
	t *testing.T,
) {
	values := map[string]string{
		"MINI_CHAT_HTTP_ADDR":        " 127.0.0.1:29090 ",
		"MINI_CHAT_SHUTDOWN_TIMEOUT": " 15s ",
		"MINI_CHAT_POSTGRES_URL":     " postgres://mini-chat-override ",
		"MINI_CHAT_JWT_SECRET":       "  0123456789abcdef0123456789abcdef  ",
		"MINI_CHAT_ACCESS_TOKEN_TTL": " 45m ",
	}

	result, err := load(
		func(
			key string,
		) (string, bool) {
			value, exists := values[key]

			return value, exists
		},
	)
	if err != nil {
		t.Fatalf(
			"load() error = %v, want nil",
			err,
		)
	}

	if result.HTTPAddr !=
		"127.0.0.1:29090" {
		t.Errorf(
			"HTTPAddr = %q, want %q",
			result.HTTPAddr,
			"127.0.0.1:29090",
		)
	}

	if result.ShutdownTimeout !=
		15*time.Second {
		t.Errorf(
			"ShutdownTimeout = %v, want %v",
			result.ShutdownTimeout,
			15*time.Second,
		)
	}

	if result.PostgresURL !=
		"postgres://mini-chat-override" {
		t.Errorf(
			"PostgresURL = %q, want %q",
			result.PostgresURL,
			"postgres://mini-chat-override",
		)
	}

	if result.JWTSecret !=
		"  0123456789abcdef0123456789abcdef  " {
		t.Errorf("JWTSecret was not preserved")
	}

	if result.AccessTokenTTL !=
		45*time.Minute {
		t.Errorf(
			"AccessTokenTTL = %v, want %v",
			result.AccessTokenTTL,
			45*time.Minute,
		)
	}
}

func TestLoadRejectsInvalidEnvironment(
	t *testing.T,
) {
	tests := []struct {
		name    string
		values  map[string]string
		unset   []string
		wantErr error
	}{
		{
			name: "empty HTTP address",
			values: map[string]string{
				"MINI_CHAT_HTTP_ADDR": " ",
			},
			wantErr: ErrHTTPAddrRequired,
		},
		{
			name: "empty shutdown timeout",
			values: map[string]string{
				"MINI_CHAT_SHUTDOWN_TIMEOUT": " ",
			},
			wantErr: ErrShutdownTimeoutInvalid,
		},
		{
			name: "malformed shutdown timeout",
			values: map[string]string{
				"MINI_CHAT_SHUTDOWN_TIMEOUT": "ten-seconds",
			},
			wantErr: ErrShutdownTimeoutInvalid,
		},
		{
			name: "zero shutdown timeout",
			values: map[string]string{
				"MINI_CHAT_SHUTDOWN_TIMEOUT": "0s",
			},
			wantErr: ErrShutdownTimeoutInvalid,
		},
		{
			name: "negative shutdown timeout",
			values: map[string]string{
				"MINI_CHAT_SHUTDOWN_TIMEOUT": "-1s",
			},
			wantErr: ErrShutdownTimeoutInvalid,
		},
		{
			name:    "missing PostgreSQL URL",
			unset:   []string{"MINI_CHAT_POSTGRES_URL"},
			wantErr: ErrPostgresURLRequired,
		},
		{
			name: "empty PostgreSQL URL",
			values: map[string]string{
				"MINI_CHAT_POSTGRES_URL": " ",
			},
			wantErr: ErrPostgresURLRequired,
		},
		{
			name:    "missing JWT secret",
			unset:   []string{"MINI_CHAT_JWT_SECRET"},
			wantErr: ErrJWTSecretInvalid,
		},
		{
			name: "empty JWT secret",
			values: map[string]string{
				"MINI_CHAT_JWT_SECRET": "                                ",
			},
			wantErr: ErrJWTSecretInvalid,
		},
		{
			name: "short JWT secret",
			values: map[string]string{
				"MINI_CHAT_JWT_SECRET": "too-short",
			},
			wantErr: ErrJWTSecretInvalid,
		},
		{
			name: "empty access token TTL",
			values: map[string]string{
				"MINI_CHAT_ACCESS_TOKEN_TTL": " ",
			},
			wantErr: ErrAccessTokenTTLInvalid,
		},
		{
			name: "malformed access token TTL",
			values: map[string]string{
				"MINI_CHAT_ACCESS_TOKEN_TTL": "one-hour",
			},
			wantErr: ErrAccessTokenTTLInvalid,
		},
		{
			name: "zero access token TTL",
			values: map[string]string{
				"MINI_CHAT_ACCESS_TOKEN_TTL": "0s",
			},
			wantErr: ErrAccessTokenTTLInvalid,
		},
		{
			name: "negative access token TTL",
			values: map[string]string{
				"MINI_CHAT_ACCESS_TOKEN_TTL": "-1s",
			},
			wantErr: ErrAccessTokenTTLInvalid,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				values := map[string]string{
					"MINI_CHAT_POSTGRES_URL": "postgres://mini-chat-test",
					"MINI_CHAT_JWT_SECRET":   "0123456789abcdef0123456789abcdef",
				}
				for key, value := range test.values {
					values[key] = value
				}
				for _, key := range test.unset {
					delete(values, key)
				}

				_, err := load(
					func(
						key string,
					) (string, bool) {
						value, exists := values[key]

						return value, exists
					},
				)

				if !errors.Is(
					err,
					test.wantErr,
				) {
					t.Fatalf(
						"load() error = %v, want %v",
						err,
						test.wantErr,
					)
				}
			},
		)
	}
}
