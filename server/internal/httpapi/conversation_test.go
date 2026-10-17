package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	createdirectapp "github.com/imamsyafii-dev/mini-chat/server/internal/app/createdirect"
	conversationdomain "github.com/imamsyafii-dev/mini-chat/server/internal/domain/conversation"
	identitydomain "github.com/imamsyafii-dev/mini-chat/server/internal/domain/identity"
)

const validCreateDirectConversationJSON = `{
	"recipient_username": "  BOB_01  "
}`

type fakeCreateDirectConversationUseCase struct {
	called int
	input  createdirectapp.Input
	result createdirectapp.Result
	err    error
}

func (
	fake *fakeCreateDirectConversationUseCase,
) Create(
	_ context.Context,
	input createdirectapp.Input,
) (createdirectapp.Result, error) {
	fake.called++
	fake.input = input

	return fake.result, fake.err
}

var _ CreateDirectConversationUseCase = (*fakeCreateDirectConversationUseCase)(nil)

type fakeListConversationsUseCase struct {
	called  int
	actorID int64
	result  []conversationdomain.DirectConversation
	err     error
}

func (
	fake *fakeListConversationsUseCase,
) List(
	_ context.Context,
	actorID int64,
) ([]conversationdomain.DirectConversation, error) {
	fake.called++
	fake.actorID = actorID

	return fake.result, fake.err
}

var _ ListConversationsUseCase = (*fakeListConversationsUseCase)(nil)

func TestCreateDirectConversationEndpointCreatesConversation(
	t *testing.T,
) {
	createdAt := time.Date(
		2026,
		time.October,
		17,
		15,
		0,
		0,
		0,
		time.UTC,
	)
	updatedAt := createdAt.Add(time.Minute)
	participantCreatedAt := createdAt.Add(-time.Hour)
	participantUpdatedAt := createdAt.Add(-time.Minute)
	useCase := &fakeCreateDirectConversationUseCase{
		result: createdirectapp.Result{
			Created: true,
			Conversation: conversationdomain.DirectConversation{
				ID:        101,
				Kind:      conversationdomain.KindDirect,
				CreatedBy: 7,
				Participant: identitydomain.Identity{
					ID:          9,
					Username:    "bob_01",
					DisplayName: "Bob Example",
					CreatedAt:   participantCreatedAt,
					UpdatedAt:   participantUpdatedAt,
				},
				CreatedAt: createdAt,
				UpdatedAt: updatedAt,
			},
		},
	}
	verifier := &fakeAccessTokenVerifier{
		identityID: 7,
	}
	recorder := httptest.NewRecorder()

	NewHandlerWithDependencies(
		Dependencies{
			AccessTokenVerifier:      verifier,
			CreateDirectConversation: useCase,
		},
	).ServeHTTP(
		recorder,
		newCreateDirectConversationRequest(
			validCreateDirectConversationJSON,
		),
	)

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusCreated,
		)
	}
	if useCase.called != 1 {
		t.Fatalf(
			"use case calls = %d, want 1",
			useCase.called,
		)
	}
	wantInput := createdirectapp.Input{
		ActorID:           7,
		RecipientUsername: "  BOB_01  ",
	}
	if useCase.input != wantInput {
		t.Errorf(
			"use case input = %#v, want %#v",
			useCase.input,
			wantInput,
		)
	}

	var response directConversationResponse
	if err := json.Unmarshal(
		recorder.Body.Bytes(),
		&response,
	); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	wantResponse := directConversationResponse{
		ID:        101,
		Kind:      conversationdomain.KindDirect,
		CreatedBy: 7,
		Participant: conversationParticipantResponse{
			ID:          9,
			Username:    "bob_01",
			DisplayName: "Bob Example",
			CreatedAt:   participantCreatedAt,
			UpdatedAt:   participantUpdatedAt,
		},
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
	if response != wantResponse {
		t.Errorf(
			"response = %#v, want %#v",
			response,
			wantResponse,
		)
	}
	for _, secret := range []string{
		"password",
		"password_hash",
	} {
		if strings.Contains(
			recorder.Body.String(),
			secret,
		) {
			t.Errorf(
				"response contains secret field %q",
				secret,
			)
		}
	}
}

func TestCreateDirectConversationEndpointReturnsOKForExistingConversation(
	t *testing.T,
) {
	useCase := &fakeCreateDirectConversationUseCase{
		result: createdirectapp.Result{
			Created: false,
			Conversation: conversationdomain.DirectConversation{
				ID:   101,
				Kind: conversationdomain.KindDirect,
			},
		},
	}
	recorder := httptest.NewRecorder()

	NewHandlerWithDependencies(
		Dependencies{
			AccessTokenVerifier: &fakeAccessTokenVerifier{
				identityID: 7,
			},
			CreateDirectConversation: useCase,
		},
	).ServeHTTP(
		recorder,
		newCreateDirectConversationRequest(
			validCreateDirectConversationJSON,
		),
	)

	if recorder.Code != http.StatusOK {
		t.Errorf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusOK,
		)
	}
}

func TestCreateDirectConversationEndpointRequiresAuthentication(
	t *testing.T,
) {
	verifier := &fakeAccessTokenVerifier{
		identityID: 7,
	}
	useCase := &fakeCreateDirectConversationUseCase{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/conversations/direct",
		strings.NewReader(
			validCreateDirectConversationJSON,
		),
	)
	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	NewHandlerWithDependencies(
		Dependencies{
			AccessTokenVerifier:      verifier,
			CreateDirectConversation: useCase,
		},
	).ServeHTTP(recorder, request)

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

func TestCreateDirectConversationHandlerRejectsMissingPrincipal(
	t *testing.T,
) {
	useCase := &fakeCreateDirectConversationUseCase{}
	recorder := httptest.NewRecorder()

	newCreateDirectConversationHandler(useCase).ServeHTTP(
		recorder,
		newCreateDirectConversationRequest(
			validCreateDirectConversationJSON,
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

func TestCreateDirectConversationEndpointRejectsInvalidRequest(
	t *testing.T,
) {
	tests := []struct {
		name        string
		body        string
		contentType string
		wantStatus  int
	}{
		{
			name:       "missing content type",
			body:       validCreateDirectConversationJSON,
			wantStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:        "malformed JSON",
			body:        `{`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name: "unknown field",
			body: `{
				"recipient_username": "bob",
				"recipient_id": 9
			}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name: "multiple JSON values",
			body: validCreateDirectConversationJSON +
				`{}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				useCase :=
					&fakeCreateDirectConversationUseCase{}
				request := httptest.NewRequest(
					http.MethodPost,
					"/conversations/direct",
					strings.NewReader(test.body),
				)
				request.Header.Set(
					"Authorization",
					"Bearer signed-access-token",
				)
				if test.contentType != "" {
					request.Header.Set(
						"Content-Type",
						test.contentType,
					)
				}
				recorder := httptest.NewRecorder()

				NewHandlerWithDependencies(
					Dependencies{
						AccessTokenVerifier: &fakeAccessTokenVerifier{
							identityID: 7,
						},
						CreateDirectConversation: useCase,
					},
				).ServeHTTP(recorder, request)

				if recorder.Code != test.wantStatus {
					t.Errorf(
						"status code = %d, want %d",
						recorder.Code,
						test.wantStatus,
					)
				}
				if useCase.called != 0 {
					t.Errorf(
						"use case calls = %d, want 0",
						useCase.called,
					)
				}
			},
		)
	}
}

func TestCreateDirectConversationEndpointRejectsOversizedBody(
	t *testing.T,
) {
	useCase := &fakeCreateDirectConversationUseCase{}
	body := fmt.Sprintf(
		`{"recipient_username":"%s"}`,
		strings.Repeat(
			"a",
			maxCreateDirectConversationRequestBytes,
		),
	)
	recorder := httptest.NewRecorder()

	NewHandlerWithDependencies(
		Dependencies{
			AccessTokenVerifier: &fakeAccessTokenVerifier{
				identityID: 7,
			},
			CreateDirectConversation: useCase,
		},
	).ServeHTTP(
		recorder,
		newCreateDirectConversationRequest(body),
	)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Errorf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusRequestEntityTooLarge,
		)
	}
	if useCase.called != 0 {
		t.Errorf(
			"use case calls = %d, want 0",
			useCase.called,
		)
	}
}

func TestCreateDirectConversationEndpointMapsErrors(
	t *testing.T,
) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantError  string
	}{
		{
			name: "wrapped invalid username",
			err: fmt.Errorf(
				"create direct: %w",
				identitydomain.ErrUsernameInvalid,
			),
			wantStatus: http.StatusBadRequest,
			wantError:  identitydomain.ErrUsernameInvalid.Error(),
		},
		{
			name: "wrapped missing recipient",
			err: fmt.Errorf(
				"create direct: %w",
				identitydomain.ErrNotFound,
			),
			wantStatus: http.StatusNotFound,
			wantError:  identitydomain.ErrNotFound.Error(),
		},
		{
			name: "wrapped self conversation",
			err: fmt.Errorf(
				"create direct: %w",
				conversationdomain.ErrSelfConversation,
			),
			wantStatus: http.StatusConflict,
			wantError: conversationdomain.
				ErrSelfConversation.Error(),
		},
		{
			name: "wrapped invalid participant",
			err: fmt.Errorf(
				"create direct: %w",
				conversationdomain.ErrParticipantIDInvalid,
			),
			wantStatus: http.StatusBadRequest,
			wantError: conversationdomain.
				ErrParticipantIDInvalid.Error(),
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
				useCase :=
					&fakeCreateDirectConversationUseCase{
						err: test.err,
					}
				recorder := httptest.NewRecorder()

				NewHandlerWithDependencies(
					Dependencies{
						AccessTokenVerifier: &fakeAccessTokenVerifier{
							identityID: 7,
						},
						CreateDirectConversation: useCase,
					},
				).ServeHTTP(
					recorder,
					newCreateDirectConversationRequest(
						validCreateDirectConversationJSON,
					),
				)

				if recorder.Code != test.wantStatus {
					t.Errorf(
						"status code = %d, want %d",
						recorder.Code,
						test.wantStatus,
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

func TestListConversationsEndpoint(
	t *testing.T,
) {
	firstTime := time.Date(
		2026,
		time.October,
		17,
		16,
		0,
		0,
		0,
		time.UTC,
	)
	secondTime := firstTime.Add(-time.Minute)
	useCase := &fakeListConversationsUseCase{
		result: []conversationdomain.DirectConversation{
			{
				ID:        20,
				Kind:      conversationdomain.KindDirect,
				CreatedBy: 7,
				Participant: identitydomain.Identity{
					ID:          9,
					Username:    "bob",
					DisplayName: "Bob",
					CreatedAt:   secondTime,
					UpdatedAt:   firstTime,
				},
				CreatedAt: secondTime,
				UpdatedAt: firstTime,
			},
			{
				ID:        19,
				Kind:      conversationdomain.KindDirect,
				CreatedBy: 8,
				Participant: identitydomain.Identity{
					ID:          8,
					Username:    "carol",
					DisplayName: "Carol",
					CreatedAt:   secondTime,
					UpdatedAt:   secondTime,
				},
				CreatedAt: secondTime,
				UpdatedAt: secondTime,
			},
		},
	}
	recorder := httptest.NewRecorder()
	request := newAuthenticatedRequest(
		http.MethodGet,
		"/conversations",
		nil,
	)

	NewHandlerWithDependencies(
		Dependencies{
			AccessTokenVerifier: &fakeAccessTokenVerifier{
				identityID: 7,
			},
			ListConversations: useCase,
		},
	).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusOK,
		)
	}
	if useCase.called != 1 || useCase.actorID != 7 {
		t.Errorf(
			"use case calls/actor = %d/%d, want 1/7",
			useCase.called,
			useCase.actorID,
		)
	}

	var response []directConversationResponse
	if err := json.Unmarshal(
		recorder.Body.Bytes(),
		&response,
	); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := []directConversationResponse{
		{
			ID:        20,
			Kind:      conversationdomain.KindDirect,
			CreatedBy: 7,
			Participant: conversationParticipantResponse{
				ID:          9,
				Username:    "bob",
				DisplayName: "Bob",
				CreatedAt:   secondTime,
				UpdatedAt:   firstTime,
			},
			CreatedAt: secondTime,
			UpdatedAt: firstTime,
		},
		{
			ID:        19,
			Kind:      conversationdomain.KindDirect,
			CreatedBy: 8,
			Participant: conversationParticipantResponse{
				ID:          8,
				Username:    "carol",
				DisplayName: "Carol",
				CreatedAt:   secondTime,
				UpdatedAt:   secondTime,
			},
			CreatedAt: secondTime,
			UpdatedAt: secondTime,
		},
	}
	if !reflect.DeepEqual(response, want) {
		t.Errorf(
			"response = %#v, want %#v",
			response,
			want,
		)
	}
	for _, secret := range []string{
		"password",
		"password_hash",
	} {
		if strings.Contains(
			recorder.Body.String(),
			secret,
		) {
			t.Errorf(
				"response contains secret field %q",
				secret,
			)
		}
	}
}

func TestListConversationsEndpointReturnsEmptyArray(
	t *testing.T,
) {
	useCase := &fakeListConversationsUseCase{
		result: make(
			[]conversationdomain.DirectConversation,
			0,
		),
	}
	recorder := httptest.NewRecorder()

	NewHandlerWithDependencies(
		Dependencies{
			AccessTokenVerifier: &fakeAccessTokenVerifier{
				identityID: 7,
			},
			ListConversations: useCase,
		},
	).ServeHTTP(
		recorder,
		newAuthenticatedRequest(
			http.MethodGet,
			"/conversations",
			nil,
		),
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusOK,
		)
	}
	if recorder.Body.String() != "[]\n" {
		t.Errorf(
			"body = %q, want %q",
			recorder.Body.String(),
			"[]\n",
		)
	}
}

func TestListConversationsEndpointRequiresAuthentication(
	t *testing.T,
) {
	verifier := &fakeAccessTokenVerifier{
		identityID: 7,
	}
	useCase := &fakeListConversationsUseCase{}
	recorder := httptest.NewRecorder()

	NewHandlerWithDependencies(
		Dependencies{
			AccessTokenVerifier: verifier,
			ListConversations:   useCase,
		},
	).ServeHTTP(
		recorder,
		httptest.NewRequest(
			http.MethodGet,
			"/conversations",
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

func TestListConversationsHandlerRejectsMissingPrincipal(
	t *testing.T,
) {
	useCase := &fakeListConversationsUseCase{}
	recorder := httptest.NewRecorder()

	newListConversationsHandler(useCase).ServeHTTP(
		recorder,
		httptest.NewRequest(
			http.MethodGet,
			"/conversations",
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

func TestListConversationsEndpointMapsErrors(
	t *testing.T,
) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantError  string
	}{
		{
			name: "wrapped invalid participant",
			err: fmt.Errorf(
				"list conversations: %w",
				conversationdomain.ErrParticipantIDInvalid,
			),
			wantStatus: http.StatusBadRequest,
			wantError: conversationdomain.
				ErrParticipantIDInvalid.Error(),
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
				useCase := &fakeListConversationsUseCase{
					err: test.err,
				}
				recorder := httptest.NewRecorder()

				NewHandlerWithDependencies(
					Dependencies{
						AccessTokenVerifier: &fakeAccessTokenVerifier{
							identityID: 7,
						},
						ListConversations: useCase,
					},
				).ServeHTTP(
					recorder,
					newAuthenticatedRequest(
						http.MethodGet,
						"/conversations",
						nil,
					),
				)

				if recorder.Code != test.wantStatus {
					t.Errorf(
						"status code = %d, want %d",
						recorder.Code,
						test.wantStatus,
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

func TestConversationRoutesRequireIndependentCompleteDependencies(
	t *testing.T,
) {
	tests := []struct {
		name       string
		deps       Dependencies
		method     string
		path       string
		wantStatus int
	}{
		{
			name: "create without verifier",
			deps: Dependencies{
				CreateDirectConversation: &fakeCreateDirectConversationUseCase{},
			},
			method:     http.MethodPost,
			path:       "/conversations/direct",
			wantStatus: http.StatusNotFound,
		},
		{
			name: "list without verifier",
			deps: Dependencies{
				ListConversations: &fakeListConversationsUseCase{},
			},
			method:     http.MethodGet,
			path:       "/conversations",
			wantStatus: http.StatusNotFound,
		},
		{
			name: "verifier without create",
			deps: Dependencies{
				AccessTokenVerifier: &fakeAccessTokenVerifier{
					identityID: 7,
				},
			},
			method:     http.MethodPost,
			path:       "/conversations/direct",
			wantStatus: http.StatusNotFound,
		},
		{
			name: "verifier without list",
			deps: Dependencies{
				AccessTokenVerifier: &fakeAccessTokenVerifier{
					identityID: 7,
				},
			},
			method:     http.MethodGet,
			path:       "/conversations",
			wantStatus: http.StatusNotFound,
		},
		{
			name: "create does not register list",
			deps: Dependencies{
				AccessTokenVerifier: &fakeAccessTokenVerifier{
					identityID: 7,
				},
				CreateDirectConversation: &fakeCreateDirectConversationUseCase{},
			},
			method:     http.MethodGet,
			path:       "/conversations",
			wantStatus: http.StatusNotFound,
		},
		{
			name: "list does not register create",
			deps: Dependencies{
				AccessTokenVerifier: &fakeAccessTokenVerifier{
					identityID: 7,
				},
				ListConversations: &fakeListConversationsUseCase{},
			},
			method:     http.MethodPost,
			path:       "/conversations/direct",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				recorder := httptest.NewRecorder()
				request := newAuthenticatedRequest(
					test.method,
					test.path,
					nil,
				)

				NewHandlerWithDependencies(
					test.deps,
				).ServeHTTP(recorder, request)

				if recorder.Code != test.wantStatus {
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

func TestConversationRoutesRejectUnsupportedMethods(
	t *testing.T,
) {
	dependencies := Dependencies{
		AccessTokenVerifier: &fakeAccessTokenVerifier{
			identityID: 7,
		},
		CreateDirectConversation: &fakeCreateDirectConversationUseCase{},
		ListConversations:        &fakeListConversationsUseCase{},
	}
	tests := []struct {
		method string
		path   string
	}{
		{
			method: http.MethodGet,
			path:   "/conversations/direct",
		},
		{
			method: http.MethodPost,
			path:   "/conversations",
		},
	}

	for _, test := range tests {
		recorder := httptest.NewRecorder()

		NewHandlerWithDependencies(
			dependencies,
		).ServeHTTP(
			recorder,
			newAuthenticatedRequest(
				test.method,
				test.path,
				nil,
			),
		)

		if recorder.Code != http.StatusMethodNotAllowed {
			t.Errorf(
				"%s %s status = %d, want %d",
				test.method,
				test.path,
				recorder.Code,
				http.StatusMethodNotAllowed,
			)
		}
	}
}

func newCreateDirectConversationRequest(
	body string,
) *http.Request {
	request := newAuthenticatedRequest(
		http.MethodPost,
		"/conversations/direct",
		strings.NewReader(body),
	)
	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	return request
}

func newAuthenticatedRequest(
	method string,
	path string,
	body io.Reader,
) *http.Request {
	request := httptest.NewRequest(
		method,
		path,
		body,
	)
	request.Header.Set(
		"Authorization",
		"Bearer signed-access-token",
	)

	return request
}
