package message

import (
	"errors"
	"strings"
	"testing"
)

func TestNewDraftNormalizesBodyAndPreservesInternalWhitespace(
	t *testing.T,
) {
	draft, err := NewDraft(
		17,
		9,
		"  Hello, Bob!\n\nHow are  you?  \n",
	)
	if err != nil {
		t.Fatalf(
			"NewDraft() error = %v, want nil",
			err,
		)
	}

	want := Draft{
		ConversationID: 17,
		SenderID:       9,
		Body:           "Hello, Bob!\n\nHow are  you?",
	}

	if draft != want {
		t.Errorf(
			"NewDraft() = %+v, want %+v",
			draft,
			want,
		)
	}
}

func TestNewDraftRejectsInvalidConversationID(
	t *testing.T,
) {
	tests := []struct {
		name           string
		conversationID int64
	}{
		{
			name:           "zero",
			conversationID: 0,
		},
		{
			name:           "negative",
			conversationID: -1,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				_, err := NewDraft(
					test.conversationID,
					9,
					"hello",
				)

				if !errors.Is(
					err,
					ErrConversationIDInvalid,
				) {
					t.Fatalf(
						"error = %v, want %v",
						err,
						ErrConversationIDInvalid,
					)
				}
			},
		)
	}
}

func TestNewDraftRejectsInvalidSenderID(
	t *testing.T,
) {
	tests := []struct {
		name     string
		senderID int64
	}{
		{
			name:     "zero",
			senderID: 0,
		},
		{
			name:     "negative",
			senderID: -1,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				_, err := NewDraft(
					17,
					test.senderID,
					"hello",
				)

				if !errors.Is(
					err,
					ErrIdentityIDInvalid,
				) {
					t.Fatalf(
						"error = %v, want %v",
						err,
						ErrIdentityIDInvalid,
					)
				}
			},
		)
	}
}

func TestNewDraftRejectsEmptyBody(
	t *testing.T,
) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "empty",
			body: "",
		},
		{
			name: "whitespace only",
			body: " \t\n ",
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				_, err := NewDraft(
					17,
					9,
					test.body,
				)

				if !errors.Is(
					err,
					ErrBodyInvalid,
				) {
					t.Fatalf(
						"error = %v, want %v",
						err,
						ErrBodyInvalid,
					)
				}
			},
		)
	}
}

func TestNewDraftAcceptsExactlyMaximumUnicodeRunes(
	t *testing.T,
) {
	body := strings.Repeat(
		"界",
		MaxBodyLength,
	)

	draft, err := NewDraft(
		17,
		9,
		body,
	)
	if err != nil {
		t.Fatalf(
			"NewDraft() error = %v, want nil",
			err,
		)
	}

	if draft.Body != body {
		t.Errorf(
			"Body length = %d bytes, want accepted unchanged",
			len(draft.Body),
		)
	}
}

func TestNewDraftRejectsBodyOverMaximumUnicodeRunes(
	t *testing.T,
) {
	body := strings.Repeat(
		"界",
		MaxBodyLength+1,
	)

	_, err := NewDraft(
		17,
		9,
		body,
	)

	if !errors.Is(
		err,
		ErrBodyInvalid,
	) {
		t.Fatalf(
			"error = %v, want %v",
			err,
			ErrBodyInvalid,
		)
	}
}

func TestNewDraftValidatesConversationBeforeSenderAndBody(
	t *testing.T,
) {
	_, err := NewDraft(
		0,
		0,
		"",
	)

	if !errors.Is(
		err,
		ErrConversationIDInvalid,
	) {
		t.Fatalf(
			"error = %v, want %v",
			err,
			ErrConversationIDInvalid,
		)
	}
}

func TestNewDraftValidatesSenderBeforeBody(
	t *testing.T,
) {
	_, err := NewDraft(
		17,
		0,
		"",
	)

	if !errors.Is(
		err,
		ErrIdentityIDInvalid,
	) {
		t.Fatalf(
			"error = %v, want %v",
			err,
			ErrIdentityIDInvalid,
		)
	}
}
