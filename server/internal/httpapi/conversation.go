package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	createdirectapp "github.com/imamsyafii-dev/mini-chat/server/internal/app/createdirect"
	conversationdomain "github.com/imamsyafii-dev/mini-chat/server/internal/domain/conversation"
	identitydomain "github.com/imamsyafii-dev/mini-chat/server/internal/domain/identity"
)

const maxCreateDirectConversationRequestBytes = 64 * 1024

type CreateDirectConversationUseCase interface {
	Create(
		ctx context.Context,
		input createdirectapp.Input,
	) (createdirectapp.Result, error)
}

type ListConversationsUseCase interface {
	List(
		ctx context.Context,
		actorID int64,
	) ([]conversationdomain.DirectConversation, error)
}

type createDirectConversationRequest struct {
	RecipientUsername string `json:"recipient_username"`
}

type conversationParticipantResponse struct {
	ID          int64     `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type directConversationResponse struct {
	ID          int64                           `json:"id"`
	Kind        conversationdomain.Kind         `json:"kind"`
	CreatedBy   int64                           `json:"created_by"`
	Participant conversationParticipantResponse `json:"participant"`
	CreatedAt   time.Time                       `json:"created_at"`
	UpdatedAt   time.Time                       `json:"updated_at"`
}

type createDirectConversationHandler struct {
	useCase CreateDirectConversationUseCase
}

func newCreateDirectConversationHandler(
	useCase CreateDirectConversationUseCase,
) http.Handler {
	return &createDirectConversationHandler{
		useCase: useCase,
	}
}

func (
	handler *createDirectConversationHandler,
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
		maxCreateDirectConversationRequestBytes,
	)

	var payload createDirectConversationRequest
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

	result, err := handler.useCase.Create(
		request.Context(),
		createdirectapp.Input{
			ActorID: principal.IdentityID,
			RecipientUsername: payload.
				RecipientUsername,
		},
	)
	if err != nil {
		writeCreateDirectConversationError(
			writer,
			err,
		)

		return
	}

	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}

	writeJSON(
		writer,
		status,
		newDirectConversationResponse(
			result.Conversation,
		),
	)
}

type listConversationsHandler struct {
	useCase ListConversationsUseCase
}

func newListConversationsHandler(
	useCase ListConversationsUseCase,
) http.Handler {
	return &listConversationsHandler{
		useCase: useCase,
	}
}

func (
	handler *listConversationsHandler,
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

	results, err := handler.useCase.List(
		request.Context(),
		principal.IdentityID,
	)
	if err != nil {
		writeListConversationsError(writer, err)

		return
	}

	response := make(
		[]directConversationResponse,
		0,
		len(results),
	)
	for _, result := range results {
		response = append(
			response,
			newDirectConversationResponse(result),
		)
	}

	writeJSON(
		writer,
		http.StatusOK,
		response,
	)
}

func newDirectConversationResponse(
	value conversationdomain.DirectConversation,
) directConversationResponse {
	return directConversationResponse{
		ID:        value.ID,
		Kind:      value.Kind,
		CreatedBy: value.CreatedBy,
		Participant: conversationParticipantResponse{
			ID: value.Participant.ID,
			Username: value.Participant.
				Username,
			DisplayName: value.Participant.
				DisplayName,
			CreatedAt: value.Participant.
				CreatedAt,
			UpdatedAt: value.Participant.
				UpdatedAt,
		},
		CreatedAt: value.CreatedAt,
		UpdatedAt: value.UpdatedAt,
	}
}

func writeCreateDirectConversationError(
	writer http.ResponseWriter,
	err error,
) {
	switch {
	case errors.Is(
		err,
		identitydomain.ErrUsernameInvalid,
	):
		writeJSON(
			writer,
			http.StatusBadRequest,
			errorResponse{
				Error: identitydomain.
					ErrUsernameInvalid.Error(),
			},
		)

	case errors.Is(
		err,
		conversationdomain.ErrParticipantIDInvalid,
	):
		writeJSON(
			writer,
			http.StatusBadRequest,
			errorResponse{
				Error: conversationdomain.
					ErrParticipantIDInvalid.Error(),
			},
		)

	case errors.Is(
		err,
		identitydomain.ErrNotFound,
	):
		writeJSON(
			writer,
			http.StatusNotFound,
			errorResponse{
				Error: identitydomain.ErrNotFound.Error(),
			},
		)

	case errors.Is(
		err,
		conversationdomain.ErrSelfConversation,
	):
		writeJSON(
			writer,
			http.StatusConflict,
			errorResponse{
				Error: conversationdomain.
					ErrSelfConversation.Error(),
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

func writeListConversationsError(
	writer http.ResponseWriter,
	err error,
) {
	if errors.Is(
		err,
		conversationdomain.ErrParticipantIDInvalid,
	) {
		writeJSON(
			writer,
			http.StatusBadRequest,
			errorResponse{
				Error: conversationdomain.
					ErrParticipantIDInvalid.Error(),
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
