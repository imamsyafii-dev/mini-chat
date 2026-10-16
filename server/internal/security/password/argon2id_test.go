package password

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestArgon2IDHasherHashAndVerify(
	t *testing.T,
) {
	hasher := NewArgon2IDHasher()

	encoded, err := hasher.Hash(
		context.Background(),
		"practice-password",
	)
	if err != nil {
		t.Fatalf(
			"Hash() error = %v, want nil",
			err,
		)
	}

	if !strings.HasPrefix(
		encoded,
		passwordHashPrefix,
	) {
		t.Fatalf(
			"hash = %q, want Argon2id prefix",
			encoded,
		)
	}

	if encoded == "practice-password" {
		t.Fatal(
			"hash must not equal plaintext",
		)
	}

	matches, err := hasher.Verify(
		context.Background(),
		"practice-password",
		encoded,
	)
	if err != nil {
		t.Fatalf(
			"Verify() error = %v, want nil",
			err,
		)
	}
	if !matches {
		t.Fatal(
			"Verify() = false, want true",
		)
	}

	matches, err = hasher.Verify(
		context.Background(),
		"wrong-password",
		encoded,
	)
	if err != nil {
		t.Fatalf(
			"Verify() wrong password error = %v, want nil",
			err,
		)
	}
	if matches {
		t.Fatal(
			"Verify() wrong password = true, want false",
		)
	}
}

func TestArgon2IDHasherRejectsMalformedHash(
	t *testing.T,
) {
	hasher := NewArgon2IDHasher()

	_, err := hasher.Verify(
		context.Background(),
		"practice-password",
		"not-an-argon2id-hash",
	)
	if !errors.Is(
		err,
		ErrEncodedHashInvalid,
	) {
		t.Fatalf(
			"Verify() error = %v, want %v",
			err,
			ErrEncodedHashInvalid,
		)
	}
}

func TestArgon2IDHasherHonorsCanceledContext(
	t *testing.T,
) {
	hasher := NewArgon2IDHasher()

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	_, err := hasher.Hash(
		ctx,
		"practice-password",
	)
	if !errors.Is(
		err,
		context.Canceled,
	) {
		t.Fatalf(
			"Hash() error = %v, want %v",
			err,
			context.Canceled,
		)
	}
}

func TestArgon2IDHasherVerifyDummy(
	t *testing.T,
) {
	hasher := NewArgon2IDHasher()

	err := hasher.VerifyDummy(
		context.Background(),
		"practice-password",
	)
	if err != nil {
		t.Fatalf(
			"VerifyDummy() error = %v, want nil",
			err,
		)
	}
}

func TestArgon2IDHasherVerifyDummyHonorsCanceledContext(
	t *testing.T,
) {
	hasher := NewArgon2IDHasher()

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	err := hasher.VerifyDummy(
		ctx,
		"practice-password",
	)
	if !errors.Is(
		err,
		context.Canceled,
	) {
		t.Fatalf(
			"VerifyDummy() error = %v, want %v",
			err,
			context.Canceled,
		)
	}
}
