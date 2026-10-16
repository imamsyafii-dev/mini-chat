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
			string,
		) (string, bool) {
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
}

func TestLoadUsesEnvironmentOverrides(
	t *testing.T,
) {
	values := map[string]string{
		"MINI_CHAT_HTTP_ADDR":        " 127.0.0.1:29090 ",
		"MINI_CHAT_SHUTDOWN_TIMEOUT": " 15s ",
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
}

func TestLoadRejectsInvalidEnvironment(
	t *testing.T,
) {
	tests := []struct {
		name    string
		values  map[string]string
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
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				_, err := load(
					func(
						key string,
					) (string, bool) {
						value, exists :=
							test.values[key]

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
