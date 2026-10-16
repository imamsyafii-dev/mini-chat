package register

import (
	"context"
	"fmt"

	"github.com/imamsyafii-dev/mini-chat/server/internal/domain/identity"
)

type Input struct {
	Username    string
	Password    string
	DisplayName string
}

type CreateIdentityParams struct {
	Username     string
	PasswordHash string
	DisplayName  string
}

type PasswordHasher interface {
	Hash(
		ctx context.Context,
		plaintext string,
	) (string, error)
}

type IdentityStore interface {
	CreateIdentity(
		ctx context.Context,
		params CreateIdentityParams,
	) (identity.Identity, error)
}

type UseCase struct {
	store  IdentityStore
	hasher PasswordHasher
}

func New(
	store IdentityStore,
	hasher PasswordHasher,
) *UseCase {
	return &UseCase{
		store:  store,
		hasher: hasher,
	}
}

func (
	useCase *UseCase,
) Register(
	ctx context.Context,
	input Input,
) (identity.Identity, error) {
	registration, err :=
		identity.NewRegistration(
			input.Username,
			input.Password,
			input.DisplayName,
		)
	if err != nil {
		return identity.Identity{}, err
	}

	passwordHash, err := useCase.hasher.Hash(
		ctx,
		registration.Password,
	)
	if err != nil {
		return identity.Identity{},
			fmt.Errorf(
				"hash password: %w",
				err,
			)
	}

	result, err :=
		useCase.store.CreateIdentity(
			ctx,
			CreateIdentityParams{
				Username:     registration.Username,
				PasswordHash: passwordHash,
				DisplayName:  registration.DisplayName,
			},
		)
	if err != nil {
		return identity.Identity{},
			fmt.Errorf(
				"create identity: %w",
				err,
			)
	}

	return result, nil
}
