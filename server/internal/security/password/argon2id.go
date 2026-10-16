package password

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argon2Memory      uint32 = 19 * 1024
	argon2Iterations  uint32 = 2
	argon2Parallelism uint8  = 1
	saltLength               = 16
	keyLength                = 32
	maxConcurrentHash        = 2

	passwordHashPrefix = "$argon2id$v=19$m=19456,t=2,p=1$"
	dummyPasswordHash  = passwordHashPrefix +
		"AAAAAAAAAAAAAAAAAAAAAA$" +
		"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
)

var ErrEncodedHashInvalid = errors.New(
	"encoded password hash is invalid",
)

type Argon2IDHasher struct {
	random io.Reader
	slots  chan struct{}
}

func NewArgon2IDHasher() *Argon2IDHasher {
	return newArgon2IDHasher(
		rand.Reader,
		maxConcurrentHash,
	)
}

func newArgon2IDHasher(
	random io.Reader,
	maxConcurrent int,
) *Argon2IDHasher {
	return &Argon2IDHasher{
		random: random,
		slots: make(
			chan struct{},
			maxConcurrent,
		),
	}
}

func (
	hasher *Argon2IDHasher,
) Hash(
	ctx context.Context,
	plaintext string,
) (string, error) {
	if err := hasher.acquire(ctx); err != nil {
		return "", err
	}
	defer hasher.release()

	salt := make([]byte, saltLength)
	if _, err := io.ReadFull(
		hasher.random,
		salt,
	); err != nil {
		return "", fmt.Errorf(
			"read password salt: %w",
			err,
		)
	}

	key := argon2.IDKey(
		[]byte(plaintext),
		salt,
		argon2Iterations,
		argon2Memory,
		argon2Parallelism,
		keyLength,
	)

	encodedSalt :=
		base64.RawStdEncoding.EncodeToString(
			salt,
		)
	encodedKey :=
		base64.RawStdEncoding.EncodeToString(
			key,
		)

	return passwordHashPrefix +
		encodedSalt +
		"$" +
		encodedKey, nil
}

func (
	hasher *Argon2IDHasher,
) Verify(
	ctx context.Context,
	plaintext string,
	encoded string,
) (bool, error) {
	salt, expectedKey, err :=
		decodeHash(encoded)
	if err != nil {
		return false, err
	}

	if err := hasher.acquire(ctx); err != nil {
		return false, err
	}
	defer hasher.release()

	actualKey := argon2.IDKey(
		[]byte(plaintext),
		salt,
		argon2Iterations,
		argon2Memory,
		argon2Parallelism,
		uint32(len(expectedKey)),
	)

	return subtle.ConstantTimeCompare(
		actualKey,
		expectedKey,
	) == 1, nil
}

func (
	hasher *Argon2IDHasher,
) VerifyDummy(
	ctx context.Context,
	plaintext string,
) error {
	_, err := hasher.Verify(
		ctx,
		plaintext,
		dummyPasswordHash,
	)

	return err
}

func decodeHash(
	encoded string,
) ([]byte, []byte, error) {
	if !strings.HasPrefix(
		encoded,
		passwordHashPrefix,
	) {
		return nil, nil,
			ErrEncodedHashInvalid
	}

	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return nil, nil,
			ErrEncodedHashInvalid
	}

	salt, err :=
		base64.RawStdEncoding.DecodeString(
			parts[4],
		)
	if err != nil ||
		len(salt) != saltLength {
		return nil, nil,
			ErrEncodedHashInvalid
	}

	key, err :=
		base64.RawStdEncoding.DecodeString(
			parts[5],
		)
	if err != nil ||
		len(key) != keyLength {
		return nil, nil,
			ErrEncodedHashInvalid
	}

	return salt, key, nil
}

func (
	hasher *Argon2IDHasher,
) acquire(
	ctx context.Context,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	select {
	case hasher.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (
	hasher *Argon2IDHasher,
) release() {
	<-hasher.slots
}
