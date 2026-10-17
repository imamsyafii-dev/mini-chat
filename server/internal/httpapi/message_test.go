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

	listmessagesapp "github.com/imamsyafii-dev/mini-chat/server/internal/app/listmessages"
	sendmessageapp "github.com/imamsyafii-dev/mini-chat/server/internal/app/sendmessage"
	messagedomain "github.com/imamsyafii-dev/mini-chat/server/internal/domain/message"
)

type fakeSendMessageUseCase struct {
	called int
	input  sendmessageapp.Input
	result messagedomain.Message
	err    error
}

func (
	fake *fakeSendMessageUseCase,
) Send(
	_ context.Context,
	input sendmessageapp.Input,
) (messagedomain.Message, error) {
	fake.called++
	fake.input = input

	return fake.result, fake.err
}

var _ SendMessageUseCase = (*fakeSendMessageUseCase)(nil)

type fakeListMessagesUseCase struct {
	called int
	input  listmessagesapp.Input
	result listmessagesapp.Page
	err    error
}

func (
	fake *fakeListMessagesUseCase,
) List(
	_ context.Context,
	input listmessagesapp.Input,
) (listmessagesapp.Page, error) {
	fake.called++
	fake.input = input

	return fake.result, fake.err
}

var _ ListMessagesUseCase = (*fakeListMessagesUseCase)(nil)

func TestSendMessageEndpointCreatesMessage(
	t *testing.T,
) {
	createdAt := time.Date(
		2026,
		time.October,
		17,
		18,
		30,
		0,
		0,
		time.UTC,
	)
	useCase := &fakeSendMessageUseCase{
		result: messagedomain.Message{
			ID:             91,
			ConversationID: 23,
			SenderID:       7,
			Body:           "Hello Bob",
			CreatedAt:      createdAt,
		},
	}
	recorder := httptest.NewRecorder()

	newMessageTestHandler(
		useCase,
		nil,
	).ServeHTTP(
		recorder,
		newAuthenticatedMessageRequest(
			http.MethodPost,
			"/conversations/23/messages",
			`{"body":"  Hello Bob  "}`,
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
	wantInput := sendmessageapp.Input{
		ActorID:        7,
		ConversationID: 23,
		Body:           "  Hello Bob  ",
	}
	if useCase.input != wantInput {
		t.Errorf(
			"input = %#v, want %#v",
			useCase.input,
			wantInput,
		)
	}

	var response messageResponse
	decodeMessageTestResponse(t, recorder, &response)
	wantResponse := messageResponse{
		ID:             91,
		ConversationID: 23,
		SenderID:       7,
		Body:           "Hello Bob",
		CreatedAt:      createdAt,
	}
	if response != wantResponse {
		t.Errorf(
			"response = %#v, want %#v",
			response,
			wantResponse,
		)
	}
}

func TestListMessagesEndpointReturnsPage(
	t *testing.T,
) {
	createdAt := time.Date(
		2026,
		time.October,
		17,
		19,
		0,
		0,
		0,
		time.UTC,
	)
	nextBeforeID := int64(81)
	useCase := &fakeListMessagesUseCase{
		result: listmessagesapp.Page{
			Messages: []messagedomain.Message{
				{
					ID:             81,
					ConversationID: 23,
					SenderID:       9,
					Body:           "First",
					CreatedAt:      createdAt,
				},
				{
					ID:             82,
					ConversationID: 23,
					SenderID:       7,
					Body:           "Second",
					CreatedAt:      createdAt.Add(time.Second),
				},
			},
			NextBeforeID: &nextBeforeID,
		},
	}
	recorder := httptest.NewRecorder()

	newMessageTestHandler(
		nil,
		useCase,
	).ServeHTTP(
		recorder,
		newAuthenticatedMessageRequest(
			http.MethodGet,
			"/conversations/23/messages?before_id=100",
			"",
		),
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusOK,
		)
	}
	if useCase.called != 1 {
		t.Fatalf(
			"use case calls = %d, want 1",
			useCase.called,
		)
	}
	if useCase.input.ActorID != 7 ||
		useCase.input.ConversationID != 23 ||
		useCase.input.BeforeID == nil ||
		*useCase.input.BeforeID != 100 {
		t.Errorf(
			"input = %#v, want actor 7, conversation 23, before 100",
			useCase.input,
		)
	}

	var response listMessagesResponse
	decodeMessageTestResponse(t, recorder, &response)
	if len(response.Messages) != 2 {
		t.Fatalf(
			"message count = %d, want 2",
			len(response.Messages),
		)
	}
	if response.Messages[0].ID != 81 ||
		response.Messages[1].ID != 82 {
		t.Errorf(
			"message IDs = [%d %d], want [81 82]",
			response.Messages[0].ID,
			response.Messages[1].ID,
		)
	}
	if response.NextBeforeID == nil ||
		*response.NextBeforeID != 81 {
		t.Errorf(
			"next before ID = %v, want 81",
			response.NextBeforeID,
		)
	}
}

func TestListMessagesEndpointReturnsAllocatedEmptyPage(
	t *testing.T,
) {
	useCase := &fakeListMessagesUseCase{}
	recorder := httptest.NewRecorder()

	newMessageTestHandler(
		nil,
		useCase,
	).ServeHTTP(
		recorder,
		newAuthenticatedMessageRequest(
			http.MethodGet,
			"/conversations/23/messages",
			"",
		),
	)

	if useCase.input.BeforeID != nil {
		t.Errorf(
			"before ID = %v, want nil",
			useCase.input.BeforeID,
		)
	}
	if got := strings.TrimSpace(
		recorder.Body.String(),
	); got != `{"messages":[],"next_before_id":null}` {
		t.Errorf(
			"response = %s, want allocated empty messages and null cursor",
			got,
		)
	}
}

func TestMessageEndpointsRequireAuthentication(
	t *testing.T,
) {
	tests := []struct {
		name   string
		method string
		body   string
	}{
		{
			name:   "send",
			method: http.MethodPost,
			body:   `{"body":"hello"}`,
		},
		{
			name:   "list",
			method: http.MethodGet,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				sendUseCase := &fakeSendMessageUseCase{}
				listUseCase := &fakeListMessagesUseCase{}
				verifier := &fakeAccessTokenVerifier{
					identityID: 7,
				}
				recorder := httptest.NewRecorder()
				request := httptest.NewRequest(
					test.method,
					"/conversations/23/messages",
					strings.NewReader(test.body),
				)

				NewHandlerWithDependencies(
					Dependencies{
						AccessTokenVerifier: verifier,
						SendMessage:         sendUseCase,
						ListMessages:        listUseCase,
					},
				).ServeHTTP(recorder, request)

				assertUnauthorized(t, recorder)
				if verifier.called != 0 {
					t.Errorf(
						"verifier calls = %d, want 0",
						verifier.called,
					)
				}
				if sendUseCase.called != 0 ||
					listUseCase.called != 0 {
					t.Errorf(
						"use case calls = send %d, list %d; want 0, 0",
						sendUseCase.called,
						listUseCase.called,
					)
				}
			},
		)
	}
}

func TestMessageHandlersRejectMissingPrincipalDirectly(
	t *testing.T,
) {
	sendUseCase := &fakeSendMessageUseCase{}
	sendRecorder := httptest.NewRecorder()
	sendRequest := httptest.NewRequest(
		http.MethodPost,
		"/conversations/23/messages",
		strings.NewReader(`{"body":"hello"}`),
	)
	sendRequest.SetPathValue("conversationID", "23")

	newSendMessageHandler(sendUseCase).ServeHTTP(
		sendRecorder,
		sendRequest,
	)
	assertUnauthorized(t, sendRecorder)

	listUseCase := &fakeListMessagesUseCase{}
	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(
		http.MethodGet,
		"/conversations/23/messages",
		nil,
	)
	listRequest.SetPathValue("conversationID", "23")

	newListMessagesHandler(listUseCase).ServeHTTP(
		listRecorder,
		listRequest,
	)
	assertUnauthorized(t, listRecorder)

	if sendUseCase.called != 0 || listUseCase.called != 0 {
		t.Errorf(
			"use case calls = send %d, list %d; want 0, 0",
			sendUseCase.called,
			listUseCase.called,
		)
	}
}

func TestMessageEndpointsRejectInvalidConversationID(
	t *testing.T,
) {
	invalidIDs := []string{
		"abc",
		"0",
		"-1",
		"9223372036854775808",
	}

	for _, method := range []string{
		http.MethodPost,
		http.MethodGet,
	} {
		for _, invalidID := range invalidIDs {
			t.Run(
				method+"_"+invalidID,
				func(t *testing.T) {
					sendUseCase := &fakeSendMessageUseCase{}
					listUseCase := &fakeListMessagesUseCase{}
					recorder := httptest.NewRecorder()
					body := ""
					if method == http.MethodPost {
						body = `{"body":"hello"}`
					}

					newMessageTestHandler(
						sendUseCase,
						listUseCase,
					).ServeHTTP(
						recorder,
						newAuthenticatedMessageRequest(
							method,
							"/conversations/"+invalidID+"/messages",
							body,
						),
					)

					assertMessageError(
						t,
						recorder,
						http.StatusBadRequest,
						messagedomain.ErrConversationIDInvalid.Error(),
					)
					if sendUseCase.called != 0 ||
						listUseCase.called != 0 {
						t.Errorf(
							"use cases were called for invalid ID",
						)
					}
				},
			)
		}
	}
}

func TestListMessagesEndpointParsesBeforeID(
	t *testing.T,
) {
	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantValue  int64
	}{
		{
			name:       "valid",
			query:      "?before_id=42",
			wantStatus: http.StatusOK,
			wantValue:  42,
		},
		{
			name:       "empty",
			query:      "?before_id=",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "duplicate",
			query:      "?before_id=42&before_id=41",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "text",
			query:      "?before_id=old",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "zero",
			query:      "?before_id=0",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "negative",
			query:      "?before_id=-1",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "overflow",
			query:      "?before_id=9223372036854775808",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				useCase := &fakeListMessagesUseCase{}
				recorder := httptest.NewRecorder()

				newMessageTestHandler(
					nil,
					useCase,
				).ServeHTTP(
					recorder,
					newAuthenticatedMessageRequest(
						http.MethodGet,
						"/conversations/23/messages"+test.query,
						"",
					),
				)

				if recorder.Code != test.wantStatus {
					t.Fatalf(
						"status code = %d, want %d",
						recorder.Code,
						test.wantStatus,
					)
				}
				if test.wantStatus == http.StatusOK {
					if useCase.input.BeforeID == nil ||
						*useCase.input.BeforeID != test.wantValue {
						t.Errorf(
							"before ID = %v, want %d",
							useCase.input.BeforeID,
							test.wantValue,
						)
					}
				} else {
					assertMessageError(
						t,
						recorder,
						http.StatusBadRequest,
						listmessagesapp.ErrBeforeIDInvalid.Error(),
					)
					if useCase.called != 0 {
						t.Errorf(
							"use case calls = %d, want 0",
							useCase.called,
						)
					}
				}
			},
		)
	}
}

func TestSendMessageEndpointValidatesRequest(
	t *testing.T,
) {
	oversizedFirstJSON := `{"body":"` +
		strings.Repeat(
			"a",
			maxSendMessageRequestBytes,
		) +
		`"}`
	oversizedSecondJSON := `{"body":"hello"}` +
		strings.Repeat(
			" ",
			maxSendMessageRequestBytes,
		)
	tests := []struct {
		name        string
		contentType string
		body        string
		wantStatus  int
		wantError   string
	}{
		{
			name:       "missing content type",
			body:       `{"body":"hello"}`,
			wantStatus: http.StatusUnsupportedMediaType,
			wantError:  "Content-Type must be application/json",
		},
		{
			name:        "wrong content type",
			contentType: "text/plain",
			body:        `{"body":"hello"}`,
			wantStatus:  http.StatusUnsupportedMediaType,
			wantError:   "Content-Type must be application/json",
		},
		{
			name:        "malformed JSON",
			contentType: "application/json",
			body:        `{"body":`,
			wantStatus:  http.StatusBadRequest,
			wantError:   errInvalidJSON.Error(),
		},
		{
			name:        "unknown field",
			contentType: "application/json",
			body:        `{"body":"hello","extra":true}`,
			wantStatus:  http.StatusBadRequest,
			wantError:   errInvalidJSON.Error(),
		},
		{
			name:        "multiple values",
			contentType: "application/json",
			body:        `{"body":"hello"} {"body":"again"}`,
			wantStatus:  http.StatusBadRequest,
			wantError:   errInvalidJSON.Error(),
		},
		{
			name:        "oversized first value",
			contentType: "application/json",
			body:        oversizedFirstJSON,
			wantStatus:  http.StatusRequestEntityTooLarge,
			wantError:   "request body is too large",
		},
		{
			name:        "oversized trailing data",
			contentType: "application/json",
			body:        oversizedSecondJSON,
			wantStatus:  http.StatusRequestEntityTooLarge,
			wantError:   "request body is too large",
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				useCase := &fakeSendMessageUseCase{}
				recorder := httptest.NewRecorder()
				request := newAuthenticatedMessageRequest(
					http.MethodPost,
					"/conversations/23/messages",
					test.body,
				)
				request.Header.Set(
					"Content-Type",
					test.contentType,
				)

				newMessageTestHandler(
					useCase,
					nil,
				).ServeHTTP(recorder, request)

				assertMessageError(
					t,
					recorder,
					test.wantStatus,
					test.wantError,
				)
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

func TestSendMessageEndpointAcceptsJSONCharset(
	t *testing.T,
) {
	useCase := &fakeSendMessageUseCase{}
	recorder := httptest.NewRecorder()
	request := newAuthenticatedMessageRequest(
		http.MethodPost,
		"/conversations/23/messages",
		`{"body":"hello"}`,
	)
	request.Header.Set(
		"Content-Type",
		"application/json; charset=utf-8",
	)

	newMessageTestHandler(
		useCase,
		nil,
	).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Errorf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusCreated,
		)
	}
	if useCase.called != 1 {
		t.Errorf(
			"use case calls = %d, want 1",
			useCase.called,
		)
	}
}

func TestMessageEndpointsMapWrappedErrors(
	t *testing.T,
) {
	tests := []struct {
		name       string
		method     string
		err        error
		wantStatus int
		wantError  string
	}{
		{
			name:       "send conversation ID invalid",
			method:     http.MethodPost,
			err:        fmt.Errorf("send: %w", messagedomain.ErrConversationIDInvalid),
			wantStatus: http.StatusBadRequest,
			wantError:  messagedomain.ErrConversationIDInvalid.Error(),
		},
		{
			name:       "send identity ID invalid",
			method:     http.MethodPost,
			err:        fmt.Errorf("send: %w", messagedomain.ErrIdentityIDInvalid),
			wantStatus: http.StatusBadRequest,
			wantError:  messagedomain.ErrIdentityIDInvalid.Error(),
		},
		{
			name:       "send body invalid",
			method:     http.MethodPost,
			err:        fmt.Errorf("send: %w", messagedomain.ErrBodyInvalid),
			wantStatus: http.StatusBadRequest,
			wantError:  messagedomain.ErrBodyInvalid.Error(),
		},
		{
			name:       "list before ID invalid",
			method:     http.MethodGet,
			err:        fmt.Errorf("list: %w", listmessagesapp.ErrBeforeIDInvalid),
			wantStatus: http.StatusBadRequest,
			wantError:  listmessagesapp.ErrBeforeIDInvalid.Error(),
		},
		{
			name:       "send conversation unavailable",
			method:     http.MethodPost,
			err:        fmt.Errorf("send: %w", messagedomain.ErrConversationUnavailable),
			wantStatus: http.StatusNotFound,
			wantError:  messagedomain.ErrConversationUnavailable.Error(),
		},
		{
			name:       "list conversation unavailable",
			method:     http.MethodGet,
			err:        fmt.Errorf("list: %w", messagedomain.ErrConversationUnavailable),
			wantStatus: http.StatusNotFound,
			wantError:  messagedomain.ErrConversationUnavailable.Error(),
		},
		{
			name:       "send internal",
			method:     http.MethodPost,
			err:        errors.New("database secret"),
			wantStatus: http.StatusInternalServerError,
			wantError:  "internal server error",
		},
		{
			name:       "list internal",
			method:     http.MethodGet,
			err:        errors.New("database secret"),
			wantStatus: http.StatusInternalServerError,
			wantError:  "internal server error",
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				sendUseCase := &fakeSendMessageUseCase{}
				listUseCase := &fakeListMessagesUseCase{}
				body := ""
				if test.method == http.MethodPost {
					sendUseCase.err = test.err
					body = `{"body":"hello"}`
				} else {
					listUseCase.err = test.err
				}
				recorder := httptest.NewRecorder()

				newMessageTestHandler(
					sendUseCase,
					listUseCase,
				).ServeHTTP(
					recorder,
					newAuthenticatedMessageRequest(
						test.method,
						"/conversations/23/messages",
						body,
					),
				)

				assertMessageError(
					t,
					recorder,
					test.wantStatus,
					test.wantError,
				)
			},
		)
	}
}

func TestMessageRoutesAreRegisteredIndependently(
	t *testing.T,
) {
	t.Run(
		"send only",
		func(t *testing.T) {
			sendUseCase := &fakeSendMessageUseCase{}
			handler := NewHandlerWithDependencies(
				Dependencies{
					AccessTokenVerifier: &fakeAccessTokenVerifier{
						identityID: 7,
					},
					SendMessage: sendUseCase,
				},
			)
			sendRecorder := httptest.NewRecorder()
			handler.ServeHTTP(
				sendRecorder,
				newAuthenticatedMessageRequest(
					http.MethodPost,
					"/conversations/23/messages",
					`{"body":"hello"}`,
				),
			)
			if sendRecorder.Code != http.StatusCreated {
				t.Errorf(
					"POST status = %d, want %d",
					sendRecorder.Code,
					http.StatusCreated,
				)
			}

			getRecorder := httptest.NewRecorder()
			handler.ServeHTTP(
				getRecorder,
				newAuthenticatedMessageRequest(
					http.MethodGet,
					"/conversations/23/messages",
					"",
				),
			)
			if getRecorder.Code != http.StatusMethodNotAllowed {
				t.Errorf(
					"GET status = %d, want %d",
					getRecorder.Code,
					http.StatusMethodNotAllowed,
				)
			}
		},
	)

	t.Run(
		"list only",
		func(t *testing.T) {
			listUseCase := &fakeListMessagesUseCase{}
			handler := NewHandlerWithDependencies(
				Dependencies{
					AccessTokenVerifier: &fakeAccessTokenVerifier{
						identityID: 7,
					},
					ListMessages: listUseCase,
				},
			)
			getRecorder := httptest.NewRecorder()
			handler.ServeHTTP(
				getRecorder,
				newAuthenticatedMessageRequest(
					http.MethodGet,
					"/conversations/23/messages",
					"",
				),
			)
			if getRecorder.Code != http.StatusOK {
				t.Errorf(
					"GET status = %d, want %d",
					getRecorder.Code,
					http.StatusOK,
				)
			}

			postRecorder := httptest.NewRecorder()
			handler.ServeHTTP(
				postRecorder,
				newAuthenticatedMessageRequest(
					http.MethodPost,
					"/conversations/23/messages",
					`{"body":"hello"}`,
				),
			)
			if postRecorder.Code != http.StatusMethodNotAllowed {
				t.Errorf(
					"POST status = %d, want %d",
					postRecorder.Code,
					http.StatusMethodNotAllowed,
				)
			}
		},
	)

	t.Run(
		"verifier missing",
		func(t *testing.T) {
			handler := NewHandlerWithDependencies(
				Dependencies{
					SendMessage:  &fakeSendMessageUseCase{},
					ListMessages: &fakeListMessagesUseCase{},
				},
			)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(
				recorder,
				httptest.NewRequest(
					http.MethodGet,
					"/conversations/23/messages",
					nil,
				),
			)
			if recorder.Code != http.StatusNotFound {
				t.Errorf(
					"status = %d, want %d",
					recorder.Code,
					http.StatusNotFound,
				)
			}
		},
	)
}

func TestMessageRoutesRejectUnsupportedMethod(
	t *testing.T,
) {
	recorder := httptest.NewRecorder()

	newMessageTestHandler(
		&fakeSendMessageUseCase{},
		&fakeListMessagesUseCase{},
	).ServeHTTP(
		recorder,
		newAuthenticatedMessageRequest(
			http.MethodPut,
			"/conversations/23/messages",
			"",
		),
	)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Errorf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusMethodNotAllowed,
		)
	}
}

func newMessageTestHandler(
	sendUseCase SendMessageUseCase,
	listUseCase ListMessagesUseCase,
) http.Handler {
	return NewHandlerWithDependencies(
		Dependencies{
			AccessTokenVerifier: &fakeAccessTokenVerifier{
				identityID: 7,
			},
			SendMessage:  sendUseCase,
			ListMessages: listUseCase,
		},
	)
}

func newAuthenticatedMessageRequest(
	method string,
	target string,
	body string,
) *http.Request {
	request := httptest.NewRequest(
		method,
		target,
		strings.NewReader(body),
	)
	request.Header.Set(
		"Authorization",
		"Bearer valid-message-token",
	)
	if method == http.MethodPost {
		request.Header.Set(
			"Content-Type",
			"application/json",
		)
	}

	return request
}

func decodeMessageTestResponse(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	destination any,
) {
	t.Helper()

	if err := json.Unmarshal(
		recorder.Body.Bytes(),
		destination,
	); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func assertMessageError(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	wantStatus int,
	wantError string,
) {
	t.Helper()

	if recorder.Code != wantStatus {
		t.Fatalf(
			"status code = %d, want %d; body = %s",
			recorder.Code,
			wantStatus,
			recorder.Body.String(),
		)
	}

	var response errorResponse
	decodeMessageTestResponse(
		t,
		recorder,
		&response,
	)
	if response.Error != wantError {
		t.Errorf(
			"error = %q, want %q",
			response.Error,
			wantError,
		)
	}
}
