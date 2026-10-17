package createdirect

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/imamsyafii-dev/mini-chat/server/internal/domain/conversation"
	"github.com/imamsyafii-dev/mini-chat/server/internal/domain/identity"
)

type fakeIdentityStore struct {
	called   bool
	username string
	result   identity.Identity
	err      error
}

func (
	fake *fakeIdentityStore,
) FindPublicIdentityByUsername(
	_ context.Context,
	username string,
) (identity.Identity, error) {
	fake.called = true
	fake.username = username

	return fake.result, fake.err
}

type fakeConversationStore struct {
	called  bool
	actorID int64
	pair    conversation.DirectPair
	result  conversation.DirectConversation
	created bool
	err     error
}

func (
	fake *fakeConversationStore,
) CreateOrGetDirectConversation(
	_ context.Context,
	actorID int64,
	pair conversation.DirectPair,
) (conversation.DirectConversation, bool, error) {
	fake.called = true
	fake.actorID = actorID
	fake.pair = pair

	return fake.result, fake.created, fake.err
}

var _ IdentityStore = (*fakeIdentityStore)(nil)

var _ ConversationStore = (*fakeConversationStore)(nil)

func TestUseCaseCreateNormalizesRecipientAndReturnsCreatedConversation(
	t *testing.T,
) {
	now := time.Date(
		2026,
		time.October,
		17,
		10,
		0,
		0,
		0,
		time.UTC,
	)
	recipient := identity.Identity{
		ID:          20,
		Username:    "bob_01",
		DisplayName: "Bob",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	expected := conversation.DirectConversation{
		ID:          100,
		Kind:        conversation.KindDirect,
		CreatedBy:   10,
		Participant: recipient,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	identityStore := &fakeIdentityStore{
		result: recipient,
	}
	conversationStore := &fakeConversationStore{
		result:  expected,
		created: true,
	}
	useCase := New(
		identityStore,
		conversationStore,
	)

	result, err := useCase.Create(
		context.Background(),
		Input{
			ActorID:           10,
			RecipientUsername: "  BOB_01  ",
		},
	)
	if err != nil {
		t.Fatalf(
			"Create() error = %v, want nil",
			err,
		)
	}

	if identityStore.username != "bob_01" {
		t.Errorf(
			"identity username = %q, want %q",
			identityStore.username,
			"bob_01",
		)
	}
	if !conversationStore.called {
		t.Fatal(
			"ConversationStore.CreateOrGetDirectConversation() was not called",
		)
	}
	if conversationStore.actorID != 10 {
		t.Errorf(
			"actor ID = %d, want %d",
			conversationStore.actorID,
			10,
		)
	}
	expectedPair := conversation.DirectPair{
		IdentityLowID:  10,
		IdentityHighID: 20,
	}
	if conversationStore.pair != expectedPair {
		t.Errorf(
			"direct pair = %#v, want %#v",
			conversationStore.pair,
			expectedPair,
		)
	}
	if result.Conversation != expected {
		t.Errorf(
			"conversation = %#v, want %#v",
			result.Conversation,
			expected,
		)
	}
	if !result.Created {
		t.Error(
			"Created = false, want true",
		)
	}
}

func TestUseCaseCreateBuildsCanonicalPairWhenActorIDIsHigher(
	t *testing.T,
) {
	identityStore := &fakeIdentityStore{
		result: identity.Identity{
			ID:       10,
			Username: "alice",
		},
	}
	conversationStore := &fakeConversationStore{}
	useCase := New(
		identityStore,
		conversationStore,
	)

	_, err := useCase.Create(
		context.Background(),
		Input{
			ActorID:           20,
			RecipientUsername: "alice",
		},
	)
	if err != nil {
		t.Fatalf(
			"Create() error = %v, want nil",
			err,
		)
	}

	expectedPair := conversation.DirectPair{
		IdentityLowID:  10,
		IdentityHighID: 20,
	}
	if conversationStore.pair != expectedPair {
		t.Errorf(
			"direct pair = %#v, want %#v",
			conversationStore.pair,
			expectedPair,
		)
	}
}

func TestUseCaseCreatePreservesExistingResult(
	t *testing.T,
) {
	expected := conversation.DirectConversation{
		ID:        100,
		Kind:      conversation.KindDirect,
		CreatedBy: 20,
	}
	identityStore := &fakeIdentityStore{
		result: identity.Identity{
			ID:       20,
			Username: "bob",
		},
	}
	conversationStore := &fakeConversationStore{
		result:  expected,
		created: false,
	}
	useCase := New(
		identityStore,
		conversationStore,
	)

	result, err := useCase.Create(
		context.Background(),
		Input{
			ActorID:           10,
			RecipientUsername: "bob",
		},
	)
	if err != nil {
		t.Fatalf(
			"Create() error = %v, want nil",
			err,
		)
	}

	if result.Conversation != expected {
		t.Errorf(
			"conversation = %#v, want %#v",
			result.Conversation,
			expected,
		)
	}
	if result.Created {
		t.Error(
			"Created = true, want false",
		)
	}
}

func TestUseCaseCreateRejectsInvalidRecipientUsernameBeforeDependencies(
	t *testing.T,
) {
	identityStore := &fakeIdentityStore{}
	conversationStore := &fakeConversationStore{}
	useCase := New(
		identityStore,
		conversationStore,
	)

	_, err := useCase.Create(
		context.Background(),
		Input{
			ActorID:           10,
			RecipientUsername: "invalid.username",
		},
	)
	if !errors.Is(
		err,
		identity.ErrUsernameInvalid,
	) {
		t.Fatalf(
			"Create() error = %v, want %v",
			err,
			identity.ErrUsernameInvalid,
		)
	}

	if identityStore.called {
		t.Fatal(
			"identity store must not run for invalid recipient username",
		)
	}
	if conversationStore.called {
		t.Fatal(
			"conversation store must not run for invalid recipient username",
		)
	}
}

func TestUseCaseCreateForwardsMissingRecipient(
	t *testing.T,
) {
	identityStore := &fakeIdentityStore{
		err: identity.ErrNotFound,
	}
	conversationStore := &fakeConversationStore{}
	useCase := New(
		identityStore,
		conversationStore,
	)

	_, err := useCase.Create(
		context.Background(),
		Input{
			ActorID:           10,
			RecipientUsername: "missing",
		},
	)
	if !errors.Is(
		err,
		identity.ErrNotFound,
	) {
		t.Fatalf(
			"Create() error = %v, want %v",
			err,
			identity.ErrNotFound,
		)
	}

	if conversationStore.called {
		t.Fatal(
			"conversation store must not run when recipient is missing",
		)
	}
}

func TestUseCaseCreateRejectsSelfConversation(
	t *testing.T,
) {
	identityStore := &fakeIdentityStore{
		result: identity.Identity{
			ID:       10,
			Username: "alice",
		},
	}
	conversationStore := &fakeConversationStore{}
	useCase := New(
		identityStore,
		conversationStore,
	)

	_, err := useCase.Create(
		context.Background(),
		Input{
			ActorID:           10,
			RecipientUsername: "alice",
		},
	)
	if !errors.Is(
		err,
		conversation.ErrSelfConversation,
	) {
		t.Fatalf(
			"Create() error = %v, want %v",
			err,
			conversation.ErrSelfConversation,
		)
	}

	if conversationStore.called {
		t.Fatal(
			"conversation store must not run for self conversation",
		)
	}
}

func TestUseCaseCreateRejectsInvalidActorID(
	t *testing.T,
) {
	identityStore := &fakeIdentityStore{
		result: identity.Identity{
			ID:       20,
			Username: "bob",
		},
	}
	conversationStore := &fakeConversationStore{}
	useCase := New(
		identityStore,
		conversationStore,
	)

	_, err := useCase.Create(
		context.Background(),
		Input{
			ActorID:           0,
			RecipientUsername: "bob",
		},
	)
	if !errors.Is(
		err,
		conversation.ErrParticipantIDInvalid,
	) {
		t.Fatalf(
			"Create() error = %v, want %v",
			err,
			conversation.ErrParticipantIDInvalid,
		)
	}

	if !identityStore.called {
		t.Fatal(
			"identity store was not called before pair validation",
		)
	}
	if conversationStore.called {
		t.Fatal(
			"conversation store must not run for invalid actor ID",
		)
	}
}

func TestUseCaseCreateForwardsConversationStoreError(
	t *testing.T,
) {
	expectedErr := errors.New(
		"conversation store unavailable",
	)
	identityStore := &fakeIdentityStore{
		result: identity.Identity{
			ID:       20,
			Username: "bob",
		},
	}
	conversationStore := &fakeConversationStore{
		err: expectedErr,
	}
	useCase := New(
		identityStore,
		conversationStore,
	)

	_, err := useCase.Create(
		context.Background(),
		Input{
			ActorID:           10,
			RecipientUsername: "bob",
		},
	)
	if !errors.Is(
		err,
		expectedErr,
	) {
		t.Fatalf(
			"Create() error = %v, want %v",
			err,
			expectedErr,
		)
	}
}
