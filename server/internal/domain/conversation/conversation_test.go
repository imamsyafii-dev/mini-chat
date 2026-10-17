package conversation

import (
	"errors"
	"testing"
	"time"

	"github.com/imamsyafii-dev/mini-chat/server/internal/domain/identity"
)

func TestNewDirectPairCanonicalizesParticipantOrder(
	t *testing.T,
) {
	fromDescending, err := NewDirectPair(
		8,
		3,
	)
	if err != nil {
		t.Fatalf(
			"NewDirectPair(8, 3) error = %v, want nil",
			err,
		)
	}

	fromAscending, err := NewDirectPair(
		3,
		8,
	)
	if err != nil {
		t.Fatalf(
			"NewDirectPair(3, 8) error = %v, want nil",
			err,
		)
	}

	want := DirectPair{
		IdentityLowID:  3,
		IdentityHighID: 8,
	}

	if fromDescending != want {
		t.Errorf(
			"NewDirectPair(8, 3) = %+v, want %+v",
			fromDescending,
			want,
		)
	}

	if fromAscending != want {
		t.Errorf(
			"NewDirectPair(3, 8) = %+v, want %+v",
			fromAscending,
			want,
		)
	}

	if fromDescending != fromAscending {
		t.Errorf(
			"pairs differ: descending = %+v, ascending = %+v",
			fromDescending,
			fromAscending,
		)
	}
}

func TestNewDirectPairRejectsInvalidParticipantID(
	t *testing.T,
) {
	tests := []struct {
		name     string
		firstID  int64
		secondID int64
	}{
		{
			name:     "first ID is zero",
			firstID:  0,
			secondID: 3,
		},
		{
			name:     "first ID is negative",
			firstID:  -1,
			secondID: 3,
		},
		{
			name:     "second ID is zero",
			firstID:  3,
			secondID: 0,
		},
		{
			name:     "second ID is negative",
			firstID:  3,
			secondID: -1,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				_, err := NewDirectPair(
					test.firstID,
					test.secondID,
				)

				if !errors.Is(
					err,
					ErrParticipantIDInvalid,
				) {
					t.Fatalf(
						"error = %v, want %v",
						err,
						ErrParticipantIDInvalid,
					)
				}
			},
		)
	}
}

func TestNewDirectPairRejectsSelfConversation(
	t *testing.T,
) {
	_, err := NewDirectPair(
		3,
		3,
	)

	if !errors.Is(
		err,
		ErrSelfConversation,
	) {
		t.Fatalf(
			"error = %v, want %v",
			err,
			ErrSelfConversation,
		)
	}
}

func TestKindDirectValue(
	t *testing.T,
) {
	if KindDirect != Kind("direct") {
		t.Errorf(
			"KindDirect = %q, want %q",
			KindDirect,
			"direct",
		)
	}
}

func TestDirectConversationContainsPublicParticipantIdentity(
	t *testing.T,
) {
	createdAt := time.Date(
		2026,
		time.October,
		16,
		10,
		0,
		0,
		0,
		time.UTC,
	)
	updatedAt := createdAt.Add(time.Minute)

	result := DirectConversation{
		ID:        11,
		Kind:      KindDirect,
		CreatedBy: 8,
		Participant: identity.Identity{
			ID:          3,
			Username:    "bob",
			DisplayName: "Bob Example",
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
		},
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}

	if result.Participant.ID != 3 ||
		result.Participant.Username != "bob" ||
		result.Participant.DisplayName != "Bob Example" {
		t.Errorf(
			"Participant = %+v, want public identity for bob",
			result.Participant,
		)
	}
}
