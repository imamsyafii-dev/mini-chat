package login

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/imamsyafii-dev/mini-chat/server/internal/domain/identity"
)

type Input struct {
	Username string
	Password string
}

type Result struct {
	Identity    identity.Identity
	AccessToken string
	ExpiresAt   time.Time
}

type IdentityStore interface {
	FindIdentityByUsername(
		ctx context.Context,
		username string,
	) (identity.Identity, string, error)
}

type PasswordVerifier interface {
	Verify(
		ctx context.Context,
		plaintext string,
		encoded string,
	) (bool, error)
	VerifyDummy(
		ctx context.Context,
		plaintext string,
	) error
}

type TokenIssuer interface {
	Issue(
		ctx context.Context,
		identityID int64,
	) (string, time.Time, error)
}

type UseCase struct {
	store    IdentityStore
	verifier PasswordVerifier
	issuer   TokenIssuer
}

func New(
	store IdentityStore,
	verifier PasswordVerifier,
	issuer TokenIssuer,
) *UseCase {
	return &UseCase{
		store:    store,
		verifier: verifier,
		issuer:   issuer,
	}
}

func (
	useCase *UseCase,
) Login(
	ctx context.Context,
	input Input,
) (Result, error) {
	credentials, err :=
		identity.NewLoginCredentials(
			input.Username,
			input.Password,
		)
	if err != nil {
		return Result{}, err
	}

	foundIdentity, passwordHash, err :=
		useCase.store.FindIdentityByUsername(
			ctx,
			credentials.Username,
		)
	if errors.Is(err, identity.ErrNotFound) {
		if dummyErr :=
			useCase.verifier.VerifyDummy(
				ctx,
				credentials.Password,
			); dummyErr != nil {
			return Result{}, fmt.Errorf(
				"verify dummy password: %w",
				dummyErr,
			)
		}

		return Result{},
			identity.ErrInvalidCredentials
	}
	if err != nil {
		return Result{}, fmt.Errorf(
			"find identity by username: %w",
			err,
		)
	}

	matches, err := useCase.verifier.Verify(
		ctx,
		credentials.Password,
		passwordHash,
	)
	if err != nil {
		return Result{}, fmt.Errorf(
			"verify password: %w",
			err,
		)
	}
	if !matches {
		return Result{},
			identity.ErrInvalidCredentials
	}

	accessToken, expiresAt, err :=
		useCase.issuer.Issue(
			ctx,
			foundIdentity.ID,
		)
	if err != nil {
		return Result{}, fmt.Errorf(
			"issue access token: %w",
			err,
		)
	}

	return Result{
		Identity:    foundIdentity,
		AccessToken: accessToken,
		ExpiresAt:   expiresAt,
	}, nil
}
