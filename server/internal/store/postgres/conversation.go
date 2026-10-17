package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/imamsyafii-dev/mini-chat/server/internal/domain/conversation"
)

const (
	findDirectConversationQuery = `
SELECT
    c.id,
    c.kind,
    c.created_by,
    c.created_at,
    c.updated_at,
    participant.id,
    participant.username,
    participant.display_name,
    participant.created_at,
    participant.updated_at
FROM direct_conversation AS direct
JOIN conversation AS c
    ON c.id = direct.conversation_id
JOIN conversation_member AS actor_member
    ON actor_member.conversation_id = c.id
   AND actor_member.identity_id = $3
JOIN conversation_member AS participant_member
    ON participant_member.conversation_id = c.id
   AND participant_member.identity_id <> $3
JOIN identity AS participant
    ON participant.id = participant_member.identity_id
WHERE direct.identity_low_id = $1
  AND direct.identity_high_id = $2
  AND c.kind = 'direct'
`
	createConversationQuery = `
INSERT INTO conversation (
    kind,
    created_by
)
VALUES ($1, $2)
RETURNING id
`
	createDirectConversationPairQuery = `
INSERT INTO direct_conversation (
    conversation_id,
    identity_low_id,
    identity_high_id
)
VALUES ($1, $2, $3)
`
	createDirectConversationMembersQuery = `
INSERT INTO conversation_member (
    conversation_id,
    identity_id,
    role
)
VALUES
    ($1, $2, 'owner'),
    ($1, $3, 'member')
`
	listDirectConversationsQuery = `
SELECT
    c.id,
    c.kind,
    c.created_by,
    c.created_at,
    c.updated_at,
    participant.id,
    participant.username,
    participant.display_name,
    participant.created_at,
    participant.updated_at
FROM direct_conversation AS direct
JOIN conversation AS c
    ON c.id = direct.conversation_id
JOIN conversation_member AS actor_member
    ON actor_member.conversation_id = c.id
   AND actor_member.identity_id = $1
JOIN identity AS participant
    ON participant.id = CASE
        WHEN direct.identity_low_id = $1
            THEN direct.identity_high_id
        ELSE direct.identity_low_id
    END
JOIN conversation_member AS participant_member
    ON participant_member.conversation_id = c.id
   AND participant_member.identity_id = participant.id
WHERE c.kind = 'direct'
  AND (
      direct.identity_low_id = $1
      OR direct.identity_high_id = $1
  )
ORDER BY c.updated_at DESC, c.id DESC
LIMIT $2
`
	directConversationPairUniqueConstraint = "direct_conversation_identity_pair_unique"
)

type conversationTransaction interface {
	QueryRow(
		ctx context.Context,
		sql string,
		args ...any,
	) pgx.Row
	Exec(
		ctx context.Context,
		sql string,
		args ...any,
	) (pgconn.CommandTag, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type beginTransaction func(
	ctx context.Context,
) (conversationTransaction, error)

type conversationRows interface {
	Next() bool
	Scan(destinations ...any) error
	Err() error
	Close()
}

type queryConversationRows func(
	ctx context.Context,
	query string,
	args ...any,
) (conversationRows, error)

type ConversationStore struct {
	db        Querier
	begin     beginTransaction
	queryRows queryConversationRows
}

func NewConversationStore(
	pool *pgxpool.Pool,
) *ConversationStore {
	return newConversationStore(
		pool,
		func(
			ctx context.Context,
		) (conversationTransaction, error) {
			return pool.Begin(ctx)
		},
		func(
			ctx context.Context,
			query string,
			args ...any,
		) (conversationRows, error) {
			return pool.Query(ctx, query, args...)
		},
	)
}

func newConversationStore(
	db Querier,
	begin beginTransaction,
	queryRows queryConversationRows,
) *ConversationStore {
	return &ConversationStore{
		db:        db,
		begin:     begin,
		queryRows: queryRows,
	}
}

func (
	store *ConversationStore,
) ListDirectConversations(
	ctx context.Context,
	actorID int64,
	limit int,
) ([]conversation.DirectConversation, error) {
	rows, err := store.queryRows(
		ctx,
		listDirectConversationsQuery,
		actorID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"query direct conversations: %w",
			err,
		)
	}
	defer rows.Close()

	results := make(
		[]conversation.DirectConversation,
		0,
	)
	for rows.Next() {
		var result conversation.DirectConversation

		err := rows.Scan(
			&result.ID,
			&result.Kind,
			&result.CreatedBy,
			&result.CreatedAt,
			&result.UpdatedAt,
			&result.Participant.ID,
			&result.Participant.Username,
			&result.Participant.DisplayName,
			&result.Participant.CreatedAt,
			&result.Participant.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"scan direct conversation: %w",
				err,
			)
		}

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate direct conversations: %w",
			err,
		)
	}

	return results, nil
}

func (
	store *ConversationStore,
) CreateOrGetDirectConversation(
	ctx context.Context,
	actorID int64,
	pair conversation.DirectPair,
) (conversation.DirectConversation, bool, error) {
	if actorID != pair.IdentityLowID &&
		actorID != pair.IdentityHighID {
		return conversation.DirectConversation{}, false,
			conversation.ErrParticipantIDInvalid
	}

	transaction, err := store.begin(ctx)
	if err != nil {
		return conversation.DirectConversation{}, false,
			fmt.Errorf(
				"begin direct conversation transaction: %w",
				err,
			)
	}

	existing, err := findDirectConversation(
		ctx,
		transaction,
		pair,
		actorID,
	)
	if err == nil {
		if commitErr := transaction.Commit(ctx); commitErr != nil {
			return conversation.DirectConversation{}, false,
				fmt.Errorf(
					"commit existing direct conversation transaction: %w",
					commitErr,
				)
		}

		return existing, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return conversation.DirectConversation{}, false,
			rollbackConversationTransaction(
				ctx,
				transaction,
				fmt.Errorf(
					"find existing direct conversation: %w",
					err,
				),
			)
	}

	var conversationID int64
	err = transaction.QueryRow(
		ctx,
		createConversationQuery,
		conversation.KindDirect,
		actorID,
	).Scan(&conversationID)
	if err != nil {
		return conversation.DirectConversation{}, false,
			rollbackConversationTransaction(
				ctx,
				transaction,
				fmt.Errorf(
					"insert direct conversation: %w",
					err,
				),
			)
	}

	_, err = transaction.Exec(
		ctx,
		createDirectConversationPairQuery,
		conversationID,
		pair.IdentityLowID,
		pair.IdentityHighID,
	)
	if err != nil {
		pairInsertErr := fmt.Errorf(
			"insert direct conversation pair: %w",
			err,
		)

		if isDirectConversationPairConflict(err) {
			rollbackErr := transaction.Rollback(ctx)
			if rollbackErr != nil {
				return conversation.DirectConversation{}, false,
					errors.Join(
						pairInsertErr,
						fmt.Errorf(
							"rollback direct conversation transaction: %w",
							rollbackErr,
						),
					)
			}

			winner, findErr := findDirectConversation(
				ctx,
				store.db,
				pair,
				actorID,
			)
			if findErr != nil {
				return conversation.DirectConversation{}, false,
					fmt.Errorf(
						"find winning direct conversation: %w",
						findErr,
					)
			}

			return winner, false, nil
		}

		return conversation.DirectConversation{}, false,
			rollbackConversationTransaction(
				ctx,
				transaction,
				pairInsertErr,
			)
	}

	participantID := pair.IdentityLowID
	if participantID == actorID {
		participantID = pair.IdentityHighID
	}

	_, err = transaction.Exec(
		ctx,
		createDirectConversationMembersQuery,
		conversationID,
		actorID,
		participantID,
	)
	if err != nil {
		return conversation.DirectConversation{}, false,
			rollbackConversationTransaction(
				ctx,
				transaction,
				fmt.Errorf(
					"insert direct conversation members: %w",
					err,
				),
			)
	}

	created, err := findDirectConversation(
		ctx,
		transaction,
		pair,
		actorID,
	)
	if err != nil {
		return conversation.DirectConversation{}, false,
			rollbackConversationTransaction(
				ctx,
				transaction,
				fmt.Errorf(
					"find created direct conversation: %w",
					err,
				),
			)
	}

	if err := transaction.Commit(ctx); err != nil {
		return conversation.DirectConversation{}, false,
			fmt.Errorf(
				"commit new direct conversation transaction: %w",
				err,
			)
	}

	return created, true, nil
}

func findDirectConversation(
	ctx context.Context,
	db Querier,
	pair conversation.DirectPair,
	actorID int64,
) (conversation.DirectConversation, error) {
	var result conversation.DirectConversation

	err := db.QueryRow(
		ctx,
		findDirectConversationQuery,
		pair.IdentityLowID,
		pair.IdentityHighID,
		actorID,
	).Scan(
		&result.ID,
		&result.Kind,
		&result.CreatedBy,
		&result.CreatedAt,
		&result.UpdatedAt,
		&result.Participant.ID,
		&result.Participant.Username,
		&result.Participant.DisplayName,
		&result.Participant.CreatedAt,
		&result.Participant.UpdatedAt,
	)
	if err != nil {
		return conversation.DirectConversation{}, err
	}

	return result, nil
}

func rollbackConversationTransaction(
	ctx context.Context,
	transaction conversationTransaction,
	cause error,
) error {
	rollbackErr := transaction.Rollback(ctx)
	if rollbackErr == nil {
		return cause
	}

	return errors.Join(
		cause,
		fmt.Errorf(
			"rollback direct conversation transaction: %w",
			rollbackErr,
		),
	)
}

func isDirectConversationPairConflict(
	err error,
) bool {
	var postgresError *pgconn.PgError

	return errors.As(err, &postgresError) &&
		postgresError.Code == uniqueViolationCode &&
		postgresError.ConstraintName ==
			directConversationPairUniqueConstraint
}
