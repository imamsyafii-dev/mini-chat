package listconversations

import (
	"context"
	"fmt"

	"github.com/imamsyafii-dev/mini-chat/server/internal/domain/conversation"
)

const defaultLimit = 50

type ConversationStore interface {
	ListDirectConversations(
		ctx context.Context,
		actorID int64,
		limit int,
	) ([]conversation.DirectConversation, error)
}

type UseCase struct {
	conversationStore ConversationStore
}

func New(
	conversationStore ConversationStore,
) *UseCase {
	return &UseCase{
		conversationStore: conversationStore,
	}
}

func (
	useCase *UseCase,
) List(
	ctx context.Context,
	actorID int64,
) ([]conversation.DirectConversation, error) {
	if actorID <= 0 {
		return nil, conversation.ErrParticipantIDInvalid
	}

	results, err := useCase.conversationStore.
		ListDirectConversations(
			ctx,
			actorID,
			defaultLimit,
		)
	if err != nil {
		return nil, fmt.Errorf(
			"list direct conversations: %w",
			err,
		)
	}

	return results, nil
}
