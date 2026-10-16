package postgres

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/imamsyafii-dev/mini-chat/server/internal/app/register"
	"github.com/imamsyafii-dev/mini-chat/server/internal/domain/identity"
)

type fakeRow struct {
	scan func(
		destinations ...any,
	) error
}

func (
	row fakeRow,
) Scan(
	destinations ...any,
) error {
	return row.scan(destinations...)
}

type fakeQuerier struct {
	query string
	args  []any
	row   pgx.Row
}

func (
	fake *fakeQuerier,
) QueryRow(
	_ context.Context,
	query string,
	args ...any,
) pgx.Row {
	fake.query = query
	fake.args = args

	return fake.row
}

var _ Querier = (*fakeQuerier)(nil)

func TestIdentityStoreCreateIdentity(
	t *testing.T,
) {
	now := time.Date(
		2026,
		time.October,
		16,
		11,
		0,
		0,
		0,
		time.UTC,
	)

	database := &fakeQuerier{
		row: fakeRow{
			scan: func(
				destinations ...any,
			) error {
				*destinations[0].(*int64) = 1
				*destinations[1].(*string) =
					"alice"
				*destinations[2].(*string) =
					"Alice Example"
				*destinations[3].(*time.Time) =
					now
				*destinations[4].(*time.Time) =
					now

				return nil
			},
		},
	}
	store := NewIdentityStore(database)

	params := register.CreateIdentityParams{
		Username:     "alice",
		PasswordHash: "encoded-password-hash",
		DisplayName:  "Alice Example",
	}

	result, err := store.CreateIdentity(
		context.Background(),
		params,
	)
	if err != nil {
		t.Fatalf(
			"CreateIdentity() error = %v, want nil",
			err,
		)
	}

	expected := identity.Identity{
		ID:          1,
		Username:    "alice",
		DisplayName: "Alice Example",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if result != expected {
		t.Errorf(
			"result = %#v, want %#v",
			result,
			expected,
		)
	}

	if !strings.Contains(
		database.query,
		"INSERT INTO identity",
	) {
		t.Errorf(
			"query = %q, want identity INSERT",
			database.query,
		)
	}

	expectedArgs := []any{
		"alice",
		"encoded-password-hash",
		"Alice Example",
	}
	if !reflect.DeepEqual(
		database.args,
		expectedArgs,
	) {
		t.Errorf(
			"args = %#v, want %#v",
			database.args,
			expectedArgs,
		)
	}
}

func TestIdentityStoreMapsUsernameConflict(
	t *testing.T,
) {
	database := &fakeQuerier{
		row: fakeRow{
			scan: func(
				...any,
			) error {
				return &pgconn.PgError{
					Code:           uniqueViolationCode,
					ConstraintName: usernameUniqueConstraint,
				}
			},
		},
	}
	store := NewIdentityStore(database)

	_, err := store.CreateIdentity(
		context.Background(),
		register.CreateIdentityParams{
			Username:     "alice",
			PasswordHash: "encoded-password-hash",
			DisplayName:  "Alice",
		},
	)
	if !errors.Is(
		err,
		identity.ErrUsernameTaken,
	) {
		t.Fatalf(
			"CreateIdentity() error = %v, want %v",
			err,
			identity.ErrUsernameTaken,
		)
	}
}

func TestIdentityStoreFindIdentityByUsername(
	t *testing.T,
) {
	now := time.Date(
		2026,
		time.October,
		16,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	database := &fakeQuerier{
		row: fakeRow{
			scan: func(
				destinations ...any,
			) error {
				*destinations[0].(*int64) = 2
				*destinations[1].(*string) =
					"alice"
				*destinations[2].(*string) =
					"encoded-password-hash"
				*destinations[3].(*string) =
					"Alice Example"
				*destinations[4].(*time.Time) =
					now
				*destinations[5].(*time.Time) =
					now

				return nil
			},
		},
	}
	store := NewIdentityStore(database)

	result, passwordHash, err :=
		store.FindIdentityByUsername(
			context.Background(),
			"alice",
		)
	if err != nil {
		t.Fatalf(
			"FindIdentityByUsername() error = %v, want nil",
			err,
		)
	}

	expected := identity.Identity{
		ID:          2,
		Username:    "alice",
		DisplayName: "Alice Example",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if result != expected {
		t.Errorf(
			"result = %#v, want %#v",
			result,
			expected,
		)
	}
	if passwordHash != "encoded-password-hash" {
		t.Errorf(
			"passwordHash = %q, want %q",
			passwordHash,
			"encoded-password-hash",
		)
	}

	if !strings.Contains(
		database.query,
		"FROM identity",
	) || !strings.Contains(
		database.query,
		"WHERE username = $1",
	) {
		t.Errorf(
			"query = %q, want identity lookup by username",
			database.query,
		)
	}

	expectedArgs := []any{"alice"}
	if !reflect.DeepEqual(
		database.args,
		expectedArgs,
	) {
		t.Errorf(
			"args = %#v, want %#v",
			database.args,
			expectedArgs,
		)
	}
}

func TestIdentityStoreMapsMissingIdentity(
	t *testing.T,
) {
	database := &fakeQuerier{
		row: fakeRow{
			scan: func(
				...any,
			) error {
				return pgx.ErrNoRows
			},
		},
	}
	store := NewIdentityStore(database)

	_, _, err := store.FindIdentityByUsername(
		context.Background(),
		"missing-user",
	)
	if !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf(
			"FindIdentityByUsername() error = %v, want %v",
			err,
			identity.ErrNotFound,
		)
	}
}

func TestIdentityStorePreservesFindDatabaseError(
	t *testing.T,
) {
	databaseError := errors.New(
		"database unavailable",
	)
	database := &fakeQuerier{
		row: fakeRow{
			scan: func(
				...any,
			) error {
				return databaseError
			},
		},
	}
	store := NewIdentityStore(database)

	_, _, err := store.FindIdentityByUsername(
		context.Background(),
		"alice",
	)
	if !errors.Is(err, databaseError) {
		t.Fatalf(
			"FindIdentityByUsername() error = %v, want wrapped %v",
			err,
			databaseError,
		)
	}
	if errors.Is(err, identity.ErrNotFound) {
		t.Fatalf(
			"FindIdentityByUsername() error = %v, must not map to %v",
			err,
			identity.ErrNotFound,
		)
	}
}

func TestIdentityStoreFindIdentityByID(
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
	database := &fakeQuerier{
		row: fakeRow{
			scan: func(
				destinations ...any,
			) error {
				*destinations[0].(*int64) = 42
				*destinations[1].(*string) = "alice"
				*destinations[2].(*string) = "Alice Example"
				*destinations[3].(*time.Time) = now
				*destinations[4].(*time.Time) = now

				return nil
			},
		},
	}
	store := NewIdentityStore(database)

	result, err := store.FindIdentityByID(
		context.Background(),
		42,
	)
	if err != nil {
		t.Fatalf(
			"FindIdentityByID() error = %v, want nil",
			err,
		)
	}

	expected := identity.Identity{
		ID:          42,
		Username:    "alice",
		DisplayName: "Alice Example",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if result != expected {
		t.Errorf(
			"result = %#v, want %#v",
			result,
			expected,
		)
	}
	if !strings.Contains(
		database.query,
		"WHERE id = $1",
	) {
		t.Errorf(
			"query = %q, want identity lookup by ID",
			database.query,
		)
	}
	if strings.Contains(
		database.query,
		"password_hash",
	) {
		t.Errorf(
			"query = %q, must not select password_hash",
			database.query,
		)
	}

	expectedArgs := []any{int64(42)}
	if !reflect.DeepEqual(database.args, expectedArgs) {
		t.Errorf(
			"args = %#v, want %#v",
			database.args,
			expectedArgs,
		)
	}
}

func TestIdentityStoreFindIdentityByIDMapsMissingIdentity(
	t *testing.T,
) {
	database := &fakeQuerier{
		row: fakeRow{
			scan: func(...any) error {
				return pgx.ErrNoRows
			},
		},
	}
	store := NewIdentityStore(database)

	_, err := store.FindIdentityByID(
		context.Background(),
		99,
	)
	if !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf(
			"FindIdentityByID() error = %v, want wrapped %v",
			err,
			identity.ErrNotFound,
		)
	}
}

func TestIdentityStoreFindIdentityByIDPreservesDatabaseError(
	t *testing.T,
) {
	databaseError := errors.New("database unavailable")
	database := &fakeQuerier{
		row: fakeRow{
			scan: func(...any) error {
				return databaseError
			},
		},
	}
	store := NewIdentityStore(database)

	_, err := store.FindIdentityByID(
		context.Background(),
		42,
	)
	if !errors.Is(err, databaseError) {
		t.Fatalf(
			"FindIdentityByID() error = %v, want wrapped %v",
			err,
			databaseError,
		)
	}
	if errors.Is(err, identity.ErrNotFound) {
		t.Fatalf(
			"FindIdentityByID() error = %v, must not map to %v",
			err,
			identity.ErrNotFound,
		)
	}
}
