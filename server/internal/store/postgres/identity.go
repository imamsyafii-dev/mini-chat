package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/imamsyafii-dev/mini-chat/server/internal/app/register"
	"github.com/imamsyafii-dev/mini-chat/server/internal/domain/identity"
)

const (
	createIdentityQuery = `
INSERT INTO identity (
    username,
    password_hash,
    display_name
)
VALUES ($1, $2, $3)
RETURNING
    id,
    username,
    display_name,
    created_at,
    updated_at
`
	uniqueViolationCode      = "23505"
	usernameUniqueConstraint = "identity_username_unique"
)

type Querier interface {
	QueryRow(
		ctx context.Context,
		sql string,
		args ...any,
	) pgx.Row
}

type IdentityStore struct {
	db Querier
}

func NewIdentityStore(
	db Querier,
) *IdentityStore {
	return &IdentityStore{
		db: db,
	}
}

func (
	store *IdentityStore,
) CreateIdentity(
	ctx context.Context,
	params register.CreateIdentityParams,
) (identity.Identity, error) {
	var result identity.Identity

	err := store.db.QueryRow(
		ctx,
		createIdentityQuery,
		params.Username,
		params.PasswordHash,
		params.DisplayName,
	).Scan(
		&result.ID,
		&result.Username,
		&result.DisplayName,
		&result.CreatedAt,
		&result.UpdatedAt,
	)
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(
			err,
			&postgresError,
		) &&
			postgresError.Code ==
				uniqueViolationCode &&
			postgresError.ConstraintName ==
				usernameUniqueConstraint {
			return identity.Identity{},
				identity.ErrUsernameTaken
		}

		return identity.Identity{},
			fmt.Errorf(
				"insert identity: %w",
				err,
			)
	}

	return result, nil
}

var _ register.IdentityStore = (*IdentityStore)(nil)
