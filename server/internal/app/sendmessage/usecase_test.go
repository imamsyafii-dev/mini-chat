package sendmessage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/imamsyafii-dev/mini-chat/server/internal/domain/message"
)

type contextKey string

const requestIDKey contextKey = "request-id"

type fakeMessageStore struct {
	called bool
	ctx    context.Context
	draft  message.Draft
	result message.Message
	err    error
}

func (
	fake *fakeMessageStore,
) CreateMessage(
	ctx context.Context,
	draft message.Draft,
) (message.Message, error) {
	fake.called = true
	fake.ctx = ctx
	fake.draft = draft

	return fake.result, fake.err
}

var _ MessageStore = (*fakeMessageStore)(nil)

func TestUseCaseSendBuildsDraftAndReturnsCreatedMessage(
	t *testing.T,
) {
	createdAt := time.Date(
		2026,
		time.October,
		17,
		16,
		0,
		0,
		0,
		time.UTC,
	)
	want := message.Message{
		ID:             31,
		ConversationID: 17,
		SenderID:       9,
		Body:           "Hello, Bob!\n\nHow are you?",
		CreatedAt:      createdAt,
	}
	store := &fakeMessageStore{
		result: want,
	}
	useCase := New(store)
	ctx := context.WithValue(
		context.Background(),
		requestIDKey,
		"request-17",
	)

	got, err := useCase.Send(
		ctx,
		Input{
			ActorID:        9,
			ConversationID: 17,
			Body:           "  Hello, Bob!\n\nHow are you?  \n",
		},
	)
	if err != nil {
		t.Fatalf(
			"Send() error = %v, want nil",
			err,
		)
	}

	wantDraft := message.Draft{
		ConversationID: 17,
		SenderID:       9,
		Body:           "Hello, Bob!\n\nHow are you?",
	}
	if !store.called {
		t.Fatal(
			"MessageStore.CreateMessage() was not called",
		)
	}
	if store.draft != wantDraft {
		t.Errorf(
			"draft = %#v, want %#v",
			store.draft,
			wantDraft,
		)
	}
	if store.ctx != ctx {
		t.Error(
			"context was not forwarded to message store",
		)
	}
	if got != want {
		t.Errorf(
			"Send() = %#v, want %#v",
			got,
			want,
		)
	}
}

func TestUseCaseSendRejectsInvalidConversationBeforeStore(
	t *testing.T,
) {
	for _, conversationID := range []int64{0, -1} {
		store := &fakeMessageStore{}
		useCase := New(store)

		_, err := useCase.Send(
			context.Background(),
			Input{
				ActorID:        9,
				ConversationID: conversationID,
				Body:           "hello",
			},
		)
		if !errors.Is(
			err,
			message.ErrConversationIDInvalid,
		) {
			t.Fatalf(
				"Send() error = %v, want %v",
				err,
				message.ErrConversationIDInvalid,
			)
		}
		if store.called {
			t.Fatalf(
				"store called for invalid conversation ID %d",
				conversationID,
			)
		}
	}
}

func TestUseCaseSendRejectsInvalidActorBeforeStore(
	t *testing.T,
) {
	for _, actorID := range []int64{0, -1} {
		store := &fakeMessageStore{}
		useCase := New(store)

		_, err := useCase.Send(
			context.Background(),
			Input{
				ActorID:        actorID,
				ConversationID: 17,
				Body:           "hello",
			},
		)
		if !errors.Is(
			err,
			message.ErrIdentityIDInvalid,
		) {
			t.Fatalf(
				"Send() error = %v, want %v",
				err,
				message.ErrIdentityIDInvalid,
			)
		}
		if store.called {
			t.Fatalf(
				"store called for invalid actor ID %d",
				actorID,
			)
		}
	}
}

func TestUseCaseSendRejectsWhitespaceBodyBeforeStore(
	t *testing.T,
) {
	store := &fakeMessageStore{}
	useCase := New(store)

	_, err := useCase.Send(
		context.Background(),
		Input{
			ActorID:        9,
			ConversationID: 17,
			Body:           " \t\n ",
		},
	)
	if !errors.Is(
		err,
		message.ErrBodyInvalid,
	) {
		t.Fatalf(
			"Send() error = %v, want %v",
			err,
			message.ErrBodyInvalid,
		)
	}
	if store.called {
		t.Fatal(
			"store called for whitespace-only body",
		)
	}
}

func TestUseCaseSendWrapsConversationUnavailable(
	t *testing.T,
) {
	store := &fakeMessageStore{
		err: message.ErrConversationUnavailable,
	}
	useCase := New(store)

	_, err := useCase.Send(
		context.Background(),
		Input{
			ActorID:        9,
			ConversationID: 17,
			Body:           "hello",
		},
	)
	if !errors.Is(
		err,
		message.ErrConversationUnavailable,
	) {
		t.Fatalf(
			"Send() error = %v, want wrapped %v",
			err,
			message.ErrConversationUnavailable,
		)
	}
}

func TestUseCaseSendWrapsStoreFailure(
	t *testing.T,
) {
	storeErr := errors.New("store failed")
	store := &fakeMessageStore{
		err: storeErr,
	}
	useCase := New(store)

	_, err := useCase.Send(
		context.Background(),
		Input{
			ActorID:        9,
			ConversationID: 17,
			Body:           "hello",
		},
	)
	if !errors.Is(err, storeErr) {
		t.Fatalf(
			"Send() error = %v, want wrapped %v",
			err,
			storeErr,
		)
	}
}
