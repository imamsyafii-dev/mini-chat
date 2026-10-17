package postgres

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/imamsyafii-dev/mini-chat/server/internal/domain/message"
)

type messageRow struct {
	scan func(destinations ...any) error
}

func (
	row messageRow,
) Scan(
	destinations ...any,
) error {
	return row.scan(destinations...)
}

type messageQueryCall struct {
	query string
	args  []any
}

type messageExecCall struct {
	query string
	args  []any
}

type fakeMessageQuerier struct {
	queryRows  []pgx.Row
	queryCalls []messageQueryCall
	contexts   []context.Context
}

func (
	fake *fakeMessageQuerier,
) QueryRow(
	ctx context.Context,
	query string,
	args ...any,
) pgx.Row {
	fake.contexts = append(fake.contexts, ctx)
	fake.queryCalls = append(
		fake.queryCalls,
		messageQueryCall{
			query: query,
			args:  args,
		},
	)
	if len(fake.queryRows) == 0 {
		panic("unexpected root QueryRow call")
	}

	row := fake.queryRows[0]
	fake.queryRows = fake.queryRows[1:]

	return row
}

var _ Querier = (*fakeMessageQuerier)(nil)

type fakeMessageRows struct {
	values       []message.Message
	nextIndex    int
	scanErr      error
	iterationErr error
	closes       int
}

func (fake *fakeMessageRows) Next() bool {
	if fake.nextIndex >= len(fake.values) {
		return false
	}

	fake.nextIndex++
	return true
}

func (
	fake *fakeMessageRows,
) Scan(
	destinations ...any,
) error {
	if fake.scanErr != nil {
		return fake.scanErr
	}

	value := fake.values[fake.nextIndex-1]
	*destinations[0].(*int64) = value.ID
	*destinations[1].(*int64) = value.ConversationID
	*destinations[2].(*int64) = value.SenderID
	*destinations[3].(*string) = value.Body
	*destinations[4].(*time.Time) = value.CreatedAt

	return nil
}

func (fake *fakeMessageRows) Err() error {
	return fake.iterationErr
}

func (fake *fakeMessageRows) Close() {
	fake.closes++
}

var _ messageRows = (*fakeMessageRows)(nil)

type fakeMessageTransaction struct {
	queryRows   []pgx.Row
	execErrors  []error
	queryCalls  []messageQueryCall
	execCalls   []messageExecCall
	commitErr   error
	rollbackErr error
	commits     int
	rollbacks   int
	events      []string
}

func (
	fake *fakeMessageTransaction,
) QueryRow(
	_ context.Context,
	query string,
	args ...any,
) pgx.Row {
	fake.queryCalls = append(
		fake.queryCalls,
		messageQueryCall{
			query: query,
			args:  args,
		},
	)
	fake.events = append(fake.events, "query")

	row := fake.queryRows[0]
	fake.queryRows = fake.queryRows[1:]

	return row
}

func (
	fake *fakeMessageTransaction,
) Exec(
	_ context.Context,
	query string,
	args ...any,
) (pgconn.CommandTag, error) {
	fake.execCalls = append(
		fake.execCalls,
		messageExecCall{
			query: query,
			args:  args,
		},
	)
	fake.events = append(fake.events, "exec")

	var err error
	if len(fake.execErrors) > 0 {
		err = fake.execErrors[0]
		fake.execErrors = fake.execErrors[1:]
	}

	return pgconn.NewCommandTag("UPDATE 1"), err
}

func (
	fake *fakeMessageTransaction,
) Commit(
	context.Context,
) error {
	fake.commits++
	fake.events = append(fake.events, "commit")

	return fake.commitErr
}

func (
	fake *fakeMessageTransaction,
) Rollback(
	context.Context,
) error {
	fake.rollbacks++
	fake.events = append(fake.events, "rollback")

	return fake.rollbackErr
}

var _ messageTransaction = (*fakeMessageTransaction)(nil)

func TestMessageStoreCreatesMessageInTransaction(
	t *testing.T,
) {
	createdAt := time.Date(
		2026,
		time.October,
		17,
		9,
		30,
		0,
		0,
		time.UTC,
	)
	draft := message.Draft{
		ConversationID: 31,
		SenderID:       7,
		Body:           "hello Bob",
	}
	want := message.Message{
		ID:             101,
		ConversationID: draft.ConversationID,
		SenderID:       draft.SenderID,
		Body:           draft.Body,
		CreatedAt:      createdAt,
	}
	transaction := &fakeMessageTransaction{
		queryRows: []pgx.Row{
			messageAuthorizationRow(),
			storedMessageRow(want),
		},
	}
	beginCalls := 0
	store := newMessageStore(
		&fakeMessageQuerier{},
		func(
			context.Context,
		) (messageTransaction, error) {
			beginCalls++
			return transaction, nil
		},
		nil,
	)

	got, err := store.CreateMessage(
		context.Background(),
		draft,
	)
	if err != nil {
		t.Fatalf(
			"CreateMessage() error = %v, want nil",
			err,
		)
	}
	if got != want {
		t.Errorf(
			"CreateMessage() = %#v, want %#v",
			got,
			want,
		)
	}
	if beginCalls != 1 ||
		transaction.commits != 1 ||
		transaction.rollbacks != 0 {
		t.Errorf(
			"begin/commit/rollback = %d/%d/%d, want 1/1/0",
			beginCalls,
			transaction.commits,
			transaction.rollbacks,
		)
	}
	if !reflect.DeepEqual(
		transaction.events,
		[]string{"query", "query", "exec", "commit"},
	) {
		t.Errorf(
			"events = %#v, want authorize, insert, update, commit order",
			transaction.events,
		)
	}

	assertMessageAuthorizationQuery(
		t,
		transaction.queryCalls[0],
		draft,
	)
	assertMessageInsertQuery(
		t,
		transaction.queryCalls[1],
		draft,
	)
	assertConversationMessageTimeUpdate(
		t,
		transaction.execCalls[0],
		want,
	)
}

func TestMessageStoreHidesUnavailableConversation(
	t *testing.T,
) {
	transaction := &fakeMessageTransaction{
		queryRows: []pgx.Row{
			errorMessageRow(pgx.ErrNoRows),
		},
	}
	store := messageStoreForTransaction(transaction)

	_, err := store.CreateMessage(
		context.Background(),
		message.Draft{
			ConversationID: 31,
			SenderID:       9,
			Body:           "not allowed",
		},
	)
	if !errors.Is(
		err,
		message.ErrConversationUnavailable,
	) {
		t.Fatalf(
			"error = %v, want wrapped ErrConversationUnavailable",
			err,
		)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"error = %v, must not expose pgx.ErrNoRows",
			err,
		)
	}
	if len(transaction.queryCalls) != 1 ||
		len(transaction.execCalls) != 0 ||
		transaction.rollbacks != 1 ||
		transaction.commits != 0 {
		t.Errorf(
			"queries/execs/rollback/commit = %d/%d/%d/%d, want 1/0/1/0",
			len(transaction.queryCalls),
			len(transaction.execCalls),
			transaction.rollbacks,
			transaction.commits,
		)
	}
}

func TestMessageStoreReturnsAuthorizationDatabaseFailure(
	t *testing.T,
) {
	authorizationErr := errors.New(
		"authorization query failed",
	)
	transaction := &fakeMessageTransaction{
		queryRows: []pgx.Row{
			errorMessageRow(authorizationErr),
		},
	}
	store := messageStoreForTransaction(transaction)

	_, err := store.CreateMessage(
		context.Background(),
		message.Draft{},
	)
	if !errors.Is(err, authorizationErr) {
		t.Fatalf(
			"error = %v, want wrapped %v",
			err,
			authorizationErr,
		)
	}
	if transaction.rollbacks != 1 ||
		transaction.commits != 0 {
		t.Errorf(
			"rollback/commit = %d/%d, want 1/0",
			transaction.rollbacks,
			transaction.commits,
		)
	}
}

func TestMessageStoreRollsBackInsertFailure(
	t *testing.T,
) {
	insertErr := errors.New("insert failed")
	transaction := &fakeMessageTransaction{
		queryRows: []pgx.Row{
			messageAuthorizationRow(),
			errorMessageRow(insertErr),
		},
	}
	store := messageStoreForTransaction(transaction)

	_, err := store.CreateMessage(
		context.Background(),
		message.Draft{},
	)
	if !errors.Is(err, insertErr) {
		t.Fatalf(
			"error = %v, want wrapped %v",
			err,
			insertErr,
		)
	}
	assertMessageRolledBackOnce(t, transaction)
	if len(transaction.execCalls) != 0 {
		t.Errorf(
			"exec call count = %d, want 0",
			len(transaction.execCalls),
		)
	}
}

func TestMessageStoreRollsBackConversationUpdateFailure(
	t *testing.T,
) {
	updateErr := errors.New("update failed")
	transaction := &fakeMessageTransaction{
		queryRows: []pgx.Row{
			messageAuthorizationRow(),
			storedMessageRow(message.Message{}),
		},
		execErrors: []error{updateErr},
	}
	store := messageStoreForTransaction(transaction)

	_, err := store.CreateMessage(
		context.Background(),
		message.Draft{},
	)
	if !errors.Is(err, updateErr) {
		t.Fatalf(
			"error = %v, want wrapped %v",
			err,
			updateErr,
		)
	}
	assertMessageRolledBackOnce(t, transaction)
}

func TestMessageStoreReturnsBeginFailure(
	t *testing.T,
) {
	beginErr := errors.New("begin failed")
	store := newMessageStore(
		&fakeMessageQuerier{},
		func(
			context.Context,
		) (messageTransaction, error) {
			return nil, beginErr
		},
		nil,
	)

	_, err := store.CreateMessage(
		context.Background(),
		message.Draft{},
	)
	if !errors.Is(err, beginErr) {
		t.Fatalf(
			"error = %v, want wrapped %v",
			err,
			beginErr,
		)
	}
}

func TestMessageStoreJoinsRollbackFailure(
	t *testing.T,
) {
	operationErr := errors.New("authorization failed")
	rollbackErr := errors.New("rollback failed")
	transaction := &fakeMessageTransaction{
		queryRows: []pgx.Row{
			errorMessageRow(operationErr),
		},
		rollbackErr: rollbackErr,
	}
	store := messageStoreForTransaction(transaction)

	_, err := store.CreateMessage(
		context.Background(),
		message.Draft{},
	)
	if !errors.Is(err, operationErr) ||
		!errors.Is(err, rollbackErr) {
		t.Fatalf(
			"error = %v, want joined operation and rollback errors",
			err,
		)
	}
	if transaction.rollbacks != 1 {
		t.Errorf(
			"rollbacks = %d, want 1",
			transaction.rollbacks,
		)
	}
}

func TestMessageStoreDoesNotRollbackCommitFailure(
	t *testing.T,
) {
	commitErr := errors.New("commit failed")
	transaction := &fakeMessageTransaction{
		queryRows: []pgx.Row{
			messageAuthorizationRow(),
			storedMessageRow(message.Message{}),
		},
		commitErr: commitErr,
	}
	store := messageStoreForTransaction(transaction)

	_, err := store.CreateMessage(
		context.Background(),
		message.Draft{},
	)
	if !errors.Is(err, commitErr) {
		t.Fatalf(
			"error = %v, want wrapped %v",
			err,
			commitErr,
		)
	}
	if transaction.commits != 1 ||
		transaction.rollbacks != 0 {
		t.Errorf(
			"commit/rollback = %d/%d, want 1/0",
			transaction.commits,
			transaction.rollbacks,
		)
	}
}

func TestMessageStoreListsAuthorizedMessagesWithoutCursor(
	t *testing.T,
) {
	createdAt := time.Date(
		2026,
		time.October,
		17,
		17,
		0,
		0,
		0,
		time.UTC,
	)
	want := []message.Message{
		{
			ID:             12,
			ConversationID: 31,
			SenderID:       7,
			Body:           "newer",
			CreatedAt:      createdAt,
		},
		{
			ID:             11,
			ConversationID: 31,
			SenderID:       9,
			Body:           "older",
			CreatedAt:      createdAt.Add(-time.Minute),
		},
	}
	root := &fakeMessageQuerier{
		queryRows: []pgx.Row{
			messageAuthorizationRow(),
		},
	}
	rows := &fakeMessageRows{
		values: want,
	}
	var queryContext context.Context
	var query string
	var args []any
	store := newMessageStore(
		root,
		nil,
		func(
			ctx context.Context,
			actualQuery string,
			actualArgs ...any,
		) (messageRows, error) {
			queryContext = ctx
			query = actualQuery
			args = actualArgs
			return rows, nil
		},
	)
	ctx := context.WithValue(
		context.Background(),
		struct{}{},
		"request-31",
	)

	got, err := store.ListMessages(
		ctx,
		7,
		31,
		nil,
		51,
	)
	if err != nil {
		t.Fatalf(
			"ListMessages() error = %v, want nil",
			err,
		)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf(
			"ListMessages() = %#v, want %#v",
			got,
			want,
		)
	}
	if len(root.queryCalls) != 1 ||
		root.contexts[0] != ctx {
		t.Fatalf(
			"authorization calls/context = %d/%v, want 1/original",
			len(root.queryCalls),
			root.contexts[0] == ctx,
		)
	}
	authorizationCall := root.queryCalls[0]
	for _, fragment := range []string{
		"SELECT 1",
		"JOIN conversation_member AS member",
		"c.id = $1",
		"member.identity_id = $2",
	} {
		if !strings.Contains(
			authorizationCall.query,
			fragment,
		) {
			t.Errorf(
				"authorization query = %q, want fragment %q",
				authorizationCall.query,
				fragment,
			)
		}
	}
	if strings.Contains(
		authorizationCall.query,
		"FOR UPDATE",
	) {
		t.Errorf(
			"read authorization query must not lock rows: %q",
			authorizationCall.query,
		)
	}
	if !reflect.DeepEqual(
		authorizationCall.args,
		[]any{int64(31), int64(7)},
	) {
		t.Errorf(
			"authorization args = %#v, want conversation and actor IDs",
			authorizationCall.args,
		)
	}
	if queryContext != ctx {
		t.Error("history query did not receive original context")
	}
	for _, fragment := range []string{
		"FROM message",
		"JOIN conversation_member AS member",
		"member.identity_id = $2",
		"($3::BIGINT IS NULL OR message.id < $3)",
		"ORDER BY message.id DESC",
		"LIMIT $4",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf(
				"history query = %q, want fragment %q",
				query,
				fragment,
			)
		}
	}
	if !reflect.DeepEqual(
		args,
		[]any{int64(31), int64(7), nil, 51},
	) {
		t.Errorf(
			"history args = %#v, want conversation, actor, nil cursor, limit",
			args,
		)
	}
	if rows.closes != 1 {
		t.Errorf(
			"rows closes = %d, want 1",
			rows.closes,
		)
	}
}

func TestMessageStoreDereferencesMessageCursor(
	t *testing.T,
) {
	root := &fakeMessageQuerier{
		queryRows: []pgx.Row{
			messageAuthorizationRow(),
		},
	}
	rows := &fakeMessageRows{}
	var args []any
	store := newMessageStore(
		root,
		nil,
		func(
			_ context.Context,
			_ string,
			actualArgs ...any,
		) (messageRows, error) {
			args = actualArgs
			return rows, nil
		},
	)
	beforeID := int64(91)

	_, err := store.ListMessages(
		context.Background(),
		7,
		31,
		&beforeID,
		25,
	)
	if err != nil {
		t.Fatalf(
			"ListMessages() error = %v, want nil",
			err,
		)
	}
	if !reflect.DeepEqual(
		args,
		[]any{int64(31), int64(7), int64(91), 25},
	) {
		t.Errorf(
			"history args = %#v, want dereferenced cursor",
			args,
		)
	}
}

func TestMessageStoreReturnsNonNilEmptyHistory(
	t *testing.T,
) {
	root := &fakeMessageQuerier{
		queryRows: []pgx.Row{
			messageAuthorizationRow(),
		},
	}
	rows := &fakeMessageRows{}
	store := newMessageStore(
		root,
		nil,
		func(
			context.Context,
			string,
			...any,
		) (messageRows, error) {
			return rows, nil
		},
	)

	got, err := store.ListMessages(
		context.Background(),
		7,
		31,
		nil,
		51,
	)
	if err != nil {
		t.Fatalf(
			"ListMessages() error = %v, want nil",
			err,
		)
	}
	if got == nil || len(got) != 0 {
		t.Errorf(
			"ListMessages() = %#v, want non-nil empty slice",
			got,
		)
	}
	if rows.closes != 1 {
		t.Errorf(
			"rows closes = %d, want 1",
			rows.closes,
		)
	}
}

func TestMessageStoreHidesUnavailableMessageHistory(
	t *testing.T,
) {
	root := &fakeMessageQuerier{
		queryRows: []pgx.Row{
			errorMessageRow(pgx.ErrNoRows),
		},
	}
	queryCalls := 0
	store := newMessageStore(
		root,
		nil,
		func(
			context.Context,
			string,
			...any,
		) (messageRows, error) {
			queryCalls++
			return nil, nil
		},
	)

	_, err := store.ListMessages(
		context.Background(),
		9,
		31,
		nil,
		51,
	)
	if !errors.Is(
		err,
		message.ErrConversationUnavailable,
	) {
		t.Fatalf(
			"error = %v, want wrapped ErrConversationUnavailable",
			err,
		)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"error = %v, must not expose pgx.ErrNoRows",
			err,
		)
	}
	if queryCalls != 0 {
		t.Errorf(
			"history query calls = %d, want 0",
			queryCalls,
		)
	}
}

func TestMessageStoreReturnsHistoryAuthorizationDatabaseFailure(
	t *testing.T,
) {
	authorizationErr := errors.New("authorization failed")
	root := &fakeMessageQuerier{
		queryRows: []pgx.Row{
			errorMessageRow(authorizationErr),
		},
	}
	queryCalls := 0
	store := newMessageStore(
		root,
		nil,
		func(
			context.Context,
			string,
			...any,
		) (messageRows, error) {
			queryCalls++
			return nil, nil
		},
	)

	_, err := store.ListMessages(
		context.Background(),
		7,
		31,
		nil,
		51,
	)
	if !errors.Is(err, authorizationErr) {
		t.Fatalf(
			"error = %v, want wrapped %v",
			err,
			authorizationErr,
		)
	}
	if queryCalls != 0 {
		t.Errorf(
			"history query calls = %d, want 0",
			queryCalls,
		)
	}
}

func TestMessageStoreReturnsHistoryQueryFailure(
	t *testing.T,
) {
	queryErr := errors.New("query failed")
	root := &fakeMessageQuerier{
		queryRows: []pgx.Row{
			messageAuthorizationRow(),
		},
	}
	store := newMessageStore(
		root,
		nil,
		func(
			context.Context,
			string,
			...any,
		) (messageRows, error) {
			return nil, queryErr
		},
	)

	_, err := store.ListMessages(
		context.Background(),
		7,
		31,
		nil,
		51,
	)
	if !errors.Is(err, queryErr) {
		t.Fatalf(
			"error = %v, want wrapped %v",
			err,
			queryErr,
		)
	}
}

func TestMessageStoreClosesRowsAfterHistoryScanFailure(
	t *testing.T,
) {
	scanErr := errors.New("scan failed")
	root := &fakeMessageQuerier{
		queryRows: []pgx.Row{
			messageAuthorizationRow(),
		},
	}
	rows := &fakeMessageRows{
		values:  []message.Message{{ID: 1}},
		scanErr: scanErr,
	}
	store := newMessageStore(
		root,
		nil,
		func(
			context.Context,
			string,
			...any,
		) (messageRows, error) {
			return rows, nil
		},
	)

	_, err := store.ListMessages(
		context.Background(),
		7,
		31,
		nil,
		51,
	)
	if !errors.Is(err, scanErr) {
		t.Fatalf(
			"error = %v, want wrapped %v",
			err,
			scanErr,
		)
	}
	if rows.closes != 1 {
		t.Errorf(
			"rows closes = %d, want 1",
			rows.closes,
		)
	}
}

func TestMessageStoreClosesRowsAfterHistoryIterationFailure(
	t *testing.T,
) {
	iterationErr := errors.New("iteration failed")
	root := &fakeMessageQuerier{
		queryRows: []pgx.Row{
			messageAuthorizationRow(),
		},
	}
	rows := &fakeMessageRows{
		iterationErr: iterationErr,
	}
	store := newMessageStore(
		root,
		nil,
		func(
			context.Context,
			string,
			...any,
		) (messageRows, error) {
			return rows, nil
		},
	)

	_, err := store.ListMessages(
		context.Background(),
		7,
		31,
		nil,
		51,
	)
	if !errors.Is(err, iterationErr) {
		t.Fatalf(
			"error = %v, want wrapped %v",
			err,
			iterationErr,
		)
	}
	if rows.closes != 1 {
		t.Errorf(
			"rows closes = %d, want 1",
			rows.closes,
		)
	}
}

func messageStoreForTransaction(
	transaction messageTransaction,
) *MessageStore {
	return newMessageStore(
		&fakeMessageQuerier{},
		func(
			context.Context,
		) (messageTransaction, error) {
			return transaction, nil
		},
		nil,
	)
}

func messageAuthorizationRow() pgx.Row {
	return messageRow{
		scan: func(destinations ...any) error {
			*destinations[0].(*int) = 1
			return nil
		},
	}
}

func storedMessageRow(
	value message.Message,
) pgx.Row {
	return messageRow{
		scan: func(destinations ...any) error {
			*destinations[0].(*int64) = value.ID
			*destinations[1].(*int64) = value.ConversationID
			*destinations[2].(*int64) = value.SenderID
			*destinations[3].(*string) = value.Body
			*destinations[4].(*time.Time) = value.CreatedAt
			return nil
		},
	}
}

func errorMessageRow(
	err error,
) pgx.Row {
	return messageRow{
		scan: func(...any) error {
			return err
		},
	}
}

func assertMessageAuthorizationQuery(
	t *testing.T,
	call messageQueryCall,
	draft message.Draft,
) {
	t.Helper()

	for _, fragment := range []string{
		"SELECT 1",
		"JOIN conversation_member AS member",
		"FOR UPDATE OF c, member",
	} {
		if !strings.Contains(call.query, fragment) {
			t.Errorf(
				"authorization query = %q, want fragment %q",
				call.query,
				fragment,
			)
		}
	}
	if !reflect.DeepEqual(
		call.args,
		[]any{draft.ConversationID, draft.SenderID},
	) {
		t.Errorf(
			"authorization args = %#v, want conversation and sender IDs",
			call.args,
		)
	}
}

func assertMessageInsertQuery(
	t *testing.T,
	call messageQueryCall,
	draft message.Draft,
) {
	t.Helper()

	for _, fragment := range []string{
		"INSERT INTO message",
		"RETURNING",
		"created_at",
	} {
		if !strings.Contains(call.query, fragment) {
			t.Errorf(
				"insert query = %q, want fragment %q",
				call.query,
				fragment,
			)
		}
	}
	if !reflect.DeepEqual(
		call.args,
		[]any{
			draft.ConversationID,
			draft.SenderID,
			draft.Body,
		},
	) {
		t.Errorf(
			"insert args = %#v, want draft fields",
			call.args,
		)
	}
}

func assertConversationMessageTimeUpdate(
	t *testing.T,
	call messageExecCall,
	value message.Message,
) {
	t.Helper()

	for _, fragment := range []string{
		"UPDATE conversation",
		"GREATEST(updated_at, $2)",
	} {
		if !strings.Contains(call.query, fragment) {
			t.Errorf(
				"update query = %q, want fragment %q",
				call.query,
				fragment,
			)
		}
	}
	if !reflect.DeepEqual(
		call.args,
		[]any{value.ConversationID, value.CreatedAt},
	) {
		t.Errorf(
			"update args = %#v, want conversation ID and created time",
			call.args,
		)
	}
}

func assertMessageRolledBackOnce(
	t *testing.T,
	transaction *fakeMessageTransaction,
) {
	t.Helper()

	if transaction.rollbacks != 1 ||
		transaction.commits != 0 {
		t.Errorf(
			"rollback/commit = %d/%d, want 1/0",
			transaction.rollbacks,
			transaction.commits,
		)
	}
}
