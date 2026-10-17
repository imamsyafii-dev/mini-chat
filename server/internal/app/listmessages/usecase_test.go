package listmessages

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/imamsyafii-dev/mini-chat/server/internal/domain/message"
)

type listMessagesContextKey string

const listMessagesRequestIDKey listMessagesContextKey = "request-id"

type fakeMessageStore struct {
	calls          int
	ctx            context.Context
	actorID        int64
	conversationID int64
	beforeID       *int64
	limit          int
	result         []message.Message
	err            error
}

func (
	fake *fakeMessageStore,
) ListMessages(
	ctx context.Context,
	actorID int64,
	conversationID int64,
	beforeID *int64,
	limit int,
) ([]message.Message, error) {
	fake.calls++
	fake.ctx = ctx
	fake.actorID = actorID
	fake.conversationID = conversationID
	fake.beforeID = beforeID
	fake.limit = limit

	return fake.result, fake.err
}

var _ MessageStore = (*fakeMessageStore)(nil)

func TestUseCaseListForwardsInputContextAndFetchLimit(
	t *testing.T,
) {
	beforeID := int64(101)
	want := []message.Message{
		{ID: 100, ConversationID: 31, SenderID: 7},
		{ID: 99, ConversationID: 31, SenderID: 9},
	}
	store := &fakeMessageStore{
		result: want,
	}
	useCase := New(store)
	ctx := context.WithValue(
		context.Background(),
		listMessagesRequestIDKey,
		"request-31",
	)

	page, err := useCase.List(
		ctx,
		Input{
			ActorID:        7,
			ConversationID: 31,
			BeforeID:       &beforeID,
		},
	)
	if err != nil {
		t.Fatalf(
			"List() error = %v, want nil",
			err,
		)
	}
	if store.calls != 1 ||
		store.ctx != ctx ||
		store.actorID != 7 ||
		store.conversationID != 31 ||
		store.beforeID != &beforeID ||
		store.limit != 51 {
		t.Errorf(
			"store calls/context/actor/conversation/cursor/limit = %d/%v/%d/%d/%v/%d, want 1/original/7/31/original/51",
			store.calls,
			store.ctx == ctx,
			store.actorID,
			store.conversationID,
			store.beforeID == &beforeID,
			store.limit,
		)
	}
	wantChronological := []message.Message{
		want[1],
		want[0],
	}
	if !reflect.DeepEqual(
		page.Messages,
		wantChronological,
	) {
		t.Errorf(
			"Messages = %#v, want %#v",
			page.Messages,
			wantChronological,
		)
	}
	if page.NextBeforeID != nil {
		t.Errorf(
			"NextBeforeID = %v, want nil",
			*page.NextBeforeID,
		)
	}
}

func TestUseCaseListBuildsPageFromFiftyOneDescendingMessages(
	t *testing.T,
) {
	results := descendingMessages(100, 50)
	original := append(
		[]message.Message(nil),
		results...,
	)
	store := &fakeMessageStore{
		result: results,
	}
	useCase := New(store)

	page, err := useCase.List(
		context.Background(),
		Input{
			ActorID:        7,
			ConversationID: 31,
		},
	)
	if err != nil {
		t.Fatalf(
			"List() error = %v, want nil",
			err,
		)
	}
	if len(page.Messages) != 50 {
		t.Fatalf(
			"message count = %d, want 50",
			len(page.Messages),
		)
	}
	for index, value := range page.Messages {
		wantID := int64(51 + index)
		if value.ID != wantID {
			t.Fatalf(
				"Messages[%d].ID = %d, want %d",
				index,
				value.ID,
				wantID,
			)
		}
	}
	if page.NextBeforeID == nil ||
		*page.NextBeforeID != 51 {
		t.Fatalf(
			"NextBeforeID = %v, want 51",
			page.NextBeforeID,
		)
	}
	if !reflect.DeepEqual(results, original) {
		t.Errorf(
			"store result was mutated: got %#v, want %#v",
			results,
			original,
		)
	}
}

func TestUseCaseListReturnsExactlyFiftyMessagesWithoutCursor(
	t *testing.T,
) {
	store := &fakeMessageStore{
		result: descendingMessages(50, 1),
	}
	useCase := New(store)

	page, err := useCase.List(
		context.Background(),
		Input{
			ActorID:        7,
			ConversationID: 31,
		},
	)
	if err != nil {
		t.Fatalf(
			"List() error = %v, want nil",
			err,
		)
	}
	if len(page.Messages) != 50 ||
		page.Messages[0].ID != 1 ||
		page.Messages[49].ID != 50 {
		t.Errorf(
			"message count/first/last = %d/%d/%d, want 50/1/50",
			len(page.Messages),
			page.Messages[0].ID,
			page.Messages[49].ID,
		)
	}
	if page.NextBeforeID != nil {
		t.Errorf(
			"NextBeforeID = %v, want nil",
			*page.NextBeforeID,
		)
	}
}

func TestUseCaseListReturnsFewerMessagesChronologically(
	t *testing.T,
) {
	store := &fakeMessageStore{
		result: descendingMessages(13, 10),
	}
	useCase := New(store)

	page, err := useCase.List(
		context.Background(),
		Input{
			ActorID:        7,
			ConversationID: 31,
		},
	)
	if err != nil {
		t.Fatalf(
			"List() error = %v, want nil",
			err,
		)
	}
	wantIDs := []int64{10, 11, 12, 13}
	for index, wantID := range wantIDs {
		if page.Messages[index].ID != wantID {
			t.Errorf(
				"Messages[%d].ID = %d, want %d",
				index,
				page.Messages[index].ID,
				wantID,
			)
		}
	}
	if page.NextBeforeID != nil {
		t.Errorf(
			"NextBeforeID = %v, want nil",
			*page.NextBeforeID,
		)
	}
}

func TestUseCaseListConvertsNilStoreResultToNonNilEmptyMessages(
	t *testing.T,
) {
	store := &fakeMessageStore{}
	useCase := New(store)

	page, err := useCase.List(
		context.Background(),
		Input{
			ActorID:        7,
			ConversationID: 31,
		},
	)
	if err != nil {
		t.Fatalf(
			"List() error = %v, want nil",
			err,
		)
	}
	if page.Messages == nil ||
		len(page.Messages) != 0 {
		t.Errorf(
			"Messages = %#v, want non-nil empty slice",
			page.Messages,
		)
	}
	if page.NextBeforeID != nil {
		t.Errorf(
			"NextBeforeID = %v, want nil",
			*page.NextBeforeID,
		)
	}
}

func TestUseCaseListRejectsInvalidActorBeforeStore(
	t *testing.T,
) {
	for _, actorID := range []int64{0, -1} {
		store := &fakeMessageStore{}
		useCase := New(store)

		_, err := useCase.List(
			context.Background(),
			Input{
				ActorID:        actorID,
				ConversationID: 31,
			},
		)
		if err != message.ErrIdentityIDInvalid {
			t.Fatalf(
				"List() error = %v, want exact %v",
				err,
				message.ErrIdentityIDInvalid,
			)
		}
		if store.calls != 0 {
			t.Fatalf(
				"store calls = %d, want 0",
				store.calls,
			)
		}
	}
}

func TestUseCaseListRejectsInvalidConversationBeforeStore(
	t *testing.T,
) {
	for _, conversationID := range []int64{0, -1} {
		store := &fakeMessageStore{}
		useCase := New(store)

		_, err := useCase.List(
			context.Background(),
			Input{
				ActorID:        7,
				ConversationID: conversationID,
			},
		)
		if err != message.ErrConversationIDInvalid {
			t.Fatalf(
				"List() error = %v, want exact %v",
				err,
				message.ErrConversationIDInvalid,
			)
		}
		if store.calls != 0 {
			t.Fatalf(
				"store calls = %d, want 0",
				store.calls,
			)
		}
	}
}

func TestUseCaseListRejectsInvalidBeforeIDBeforeStore(
	t *testing.T,
) {
	for _, value := range []int64{0, -1} {
		beforeID := value
		store := &fakeMessageStore{}
		useCase := New(store)

		_, err := useCase.List(
			context.Background(),
			Input{
				ActorID:        7,
				ConversationID: 31,
				BeforeID:       &beforeID,
			},
		)
		if err != ErrBeforeIDInvalid {
			t.Fatalf(
				"List() error = %v, want exact %v",
				err,
				ErrBeforeIDInvalid,
			)
		}
		if store.calls != 0 {
			t.Fatalf(
				"store calls = %d, want 0",
				store.calls,
			)
		}
	}
}

func TestUseCaseListWrapsStoreErrors(
	t *testing.T,
) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "conversation unavailable",
			err:  message.ErrConversationUnavailable,
		},
		{
			name: "database error",
			err:  errors.New("database failed"),
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				store := &fakeMessageStore{
					err: test.err,
				}
				useCase := New(store)

				_, err := useCase.List(
					context.Background(),
					Input{
						ActorID:        7,
						ConversationID: 31,
					},
				)
				if !errors.Is(err, test.err) {
					t.Fatalf(
						"List() error = %v, want wrapped %v",
						err,
						test.err,
					)
				}
			},
		)
	}
}

func TestUseCaseListDoesNotMutateInputCursorAndAllocatesNextCursor(
	t *testing.T,
) {
	beforeID := int64(101)
	store := &fakeMessageStore{
		result: descendingMessages(100, 50),
	}
	useCase := New(store)

	page, err := useCase.List(
		context.Background(),
		Input{
			ActorID:        7,
			ConversationID: 31,
			BeforeID:       &beforeID,
		},
	)
	if err != nil {
		t.Fatalf(
			"List() error = %v, want nil",
			err,
		)
	}
	if beforeID != 101 {
		t.Errorf(
			"input cursor = %d, want unchanged 101",
			beforeID,
		)
	}
	if page.NextBeforeID == nil ||
		*page.NextBeforeID != 51 {
		t.Fatalf(
			"NextBeforeID = %v, want 51",
			page.NextBeforeID,
		)
	}
	if page.NextBeforeID == &beforeID {
		t.Error("NextBeforeID must use a fresh pointer")
	}
	*page.NextBeforeID = 40
	if beforeID != 101 ||
		store.result[49].ID != 51 {
		t.Error(
			"mutating NextBeforeID changed input or store result",
		)
	}
}

func descendingMessages(
	from int64,
	to int64,
) []message.Message {
	result := make(
		[]message.Message,
		0,
		from-to+1,
	)
	createdAt := time.Date(
		2026,
		time.October,
		17,
		17,
		0,
		0,
		0,
		time.UTC,
	)
	for id := from; id >= to; id-- {
		result = append(
			result,
			message.Message{
				ID:             id,
				ConversationID: 31,
				SenderID:       7,
				Body:           "message",
				CreatedAt: createdAt.Add(
					time.Duration(id) * time.Second,
				),
			},
		)
	}

	return result
}
