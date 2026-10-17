package listconversations

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/imamsyafii-dev/mini-chat/server/internal/domain/conversation"
)

type fakeConversationStore struct {
	called  bool
	actorID int64
	limit   int
	result  []conversation.DirectConversation
	err     error
}

func (
	fake *fakeConversationStore,
) ListDirectConversations(
	_ context.Context,
	actorID int64,
	limit int,
) ([]conversation.DirectConversation, error) {
	fake.called = true
	fake.actorID = actorID
	fake.limit = limit

	return fake.result, fake.err
}

var _ ConversationStore = (*fakeConversationStore)(nil)

func TestUseCaseListForwardsActorAndDefaultLimit(
	t *testing.T,
) {
	now := time.Date(
		2026,
		time.October,
		17,
		14,
		0,
		0,
		0,
		time.UTC,
	)
	want := []conversation.DirectConversation{
		{
			ID:        20,
			Kind:      conversation.KindDirect,
			CreatedBy: 7,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	store := &fakeConversationStore{
		result: want,
	}
	useCase := New(store)

	got, err := useCase.List(
		context.Background(),
		7,
	)
	if err != nil {
		t.Fatalf(
			"List() error = %v, want nil",
			err,
		)
	}
	if !store.called ||
		store.actorID != 7 ||
		store.limit != 50 {
		t.Errorf(
			"store called/actor/limit = %v/%d/%d, want true/7/50",
			store.called,
			store.actorID,
			store.limit,
		)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf(
			"List() = %#v, want %#v",
			got,
			want,
		)
	}
}

func TestUseCaseListPreservesNonNilEmptyResult(
	t *testing.T,
) {
	store := &fakeConversationStore{
		result: make(
			[]conversation.DirectConversation,
			0,
		),
	}
	useCase := New(store)

	got, err := useCase.List(
		context.Background(),
		7,
	)
	if err != nil {
		t.Fatalf(
			"List() error = %v, want nil",
			err,
		)
	}
	if got == nil {
		t.Fatal("List() = nil, want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf(
			"len(List()) = %d, want 0",
			len(got),
		)
	}
}

func TestUseCaseListRejectsInvalidActorBeforeStore(
	t *testing.T,
) {
	for _, actorID := range []int64{0, -1} {
		store := &fakeConversationStore{}
		useCase := New(store)

		_, err := useCase.List(
			context.Background(),
			actorID,
		)
		if !errors.Is(
			err,
			conversation.ErrParticipantIDInvalid,
		) {
			t.Fatalf(
				"List(%d) error = %v, want %v",
				actorID,
				err,
				conversation.ErrParticipantIDInvalid,
			)
		}
		if store.called {
			t.Fatalf(
				"store called for invalid actor %d",
				actorID,
			)
		}
	}
}

func TestUseCaseListWrapsStoreFailure(
	t *testing.T,
) {
	storeErr := errors.New("store failed")
	store := &fakeConversationStore{
		err: storeErr,
	}
	useCase := New(store)

	_, err := useCase.List(
		context.Background(),
		7,
	)
	if !errors.Is(err, storeErr) {
		t.Fatalf(
			"List() error = %v, want wrapped %v",
			err,
			storeErr,
		)
	}
}
