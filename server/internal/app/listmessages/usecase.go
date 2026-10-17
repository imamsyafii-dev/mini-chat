package listmessages

import (
	"context"
	"errors"
	"fmt"

	"github.com/imamsyafii-dev/mini-chat/server/internal/domain/message"
)

const (
	pageSize   = 50
	fetchLimit = pageSize + 1
)

var ErrBeforeIDInvalid = errors.New(
	"message before ID is invalid",
)

type Input struct {
	ActorID        int64
	ConversationID int64
	BeforeID       *int64
}

type Page struct {
	Messages     []message.Message
	NextBeforeID *int64
}

type MessageStore interface {
	ListMessages(
		ctx context.Context,
		actorID int64,
		conversationID int64,
		beforeID *int64,
		limit int,
	) ([]message.Message, error)
}

type UseCase struct {
	messageStore MessageStore
}

func New(
	messageStore MessageStore,
) *UseCase {
	return &UseCase{
		messageStore: messageStore,
	}
}

func (
	useCase *UseCase,
) List(
	ctx context.Context,
	input Input,
) (Page, error) {
	if input.ActorID <= 0 {
		return Page{}, message.ErrIdentityIDInvalid
	}
	if input.ConversationID <= 0 {
		return Page{}, message.ErrConversationIDInvalid
	}
	if input.BeforeID != nil &&
		*input.BeforeID <= 0 {
		return Page{}, ErrBeforeIDInvalid
	}

	results, err := useCase.messageStore.
		ListMessages(
			ctx,
			input.ActorID,
			input.ConversationID,
			input.BeforeID,
			fetchLimit,
		)
	if err != nil {
		return Page{}, fmt.Errorf(
			"list messages: %w",
			err,
		)
	}

	resultCount := len(results)
	if resultCount > pageSize {
		resultCount = pageSize
	}

	messages := make(
		[]message.Message,
		resultCount,
	)
	for index := 0; index < resultCount; index++ {
		messages[index] = results[resultCount-1-index]
	}

	var nextBeforeID *int64
	if len(results) > pageSize {
		oldestRetainedID := results[pageSize-1].ID
		nextBeforeID = &oldestRetainedID
	}

	return Page{
		Messages:     messages,
		NextBeforeID: nextBeforeID,
	}, nil
}
