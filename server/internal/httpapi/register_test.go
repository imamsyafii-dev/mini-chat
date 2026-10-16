package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	registerapp "github.com/imamsyafii-dev/mini-chat/server/internal/app/register"
	identitydomain "github.com/imamsyafii-dev/mini-chat/server/internal/domain/identity"
)

const validRegisterJSON = `{
	"username": "alice",
	"password": "practice-password",
	"display_name": "Alice Example"
}`

type fakeRegisterer struct {
	called bool
	input  registerapp.Input
	result identitydomain.Identity
	err    error
}

func (
	fake *fakeRegisterer,
) Register(
	_ context.Context,
	input registerapp.Input,
) (identitydomain.Identity, error) {
	fake.called = true
	fake.input = input

	return fake.result, fake.err
}

var _ Registerer = (*fakeRegisterer)(nil)

func TestRegisterEndpoint(
	t *testing.T,
) {
	now := time.Date(
		2026,
		time.October,
		16,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	registerer := &fakeRegisterer{
		result: identitydomain.Identity{
			ID:          1,
			Username:    "alice",
			DisplayName: "Alice Example",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/auth/register",
		strings.NewReader(validRegisterJSON),
	)
	request.Header.Set(
		"Content-Type",
		"application/json",
	)
	recorder := httptest.NewRecorder()

	NewHandlerWithDependencies(
		Dependencies{
			Register: registerer,
		},
	).ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusCreated,
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

	expectedInput := registerapp.Input{
		Username:    "alice",
		Password:    "practice-password",
		DisplayName: "Alice Example",
	}
	if registerer.input != expectedInput {
		t.Errorf(
			"register input = %#v, want %#v",
			registerer.input,
			expectedInput,
		)
	}

	body := recorder.Body.Bytes()

	var response registerResponse
	if err := json.Unmarshal(
		body,
		&response,
	); err != nil {
		t.Fatalf(
			"decode response: %v",
			err,
		)
	}

	expectedResponse := registerResponse{
		ID:          1,
		Username:    "alice",
		DisplayName: "Alice Example",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if response != expectedResponse {
		t.Errorf(
			"response = %#v, want %#v",
			response,
			expectedResponse,
		)
	}

	if strings.Contains(
		string(body),
		"password",
	) {
		t.Fatal(
			"response must not contain password data",
		)
	}
}

func TestRegisterEndpointRequiresJSONContentType(
	t *testing.T,
) {
	registerer := &fakeRegisterer{}

	request := httptest.NewRequest(
		http.MethodPost,
		"/auth/register",
		strings.NewReader(validRegisterJSON),
	)
	recorder := httptest.NewRecorder()

	NewHandlerWithDependencies(
		Dependencies{
			Register: registerer,
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

	if registerer.called {
		t.Fatal(
			"register use case must not be called",
		)
	}
}

func TestRegisterEndpointRejectsInvalidJSON(
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
				"display_name": "Alice",
				"email": "alice@example.com"
			}`,
		},
		{
			name: "multiple JSON values",
			body: validRegisterJSON + `{}`,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				registerer :=
					&fakeRegisterer{}

				request := httptest.NewRequest(
					http.MethodPost,
					"/auth/register",
					strings.NewReader(
						test.body,
					),
				)
				request.Header.Set(
					"Content-Type",
					"application/json",
				)
				recorder :=
					httptest.NewRecorder()

				NewHandlerWithDependencies(
					Dependencies{
						Register: registerer,
					},
				).ServeHTTP(
					recorder,
					request,
				)

				if recorder.Code !=
					http.StatusBadRequest {
					t.Errorf(
						"status code = %d, want %d",
						recorder.Code,
						http.StatusBadRequest,
					)
				}

				if registerer.called {
					t.Fatal(
						"register use case must not be called",
					)
				}
			},
		)
	}
}

func TestRegisterEndpointMapsErrors(
	t *testing.T,
) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{
			name: "invalid username",
			err: identitydomain.
				ErrUsernameInvalid,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid password",
			err: identitydomain.
				ErrPasswordInvalid,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid display name",
			err: identitydomain.
				ErrDisplayNameInvalid,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "username taken",
			err: identitydomain.
				ErrUsernameTaken,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "internal error",
			err:        errors.New("database unavailable"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				registerer :=
					&fakeRegisterer{
						err: test.err,
					}

				request := httptest.NewRequest(
					http.MethodPost,
					"/auth/register",
					strings.NewReader(
						validRegisterJSON,
					),
				)
				request.Header.Set(
					"Content-Type",
					"application/json",
				)
				recorder :=
					httptest.NewRecorder()

				NewHandlerWithDependencies(
					Dependencies{
						Register: registerer,
					},
				).ServeHTTP(
					recorder,
					request,
				)

				if recorder.Code !=
					test.wantStatus {
					t.Errorf(
						"status code = %d, want %d",
						recorder.Code,
						test.wantStatus,
					)
				}
			},
		)
	}
}
