package conversation

import (
	"errors"
	"time"

	"github.com/imamsyafii-dev/mini-chat/server/internal/domain/identity"
)

type Kind string

const KindDirect Kind = "direct"

var (
	ErrParticipantIDInvalid = errors.New(
		"participant identity ID is invalid",
	)
	ErrSelfConversation = errors.New(
		"direct conversation with self is not allowed",
	)
)

type DirectPair struct {
	IdentityLowID  int64
	IdentityHighID int64
}

type DirectConversation struct {
	ID          int64
	Kind        Kind
	CreatedBy   int64
	Participant identity.Identity
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewDirectPair(
	firstID int64,
	secondID int64,
) (DirectPair, error) {
	if firstID <= 0 || secondID <= 0 {
		return DirectPair{},
			ErrParticipantIDInvalid
	}

	if firstID == secondID {
		return DirectPair{},
			ErrSelfConversation
	}

	if firstID > secondID {
		firstID, secondID = secondID, firstID
	}

	return DirectPair{
		IdentityLowID:  firstID,
		IdentityHighID: secondID,
	}, nil
}
