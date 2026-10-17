package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/imamsyafii-dev/mini-chat/server/internal/domain/message"
)

const (
	authorizeMessageSenderQuery = `
SELECT 1
FROM conversation AS c
JOIN conversation_member AS member
    ON member.conversation_id = c.id
WHERE c.id = $1
  AND member.identity_id = $2
FOR UPDATE OF c, member
`
	createMessageQuery = `
INSERT INTO message (
    conversation_id,
    sender_id,
    body
)
VALUES ($1, $2, $3)
RETURNING
    id,
    conversation_id,
    sender_id,
    body,
    created_at
`
	updateConversationMessageTimeQuery = `
UPDATE conversation
SET updated_at = GREATEST(updated_at, $2)
WHERE id = $1
`
	authorizeMessageReaderQuery = `
SELECT 1
FROM conversation AS c
JOIN conversation_member AS member
    ON member.conversation_id = c.id
WHERE c.id = $1
  AND member.identity_id = $2
`
	listMessagesQuery = `
SELECT
    message.id,
    message.conversation_id,
    message.sender_id,
    message.body,
    message.created_at
FROM message
JOIN conversation_member AS member
    ON member.conversation_id = message.conversation_id
   AND member.identity_id = $2
WHERE message.conversation_id = $1
  AND ($3::BIGINT IS NULL OR message.id < $3)
ORDER BY message.id DESC
LIMIT $4
`
)

type messageTransaction interface {
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

type beginMessageTransaction func(
	ctx context.Context,
) (messageTransaction, error)

type messageRows interface {
	Next() bool
	Scan(destinations ...any) error
	Err() error
	Close()
}

type queryMessageRows func(
	ctx context.Context,
	query string,
	args ...any,
) (messageRows, error)

type MessageStore struct {
	db        Querier
	begin     beginMessageTransaction
	queryRows queryMessageRows
}

func NewMessageStore(
	pool *pgxpool.Pool,
) *MessageStore {
	return newMessageStore(
		pool,
		func(
			ctx context.Context,
		) (messageTransaction, error) {
			return pool.Begin(ctx)
		},
		func(
			ctx context.Context,
			query string,
			args ...any,
		) (messageRows, error) {
			return pool.Query(ctx, query, args...)
		},
	)
}

func newMessageStore(
	db Querier,
	begin beginMessageTransaction,
	queryRows queryMessageRows,
) *MessageStore {
	return &MessageStore{
		db:        db,
		begin:     begin,
		queryRows: queryRows,
	}
}

func (
	store *MessageStore,
) CreateMessage(
	ctx context.Context,
	draft message.Draft,
) (message.Message, error) {
	transaction, err := store.begin(ctx)
	if err != nil {
		return message.Message{}, fmt.Errorf(
			"begin message transaction: %w",
			err,
		)
	}

	var authorizationMarker int
	err = transaction.QueryRow(
		ctx,
		authorizeMessageSenderQuery,
		draft.ConversationID,
		draft.SenderID,
	).Scan(&authorizationMarker)
	if errors.Is(err, pgx.ErrNoRows) {
		return message.Message{}, rollbackMessageTransaction(
			ctx,
			transaction,
			fmt.Errorf(
				"authorize message sender: %w",
				message.ErrConversationUnavailable,
			),
		)
	}
	if err != nil {
		return message.Message{}, rollbackMessageTransaction(
			ctx,
			transaction,
			fmt.Errorf(
				"authorize message sender: %w",
				err,
			),
		)
	}

	var result message.Message
	err = transaction.QueryRow(
		ctx,
		createMessageQuery,
		draft.ConversationID,
		draft.SenderID,
		draft.Body,
	).Scan(
		&result.ID,
		&result.ConversationID,
		&result.SenderID,
		&result.Body,
		&result.CreatedAt,
	)
	if err != nil {
		return message.Message{}, rollbackMessageTransaction(
			ctx,
			transaction,
			fmt.Errorf(
				"insert message: %w",
				err,
			),
		)
	}

	_, err = transaction.Exec(
		ctx,
		updateConversationMessageTimeQuery,
		result.ConversationID,
		result.CreatedAt,
	)
	if err != nil {
		return message.Message{}, rollbackMessageTransaction(
			ctx,
			transaction,
			fmt.Errorf(
				"update conversation message time: %w",
				err,
			),
		)
	}

	if err := transaction.Commit(ctx); err != nil {
		return message.Message{}, fmt.Errorf(
			"commit message transaction: %w",
			err,
		)
	}

	return result, nil
}

func (
	store *MessageStore,
) ListMessages(
	ctx context.Context,
	actorID int64,
	conversationID int64,
	beforeID *int64,
	limit int,
) ([]message.Message, error) {
	var authorizationMarker int
	err := store.db.QueryRow(
		ctx,
		authorizeMessageReaderQuery,
		conversationID,
		actorID,
	).Scan(&authorizationMarker)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf(
			"authorize message reader: %w",
			message.ErrConversationUnavailable,
		)
	}
	if err != nil {
		return nil, fmt.Errorf(
			"authorize message reader: %w",
			err,
		)
	}

	var cursor any
	if beforeID != nil {
		cursor = *beforeID
	}

	rows, err := store.queryRows(
		ctx,
		listMessagesQuery,
		conversationID,
		actorID,
		cursor,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"query messages: %w",
			err,
		)
	}
	defer rows.Close()

	results := make([]message.Message, 0)
	for rows.Next() {
		var result message.Message
		if err := rows.Scan(
			&result.ID,
			&result.ConversationID,
			&result.SenderID,
			&result.Body,
			&result.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf(
				"scan message: %w",
				err,
			)
		}

		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate messages: %w",
			err,
		)
	}

	return results, nil
}

func rollbackMessageTransaction(
	ctx context.Context,
	transaction messageTransaction,
	cause error,
) error {
	rollbackErr := transaction.Rollback(ctx)
	if rollbackErr == nil {
		return cause
	}

	return errors.Join(
		cause,
		fmt.Errorf(
			"rollback message transaction: %w",
			rollbackErr,
		),
	)
}
