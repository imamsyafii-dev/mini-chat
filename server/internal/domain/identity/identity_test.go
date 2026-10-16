package identity

import (
	"errors"
	"strings"
	"testing"
)

func TestNewRegistrationNormalizesInput(
	t *testing.T,
) {
	result, err := NewRegistration(
		"  Alice_01  ",
		"practice-password",
		"  Alice Example  ",
	)
	if err != nil {
		t.Fatalf(
			"NewRegistration() error = %v, want nil",
			err,
		)
	}

	if result.Username != "alice_01" {
		t.Errorf(
			"Username = %q, want %q",
			result.Username,
			"alice_01",
		)
	}

	if result.Password !=
		"practice-password" {
		t.Errorf(
			"Password was unexpectedly changed",
		)
	}

	if result.DisplayName !=
		"Alice Example" {
		t.Errorf(
			"DisplayName = %q, want %q",
			result.DisplayName,
			"Alice Example",
		)
	}
}

func TestNewRegistrationRejectsInvalidInput(
	t *testing.T,
) {
	tests := []struct {
		name        string
		username    string
		password    string
		displayName string
		wantErr     error
	}{
		{
			name:        "username too short",
			username:    "ab",
			password:    "practice-password",
			displayName: "Alice",
			wantErr:     ErrUsernameInvalid,
		},
		{
			name:        "username contains unsupported character",
			username:    "alice.smith",
			password:    "practice-password",
			displayName: "Alice",
			wantErr:     ErrUsernameInvalid,
		},
		{
			name:        "password too short",
			username:    "alice",
			password:    "short",
			displayName: "Alice",
			wantErr:     ErrPasswordInvalid,
		},
		{
			name:        "password only contains spaces",
			username:    "alice",
			password:    strings.Repeat(" ", 8),
			displayName: "Alice",
			wantErr:     ErrPasswordInvalid,
		},
		{
			name:     "password too long",
			username: "alice",
			password: strings.Repeat(
				"a",
				MaxPasswordLength+1,
			),
			displayName: "Alice",
			wantErr:     ErrPasswordInvalid,
		},
		{
			name:        "display name is empty",
			username:    "alice",
			password:    "practice-password",
			displayName: " ",
			wantErr:     ErrDisplayNameInvalid,
		},
		{
			name:     "display name is too long",
			username: "alice",
			password: "practice-password",
			displayName: strings.Repeat(
				"a",
				MaxDisplayNameLength+1,
			),
			wantErr: ErrDisplayNameInvalid,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				_, err := NewRegistration(
					test.username,
					test.password,
					test.displayName,
				)

				if !errors.Is(
					err,
					test.wantErr,
				) {
					t.Fatalf(
						"error = %v, want %v",
						err,
						test.wantErr,
					)
				}
			},
		)
	}
}

func TestNewLoginCredentialsNormalizesUsernameAndPreservesPassword(
	t *testing.T,
) {
	const password = "  practice-password  "

	result, err := NewLoginCredentials(
		"  Alice_01  ",
		password,
	)
	if err != nil {
		t.Fatalf(
			"NewLoginCredentials() error = %v, want nil",
			err,
		)
	}

	if result.Username != "alice_01" {
		t.Errorf(
			"Username = %q, want %q",
			result.Username,
			"alice_01",
		)
	}

	if result.Password != password {
		t.Errorf(
			"Password = %q, want password preserved byte-for-byte",
			result.Password,
		)
	}
}

func TestNewLoginCredentialsRejectsInvalidCredentials(
	t *testing.T,
) {
	tests := []struct {
		name     string
		username string
		password string
	}{
		{
			name:     "username too short",
			username: "ab",
			password: "practice-password",
		},
		{
			name:     "username too long",
			username: strings.Repeat("a", MaxUsernameLength+1),
			password: "practice-password",
		},
		{
			name:     "username contains unsupported character",
			username: "alice.smith",
			password: "practice-password",
		},
		{
			name:     "password too short",
			username: "alice",
			password: "short",
		},
		{
			name:     "password too long",
			username: "alice",
			password: strings.Repeat("a", MaxPasswordLength+1),
		},
		{
			name:     "password only contains spaces",
			username: "alice",
			password: strings.Repeat(" ", MinPasswordLength),
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				_, err := NewLoginCredentials(
					test.username,
					test.password,
				)

				if !errors.Is(
					err,
					ErrInvalidCredentials,
				) {
					t.Fatalf(
						"error = %v, want %v",
						err,
						ErrInvalidCredentials,
					)
				}
			},
		)
	}
}
