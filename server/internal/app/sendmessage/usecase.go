package sendmessage

import (
	"context"
	"fmt"

	"github.com/imamsyafii-dev/mini-chat/server/internal/domain/message"
)

type Input struct {
	ActorID        int64
	ConversationID int64
	Body           string
}

type MessageStore interface {
	CreateMessage(
		ctx context.Context,
		draft message.Draft,
	) (message.Message, error)
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
) Send(
	ctx context.Context,
	input Input,
) (message.Message, error) {
	draft, err := message.NewDraft(
		input.ConversationID,
		input.ActorID,
		input.Body,
	)
	if err != nil {
		return message.Message{}, err
	}

	result, err := useCase.messageStore.
		CreateMessage(
			ctx,
			draft,
		)
	if err != nil {
		return message.Message{}, fmt.Errorf(
			"create message: %w",
			err,
		)
	}

	return result, nil
}
