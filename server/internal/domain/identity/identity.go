package identity

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MinUsernameLength    = 3
	MaxUsernameLength    = 40
	MinPasswordLength    = 8
	MaxPasswordLength    = 128
	MaxDisplayNameLength = 100
)

var (
	ErrUsernameInvalid = errors.New(
		"username is invalid",
	)
	ErrPasswordInvalid = errors.New(
		"password is invalid",
	)
	ErrDisplayNameInvalid = errors.New(
		"display name is invalid",
	)
	ErrUsernameTaken = errors.New(
		"username is already registered",
	)
)

type Identity struct {
	ID          int64
	Username    string
	DisplayName string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Registration struct {
	Username    string
	Password    string
	DisplayName string
}

func NewRegistration(
	username string,
	password string,
	displayName string,
) (Registration, error) {
	username = strings.ToLower(
		strings.TrimSpace(username),
	)
	displayName = strings.TrimSpace(
		displayName,
	)

	if !validUsername(username) {
		return Registration{},
			ErrUsernameInvalid
	}

	passwordLength :=
		utf8.RuneCountInString(password)
	if passwordLength < MinPasswordLength ||
		passwordLength > MaxPasswordLength ||
		strings.TrimSpace(password) == "" {
		return Registration{},
			ErrPasswordInvalid
	}

	displayNameLength :=
		utf8.RuneCountInString(displayName)
	if displayNameLength == 0 ||
		displayNameLength >
			MaxDisplayNameLength {
		return Registration{},
			ErrDisplayNameInvalid
	}

	return Registration{
		Username:    username,
		Password:    password,
		DisplayName: displayName,
	}, nil
}

func validUsername(
	username string,
) bool {
	length := len(username)
	if length < MinUsernameLength ||
		length > MaxUsernameLength {
		return false
	}

	for _, character := range username {
		isLowercaseLetter :=
			character >= 'a' &&
				character <= 'z'
		isNumber :=
			character >= '0' &&
				character <= '9'

		if !isLowercaseLetter &&
			!isNumber &&
			character != '_' {
			return false
		}
	}

	return true
}
