package store_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/modelcatalog"
	"github.com/postpilot/backend/internal/modelcatalog/store"
	"github.com/postpilot/backend/internal/platform/db"
)

// txRows writes through the transaction it is bound to, so a test can see whether its write
// lands or rolls back with the registrations beside it.
type txRows struct {
	tx   *sql.Tx
	fail error
}

func (r txRows) List(ctx context.Context) ([]modelcatalog.StoredSet, error) {
	rows, err := r.tx.QueryContext(ctx, `SELECT id, label FROM recommendation_sets ORDER BY position, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []modelcatalog.StoredSet
	for rows.Next() {
		var set modelcatalog.StoredSet
		if err := rows.Scan(&set.ID, &set.Label); err != nil {
			return nil, err
		}
		out = append(out, set)
	}
	return out, rows.Err()
}

func (r txRows) Replace(ctx context.Context, sets []modelcatalog.StoredSet, at time.Time) error {
	if _, err := r.tx.ExecContext(ctx, `DELETE FROM recommendation_sets`); err != nil {
		return err
	}
	for index, set := range sets {
		stamp := at.UTC().Format(time.RFC3339)
		if _, err := r.tx.ExecContext(ctx, `INSERT INTO recommendation_sets(id,label,position,created_at,updated_at) VALUES(?,?,?,?,?)`,
			set.ID, set.Label, index+1, stamp, stamp); err != nil {
			return err
		}
	}
	return r.fail
}

func newDocumentStore(t *testing.T, fail error) (*store.Store, *db.DB) {
	t.Helper()
	handle, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { handle.Close() })
	if err := db.Migrate(context.Background(), handle.Writer); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	s := store.New(handle.Writer, handle.Reader)
	s.SetRecommendationsForTx(func(tx *sql.Tx) modelcatalog.RecommendationRows { return txRows{tx: tx, fail: fail} })
	return s, handle
}

// MODEL-73: the registrations and the set list are one transaction.
func TestSyncDocumentWritesRegistrationsAndSetsTogether(t *testing.T) {
	s, _ := newDocumentStore(t, nil)
	ctx := context.Background()
	row, err := s.Get(ctx, "anthropic/claude-sonnet-5")
	if err != nil {
		t.Fatal(err)
	}
	sets := []modelcatalog.StoredSet{{ID: "fresh", Label: "Fresh"}}
	if err := s.SyncDocument(ctx, []modelcatalog.PurposeWrite{
		{Model: row, Purpose: modelcatalog.PurposeWriting, Register: true, Level: modelcatalog.LevelTop},
	}, &sets, testNow); err != nil {
		t.Fatal(err)
	}
	after, err := s.Get(ctx, "anthropic/claude-sonnet-5")
	if err != nil || len(after.Purposes) != 1 {
		t.Fatalf("registration = %+v, %v", after.Purposes, err)
	}
	stored, err := s.RecommendationSets(ctx)
	if err != nil || len(stored) != 1 || stored[0].ID != "fresh" {
		t.Fatalf("sets = %+v, %v (the seeded set was replaced by the document's list)", stored, err)
	}
}

func TestSyncDocumentRollsTheRegistrationsBackWhenTheSetWriteFails(t *testing.T) {
	s, _ := newDocumentStore(t, errors.New("set write refused"))
	ctx := context.Background()
	row, err := s.Get(ctx, "anthropic/claude-sonnet-5")
	if err != nil {
		t.Fatal(err)
	}
	sets := []modelcatalog.StoredSet{{ID: "fresh", Label: "Fresh"}}
	if err := s.SyncDocument(ctx, []modelcatalog.PurposeWrite{
		{Model: row, Purpose: modelcatalog.PurposeWriting, Register: true},
	}, &sets, testNow); err == nil {
		t.Fatal("a failed set write reported success")
	}
	after, err := s.Get(ctx, "anthropic/claude-sonnet-5")
	if err != nil || len(after.Purposes) != 0 {
		t.Fatalf("registration survived the rollback: %+v, %v", after.Purposes, err)
	}
	stored, err := s.RecommendationSets(ctx)
	if err != nil || len(stored) != 1 || stored[0].ID != "balanced-2026-08" {
		t.Fatalf("sets = %+v, %v, want the seeded set untouched", stored, err)
	}
}

// Nil sets leave the table alone; an unattached store refuses a set write instead of skipping it.
func TestSyncDocumentLeavesTheSetsWhenTheDocumentHasNoSection(t *testing.T) {
	s, _ := newDocumentStore(t, errors.New("must not be called"))
	ctx := context.Background()
	if err := s.SyncDocument(ctx, nil, nil, testNow); err != nil {
		t.Fatalf("a document without sets touched them: %v", err)
	}
	unwired := newStore(t)
	sets := []modelcatalog.StoredSet{}
	if err := unwired.SyncDocument(ctx, nil, &sets, testNow); err == nil {
		t.Fatal("an unattached store accepted a set write")
	}
	if _, err := unwired.RecommendationSets(ctx); err == nil {
		t.Fatal("an unattached store answered a set read")
	}
}
