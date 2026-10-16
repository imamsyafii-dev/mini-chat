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
