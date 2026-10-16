package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/imamsyafii-dev/mini-chat/server/internal/domain/identity"
)

type fakeCurrentUserUseCase struct {
	called     int
	identityID int64
	result     identity.Identity
	err        error
}

func (
	fake *fakeCurrentUserUseCase,
) Get(
	_ context.Context,
	identityID int64,
) (identity.Identity, error) {
	fake.called++
	fake.identityID = identityID

	return fake.result, fake.err
}

var _ CurrentUserUseCase = (*fakeCurrentUserUseCase)(nil)

func TestCurrentUserEndpoint(
	t *testing.T,
) {
	now := time.Date(
		2026,
		time.October,
		16,
		14,
		0,
		0,
		0,
		time.UTC,
	)
	verifier := &fakeAccessTokenVerifier{
		identityID: 42,
	}
	useCase := &fakeCurrentUserUseCase{
		result: identity.Identity{
			ID:          42,
			Username:    "alice",
			DisplayName: "Alice Example",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
	request := httptest.NewRequest(
		http.MethodGet,
		"/auth/me",
		nil,
	)
	request.Header.Set(
		"Authorization",
		"Bearer signed-access-token",
	)
	recorder := httptest.NewRecorder()

	NewHandlerWithDependencies(
		Dependencies{
			AccessTokenVerifier: verifier,
			CurrentUser:         useCase,
		},
	).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusOK,
		)
	}
	if verifier.raw != "signed-access-token" {
		t.Errorf(
			"verified token = %q, want signed-access-token",
			verifier.raw,
		)
	}
	if useCase.called != 1 || useCase.identityID != 42 {
		t.Errorf(
			"use case calls = %d, identity ID = %d, want 1, 42",
			useCase.called,
			useCase.identityID,
		)
	}

	var response currentUserResponse
	if err := json.Unmarshal(
		recorder.Body.Bytes(),
		&response,
	); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	expected := currentUserResponse{
		ID:          42,
		Username:    "alice",
		DisplayName: "Alice Example",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if response != expected {
		t.Errorf(
			"response = %#v, want %#v",
			response,
			expected,
		)
	}
	for _, secret := range []string{
		"password",
		"password_hash",
	} {
		if strings.Contains(recorder.Body.String(), secret) {
			t.Errorf(
				"response contains secret field %q",
				secret,
			)
		}
	}
}

func TestCurrentUserEndpointRequiresAuthentication(
	t *testing.T,
) {
	verifier := &fakeAccessTokenVerifier{
		identityID: 42,
	}
	useCase := &fakeCurrentUserUseCase{}
	recorder := httptest.NewRecorder()

	NewHandlerWithDependencies(
		Dependencies{
			AccessTokenVerifier: verifier,
			CurrentUser:         useCase,
		},
	).ServeHTTP(
		recorder,
		httptest.NewRequest(
			http.MethodGet,
			"/auth/me",
			nil,
		),
	)

	assertUnauthorized(t, recorder)
	if verifier.called != 0 {
		t.Errorf(
			"verifier calls = %d, want 0",
			verifier.called,
		)
	}
	if useCase.called != 0 {
		t.Errorf(
			"use case calls = %d, want 0",
			useCase.called,
		)
	}
}

func TestCurrentUserHandlerRejectsMissingPrincipal(
	t *testing.T,
) {
	useCase := &fakeCurrentUserUseCase{}
	recorder := httptest.NewRecorder()

	newCurrentUserHandler(useCase).ServeHTTP(
		recorder,
		httptest.NewRequest(
			http.MethodGet,
			"/auth/me",
			nil,
		),
	)

	assertUnauthorized(t, recorder)
	if useCase.called != 0 {
		t.Errorf(
			"use case calls = %d, want 0",
			useCase.called,
		)
	}
}

func TestCurrentUserHandlerMapsErrors(
	t *testing.T,
) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantError  string
		wantAuth   string
	}{
		{
			name:       "identity not found",
			err:        identity.ErrNotFound,
			wantStatus: http.StatusUnauthorized,
			wantError:  "unauthorized",
			wantAuth:   "Bearer",
		},
		{
			name: "wrapped identity not found",
			err: fmt.Errorf(
				"current user: %w",
				identity.ErrNotFound,
			),
			wantStatus: http.StatusUnauthorized,
			wantError:  "unauthorized",
			wantAuth:   "Bearer",
		},
		{
			name:       "internal error",
			err:        errors.New("database unavailable"),
			wantStatus: http.StatusInternalServerError,
			wantError:  "internal server error",
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				useCase := &fakeCurrentUserUseCase{
					err: test.err,
				}
				request := httptest.NewRequest(
					http.MethodGet,
					"/auth/me",
					nil,
				)
				request = request.WithContext(
					contextWithPrincipal(
						request.Context(),
						Principal{IdentityID: 42},
					),
				)
				recorder := httptest.NewRecorder()

				newCurrentUserHandler(useCase).ServeHTTP(
					recorder,
					request,
				)

				if recorder.Code != test.wantStatus {
					t.Errorf(
						"status code = %d, want %d",
						recorder.Code,
						test.wantStatus,
					)
				}
				if recorder.Header().Get(
					"WWW-Authenticate",
				) != test.wantAuth {
					t.Errorf(
						"WWW-Authenticate = %q, want %q",
						recorder.Header().Get("WWW-Authenticate"),
						test.wantAuth,
					)
				}

				var response errorResponse
				if err := json.Unmarshal(
					recorder.Body.Bytes(),
					&response,
				); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if response.Error != test.wantError {
					t.Errorf(
						"error = %q, want %q",
						response.Error,
						test.wantError,
					)
				}
			},
		)
	}
}

func TestCurrentUserRouteRequiresCompleteDependencies(
	t *testing.T,
) {
	tests := []struct {
		name     string
		verifier *fakeAccessTokenVerifier
		useCase  *fakeCurrentUserUseCase
	}{
		{
			name:     "verifier only",
			verifier: &fakeAccessTokenVerifier{identityID: 42},
		},
		{
			name:    "current user only",
			useCase: &fakeCurrentUserUseCase{},
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				dependencies := Dependencies{}
				if test.verifier != nil {
					dependencies.AccessTokenVerifier = test.verifier
				}
				if test.useCase != nil {
					dependencies.CurrentUser = test.useCase
				}
				request := httptest.NewRequest(
					http.MethodGet,
					"/auth/me",
					nil,
				)
				request.Header.Set(
					"Authorization",
					"Bearer signed-access-token",
				)
				recorder := httptest.NewRecorder()

				NewHandlerWithDependencies(
					dependencies,
				).ServeHTTP(recorder, request)

				if recorder.Code != http.StatusNotFound {
					t.Errorf(
						"status code = %d, want %d",
						recorder.Code,
						http.StatusNotFound,
					)
				}
				if test.verifier != nil && test.verifier.called != 0 {
					t.Errorf(
						"verifier calls = %d, want 0",
						test.verifier.called,
					)
				}
				if test.useCase != nil && test.useCase.called != 0 {
					t.Errorf(
						"use case calls = %d, want 0",
						test.useCase.called,
					)
				}
			},
		)
	}
}

func assertUnauthorized(
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
			recorder.Header().Get("WWW-Authenticate"),
		)
	}

	var response errorResponse
	if err := json.Unmarshal(
		recorder.Body.Bytes(),
		&response,
	); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error != "unauthorized" {
		t.Errorf(
			"error = %q, want unauthorized",
			response.Error,
		)
	}
}
