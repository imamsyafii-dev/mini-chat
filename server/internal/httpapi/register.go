package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	registerapp "github.com/imamsyafii-dev/mini-chat/server/internal/app/register"
	identitydomain "github.com/imamsyafii-dev/mini-chat/server/internal/domain/identity"
)

const maxRegisterRequestBytes = 64 * 1024

var errInvalidJSON = errors.New(
	"invalid JSON request",
)

type Registerer interface {
	Register(
		ctx context.Context,
		input registerapp.Input,
	) (identitydomain.Identity, error)
}

type registerRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type registerResponse struct {
	ID          int64     `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type registerHandler struct {
	useCase Registerer
}

func newRegisterHandler(
	useCase Registerer,
) http.Handler {
	return &registerHandler{
		useCase: useCase,
	}
}

func (
	handler *registerHandler,
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
		maxRegisterRequestBytes,
	)

	var payload registerRequest
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

	result, err := handler.useCase.Register(
		request.Context(),
		registerapp.Input{
			Username:    payload.Username,
			Password:    payload.Password,
			DisplayName: payload.DisplayName,
		},
	)
	if err != nil {
		writeRegisterError(writer, err)

		return
	}

	writeJSON(
		writer,
		http.StatusCreated,
		registerResponse{
			ID:          result.ID,
			Username:    result.Username,
			DisplayName: result.DisplayName,
			CreatedAt:   result.CreatedAt,
			UpdatedAt:   result.UpdatedAt,
		},
	)
}

func writeDecodeError(
	writer http.ResponseWriter,
	err error,
) {
	var maxBytesError *http.MaxBytesError
	if errors.As(
		err,
		&maxBytesError,
	) {
		writeJSON(
			writer,
			http.StatusRequestEntityTooLarge,
			errorResponse{
				Error: "request body is too large",
			},
		)

		return
	}

	writeJSON(
		writer,
		http.StatusBadRequest,
		errorResponse{
			Error: errInvalidJSON.Error(),
		},
	)
}

func writeRegisterError(
	writer http.ResponseWriter,
	err error,
) {
	switch {
	case errors.Is(
		err,
		identitydomain.ErrUsernameInvalid,
	),
		errors.Is(
			err,
			identitydomain.ErrPasswordInvalid,
		),
		errors.Is(
			err,
			identitydomain.ErrDisplayNameInvalid,
		):
		writeJSON(
			writer,
			http.StatusBadRequest,
			errorResponse{
				Error: err.Error(),
			},
		)

	case errors.Is(
		err,
		identitydomain.ErrUsernameTaken,
	):
		writeJSON(
			writer,
			http.StatusConflict,
			errorResponse{
				Error: identitydomain.ErrUsernameTaken.Error(),
			},
		)

	default:
		writeJSON(
			writer,
			http.StatusInternalServerError,
			errorResponse{
				Error: "internal server error",
			},
		)
	}
}

func writeJSON(
	writer http.ResponseWriter,
	status int,
	value any,
) {
	writer.Header().Set(
		"Content-Type",
		"application/json",
	)
	writer.WriteHeader(status)

	_ = json.NewEncoder(writer).Encode(value)
}
