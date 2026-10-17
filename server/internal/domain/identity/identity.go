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
	ErrInvalidCredentials = errors.New(
		"invalid credentials",
	)
	ErrNotFound = errors.New(
		"identity not found",
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

type LoginCredentials struct {
	Username string
	Password string
}

func NewRegistration(
	username string,
	password string,
	displayName string,
) (Registration, error) {
	username, err := NormalizeUsername(username)
	if err != nil {
		return Registration{}, err
	}

	displayName = strings.TrimSpace(
		displayName,
	)

	if !validPassword(password) {
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

func NewLoginCredentials(
	username string,
	password string,
) (LoginCredentials, error) {
	username, err := NormalizeUsername(username)
	if err != nil {
		return LoginCredentials{},
			ErrInvalidCredentials
	}

	if !validPassword(password) {
		return LoginCredentials{},
			ErrInvalidCredentials
	}

	return LoginCredentials{
		Username: username,
		Password: password,
	}, nil
}

func NormalizeUsername(
	username string,
) (string, error) {
	username = strings.ToLower(
		strings.TrimSpace(username),
	)

	if !validUsername(username) {
		return "", ErrUsernameInvalid
	}

	return username, nil
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

func validPassword(
	password string,
) bool {
	length := utf8.RuneCountInString(password)

	return length >= MinPasswordLength &&
		length <= MaxPasswordLength &&
		strings.TrimSpace(password) != ""
}
