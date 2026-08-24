package repository_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"stage-rigging-clearance/internal/application"
	"stage-rigging-clearance/internal/repository"
)

func TestOpenRejectsTamperedAuditChain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tampered.db")
	store, err := repository.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(store)
	_, err = service.CreateCase(context.Background(), application.CreateCaseCommand{
		CommandMeta: application.CommandMeta{ExpectedVersion: 0, IdempotencyKey: "recovery-create-01", Actor: "检验员", Role: application.RoleInspector},
		CaseNumber:  "REC-001", VenueName: "恢复测试剧院", Scope: "一号吊杆",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE audit_events SET actor='篡改者' WHERE sequence=1"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := repository.Open(path)
	if reopened != nil {
		_ = reopened.Close()
	}
	if err == nil {
		t.Fatal("审计链被篡改后存储仍然接受写入")
	}
}
