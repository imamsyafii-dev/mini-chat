package message

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxBodyLength = 4000

var (
	ErrConversationIDInvalid = errors.New(
		"conversation ID is invalid",
	)
	ErrIdentityIDInvalid = errors.New(
		"identity ID is invalid",
	)
	ErrBodyInvalid = errors.New(
		"message body is invalid",
	)
	ErrConversationUnavailable = errors.New(
		"conversation is unavailable",
	)
)

type Message struct {
	ID             int64
	ConversationID int64
	SenderID       int64
	Body           string
	CreatedAt      time.Time
}

type Draft struct {
	ConversationID int64
	SenderID       int64
	Body           string
}

func NewDraft(
	conversationID int64,
	senderID int64,
	body string,
) (Draft, error) {
	if conversationID <= 0 {
		return Draft{},
			ErrConversationIDInvalid
	}

	if senderID <= 0 {
		return Draft{},
			ErrIdentityIDInvalid
	}

	body = strings.TrimSpace(body)
	bodyLength := utf8.RuneCountInString(body)
	if bodyLength == 0 ||
		bodyLength > MaxBodyLength {
		return Draft{},
			ErrBodyInvalid
	}

	return Draft{
		ConversationID: conversationID,
		SenderID:       senderID,
		Body:           body,
	}, nil
}
