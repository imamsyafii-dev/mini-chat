package register

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/imamsyafii-dev/mini-chat/server/internal/domain/identity"
)

type fakePasswordHasher struct {
	called    bool
	plaintext string
	result    string
	err       error
}

func (
	fake *fakePasswordHasher,
) Hash(
	_ context.Context,
	plaintext string,
) (string, error) {
	fake.called = true
	fake.plaintext = plaintext

	return fake.result, fake.err
}

type fakeIdentityStore struct {
	called bool
	params CreateIdentityParams
	result identity.Identity
	err    error
}

func (
	fake *fakeIdentityStore,
) CreateIdentity(
	_ context.Context,
	params CreateIdentityParams,
) (identity.Identity, error) {
	fake.called = true
	fake.params = params

	return fake.result, fake.err
}

var _ PasswordHasher = (*fakePasswordHasher)(nil)

var _ IdentityStore = (*fakeIdentityStore)(nil)

func TestUseCaseRegister(
	t *testing.T,
) {
	now := time.Date(
		2026,
		time.October,
		16,
		10,
		0,
		0,
		0,
		time.UTC,
	)

	expected := identity.Identity{
		ID:          1,
		Username:    "alice",
		DisplayName: "Alice Example",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	hasher := &fakePasswordHasher{
		result: "encoded-password-hash",
	}
	store := &fakeIdentityStore{
		result: expected,
	}
	useCase := New(store, hasher)

	result, err := useCase.Register(
		context.Background(),
		Input{
			Username:    "  Alice  ",
			Password:    "practice-password",
			DisplayName: "  Alice Example  ",
		},
	)
	if err != nil {
		t.Fatalf(
			"Register() error = %v, want nil",
			err,
		)
	}

	if result != expected {
		t.Errorf(
			"Register() result = %#v, want %#v",
			result,
			expected,
		)
	}

	if !hasher.called {
		t.Fatal(
			"PasswordHasher.Hash() was not called",
		)
	}
	if hasher.plaintext !=
		"practice-password" {
		t.Errorf(
			"hashed plaintext = %q, want %q",
			hasher.plaintext,
			"practice-password",
		)
	}

	if !store.called {
		t.Fatal(
			"IdentityStore.CreateIdentity() was not called",
		)
	}

	expectedParams := CreateIdentityParams{
		Username:     "alice",
		PasswordHash: "encoded-password-hash",
		DisplayName:  "Alice Example",
	}
	if store.params != expectedParams {
		t.Errorf(
			"store params = %#v, want %#v",
			store.params,
			expectedParams,
		)
	}
}

func TestUseCaseRegisterRejectsInvalidInput(
	t *testing.T,
) {
	hasher := &fakePasswordHasher{}
	store := &fakeIdentityStore{}
	useCase := New(store, hasher)

	_, err := useCase.Register(
		context.Background(),
		Input{
			Username:    "ab",
			Password:    "practice-password",
			DisplayName: "Alice",
		},
	)
	if !errors.Is(
		err,
		identity.ErrUsernameInvalid,
	) {
		t.Fatalf(
			"Register() error = %v, want %v",
			err,
			identity.ErrUsernameInvalid,
		)
	}

	if hasher.called {
		t.Fatal(
			"hasher must not run for invalid input",
		)
	}
	if store.called {
		t.Fatal(
			"store must not run for invalid input",
		)
	}
}

func TestUseCaseRegisterForwardsUsernameTaken(
	t *testing.T,
) {
	hasher := &fakePasswordHasher{
		result: "encoded-password-hash",
	}
	store := &fakeIdentityStore{
		err: identity.ErrUsernameTaken,
	}
	useCase := New(store, hasher)

	_, err := useCase.Register(
		context.Background(),
		Input{
			Username:    "alice",
			Password:    "practice-password",
			DisplayName: "Alice",
		},
	)
	if !errors.Is(
		err,
		identity.ErrUsernameTaken,
	) {
		t.Fatalf(
			"Register() error = %v, want %v",
			err,
			identity.ErrUsernameTaken,
		)
	}
}
