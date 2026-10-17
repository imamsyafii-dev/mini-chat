package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"time"

	listmessagesapp "github.com/imamsyafii-dev/mini-chat/server/internal/app/listmessages"
	sendmessageapp "github.com/imamsyafii-dev/mini-chat/server/internal/app/sendmessage"
	messagedomain "github.com/imamsyafii-dev/mini-chat/server/internal/domain/message"
)

const maxSendMessageRequestBytes = 64 * 1024

type SendMessageUseCase interface {
	Send(
		ctx context.Context,
		input sendmessageapp.Input,
	) (messagedomain.Message, error)
}

type ListMessagesUseCase interface {
	List(
		ctx context.Context,
		input listmessagesapp.Input,
	) (listmessagesapp.Page, error)
}

type sendMessageRequest struct {
	Body string `json:"body"`
}

type messageResponse struct {
	ID             int64     `json:"id"`
	ConversationID int64     `json:"conversation_id"`
	SenderID       int64     `json:"sender_id"`
	Body           string    `json:"body"`
	CreatedAt      time.Time `json:"created_at"`
}

type listMessagesResponse struct {
	Messages     []messageResponse `json:"messages"`
	NextBeforeID *int64            `json:"next_before_id"`
}

type sendMessageHandler struct {
	useCase SendMessageUseCase
}

func newSendMessageHandler(
	useCase SendMessageUseCase,
) http.Handler {
	return &sendMessageHandler{
		useCase: useCase,
	}
}

func (
	handler *sendMessageHandler,
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

	conversationID, err := parseConversationID(
		request.PathValue("conversationID"),
	)
	if err != nil {
		writeMessageError(writer, err)

		return
	}

	mediaType, _, err := mime.ParseMediaType(
		request.Header.Get("Content-Type"),
	)
	if err != nil || mediaType != "application/json" {
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
		maxSendMessageRequestBytes,
	)

	var payload sendMessageRequest
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&payload); err != nil {
		writeDecodeError(writer, err)

		return
	}

	if err := decoder.Decode(
		&struct{}{},
	); !errors.Is(err, io.EOF) {
		writeDecodeError(writer, err)

		return
	}

	result, err := handler.useCase.Send(
		request.Context(),
		sendmessageapp.Input{
			ActorID:        principal.IdentityID,
			ConversationID: conversationID,
			Body:           payload.Body,
		},
	)
	if err != nil {
		writeMessageError(writer, err)

		return
	}

	writeJSON(
		writer,
		http.StatusCreated,
		newMessageResponse(result),
	)
}

type listMessagesHandler struct {
	useCase ListMessagesUseCase
}

func newListMessagesHandler(
	useCase ListMessagesUseCase,
) http.Handler {
	return &listMessagesHandler{
		useCase: useCase,
	}
}

func (
	handler *listMessagesHandler,
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

	conversationID, err := parseConversationID(
		request.PathValue("conversationID"),
	)
	if err != nil {
		writeMessageError(writer, err)

		return
	}

	beforeID, err := parseBeforeID(request)
	if err != nil {
		writeMessageError(writer, err)

		return
	}

	page, err := handler.useCase.List(
		request.Context(),
		listmessagesapp.Input{
			ActorID:        principal.IdentityID,
			ConversationID: conversationID,
			BeforeID:       beforeID,
		},
	)
	if err != nil {
		writeMessageError(writer, err)

		return
	}

	messages := make(
		[]messageResponse,
		0,
		len(page.Messages),
	)
	for _, result := range page.Messages {
		messages = append(
			messages,
			newMessageResponse(result),
		)
	}

	writeJSON(
		writer,
		http.StatusOK,
		listMessagesResponse{
			Messages:     messages,
			NextBeforeID: page.NextBeforeID,
		},
	)
}

func parseConversationID(
	raw string,
) (int64, error) {
	conversationID, err := strconv.ParseInt(
		raw,
		10,
		64,
	)
	if err != nil || conversationID <= 0 {
		return 0, messagedomain.ErrConversationIDInvalid
	}

	return conversationID, nil
}

func parseBeforeID(
	request *http.Request,
) (*int64, error) {
	values, present := request.URL.Query()["before_id"]
	if !present {
		return nil, nil
	}
	if len(values) != 1 || values[0] == "" {
		return nil, listmessagesapp.ErrBeforeIDInvalid
	}

	beforeID, err := strconv.ParseInt(
		values[0],
		10,
		64,
	)
	if err != nil || beforeID <= 0 {
		return nil, listmessagesapp.ErrBeforeIDInvalid
	}

	return &beforeID, nil
}

func newMessageResponse(
	value messagedomain.Message,
) messageResponse {
	return messageResponse{
		ID:             value.ID,
		ConversationID: value.ConversationID,
		SenderID:       value.SenderID,
		Body:           value.Body,
		CreatedAt:      value.CreatedAt,
	}
}

func writeMessageError(
	writer http.ResponseWriter,
	err error,
) {
	switch {
	case errors.Is(
		err,
		messagedomain.ErrConversationIDInvalid,
	):
		writeJSON(
			writer,
			http.StatusBadRequest,
			errorResponse{
				Error: messagedomain.
					ErrConversationIDInvalid.Error(),
			},
		)

	case errors.Is(
		err,
		messagedomain.ErrIdentityIDInvalid,
	):
		writeJSON(
			writer,
			http.StatusBadRequest,
			errorResponse{
				Error: messagedomain.
					ErrIdentityIDInvalid.Error(),
			},
		)

	case errors.Is(
		err,
		messagedomain.ErrBodyInvalid,
	):
		writeJSON(
			writer,
			http.StatusBadRequest,
			errorResponse{
				Error: messagedomain.
					ErrBodyInvalid.Error(),
			},
		)

	case errors.Is(
		err,
		listmessagesapp.ErrBeforeIDInvalid,
	):
		writeJSON(
			writer,
			http.StatusBadRequest,
			errorResponse{
				Error: listmessagesapp.
					ErrBeforeIDInvalid.Error(),
			},
		)

	case errors.Is(
		err,
		messagedomain.ErrConversationUnavailable,
	):
		writeJSON(
			writer,
			http.StatusNotFound,
			errorResponse{
				Error: messagedomain.
					ErrConversationUnavailable.Error(),
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
