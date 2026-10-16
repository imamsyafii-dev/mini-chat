package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/imamsyafii-dev/mini-chat/server/internal/domain/identity"
)

type CurrentUserUseCase interface {
	Get(
		ctx context.Context,
		identityID int64,
	) (identity.Identity, error)
}

type currentUserResponse struct {
	ID          int64     `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type currentUserHandler struct {
	useCase CurrentUserUseCase
}

func newCurrentUserHandler(
	useCase CurrentUserUseCase,
) http.Handler {
	return &currentUserHandler{
		useCase: useCase,
	}
}

func (
	handler *currentUserHandler,
) ServeHTTP(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, ok := PrincipalFromContext(
		request.Context(),
	)
	if !ok {
		writeUnauthorized(writer)

		return
	}

	result, err := handler.useCase.Get(
		request.Context(),
		principal.IdentityID,
	)
	if err != nil {
		if errors.Is(err, identity.ErrNotFound) {
			writeUnauthorized(writer)

			return
		}

		writeJSON(
			writer,
			http.StatusInternalServerError,
			errorResponse{
				Error: "internal server error",
			},
		)

		return
	}

	writeJSON(
		writer,
		http.StatusOK,
		currentUserResponse{
			ID:          result.ID,
			Username:    result.Username,
			DisplayName: result.DisplayName,
			CreatedAt:   result.CreatedAt,
			UpdatedAt:   result.UpdatedAt,
		},
	)
}
