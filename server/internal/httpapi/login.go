package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	loginapp "github.com/imamsyafii-dev/mini-chat/server/internal/app/login"
	identitydomain "github.com/imamsyafii-dev/mini-chat/server/internal/domain/identity"
)

const maxLoginRequestBytes = 64 * 1024

type LoginUseCase interface {
	Login(
		ctx context.Context,
		input loginapp.Input,
	) (loginapp.Result, error)
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginIdentityResponse struct {
	ID          int64     `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type loginResponse struct {
	AccessToken string                `json:"access_token"`
	TokenType   string                `json:"token_type"`
	ExpiresAt   time.Time             `json:"expires_at"`
	Identity    loginIdentityResponse `json:"identity"`
}

type loginHandler struct {
	useCase LoginUseCase
}

func newLoginHandler(
	useCase LoginUseCase,
) http.Handler {
	return &loginHandler{
		useCase: useCase,
	}
}

func (
	handler *loginHandler,
) ServeHTTP(
	writer http.ResponseWriter,
	request *http.Request,
) {
	mediaType, _, err := mime.ParseMediaType(
		request.Header.Get("Content-Type"),
	)
	if err != nil ||
		mediaType != "application/json" {
		writeJSON(
			writer,
			http.StatusUnsupportedMediaType,
			errorResponse{
				Error: "Content-Type must be application/json",
			},
		)

		return
	}

	request.Body = http.MaxBytesReader(
		writer,
		request.Body,
		maxLoginRequestBytes,
	)

	var payload loginRequest
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&payload); err != nil {
		writeDecodeError(writer, err)

		return
	}

	if err := decoder.Decode(
		&struct{}{},
	); !errors.Is(err, io.EOF) {
		writeJSON(
			writer,
			http.StatusBadRequest,
			errorResponse{
				Error: errInvalidJSON.Error(),
			},
		)

		return
	}

	result, err := handler.useCase.Login(
		request.Context(),
		loginapp.Input{
			Username: payload.Username,
			Password: payload.Password,
		},
	)
	if err != nil {
		writeLoginError(writer, err)

		return
	}

	writeJSON(
		writer,
		http.StatusOK,
		loginResponse{
			AccessToken: result.AccessToken,
			TokenType:   "Bearer",
			ExpiresAt:   result.ExpiresAt,
			Identity: loginIdentityResponse{
				ID: result.Identity.ID,
				Username: result.Identity.
					Username,
				DisplayName: result.Identity.
					DisplayName,
				CreatedAt: result.Identity.
					CreatedAt,
				UpdatedAt: result.Identity.
					UpdatedAt,
			},
		},
	)
}

func writeLoginError(
	writer http.ResponseWriter,
	err error,
) {
	if errors.Is(
		err,
		identitydomain.ErrInvalidCredentials,
	) {
		writeJSON(
			writer,
			http.StatusUnauthorized,
			errorResponse{
				Error: identitydomain.
					ErrInvalidCredentials.Error(),
			},
		)

		return
	}

	writeJSON(
		writer,
		http.StatusInternalServerError,
		errorResponse{
			Error: "internal server error",
		},
	)
}
