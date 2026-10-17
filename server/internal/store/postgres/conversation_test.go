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

	"github.com/imamsyafii-dev/mini-chat/server/internal/domain/conversation"
)

type conversationRow struct {
	scan func(destinations ...any) error
}

func (
	row conversationRow,
) Scan(
	destinations ...any,
) error {
	return row.scan(destinations...)
}

type conversationQueryCall struct {
	query string
	args  []any
}

type conversationExecCall struct {
	query string
	args  []any
}

type fakeConversationRows struct {
	values  []conversation.DirectConversation
	index   int
	scanErr error
	err     error
	closed  bool
}

func (
	rows *fakeConversationRows,
) Next() bool {
	return rows.index < len(rows.values)
}

func (
	rows *fakeConversationRows,
) Scan(
	destinations ...any,
) error {
	if rows.scanErr != nil {
		return rows.scanErr
	}

	value := rows.values[rows.index]
	rows.index++

	return scanDirectConversation(
		destinations,
		value,
	)
}

func (
	rows *fakeConversationRows,
) Err() error {
	return rows.err
}

func (
	rows *fakeConversationRows,
) Close() {
	rows.closed = true
}

var _ conversationRows = (*fakeConversationRows)(nil)

type fakeConversationRoot struct {
	rows   []pgx.Row
	calls  []conversationQueryCall
	events *[]string
}

func (
	fake *fakeConversationRoot,
) QueryRow(
	_ context.Context,
	query string,
	args ...any,
) pgx.Row {
	fake.calls = append(
		fake.calls,
		conversationQueryCall{
			query: query,
			args:  args,
		},
	)
	if fake.events != nil {
		*fake.events = append(
			*fake.events,
			"root query",
		)
	}

	row := fake.rows[0]
	fake.rows = fake.rows[1:]

	return row
}

type fakeConversationTransaction struct {
	queryRows   []pgx.Row
	execErrors  []error
	queryCalls  []conversationQueryCall
	execCalls   []conversationExecCall
	commitErr   error
	rollbackErr error
	commits     int
	rollbacks   int
	events      *[]string
}

func (
	fake *fakeConversationTransaction,
) QueryRow(
	_ context.Context,
	query string,
	args ...any,
) pgx.Row {
	fake.queryCalls = append(
		fake.queryCalls,
		conversationQueryCall{
			query: query,
			args:  args,
		},
	)
	if fake.events != nil {
		*fake.events = append(
			*fake.events,
			"transaction query",
		)
	}

	row := fake.queryRows[0]
	fake.queryRows = fake.queryRows[1:]

	return row
}

func (
	fake *fakeConversationTransaction,
) Exec(
	_ context.Context,
	query string,
	args ...any,
) (pgconn.CommandTag, error) {
	fake.execCalls = append(
		fake.execCalls,
		conversationExecCall{
			query: query,
			args:  args,
		},
	)
	if fake.events != nil {
		*fake.events = append(
			*fake.events,
			"transaction exec",
		)
	}

	var err error
	if len(fake.execErrors) > 0 {
		err = fake.execErrors[0]
		fake.execErrors = fake.execErrors[1:]
	}

	return pgconn.NewCommandTag("INSERT 0 1"), err
}

func (
	fake *fakeConversationTransaction,
) Commit(
	_ context.Context,
) error {
	fake.commits++
	if fake.events != nil {
		*fake.events = append(
			*fake.events,
			"commit",
		)
	}

	return fake.commitErr
}

func (
	fake *fakeConversationTransaction,
) Rollback(
	_ context.Context,
) error {
	fake.rollbacks++
	if fake.events != nil {
		*fake.events = append(
			*fake.events,
			"rollback",
		)
	}

	return fake.rollbackErr
}

var _ conversationTransaction = (*fakeConversationTransaction)(nil)

func TestConversationStoreReturnsExistingDirectConversation(
	t *testing.T,
) {
	pair := conversation.DirectPair{
		IdentityLowID:  3,
		IdentityHighID: 8,
	}
	want := directConversationFixture(41, 8, 3)
	transaction := &fakeConversationTransaction{
		queryRows: []pgx.Row{
			directConversationRow(want),
		},
	}
	beginCalls := 0
	store := newConversationStore(
		&fakeConversationRoot{},
		func(
			context.Context,
		) (conversationTransaction, error) {
			beginCalls++
			return transaction, nil
		},
		nil,
	)

	got, created, err :=
		store.CreateOrGetDirectConversation(
			context.Background(),
			8,
			pair,
		)
	if err != nil {
		t.Fatalf(
			"CreateOrGetDirectConversation() error = %v, want nil",
			err,
		)
	}
	if created {
		t.Fatal("created = true, want false")
	}
	if got != want {
		t.Errorf(
			"result = %#v, want %#v",
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
	assertDirectLookup(
		t,
		transaction.queryCalls[0],
		pair,
		8,
	)
}

func TestConversationStoreCreatesDirectConversation(
	t *testing.T,
) {
	pair := conversation.DirectPair{
		IdentityLowID:  3,
		IdentityHighID: 8,
	}
	want := directConversationFixture(42, 3, 8)
	transaction := &fakeConversationTransaction{
		queryRows: []pgx.Row{
			errorConversationRow(pgx.ErrNoRows),
			conversationIDRow(42),
			directConversationRow(want),
		},
	}
	store := newConversationStore(
		&fakeConversationRoot{},
		func(
			context.Context,
		) (conversationTransaction, error) {
			return transaction, nil
		},
		nil,
	)

	got, created, err :=
		store.CreateOrGetDirectConversation(
			context.Background(),
			3,
			pair,
		)
	if err != nil {
		t.Fatalf(
			"CreateOrGetDirectConversation() error = %v, want nil",
			err,
		)
	}
	if !created {
		t.Fatal("created = false, want true")
	}
	if got != want {
		t.Errorf(
			"result = %#v, want %#v",
			got,
			want,
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
	if len(transaction.queryCalls) != 3 {
		t.Fatalf(
			"query call count = %d, want 3",
			len(transaction.queryCalls),
		)
	}
	assertDirectLookup(
		t,
		transaction.queryCalls[0],
		pair,
		3,
	)
	if !strings.Contains(
		transaction.queryCalls[1].query,
		"INSERT INTO conversation",
	) {
		t.Errorf(
			"conversation query = %q, want INSERT",
			transaction.queryCalls[1].query,
		)
	}
	if !reflect.DeepEqual(
		transaction.queryCalls[1].args,
		[]any{conversation.KindDirect, int64(3)},
	) {
		t.Errorf(
			"conversation args = %#v, want kind and actor",
			transaction.queryCalls[1].args,
		)
	}
	assertDirectLookup(
		t,
		transaction.queryCalls[2],
		pair,
		3,
	)
	assertCreateExecs(
		t,
		transaction.execCalls,
		42,
		pair,
		3,
		8,
	)
}

func TestConversationStoreMakesHighIdentityTheOwner(
	t *testing.T,
) {
	pair := conversation.DirectPair{
		IdentityLowID:  3,
		IdentityHighID: 8,
	}
	transaction := &fakeConversationTransaction{
		queryRows: []pgx.Row{
			errorConversationRow(pgx.ErrNoRows),
			conversationIDRow(43),
			directConversationRow(
				directConversationFixture(43, 8, 3),
			),
		},
	}
	store := newConversationStore(
		&fakeConversationRoot{},
		func(
			context.Context,
		) (conversationTransaction, error) {
			return transaction, nil
		},
		nil,
	)

	_, _, err := store.CreateOrGetDirectConversation(
		context.Background(),
		8,
		pair,
	)
	if err != nil {
		t.Fatalf(
			"CreateOrGetDirectConversation() error = %v, want nil",
			err,
		)
	}
	assertCreateExecs(
		t,
		transaction.execCalls,
		43,
		pair,
		8,
		3,
	)
}

func TestConversationStoreRecoversConcurrentPairWinner(
	t *testing.T,
) {
	events := make([]string, 0)
	pair := conversation.DirectPair{
		IdentityLowID:  3,
		IdentityHighID: 8,
	}
	want := directConversationFixture(44, 3, 8)
	transaction := &fakeConversationTransaction{
		queryRows: []pgx.Row{
			errorConversationRow(pgx.ErrNoRows),
			conversationIDRow(90),
		},
		execErrors: []error{
			&pgconn.PgError{
				Code:           uniqueViolationCode,
				ConstraintName: directConversationPairUniqueConstraint,
			},
		},
		events: &events,
	}
	root := &fakeConversationRoot{
		rows: []pgx.Row{
			directConversationRow(want),
		},
		events: &events,
	}
	store := newConversationStore(
		root,
		func(
			context.Context,
		) (conversationTransaction, error) {
			return transaction, nil
		},
		nil,
	)

	got, created, err :=
		store.CreateOrGetDirectConversation(
			context.Background(),
			3,
			pair,
		)
	if err != nil {
		t.Fatalf(
			"CreateOrGetDirectConversation() error = %v, want nil",
			err,
		)
	}
	if created {
		t.Fatal("created = true, want false")
	}
	if got != want {
		t.Errorf(
			"result = %#v, want %#v",
			got,
			want,
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
	if len(transaction.execCalls) != 1 {
		t.Errorf(
			"transaction exec count = %d, want 1; members must not be inserted",
			len(transaction.execCalls),
		)
	}
	if len(root.calls) != 1 {
		t.Fatalf(
			"root query count = %d, want 1",
			len(root.calls),
		)
	}
	assertDirectLookup(t, root.calls[0], pair, 3)
	rollbackIndex := indexOfEvent(events, "rollback")
	rootIndex := indexOfEvent(events, "root query")
	if rollbackIndex < 0 || rootIndex <= rollbackIndex {
		t.Errorf(
			"events = %#v, want rollback before root query",
			events,
		)
	}
}

func TestConversationStoreDoesNotRecoverUnrelatedUniqueViolation(
	t *testing.T,
) {
	uniqueErr := &pgconn.PgError{
		Code:           uniqueViolationCode,
		ConstraintName: "some_other_constraint",
	}
	transaction := &fakeConversationTransaction{
		queryRows: []pgx.Row{
			errorConversationRow(pgx.ErrNoRows),
			conversationIDRow(45),
		},
		execErrors: []error{uniqueErr},
	}
	root := &fakeConversationRoot{}
	store := newConversationStore(
		root,
		func(
			context.Context,
		) (conversationTransaction, error) {
			return transaction, nil
		},
		nil,
	)

	_, _, err := store.CreateOrGetDirectConversation(
		context.Background(),
		3,
		conversation.DirectPair{
			IdentityLowID:  3,
			IdentityHighID: 8,
		},
	)
	if !errors.Is(err, uniqueErr) {
		t.Fatalf(
			"error = %v, want wrapped %v",
			err,
			uniqueErr,
		)
	}
	if transaction.rollbacks != 1 ||
		len(root.calls) != 0 {
		t.Errorf(
			"rollbacks/root queries = %d/%d, want 1/0",
			transaction.rollbacks,
			len(root.calls),
		)
	}
}

func TestConversationStoreRollsBackPreCommitFailures(
	t *testing.T,
) {
	tests := []struct {
		name       string
		queryRows  []pgx.Row
		execErrors []error
		wantErr    error
	}{
		{
			name: "initial lookup",
			queryRows: []pgx.Row{
				errorConversationRow(
					errors.New("initial lookup failed"),
				),
			},
			wantErr: errors.New("initial lookup failed"),
		},
		{
			name: "conversation insert",
			queryRows: []pgx.Row{
				errorConversationRow(pgx.ErrNoRows),
				errorConversationRow(
					errors.New("conversation insert failed"),
				),
			},
			wantErr: errors.New("conversation insert failed"),
		},
		{
			name: "pair insert",
			queryRows: []pgx.Row{
				errorConversationRow(pgx.ErrNoRows),
				conversationIDRow(46),
			},
			execErrors: []error{
				errors.New("pair insert failed"),
			},
			wantErr: errors.New("pair insert failed"),
		},
		{
			name: "member insert",
			queryRows: []pgx.Row{
				errorConversationRow(pgx.ErrNoRows),
				conversationIDRow(47),
			},
			execErrors: []error{
				nil,
				errors.New("member insert failed"),
			},
			wantErr: errors.New("member insert failed"),
		},
		{
			name: "final lookup",
			queryRows: []pgx.Row{
				errorConversationRow(pgx.ErrNoRows),
				conversationIDRow(48),
				errorConversationRow(
					errors.New("final lookup failed"),
				),
			},
			execErrors: []error{nil, nil},
			wantErr:    errors.New("final lookup failed"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transaction := &fakeConversationTransaction{
				queryRows:  test.queryRows,
				execErrors: test.execErrors,
			}
			store := newConversationStore(
				&fakeConversationRoot{},
				func(
					context.Context,
				) (conversationTransaction, error) {
					return transaction, nil
				},
				nil,
			)

			_, _, err :=
				store.CreateOrGetDirectConversation(
					context.Background(),
					3,
					conversation.DirectPair{
						IdentityLowID:  3,
						IdentityHighID: 8,
					},
				)
			if err == nil ||
				!strings.Contains(
					err.Error(),
					test.wantErr.Error(),
				) {
				t.Fatalf(
					"error = %v, want message containing %q",
					err,
					test.wantErr,
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
		})
	}
}

func TestConversationStoreJoinsRollbackFailure(
	t *testing.T,
) {
	operationErr := errors.New("lookup failed")
	rollbackErr := errors.New("rollback failed")
	transaction := &fakeConversationTransaction{
		queryRows: []pgx.Row{
			errorConversationRow(operationErr),
		},
		rollbackErr: rollbackErr,
	}
	store := newConversationStore(
		&fakeConversationRoot{},
		func(
			context.Context,
		) (conversationTransaction, error) {
			return transaction, nil
		},
		nil,
	)

	_, _, err := store.CreateOrGetDirectConversation(
		context.Background(),
		3,
		conversation.DirectPair{
			IdentityLowID:  3,
			IdentityHighID: 8,
		},
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

func TestConversationStoreDoesNotFindWinnerAfterRollbackFailure(
	t *testing.T,
) {
	rollbackErr := errors.New("rollback failed")
	transaction := &fakeConversationTransaction{
		queryRows: []pgx.Row{
			errorConversationRow(pgx.ErrNoRows),
			conversationIDRow(49),
		},
		execErrors: []error{
			&pgconn.PgError{
				Code:           uniqueViolationCode,
				ConstraintName: directConversationPairUniqueConstraint,
			},
		},
		rollbackErr: rollbackErr,
	}
	root := &fakeConversationRoot{}
	store := newConversationStore(
		root,
		func(
			context.Context,
		) (conversationTransaction, error) {
			return transaction, nil
		},
		nil,
	)

	_, _, err := store.CreateOrGetDirectConversation(
		context.Background(),
		3,
		conversation.DirectPair{
			IdentityLowID:  3,
			IdentityHighID: 8,
		},
	)
	if !errors.Is(err, rollbackErr) {
		t.Fatalf(
			"error = %v, want wrapped %v",
			err,
			rollbackErr,
		)
	}
	if len(root.calls) != 0 {
		t.Errorf(
			"root query count = %d, want 0",
			len(root.calls),
		)
	}
}

func TestConversationStoreReturnsWinnerLookupFailure(
	t *testing.T,
) {
	winnerErr := errors.New("winner lookup failed")
	transaction := &fakeConversationTransaction{
		queryRows: []pgx.Row{
			errorConversationRow(pgx.ErrNoRows),
			conversationIDRow(50),
		},
		execErrors: []error{
			&pgconn.PgError{
				Code:           uniqueViolationCode,
				ConstraintName: directConversationPairUniqueConstraint,
			},
		},
	}
	root := &fakeConversationRoot{
		rows: []pgx.Row{
			errorConversationRow(winnerErr),
		},
	}
	store := newConversationStore(
		root,
		func(
			context.Context,
		) (conversationTransaction, error) {
			return transaction, nil
		},
		nil,
	)

	_, _, err := store.CreateOrGetDirectConversation(
		context.Background(),
		3,
		conversation.DirectPair{
			IdentityLowID:  3,
			IdentityHighID: 8,
		},
	)
	if !errors.Is(err, winnerErr) {
		t.Fatalf(
			"error = %v, want wrapped %v",
			err,
			winnerErr,
		)
	}
}

func TestConversationStoreDoesNotRollbackCommitFailure(
	t *testing.T,
) {
	commitErr := errors.New("commit failed")
	transaction := &fakeConversationTransaction{
		queryRows: []pgx.Row{
			directConversationRow(
				directConversationFixture(51, 3, 8),
			),
		},
		commitErr: commitErr,
	}
	store := newConversationStore(
		&fakeConversationRoot{},
		func(
			context.Context,
		) (conversationTransaction, error) {
			return transaction, nil
		},
		nil,
	)

	_, _, err := store.CreateOrGetDirectConversation(
		context.Background(),
		3,
		conversation.DirectPair{
			IdentityLowID:  3,
			IdentityHighID: 8,
		},
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

func TestConversationStoreReturnsBeginFailure(
	t *testing.T,
) {
	beginErr := errors.New("begin failed")
	store := newConversationStore(
		&fakeConversationRoot{},
		func(
			context.Context,
		) (conversationTransaction, error) {
			return nil, beginErr
		},
		nil,
	)

	_, _, err := store.CreateOrGetDirectConversation(
		context.Background(),
		3,
		conversation.DirectPair{
			IdentityLowID:  3,
			IdentityHighID: 8,
		},
	)
	if !errors.Is(err, beginErr) {
		t.Fatalf(
			"error = %v, want wrapped %v",
			err,
			beginErr,
		)
	}
}

func TestConversationStoreRejectsActorOutsidePairBeforeBegin(
	t *testing.T,
) {
	beginCalls := 0
	store := newConversationStore(
		&fakeConversationRoot{},
		func(
			context.Context,
		) (conversationTransaction, error) {
			beginCalls++
			return &fakeConversationTransaction{}, nil
		},
		nil,
	)

	_, _, err := store.CreateOrGetDirectConversation(
		context.Background(),
		9,
		conversation.DirectPair{
			IdentityLowID:  3,
			IdentityHighID: 8,
		},
	)
	if !errors.Is(
		err,
		conversation.ErrParticipantIDInvalid,
	) {
		t.Fatalf(
			"error = %v, want %v",
			err,
			conversation.ErrParticipantIDInvalid,
		)
	}
	if beginCalls != 0 {
		t.Errorf(
			"begin calls = %d, want 0",
			beginCalls,
		)
	}
}

func TestConversationStoreListsDirectConversations(
	t *testing.T,
) {
	first := directConversationFixture(52, 7, 12)
	first.Participant.Username = "latest"
	first.UpdatedAt = first.UpdatedAt.Add(time.Hour)
	second := directConversationFixture(51, 12, 14)
	second.Participant.Username = "earlier"
	rows := &fakeConversationRows{
		values: []conversation.DirectConversation{
			first,
			second,
		},
	}
	var call conversationQueryCall
	store := newConversationStore(
		nil,
		nil,
		func(
			_ context.Context,
			query string,
			args ...any,
		) (conversationRows, error) {
			call = conversationQueryCall{
				query: query,
				args:  args,
			}
			return rows, nil
		},
	)

	got, err := store.ListDirectConversations(
		context.Background(),
		7,
		50,
	)
	if err != nil {
		t.Fatalf(
			"ListDirectConversations() error = %v, want nil",
			err,
		)
	}
	want := []conversation.DirectConversation{
		first,
		second,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf(
			"ListDirectConversations() = %#v, want %#v",
			got,
			want,
		)
	}
	if !rows.closed {
		t.Error("rows were not closed after successful listing")
	}
	if !reflect.DeepEqual(
		call.args,
		[]any{int64(7), 50},
	) {
		t.Errorf(
			"query args = %#v, want actor ID and limit",
			call.args,
		)
	}

	requiredSQL := []string{
		"JOIN conversation_member AS actor_member",
		"actor_member.identity_id = $1",
		"JOIN identity AS participant",
		"WHEN direct.identity_low_id = $1",
		"THEN direct.identity_high_id",
		"ELSE direct.identity_low_id",
		"JOIN conversation_member AS participant_member",
		"participant_member.identity_id = participant.id",
		"direct.identity_low_id = $1",
		"direct.identity_high_id = $1",
		"c.kind = 'direct'",
		"ORDER BY c.updated_at DESC, c.id DESC",
		"LIMIT $2",
	}
	for _, fragment := range requiredSQL {
		if !strings.Contains(call.query, fragment) {
			t.Errorf(
				"list query does not contain %q",
				fragment,
			)
		}
	}
	if strings.Contains(
		strings.ToLower(call.query),
		"password",
	) {
		t.Error("list query must not select password data")
	}
}

func TestConversationStoreReturnsNonNilEmptyConversationList(
	t *testing.T,
) {
	rows := &fakeConversationRows{}
	store := newConversationStore(
		nil,
		nil,
		func(
			context.Context,
			string,
			...any,
		) (conversationRows, error) {
			return rows, nil
		},
	)

	got, err := store.ListDirectConversations(
		context.Background(),
		7,
		50,
	)
	if err != nil {
		t.Fatalf(
			"ListDirectConversations() error = %v, want nil",
			err,
		)
	}
	if got == nil {
		t.Fatal(
			"ListDirectConversations() = nil, want non-nil empty slice",
		)
	}
	if len(got) != 0 {
		t.Errorf(
			"conversation count = %d, want 0",
			len(got),
		)
	}
	if !rows.closed {
		t.Error("empty rows were not closed")
	}
}

func TestConversationStoreReturnsListQueryFailure(
	t *testing.T,
) {
	queryErr := errors.New("query failed")
	store := newConversationStore(
		nil,
		nil,
		func(
			context.Context,
			string,
			...any,
		) (conversationRows, error) {
			return nil, queryErr
		},
	)

	_, err := store.ListDirectConversations(
		context.Background(),
		7,
		50,
	)
	if !errors.Is(err, queryErr) {
		t.Fatalf(
			"error = %v, want wrapped %v",
			err,
			queryErr,
		)
	}
}

func TestConversationStoreClosesRowsAfterListScanFailure(
	t *testing.T,
) {
	scanErr := errors.New("scan failed")
	rows := &fakeConversationRows{
		values: []conversation.DirectConversation{
			directConversationFixture(53, 7, 12),
		},
		scanErr: scanErr,
	}
	store := newConversationStore(
		nil,
		nil,
		func(
			context.Context,
			string,
			...any,
		) (conversationRows, error) {
			return rows, nil
		},
	)

	_, err := store.ListDirectConversations(
		context.Background(),
		7,
		50,
	)
	if !errors.Is(err, scanErr) {
		t.Fatalf(
			"error = %v, want wrapped %v",
			err,
			scanErr,
		)
	}
	if !rows.closed {
		t.Error("rows were not closed after scan failure")
	}
}

func TestConversationStoreClosesRowsAfterListIterationFailure(
	t *testing.T,
) {
	iterationErr := errors.New("iteration failed")
	rows := &fakeConversationRows{
		err: iterationErr,
	}
	store := newConversationStore(
		nil,
		nil,
		func(
			context.Context,
			string,
			...any,
		) (conversationRows, error) {
			return rows, nil
		},
	)

	_, err := store.ListDirectConversations(
		context.Background(),
		7,
		50,
	)
	if !errors.Is(err, iterationErr) {
		t.Fatalf(
			"error = %v, want wrapped %v",
			err,
			iterationErr,
		)
	}
	if !rows.closed {
		t.Error("rows were not closed after iteration failure")
	}
}

func directConversationFixture(
	conversationID int64,
	createdBy int64,
	participantID int64,
) conversation.DirectConversation {
	now := time.Date(
		2026,
		time.October,
		16,
		15,
		0,
		0,
		0,
		time.UTC,
	)

	result := conversation.DirectConversation{
		ID:        conversationID,
		Kind:      conversation.KindDirect,
		CreatedBy: createdBy,
		CreatedAt: now,
		UpdatedAt: now,
	}
	result.Participant.ID = participantID
	result.Participant.Username = "participant"
	result.Participant.DisplayName = "Participant"
	result.Participant.CreatedAt = now
	result.Participant.UpdatedAt = now

	return result
}

func directConversationRow(
	value conversation.DirectConversation,
) pgx.Row {
	if value.Participant.ID == 0 {
		value.Participant.ID = 8
		value.Participant.Username = "participant"
		value.Participant.DisplayName = "Participant"
		value.Participant.CreatedAt = value.CreatedAt
		value.Participant.UpdatedAt = value.UpdatedAt
	}

	return conversationRow{
		scan: func(destinations ...any) error {
			return scanDirectConversation(
				destinations,
				value,
			)
		},
	}
}

func scanDirectConversation(
	destinations []any,
	value conversation.DirectConversation,
) error {
	*destinations[0].(*int64) = value.ID
	*destinations[1].(*conversation.Kind) = value.Kind
	*destinations[2].(*int64) = value.CreatedBy
	*destinations[3].(*time.Time) = value.CreatedAt
	*destinations[4].(*time.Time) = value.UpdatedAt
	*destinations[5].(*int64) = value.Participant.ID
	*destinations[6].(*string) =
		value.Participant.Username
	*destinations[7].(*string) =
		value.Participant.DisplayName
	*destinations[8].(*time.Time) =
		value.Participant.CreatedAt
	*destinations[9].(*time.Time) =
		value.Participant.UpdatedAt

	return nil
}

func conversationIDRow(
	conversationID int64,
) pgx.Row {
	return conversationRow{
		scan: func(destinations ...any) error {
			*destinations[0].(*int64) = conversationID

			return nil
		},
	}
}

func errorConversationRow(
	err error,
) pgx.Row {
	return conversationRow{
		scan: func(...any) error {
			return err
		},
	}
}

func assertDirectLookup(
	t *testing.T,
	call conversationQueryCall,
	pair conversation.DirectPair,
	actorID int64,
) {
	t.Helper()

	for _, fragment := range []string{
		"FROM direct_conversation",
		"JOIN conversation_member AS actor_member",
		"JOIN conversation_member AS participant_member",
		"c.kind = 'direct'",
	} {
		if !strings.Contains(call.query, fragment) {
			t.Errorf(
				"lookup query = %q, want fragment %q",
				call.query,
				fragment,
			)
		}
	}
	if strings.Contains(call.query, "password") {
		t.Errorf(
			"lookup query = %q, must not select password data",
			call.query,
		)
	}
	wantArgs := []any{
		pair.IdentityLowID,
		pair.IdentityHighID,
		actorID,
	}
	if !reflect.DeepEqual(call.args, wantArgs) {
		t.Errorf(
			"lookup args = %#v, want %#v",
			call.args,
			wantArgs,
		)
	}
}

func assertCreateExecs(
	t *testing.T,
	calls []conversationExecCall,
	conversationID int64,
	pair conversation.DirectPair,
	ownerID int64,
	participantID int64,
) {
	t.Helper()

	if len(calls) != 2 {
		t.Fatalf(
			"exec call count = %d, want 2",
			len(calls),
		)
	}
	if !strings.Contains(
		calls[0].query,
		"INSERT INTO direct_conversation",
	) {
		t.Errorf(
			"pair query = %q, want direct pair INSERT",
			calls[0].query,
		)
	}
	wantPairArgs := []any{
		conversationID,
		pair.IdentityLowID,
		pair.IdentityHighID,
	}
	if !reflect.DeepEqual(calls[0].args, wantPairArgs) {
		t.Errorf(
			"pair args = %#v, want %#v",
			calls[0].args,
			wantPairArgs,
		)
	}
	if !strings.Contains(
		calls[1].query,
		"INSERT INTO conversation_member",
	) || !strings.Contains(
		calls[1].query,
		"($1, $2, 'owner')",
	) || !strings.Contains(
		calls[1].query,
		"($1, $3, 'member')",
	) {
		t.Errorf(
			"member query = %q, want exactly owner and member tuples",
			calls[1].query,
		)
	}
	wantMemberArgs := []any{
		conversationID,
		ownerID,
		participantID,
	}
	if !reflect.DeepEqual(calls[1].args, wantMemberArgs) {
		t.Errorf(
			"member args = %#v, want %#v",
			calls[1].args,
			wantMemberArgs,
		)
	}
}

func indexOfEvent(
	events []string,
	want string,
) int {
	for index, event := range events {
		if event == want {
			return index
		}
	}

	return -1
}
