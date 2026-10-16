package currentuser

import (
	"context"
	"fmt"

	"github.com/imamsyafii-dev/mini-chat/server/internal/domain/identity"
)

type IdentityStore interface {
	FindIdentityByID(
		ctx context.Context,
		identityID int64,
	) (identity.Identity, error)
}

type UseCase struct {
	store IdentityStore
}

func New(
	store IdentityStore,
) *UseCase {
	return &UseCase{
		store: store,
	}
}

func (
	useCase *UseCase,
) Get(
	ctx context.Context,
	identityID int64,
) (identity.Identity, error) {
	result, err := useCase.store.FindIdentityByID(
		ctx,
		identityID,
	)
	if err != nil {
		return identity.Identity{}, fmt.Errorf(
			"find current identity: %w",
			err,
		)
	}

	return result, nil
}
