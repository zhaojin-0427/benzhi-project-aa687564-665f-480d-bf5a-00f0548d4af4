package idempotencyresponse_test

import (
	"bytes"
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"stage-rigging-clearance/internal/application"
	"stage-rigging-clearance/internal/repository"

	_ "modernc.org/sqlite"
)

func TestReplayRejectsResponseThatNoLongerMatchesAuditDigest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "idempotency.db")
	store, err := repository.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	command := application.CreateCaseCommand{
		CommandMeta: application.CommandMeta{ExpectedVersion: 0, IdempotencyKey: "stable-create-replay", Actor: "inspector-a", Role: application.RoleInspector},
		CaseNumber:  "REPLAY-CASE", VenueName: "Replay Venue", Scope: "Replay Scope",
	}
	first, err := application.NewService(store).CreateCase(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	original := append([]byte(nil), first.Body...)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	raw, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`UPDATE idempotency_records SET response='{}' WHERE key='stable-create-replay'`); err != nil {
		raw.Close()
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := repository.Open(path)
	if err != nil {
		return
	}
	defer reopened.Close()
	replayed, err := application.NewService(reopened).CreateCase(context.Background(), command)
	if err != nil {
		return
	}
	if !bytes.Equal(replayed.Body, original) {
		t.Fatalf("TestReplayRejectsResponseThatNoLongerMatchesAuditDigest: replay served a response whose digest is not the committed audit result")
	}
}
