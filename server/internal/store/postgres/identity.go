package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	currentuserapp "github.com/imamsyafii-dev/mini-chat/server/internal/app/currentuser"
	loginapp "github.com/imamsyafii-dev/mini-chat/server/internal/app/login"
	registerapp "github.com/imamsyafii-dev/mini-chat/server/internal/app/register"
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
	findIdentityByUsernameQuery = `
SELECT
    id,
    username,
    password_hash,
    display_name,
    created_at,
    updated_at
FROM identity
WHERE username = $1
`
	findPublicIdentityByUsernameQuery = `
SELECT
    id,
    username,
    display_name,
    created_at,
    updated_at
FROM identity
WHERE username = $1
`
	findIdentityByIDQuery = `
SELECT
    id,
    username,
    display_name,
    created_at,
    updated_at
FROM identity
WHERE id = $1
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
	params registerapp.CreateIdentityParams,
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

func (
	store *IdentityStore,
) FindIdentityByUsername(
	ctx context.Context,
	username string,
) (identity.Identity, string, error) {
	var result identity.Identity
	var passwordHash string

	err := store.db.QueryRow(
		ctx,
		findIdentityByUsernameQuery,
		username,
	).Scan(
		&result.ID,
		&result.Username,
		&passwordHash,
		&result.DisplayName,
		&result.CreatedAt,
		&result.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.Identity{}, "",
			fmt.Errorf(
				"find identity by username: %w",
				identity.ErrNotFound,
			)
	}
	if err != nil {
		return identity.Identity{}, "",
			fmt.Errorf(
				"find identity by username: %w",
				err,
			)
	}

	return result, passwordHash, nil
}

func (
	store *IdentityStore,
) FindPublicIdentityByUsername(
	ctx context.Context,
	username string,
) (identity.Identity, error) {
	var result identity.Identity

	err := store.db.QueryRow(
		ctx,
		findPublicIdentityByUsernameQuery,
		username,
	).Scan(
		&result.ID,
		&result.Username,
		&result.DisplayName,
		&result.CreatedAt,
		&result.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.Identity{}, fmt.Errorf(
			"find public identity by username: %w",
			identity.ErrNotFound,
		)
	}
	if err != nil {
		return identity.Identity{}, fmt.Errorf(
			"find public identity by username: %w",
			err,
		)
	}

	return result, nil
}

func (
	store *IdentityStore,
) FindIdentityByID(
	ctx context.Context,
	identityID int64,
) (identity.Identity, error) {
	var result identity.Identity

	err := store.db.QueryRow(
		ctx,
		findIdentityByIDQuery,
		identityID,
	).Scan(
		&result.ID,
		&result.Username,
		&result.DisplayName,
		&result.CreatedAt,
		&result.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.Identity{}, fmt.Errorf(
			"find identity by ID: %w",
			identity.ErrNotFound,
		)
	}
	if err != nil {
		return identity.Identity{}, fmt.Errorf(
			"find identity by ID: %w",
			err,
		)
	}

	return result, nil
}

var _ registerapp.IdentityStore = (*IdentityStore)(nil)

var _ loginapp.IdentityStore = (*IdentityStore)(nil)

var _ currentuserapp.IdentityStore = (*IdentityStore)(nil)
