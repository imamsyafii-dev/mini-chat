package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeAccessTokenVerifier struct {
	called     int
	context    context.Context
	raw        string
	identityID int64
	err        error
}

func (
	fake *fakeAccessTokenVerifier,
) Verify(
	ctx context.Context,
	raw string,
) (int64, error) {
	fake.called++
	fake.context = ctx
	fake.raw = raw

	return fake.identityID, fake.err
}

var _ AccessTokenVerifier = (*fakeAccessTokenVerifier)(nil)

type authenticationContextKey struct{}

func TestRequireAuthenticationForwardsAuthenticatedRequest(
	t *testing.T,
) {
	verifier := &fakeAccessTokenVerifier{
		identityID: 42,
	}
	nextCalled := 0
	contextKey := authenticationContextKey{}
	request := httptest.NewRequest(
		http.MethodGet,
		"/protected",
		nil,
	)
	request = request.WithContext(
		context.WithValue(
			request.Context(),
			contextKey,
			"preserved",
		),
	)
	request.Header.Set(
		"Authorization",
		"Bearer signed-access-token",
	)
	recorder := httptest.NewRecorder()

	requireAuthentication(
		verifier,
		http.HandlerFunc(
			func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				nextCalled++

				principal, ok := PrincipalFromContext(
					request.Context(),
				)
				if !ok {
					t.Fatal("principal is missing")
				}

				if principal.IdentityID != 42 {
					t.Errorf(
						"identity ID = %d, want 42",
						principal.IdentityID,
					)
				}

				if request.Context().Value(
					contextKey,
				) != "preserved" {
					t.Error(
						"existing context value was not preserved",
					)
				}

				writer.WriteHeader(http.StatusNoContent)
			},
		),
	).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Errorf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusNoContent,
		)
	}

	if verifier.called != 1 {
		t.Errorf(
			"verifier calls = %d, want 1",
			verifier.called,
		)
	}

	if verifier.raw != "signed-access-token" {
		t.Errorf(
			"raw token = %q, want %q",
			verifier.raw,
			"signed-access-token",
		)
	}

	if verifier.context != request.Context() {
		t.Error("verifier did not receive the request context")
	}

	if nextCalled != 1 {
		t.Errorf(
			"next calls = %d, want 1",
			nextCalled,
		)
	}
}

func TestRequireAuthenticationAcceptsBearerFormatting(
	t *testing.T,
) {
	tests := []struct {
		name   string
		header string
	}{
		{
			name:   "mixed case scheme",
			header: "bEaReR signed-token",
		},
		{
			name: "repeated whitespace",
			header: "  Bearer\t  " +
				"signed-token   ",
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				verifier := &fakeAccessTokenVerifier{
					identityID: 7,
				}
				nextCalled := 0
				request := httptest.NewRequest(
					http.MethodGet,
					"/protected",
					nil,
				)
				request.Header.Set(
					"Authorization",
					test.header,
				)
				recorder := httptest.NewRecorder()

				requireAuthentication(
					verifier,
					http.HandlerFunc(
						func(
							writer http.ResponseWriter,
							_ *http.Request,
						) {
							nextCalled++
							writer.WriteHeader(
								http.StatusNoContent,
							)
						},
					),
				).ServeHTTP(recorder, request)

				if recorder.Code != http.StatusNoContent {
					t.Errorf(
						"status code = %d, want %d",
						recorder.Code,
						http.StatusNoContent,
					)
				}

				if verifier.called != 1 {
					t.Errorf(
						"verifier calls = %d, want 1",
						verifier.called,
					)
				}

				if verifier.raw != "signed-token" {
					t.Errorf(
						"raw token = %q, want signed-token",
						verifier.raw,
					)
				}

				if nextCalled != 1 {
					t.Errorf(
						"next calls = %d, want 1",
						nextCalled,
					)
				}
			},
		)
	}
}

func TestRequireAuthenticationRejectsMalformedHeader(
	t *testing.T,
) {
	tests := []struct {
		name    string
		headers []string
	}{
		{
			name: "missing header",
		},
		{
			name:    "blank header",
			headers: []string{""},
		},
		{
			name:    "wrong scheme",
			headers: []string{"Basic credentials"},
		},
		{
			name:    "missing token",
			headers: []string{"Bearer"},
		},
		{
			name:    "extra part",
			headers: []string{"Bearer token extra"},
		},
		{
			name: "duplicate headers",
			headers: []string{
				"Bearer first-token",
				"Bearer second-token",
			},
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				verifier := &fakeAccessTokenVerifier{
					identityID: 1,
				}
				nextCalled := 0
				request := httptest.NewRequest(
					http.MethodGet,
					"/protected",
					nil,
				)
				for _, header := range test.headers {
					request.Header.Add(
						"Authorization",
						header,
					)
				}
				recorder := httptest.NewRecorder()

				requireAuthentication(
					verifier,
					http.HandlerFunc(
						func(
							http.ResponseWriter,
							*http.Request,
						) {
							nextCalled++
						},
					),
				).ServeHTTP(recorder, request)

				assertUnauthorizedResponse(
					t,
					recorder,
				)

				if verifier.called != 0 {
					t.Errorf(
						"verifier calls = %d, want 0",
						verifier.called,
					)
				}

				if nextCalled != 0 {
					t.Errorf(
						"next calls = %d, want 0",
						nextCalled,
					)
				}
			},
		)
	}
}

func TestRequireAuthenticationRejectsInvalidVerification(
	t *testing.T,
) {
	tests := []struct {
		name       string
		identityID int64
		err        error
	}{
		{
			name:       "verifier error",
			identityID: 42,
			err:        errors.New("secret verifier detail"),
		},
		{
			name:       "zero identity ID",
			identityID: 0,
		},
		{
			name:       "negative identity ID",
			identityID: -1,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				verifier := &fakeAccessTokenVerifier{
					identityID: test.identityID,
					err:        test.err,
				}
				nextCalled := 0
				request := httptest.NewRequest(
					http.MethodGet,
					"/protected",
					nil,
				)
				request.Header.Set(
					"Authorization",
					"Bearer secret-raw-token",
				)
				recorder := httptest.NewRecorder()

				requireAuthentication(
					verifier,
					http.HandlerFunc(
						func(
							http.ResponseWriter,
							*http.Request,
						) {
							nextCalled++
						},
					),
				).ServeHTTP(recorder, request)

				assertUnauthorizedResponse(
					t,
					recorder,
				)

				if verifier.called != 1 {
					t.Errorf(
						"verifier calls = %d, want 1",
						verifier.called,
					)
				}

				if nextCalled != 0 {
					t.Errorf(
						"next calls = %d, want 0",
						nextCalled,
					)
				}
			},
		)
	}
}

func assertUnauthorizedResponse(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
) {
	t.Helper()

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusUnauthorized,
		)
	}

	if recorder.Header().Get(
		"WWW-Authenticate",
	) != "Bearer" {
		t.Errorf(
			"WWW-Authenticate = %q, want Bearer",
			recorder.Header().Get(
				"WWW-Authenticate",
			),
		)
	}

	if recorder.Header().Get(
		"Content-Type",
	) != "application/json" {
		t.Errorf(
			"Content-Type = %q, want application/json",
			recorder.Header().Get(
				"Content-Type",
			),
		)
	}

	var response errorResponse
	if err := json.Unmarshal(
		recorder.Body.Bytes(),
		&response,
	); err != nil {
		t.Fatalf(
			"decode response: %v",
			err,
		)
	}

	if response.Error != "unauthorized" {
		t.Errorf(
			"error = %q, want unauthorized",
			response.Error,
		)
	}

	body := recorder.Body.String()
	for _, secret := range []string{
		"secret verifier detail",
		"secret-raw-token",
	} {
		if contains := strings.Contains(body, secret); contains {
			t.Errorf(
				"response contains secret %q",
				secret,
			)
		}
	}
}
