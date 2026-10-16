package login

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/imamsyafii-dev/mini-chat/server/internal/domain/identity"
)

type fakeIdentityStore struct {
	findCalls    int
	username     string
	result       identity.Identity
	passwordHash string
	err          error
}

func (
	fake *fakeIdentityStore,
) FindIdentityByUsername(
	_ context.Context,
	username string,
) (identity.Identity, string, error) {
	fake.findCalls++
	fake.username = username

	return fake.result,
		fake.passwordHash,
		fake.err
}

type fakePasswordVerifier struct {
	verifyCalls     int
	verifyPlaintext string
	verifyEncoded   string
	verifyResult    bool
	verifyErr       error
	dummyCalls      int
	dummyPlaintext  string
	dummyErr        error
}

func (
	fake *fakePasswordVerifier,
) Verify(
	_ context.Context,
	plaintext string,
	encoded string,
) (bool, error) {
	fake.verifyCalls++
	fake.verifyPlaintext = plaintext
	fake.verifyEncoded = encoded

	return fake.verifyResult, fake.verifyErr
}

func (
	fake *fakePasswordVerifier,
) VerifyDummy(
	_ context.Context,
	plaintext string,
) error {
	fake.dummyCalls++
	fake.dummyPlaintext = plaintext

	return fake.dummyErr
}

type fakeTokenIssuer struct {
	issueCalls int
	identityID int64
	token      string
	expiresAt  time.Time
	err        error
}

func (
	fake *fakeTokenIssuer,
) Issue(
	_ context.Context,
	identityID int64,
) (string, time.Time, error) {
	fake.issueCalls++
	fake.identityID = identityID

	return fake.token, fake.expiresAt, fake.err
}

var _ IdentityStore = (*fakeIdentityStore)(nil)

var _ PasswordVerifier = (*fakePasswordVerifier)(nil)

var _ TokenIssuer = (*fakeTokenIssuer)(nil)

func TestUseCaseLogin(
	t *testing.T,
) {
	now := time.Date(
		2026,
		time.October,
		16,
		13,
		0,
		0,
		0,
		time.UTC,
	)
	expiresAt := now.Add(15 * time.Minute)
	expectedIdentity := identity.Identity{
		ID:          7,
		Username:    "alice",
		DisplayName: "Alice Example",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	store := &fakeIdentityStore{
		result:       expectedIdentity,
		passwordHash: "encoded-password-hash",
	}
	verifier := &fakePasswordVerifier{
		verifyResult: true,
	}
	issuer := &fakeTokenIssuer{
		token:     "access-token",
		expiresAt: expiresAt,
	}
	useCase := New(store, verifier, issuer)

	result, err := useCase.Login(
		context.Background(),
		Input{
			Username: "  Alice  ",
			Password: "practice-password",
		},
	)
	if err != nil {
		t.Fatalf(
			"Login() error = %v, want nil",
			err,
		)
	}

	expectedResult := Result{
		Identity:    expectedIdentity,
		AccessToken: "access-token",
		ExpiresAt:   expiresAt,
	}
	if result != expectedResult {
		t.Errorf(
			"Login() result = %#v, want %#v",
			result,
			expectedResult,
		)
	}
	if store.findCalls != 1 ||
		store.username != "alice" {
		t.Errorf(
			"store calls = %d, username = %q, want 1, %q",
			store.findCalls,
			store.username,
			"alice",
		)
	}
	if verifier.verifyCalls != 1 ||
		verifier.verifyPlaintext !=
			"practice-password" ||
		verifier.verifyEncoded !=
			"encoded-password-hash" {
		t.Errorf(
			"Verify() calls = %d, plaintext = %q, encoded = %q",
			verifier.verifyCalls,
			verifier.verifyPlaintext,
			verifier.verifyEncoded,
		)
	}
	if verifier.dummyCalls != 0 {
		t.Errorf(
			"VerifyDummy() calls = %d, want 0",
			verifier.dummyCalls,
		)
	}
	if issuer.issueCalls != 1 ||
		issuer.identityID != 7 {
		t.Errorf(
			"Issue() calls = %d, identity ID = %d, want 1, 7",
			issuer.issueCalls,
			issuer.identityID,
		)
	}
}

func TestUseCaseLoginRejectsInvalidInputWithoutDependencies(
	t *testing.T,
) {
	store := &fakeIdentityStore{}
	verifier := &fakePasswordVerifier{}
	issuer := &fakeTokenIssuer{}
	useCase := New(store, verifier, issuer)

	_, err := useCase.Login(
		context.Background(),
		Input{
			Username: "ab",
			Password: "practice-password",
		},
	)
	if !errors.Is(
		err,
		identity.ErrInvalidCredentials,
	) {
		t.Fatalf(
			"Login() error = %v, want %v",
			err,
			identity.ErrInvalidCredentials,
		)
	}
	if store.findCalls != 0 ||
		verifier.verifyCalls != 0 ||
		verifier.dummyCalls != 0 ||
		issuer.issueCalls != 0 {
		t.Fatal(
			"dependencies must not run for invalid input",
		)
	}
}

func TestUseCaseLoginHidesMissingIdentity(
	t *testing.T,
) {
	store := &fakeIdentityStore{
		err: errors.Join(
			errors.New("lookup failed"),
			identity.ErrNotFound,
		),
	}
	verifier := &fakePasswordVerifier{}
	issuer := &fakeTokenIssuer{}
	useCase := New(store, verifier, issuer)

	_, err := useCase.Login(
		context.Background(),
		Input{
			Username: "missing_user",
			Password: "exact-password",
		},
	)
	if !errors.Is(
		err,
		identity.ErrInvalidCredentials,
	) {
		t.Fatalf(
			"Login() error = %v, want %v",
			err,
			identity.ErrInvalidCredentials,
		)
	}
	if errors.Is(err, identity.ErrNotFound) {
		t.Fatalf(
			"Login() error = %v, must not expose %v",
			err,
			identity.ErrNotFound,
		)
	}
	if verifier.dummyCalls != 1 ||
		verifier.dummyPlaintext !=
			"exact-password" {
		t.Errorf(
			"VerifyDummy() calls = %d, plaintext = %q, want 1, %q",
			verifier.dummyCalls,
			verifier.dummyPlaintext,
			"exact-password",
		)
	}
	if verifier.verifyCalls != 0 ||
		issuer.issueCalls != 0 {
		t.Fatal(
			"password verification and token issue must not run for missing identity",
		)
	}
}

func TestUseCaseLoginRejectsWrongPassword(
	t *testing.T,
) {
	store := &fakeIdentityStore{
		result:       identity.Identity{ID: 7},
		passwordHash: "encoded-password-hash",
	}
	verifier := &fakePasswordVerifier{}
	issuer := &fakeTokenIssuer{}
	useCase := New(store, verifier, issuer)

	_, err := useCase.Login(
		context.Background(),
		Input{
			Username: "alice",
			Password: "wrong-password",
		},
	)
	if !errors.Is(
		err,
		identity.ErrInvalidCredentials,
	) {
		t.Fatalf(
			"Login() error = %v, want %v",
			err,
			identity.ErrInvalidCredentials,
		)
	}
	if verifier.verifyCalls != 1 {
		t.Errorf(
			"Verify() calls = %d, want 1",
			verifier.verifyCalls,
		)
	}
	if issuer.issueCalls != 0 {
		t.Errorf(
			"Issue() calls = %d, want 0",
			issuer.issueCalls,
		)
	}
}

func TestUseCaseLoginPreservesStoreError(
	t *testing.T,
) {
	storeError := errors.New("database unavailable")
	store := &fakeIdentityStore{err: storeError}
	verifier := &fakePasswordVerifier{}
	issuer := &fakeTokenIssuer{}
	useCase := New(store, verifier, issuer)

	_, err := useCase.Login(
		context.Background(),
		Input{
			Username: "alice",
			Password: "practice-password",
		},
	)
	if !errors.Is(err, storeError) {
		t.Fatalf(
			"Login() error = %v, want wrapped %v",
			err,
			storeError,
		)
	}
	if verifier.verifyCalls != 0 ||
		verifier.dummyCalls != 0 ||
		issuer.issueCalls != 0 {
		t.Fatal(
			"verification and token issue must not run after store error",
		)
	}
}

func TestUseCaseLoginPreservesVerifyError(
	t *testing.T,
) {
	verifyError := errors.New("hash is invalid")
	store := &fakeIdentityStore{
		result:       identity.Identity{ID: 7},
		passwordHash: "encoded-password-hash",
	}
	verifier := &fakePasswordVerifier{
		verifyErr: verifyError,
	}
	issuer := &fakeTokenIssuer{}
	useCase := New(store, verifier, issuer)

	_, err := useCase.Login(
		context.Background(),
		Input{
			Username: "alice",
			Password: "practice-password",
		},
	)
	if !errors.Is(err, verifyError) {
		t.Fatalf(
			"Login() error = %v, want wrapped %v",
			err,
			verifyError,
		)
	}
	if issuer.issueCalls != 0 {
		t.Errorf(
			"Issue() calls = %d, want 0",
			issuer.issueCalls,
		)
	}
}

func TestUseCaseLoginPreservesDummyVerifyError(
	t *testing.T,
) {
	dummyError := errors.New("verification unavailable")
	store := &fakeIdentityStore{
		err: identity.ErrNotFound,
	}
	verifier := &fakePasswordVerifier{
		dummyErr: dummyError,
	}
	issuer := &fakeTokenIssuer{}
	useCase := New(store, verifier, issuer)

	_, err := useCase.Login(
		context.Background(),
		Input{
			Username: "missing_user",
			Password: "practice-password",
		},
	)
	if !errors.Is(err, dummyError) {
		t.Fatalf(
			"Login() error = %v, want wrapped %v",
			err,
			dummyError,
		)
	}
	if errors.Is(
		err,
		identity.ErrInvalidCredentials,
	) {
		t.Fatalf(
			"Login() error = %v, must remain internal",
			err,
		)
	}
	if issuer.issueCalls != 0 {
		t.Errorf(
			"Issue() calls = %d, want 0",
			issuer.issueCalls,
		)
	}
}

func TestUseCaseLoginPreservesIssuerError(
	t *testing.T,
) {
	issuerError := errors.New("signing unavailable")
	store := &fakeIdentityStore{
		result:       identity.Identity{ID: 7},
		passwordHash: "encoded-password-hash",
	}
	verifier := &fakePasswordVerifier{
		verifyResult: true,
	}
	issuer := &fakeTokenIssuer{err: issuerError}
	useCase := New(store, verifier, issuer)

	_, err := useCase.Login(
		context.Background(),
		Input{
			Username: "alice",
			Password: "practice-password",
		},
	)
	if !errors.Is(err, issuerError) {
		t.Fatalf(
			"Login() error = %v, want wrapped %v",
			err,
			issuerError,
		)
	}
	if issuer.issueCalls != 1 ||
		issuer.identityID != 7 {
		t.Errorf(
			"Issue() calls = %d, identity ID = %d, want 1, 7",
			issuer.issueCalls,
			issuer.identityID,
		)
	}
}
