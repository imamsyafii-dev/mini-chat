package createdirect

import (
	"context"
	"fmt"

	"github.com/imamsyafii-dev/mini-chat/server/internal/domain/conversation"
	"github.com/imamsyafii-dev/mini-chat/server/internal/domain/identity"
)

type Input struct {
	ActorID           int64
	RecipientUsername string
}

type Result struct {
	Conversation conversation.DirectConversation
	Created      bool
}

type IdentityStore interface {
	FindPublicIdentityByUsername(
		ctx context.Context,
		username string,
	) (identity.Identity, error)
}

type ConversationStore interface {
	CreateOrGetDirectConversation(
		ctx context.Context,
		actorID int64,
		pair conversation.DirectPair,
	) (conversation.DirectConversation, bool, error)
}

type UseCase struct {
	identityStore     IdentityStore
	conversationStore ConversationStore
}

func New(
	identityStore IdentityStore,
	conversationStore ConversationStore,
) *UseCase {
	return &UseCase{
		identityStore:     identityStore,
		conversationStore: conversationStore,
	}
}

func (
	useCase *UseCase,
) Create(
	ctx context.Context,
	input Input,
) (Result, error) {
	recipientUsername, err :=
		identity.NormalizeUsername(
			input.RecipientUsername,
		)
	if err != nil {
		return Result{}, err
	}

	recipient, err :=
		useCase.identityStore.
			FindPublicIdentityByUsername(
				ctx,
				recipientUsername,
			)
	if err != nil {
		return Result{}, fmt.Errorf(
			"find recipient identity: %w",
			err,
		)
	}

	pair, err := conversation.NewDirectPair(
		input.ActorID,
		recipient.ID,
	)
	if err != nil {
		return Result{}, err
	}

	result, created, err :=
		useCase.conversationStore.
			CreateOrGetDirectConversation(
				ctx,
				input.ActorID,
				pair,
			)
	if err != nil {
		return Result{}, fmt.Errorf(
			"create or get direct conversation: %w",
			err,
		)
	}

	return Result{
		Conversation: result,
		Created:      created,
	}, nil
}
