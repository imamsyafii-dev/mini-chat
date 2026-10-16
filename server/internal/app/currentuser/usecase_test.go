package currentuser

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/imamsyafii-dev/mini-chat/server/internal/domain/identity"
)

type fakeIdentityStore struct {
	called     int
	identityID int64
	result     identity.Identity
	err        error
}

func (
	fake *fakeIdentityStore,
) FindIdentityByID(
	_ context.Context,
	identityID int64,
) (identity.Identity, error) {
	fake.called++
	fake.identityID = identityID

	return fake.result, fake.err
}

var _ IdentityStore = (*fakeIdentityStore)(nil)

func TestUseCaseGet(
	t *testing.T,
) {
	now := time.Date(
		2026,
		time.October,
		16,
		14,
		0,
		0,
		0,
		time.UTC,
	)
	expected := identity.Identity{
		ID:          42,
		Username:    "alice",
		DisplayName: "Alice Example",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	store := &fakeIdentityStore{
		result: expected,
	}
	useCase := New(store)

	result, err := useCase.Get(
		context.Background(),
		42,
	)
	if err != nil {
		t.Fatalf(
			"Get() error = %v, want nil",
			err,
		)
	}
	if result != expected {
		t.Errorf(
			"Get() result = %#v, want %#v",
			result,
			expected,
		)
	}
	if store.called != 1 || store.identityID != 42 {
		t.Errorf(
			"store calls = %d, identity ID = %d, want 1, 42",
			store.called,
			store.identityID,
		)
	}
}

func TestUseCaseGetPreservesErrors(
	t *testing.T,
) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "identity not found",
			err:  identity.ErrNotFound,
		},
		{
			name: "database error",
			err:  errors.New("database unavailable"),
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				store := &fakeIdentityStore{
					err: test.err,
				}
				useCase := New(store)

				_, err := useCase.Get(
					context.Background(),
					7,
				)
				if !errors.Is(err, test.err) {
					t.Fatalf(
						"Get() error = %v, want wrapped %v",
						err,
						test.err,
					)
				}
				if store.called != 1 || store.identityID != 7 {
					t.Errorf(
						"store calls = %d, identity ID = %d, want 1, 7",
						store.called,
						store.identityID,
					)
				}
			},
		)
	}
}
