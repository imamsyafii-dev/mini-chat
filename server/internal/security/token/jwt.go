package token

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	accessTokenIssuer = "mini-chat"
	minimumSecretSize = 32
)

var (
	ErrJWTSecretInvalid = errors.New(
		"JWT secret is invalid",
	)
	ErrAccessTokenTTLInvalid = errors.New(
		"access token TTL is invalid",
	)
	ErrClockRequired = errors.New(
		"clock is required",
	)
	ErrIdentityIDInvalid = errors.New(
		"identity ID is invalid",
	)
	ErrAccessTokenInvalid = errors.New(
		"access token is invalid",
	)
)

type clockFunc func() time.Time

type JWTManager struct {
	secret []byte
	ttl    time.Duration
	now    clockFunc
}

func NewJWTManager(
	secret []byte,
	ttl time.Duration,
) (*JWTManager, error) {
	return newJWTManager(
		secret,
		ttl,
		time.Now,
	)
}

func newJWTManager(
	secret []byte,
	ttl time.Duration,
	now clockFunc,
) (*JWTManager, error) {
	if len(bytes.TrimSpace(secret)) == 0 ||
		len(secret) < minimumSecretSize {
		return nil, ErrJWTSecretInvalid
	}

	if ttl <= 0 {
		return nil, ErrAccessTokenTTLInvalid
	}

	if now == nil {
		return nil, ErrClockRequired
	}

	secretCopy := append([]byte(nil), secret...)

	return &JWTManager{
		secret: secretCopy,
		ttl:    ttl,
		now:    now,
	}, nil
}

func (
	manager *JWTManager,
) Issue(
	ctx context.Context,
	identityID int64,
) (string, time.Time, error) {
	if err := ctx.Err(); err != nil {
		return "", time.Time{}, err
	}

	if identityID <= 0 {
		return "", time.Time{},
			ErrIdentityIDInvalid
	}

	issuedAt := manager.now().UTC().Truncate(
		time.Second,
	)
	expiresAt := issuedAt.Add(manager.ttl).
		UTC().Truncate(time.Second)
	if !expiresAt.After(issuedAt) {
		expiresAt = issuedAt.Add(time.Second)
	}

	claims := jwt.RegisteredClaims{
		Issuer: accessTokenIssuer,
		Subject: strconv.FormatInt(
			identityID,
			10,
		),
		IssuedAt: jwt.NewNumericDate(
			issuedAt,
		),
		ExpiresAt: jwt.NewNumericDate(
			expiresAt,
		),
	}

	accessToken, err := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	).SignedString(manager.secret)
	if err != nil {
		return "", time.Time{}, err
	}

	if err := ctx.Err(); err != nil {
		return "", time.Time{}, err
	}

	return accessToken, expiresAt, nil
}

func (
	manager *JWTManager,
) Verify(
	ctx context.Context,
	raw string,
) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	claims := jwt.RegisteredClaims{}
	parsedToken, err := jwt.ParseWithClaims(
		raw,
		&claims,
		func(
			parsedToken *jwt.Token,
		) (any, error) {
			if parsedToken.Method !=
				jwt.SigningMethodHS256 {
				return nil,
					ErrAccessTokenInvalid
			}

			return manager.secret, nil
		},
		jwt.WithValidMethods(
			[]string{
				jwt.SigningMethodHS256.Alg(),
			},
		),
		jwt.WithIssuer(accessTokenIssuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(manager.now),
	)

	if contextErr := ctx.Err(); contextErr != nil {
		return 0, contextErr
	}

	if err != nil ||
		parsedToken == nil ||
		!parsedToken.Valid ||
		claims.IssuedAt == nil ||
		claims.ExpiresAt == nil ||
		!claims.ExpiresAt.Time.After(
			claims.IssuedAt.Time,
		) {
		return 0, ErrAccessTokenInvalid
	}

	identityID, err := strconv.ParseInt(
		claims.Subject,
		10,
		64,
	)
	if err != nil ||
		identityID <= 0 ||
		strconv.FormatInt(identityID, 10) !=
			claims.Subject {
		return 0, ErrAccessTokenInvalid
	}

	return identityID, nil
}
