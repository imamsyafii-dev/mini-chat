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

	loginapp "github.com/imamsyafii-dev/mini-chat/server/internal/app/login"
	identitydomain "github.com/imamsyafii-dev/mini-chat/server/internal/domain/identity"
)

const validLoginJSON = `{
	"username": " Alice ",
	"password": "practice-password"
}`

type fakeLoginUseCase struct {
	called bool
	input  loginapp.Input
	result loginapp.Result
	err    error
}

func (
	fake *fakeLoginUseCase,
) Login(
	_ context.Context,
	input loginapp.Input,
) (loginapp.Result, error) {
	fake.called = true
	fake.input = input

	return fake.result, fake.err
}

var _ LoginUseCase = (*fakeLoginUseCase)(nil)

func TestLoginEndpoint(
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
	expiresAt := now.Add(time.Hour)

	loginUseCase := &fakeLoginUseCase{
		result: loginapp.Result{
			Identity: identitydomain.Identity{
				ID:          1,
				Username:    "alice",
				DisplayName: "Alice Example",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			AccessToken: "signed-access-token",
			ExpiresAt:   expiresAt,
		},
	}

	request := newLoginRequest(validLoginJSON)
	recorder := httptest.NewRecorder()

	NewHandlerWithDependencies(
		Dependencies{
			Login: loginUseCase,
		},
	).ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusOK,
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

	expectedInput := loginapp.Input{
		Username: " Alice ",
		Password: "practice-password",
	}
	if loginUseCase.input != expectedInput {
		t.Errorf(
			"login input = %#v, want %#v",
			loginUseCase.input,
			expectedInput,
		)
	}

	var response loginResponse
	if err := json.Unmarshal(
		recorder.Body.Bytes(),
		&response,
	); err != nil {
		t.Fatalf(
			"decode response: %v",
			err,
		)
	}

	expectedResponse := loginResponse{
		AccessToken: "signed-access-token",
		TokenType:   "Bearer",
		ExpiresAt:   expiresAt,
		Identity: loginIdentityResponse{
			ID:          1,
			Username:    "alice",
			DisplayName: "Alice Example",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
	if response != expectedResponse {
		t.Errorf(
			"response = %#v, want %#v",
			response,
			expectedResponse,
		)
	}

	body := recorder.Body.String()
	for _, secret := range []string{
		"practice-password",
		"password",
	} {
		if strings.Contains(body, secret) {
			t.Errorf(
				"response contains secret %q",
				secret,
			)
		}
	}
}

func TestLoginEndpointRequiresJSONContentType(
	t *testing.T,
) {
	loginUseCase := &fakeLoginUseCase{}
	request := httptest.NewRequest(
		http.MethodPost,
		"/auth/login",
		strings.NewReader(validLoginJSON),
	)
	recorder := httptest.NewRecorder()

	NewHandlerWithDependencies(
		Dependencies{
			Login: loginUseCase,
		},
	).ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code !=
		http.StatusUnsupportedMediaType {
		t.Errorf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusUnsupportedMediaType,
		)
	}

	if loginUseCase.called {
		t.Fatal("login use case must not be called")
	}
}

func TestLoginEndpointRejectsInvalidJSON(
	t *testing.T,
) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "malformed JSON",
			body: `{`,
		},
		{
			name: "unknown email field",
			body: `{
				"username": "alice",
				"password": "practice-password",
				"email": "alice@example.com"
			}`,
		},
		{
			name: "multiple JSON values",
			body: validLoginJSON + `{}`,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				loginUseCase :=
					&fakeLoginUseCase{}
				recorder :=
					httptest.NewRecorder()

				NewHandlerWithDependencies(
					Dependencies{
						Login: loginUseCase,
					},
				).ServeHTTP(
					recorder,
					newLoginRequest(test.body),
				)

				if recorder.Code !=
					http.StatusBadRequest {
					t.Errorf(
						"status code = %d, want %d",
						recorder.Code,
						http.StatusBadRequest,
					)
				}

				if loginUseCase.called {
					t.Fatal(
						"login use case must not be called",
					)
				}
			},
		)
	}
}

func TestLoginEndpointRejectsOversizedBody(
	t *testing.T,
) {
	loginUseCase := &fakeLoginUseCase{}
	body := fmt.Sprintf(
		`{"username":"alice","password":"%s"}`,
		strings.Repeat(
			"a",
			maxLoginRequestBytes,
		),
	)
	recorder := httptest.NewRecorder()

	NewHandlerWithDependencies(
		Dependencies{
			Login: loginUseCase,
		},
	).ServeHTTP(
		recorder,
		newLoginRequest(body),
	)

	if recorder.Code !=
		http.StatusRequestEntityTooLarge {
		t.Errorf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusRequestEntityTooLarge,
		)
	}

	if loginUseCase.called {
		t.Fatal("login use case must not be called")
	}
}

func TestLoginEndpointMapsErrors(
	t *testing.T,
) {
	tests := []struct {
		name      string
		err       error
		wantCode  int
		wantError string
	}{
		{
			name: "invalid credentials",
			err: identitydomain.
				ErrInvalidCredentials,
			wantCode: http.StatusUnauthorized,
			wantError: identitydomain.
				ErrInvalidCredentials.Error(),
		},
		{
			name: "wrapped invalid credentials",
			err: fmt.Errorf(
				"login: %w",
				identitydomain.
					ErrInvalidCredentials,
			),
			wantCode: http.StatusUnauthorized,
			wantError: identitydomain.
				ErrInvalidCredentials.Error(),
		},
		{
			name:      "internal error",
			err:       errors.New("database unavailable"),
			wantCode:  http.StatusInternalServerError,
			wantError: "internal server error",
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				loginUseCase :=
					&fakeLoginUseCase{
						err: test.err,
					}
				recorder :=
					httptest.NewRecorder()

				NewHandlerWithDependencies(
					Dependencies{
						Login: loginUseCase,
					},
				).ServeHTTP(
					recorder,
					newLoginRequest(validLoginJSON),
				)

				if recorder.Code != test.wantCode {
					t.Errorf(
						"status code = %d, want %d",
						recorder.Code,
						test.wantCode,
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

func newLoginRequest(
	body string,
) *http.Request {
	request := httptest.NewRequest(
		http.MethodPost,
		"/auth/login",
		strings.NewReader(body),
	)
	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	return request
}
