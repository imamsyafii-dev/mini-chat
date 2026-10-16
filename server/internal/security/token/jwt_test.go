package token

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var testSecret = []byte(
	"0123456789abcdef0123456789abcdef",
)

func TestNewJWTManagerRejectsInvalidSettings(
	t *testing.T,
) {
	tests := []struct {
		name    string
		secret  []byte
		ttl     time.Duration
		now     clockFunc
		wantErr error
	}{
		{
			name:    "missing secret",
			secret:  nil,
			ttl:     time.Hour,
			now:     time.Now,
			wantErr: ErrJWTSecretInvalid,
		},
		{
			name:    "whitespace secret",
			secret:  []byte(strings.Repeat(" ", 32)),
			ttl:     time.Hour,
			now:     time.Now,
			wantErr: ErrJWTSecretInvalid,
		},
		{
			name:    "short secret",
			secret:  []byte("too-short"),
			ttl:     time.Hour,
			now:     time.Now,
			wantErr: ErrJWTSecretInvalid,
		},
		{
			name:    "zero TTL",
			secret:  testSecret,
			ttl:     0,
			now:     time.Now,
			wantErr: ErrAccessTokenTTLInvalid,
		},
		{
			name:    "negative TTL",
			secret:  testSecret,
			ttl:     -time.Second,
			now:     time.Now,
			wantErr: ErrAccessTokenTTLInvalid,
		},
		{
			name:    "nil clock",
			secret:  testSecret,
			ttl:     time.Hour,
			now:     nil,
			wantErr: ErrClockRequired,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				_, err := newJWTManager(
					test.secret,
					test.ttl,
					test.now,
				)
				if !errors.Is(err, test.wantErr) {
					t.Fatalf(
						"newJWTManager() error = %v, want %v",
						err,
						test.wantErr,
					)
				}
			},
		)
	}
}

func TestNewJWTManagerCopiesSecret(
	t *testing.T,
) {
	secret := append([]byte(nil), testSecret...)
	manager, err := NewJWTManager(
		secret,
		time.Hour,
	)
	if err != nil {
		t.Fatalf(
			"NewJWTManager() error = %v, want nil",
			err,
		)
	}

	secret[0] = 'x'
	if string(manager.secret) != string(testSecret) {
		t.Fatal("manager secret changed with constructor input")
	}
}

func TestNewJWTManagerPreservesSecretBytes(
	t *testing.T,
) {
	secret := []byte(
		"  0123456789abcdef0123456789abcdef  ",
	)
	manager, err := NewJWTManager(
		secret,
		time.Hour,
	)
	if err != nil {
		t.Fatalf(
			"NewJWTManager() error = %v, want nil",
			err,
		)
	}

	if string(manager.secret) != string(secret) {
		t.Fatalf(
			"manager secret = %q, want exact input",
			manager.secret,
		)
	}
}

func TestIssueCreatesDeterministicHS256Token(
	t *testing.T,
) {
	now := time.Date(
		2026,
		time.October,
		16,
		12,
		34,
		56,
		987654321,
		time.FixedZone("test", 7*60*60),
	)
	clockCalls := 0
	manager := mustJWTManager(
		t,
		testSecret,
		90*time.Minute,
		func() time.Time {
			clockCalls++

			return now
		},
	)

	raw, expiresAt, err := manager.Issue(
		context.Background(),
		42,
	)
	if err != nil {
		t.Fatalf("Issue() error = %v, want nil", err)
	}

	if clockCalls != 1 {
		t.Fatalf(
			"clock calls = %d, want 1",
			clockCalls,
		)
	}

	wantIssuedAt := now.UTC().Truncate(time.Second)
	wantExpiresAt := wantIssuedAt.Add(90 * time.Minute)
	if !expiresAt.Equal(wantExpiresAt) {
		t.Errorf(
			"expiresAt = %v, want %v",
			expiresAt,
			wantExpiresAt,
		)
	}

	claims := jwt.RegisteredClaims{}
	parsed, _, err := jwt.NewParser().ParseUnverified(
		raw,
		&claims,
	)
	if err != nil {
		t.Fatalf(
			"ParseUnverified() error = %v, want nil",
			err,
		)
	}

	if parsed.Method != jwt.SigningMethodHS256 {
		t.Errorf(
			"signing method = %v, want HS256",
			parsed.Method,
		)
	}
	if claims.Issuer != accessTokenIssuer {
		t.Errorf(
			"issuer = %q, want %q",
			claims.Issuer,
			accessTokenIssuer,
		)
	}
	if claims.Subject != "42" {
		t.Errorf("subject = %q, want 42", claims.Subject)
	}
	if claims.IssuedAt == nil ||
		!claims.IssuedAt.Time.Equal(wantIssuedAt) {
		t.Errorf(
			"issued at = %v, want %v",
			claims.IssuedAt,
			wantIssuedAt,
		)
	}
	if claims.ExpiresAt == nil ||
		!claims.ExpiresAt.Time.Equal(wantExpiresAt) {
		t.Errorf(
			"expires at = %v, want %v",
			claims.ExpiresAt,
			wantExpiresAt,
		)
	}
}

func TestIssueAndVerify(
	t *testing.T,
) {
	now := time.Date(
		2026, time.October, 16,
		10, 0, 0, 0, time.UTC,
	)
	manager := mustJWTManager(
		t,
		testSecret,
		time.Hour,
		func() time.Time { return now },
	)

	raw, _, err := manager.Issue(
		context.Background(),
		91,
	)
	if err != nil {
		t.Fatalf("Issue() error = %v, want nil", err)
	}

	identityID, err := manager.Verify(
		context.Background(),
		raw,
	)
	if err != nil {
		t.Fatalf("Verify() error = %v, want nil", err)
	}
	if identityID != 91 {
		t.Errorf("identity ID = %d, want 91", identityID)
	}
}

func TestIssueRejectsInvalidIdentityID(
	t *testing.T,
) {
	manager := mustJWTManager(
		t,
		testSecret,
		time.Hour,
		time.Now,
	)

	for _, identityID := range []int64{0, -1} {
		_, _, err := manager.Issue(
			context.Background(),
			identityID,
		)
		if !errors.Is(err, ErrIdentityIDInvalid) {
			t.Errorf(
				"Issue(%d) error = %v, want %v",
				identityID,
				err,
				ErrIdentityIDInvalid,
			)
		}
	}
}

func TestVerifyRejectsInvalidTokens(
	t *testing.T,
) {
	now := time.Date(
		2026, time.October, 16,
		10, 0, 0, 0, time.UTC,
	)
	manager := mustJWTManager(
		t,
		testSecret,
		time.Hour,
		func() time.Time { return now },
	)
	validClaims := jwt.RegisteredClaims{
		Issuer:    accessTokenIssuer,
		Subject:   "17",
		IssuedAt:  jwt.NewNumericDate(now.Add(-time.Minute)),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}

	validRaw := signClaims(
		t,
		jwt.SigningMethodHS256,
		validClaims,
		testSecret,
	)
	signatureStart := strings.LastIndex(validRaw, ".") + 1
	tamperedSignatureByte := byte('A')
	if validRaw[signatureStart] == tamperedSignatureByte {
		tamperedSignatureByte = 'B'
	}
	tamperedRaw := validRaw[:signatureStart] +
		string(tamperedSignatureByte) +
		validRaw[signatureStart+1:]

	tests := []struct {
		name string
		raw  func(t *testing.T) string
	}{
		{
			name: "malformed",
			raw: func(*testing.T) string {
				return "not-a-jwt"
			},
		},
		{
			name: "tampered signature",
			raw: func(*testing.T) string {
				return tamperedRaw
			},
		},
		{
			name: "wrong key",
			raw: func(t *testing.T) string {
				return signClaims(
					t,
					jwt.SigningMethodHS256,
					validClaims,
					[]byte("abcdef0123456789abcdef0123456789"),
				)
			},
		},
		{
			name: "non HS256 method",
			raw: func(t *testing.T) string {
				return signClaims(
					t,
					jwt.SigningMethodHS384,
					validClaims,
					testSecret,
				)
			},
		},
		{
			name: "wrong issuer",
			raw: func(t *testing.T) string {
				claims := validClaims
				claims.Issuer = "another-service"

				return signClaims(
					t,
					jwt.SigningMethodHS256,
					claims,
					testSecret,
				)
			},
		},
		{
			name: "missing issuer",
			raw: func(t *testing.T) string {
				claims := validClaims
				claims.Issuer = ""

				return signClaims(
					t,
					jwt.SigningMethodHS256,
					claims,
					testSecret,
				)
			},
		},
		{
			name: "missing expiration",
			raw: func(t *testing.T) string {
				claims := validClaims
				claims.ExpiresAt = nil

				return signClaims(
					t,
					jwt.SigningMethodHS256,
					claims,
					testSecret,
				)
			},
		},
		{
			name: "missing issued at",
			raw: func(t *testing.T) string {
				claims := validClaims
				claims.IssuedAt = nil

				return signClaims(
					t,
					jwt.SigningMethodHS256,
					claims,
					testSecret,
				)
			},
		},
		{
			name: "expired",
			raw: func(t *testing.T) string {
				claims := validClaims
				claims.IssuedAt = jwt.NewNumericDate(
					now.Add(-2 * time.Hour),
				)
				claims.ExpiresAt = jwt.NewNumericDate(
					now.Add(-time.Hour),
				)

				return signClaims(
					t,
					jwt.SigningMethodHS256,
					claims,
					testSecret,
				)
			},
		},
		{
			name: "issued in the future",
			raw: func(t *testing.T) string {
				claims := validClaims
				claims.IssuedAt = jwt.NewNumericDate(
					now.Add(time.Minute),
				)

				return signClaims(
					t,
					jwt.SigningMethodHS256,
					claims,
					testSecret,
				)
			},
		},
		{
			name: "expiration equals issued at",
			raw: func(t *testing.T) string {
				claims := validClaims
				claims.IssuedAt = jwt.NewNumericDate(
					now.Add(-time.Minute),
				)
				claims.ExpiresAt = jwt.NewNumericDate(
					now.Add(-time.Minute),
				)

				return signClaims(
					t,
					jwt.SigningMethodHS256,
					claims,
					testSecret,
				)
			},
		},
		{
			name: "expiration before issued at",
			raw: func(t *testing.T) string {
				claims := validClaims
				claims.IssuedAt = jwt.NewNumericDate(now)
				claims.ExpiresAt = jwt.NewNumericDate(
					now.Add(-time.Minute),
				)

				return signClaims(
					t,
					jwt.SigningMethodHS256,
					claims,
					testSecret,
				)
			},
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				_, err := manager.Verify(
					context.Background(),
					test.raw(t),
				)
				if !errors.Is(
					err,
					ErrAccessTokenInvalid,
				) {
					t.Fatalf(
						"Verify() error = %v, want %v",
						err,
						ErrAccessTokenInvalid,
					)
				}
			},
		)
	}
}

func TestVerifyRejectsInvalidSubjects(
	t *testing.T,
) {
	now := time.Date(
		2026, time.October, 16,
		10, 0, 0, 0, time.UTC,
	)
	manager := mustJWTManager(
		t,
		testSecret,
		time.Hour,
		func() time.Time { return now },
	)

	for _, subject := range []string{
		"",
		"0",
		"-1",
		"abc",
		"01",
		"9223372036854775808",
	} {
		t.Run(
			subject,
			func(t *testing.T) {
				claims := jwt.RegisteredClaims{
					Issuer:    accessTokenIssuer,
					Subject:   subject,
					IssuedAt:  jwt.NewNumericDate(now.Add(-time.Minute)),
					ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
				}
				raw := signClaims(
					t,
					jwt.SigningMethodHS256,
					claims,
					testSecret,
				)

				_, err := manager.Verify(
					context.Background(),
					raw,
				)
				if !errors.Is(
					err,
					ErrAccessTokenInvalid,
				) {
					t.Fatalf(
						"Verify() error = %v, want %v",
						err,
						ErrAccessTokenInvalid,
					)
				}
			},
		)
	}
}

func TestIssueAndVerifyPreserveContextErrors(
	t *testing.T,
) {
	manager := mustJWTManager(
		t,
		testSecret,
		time.Hour,
		time.Now,
	)

	canceledContext, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	_, _, issueErr := manager.Issue(
		canceledContext,
		1,
	)
	if !errors.Is(issueErr, context.Canceled) {
		t.Errorf(
			"Issue() error = %v, want context canceled",
			issueErr,
		)
	}

	_, verifyErr := manager.Verify(
		canceledContext,
		"not-a-jwt",
	)
	if !errors.Is(verifyErr, context.Canceled) {
		t.Errorf(
			"Verify() error = %v, want context canceled",
			verifyErr,
		)
	}

	deadlineContext, deadlineCancel :=
		context.WithDeadline(
			context.Background(),
			time.Now().Add(-time.Second),
		)
	defer deadlineCancel()

	_, _, issueErr = manager.Issue(
		deadlineContext,
		1,
	)
	if !errors.Is(
		issueErr,
		context.DeadlineExceeded,
	) {
		t.Errorf(
			"Issue() error = %v, want deadline exceeded",
			issueErr,
		)
	}

	_, verifyErr = manager.Verify(
		deadlineContext,
		"not-a-jwt",
	)
	if !errors.Is(
		verifyErr,
		context.DeadlineExceeded,
	) {
		t.Errorf(
			"Verify() error = %v, want deadline exceeded",
			verifyErr,
		)
	}
}

func mustJWTManager(
	t *testing.T,
	secret []byte,
	ttl time.Duration,
	now clockFunc,
) *JWTManager {
	t.Helper()

	manager, err := newJWTManager(
		secret,
		ttl,
		now,
	)
	if err != nil {
		t.Fatalf(
			"newJWTManager() error = %v, want nil",
			err,
		)
	}

	return manager
}

func signClaims(
	t *testing.T,
	method jwt.SigningMethod,
	claims jwt.Claims,
	secret []byte,
) string {
	t.Helper()

	raw, err := jwt.NewWithClaims(
		method,
		claims,
	).SignedString(secret)
	if err != nil {
		t.Fatalf(
			"SignedString() error = %v, want nil",
			err,
		)
	}

	return raw
}
