package auditvalidationcountcache_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"stage-rigging-clearance/internal/application"
	"stage-rigging-clearance/internal/httpapi"
	"stage-rigging-clearance/internal/repository"
)

func TestAuditCacheRevalidatesSameLengthTrail(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "audit-cache.db")
	store, err := repository.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	service := application.NewService(store)
	_, err = service.CreateCase(context.Background(), application.CreateCaseCommand{
		CommandMeta: application.CommandMeta{ExpectedVersion: 0, IdempotencyKey: "audit-cache-create-0001", Actor: "初始检验员", Role: application.RoleInspector},
		CaseNumber:  "RIG-AUDIT-CACHE", VenueName: "缓存完整性剧院", Scope: "主舞台吊挂系统",
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := httpapi.New(service).Handler()
	prime := httptest.NewRecorder()
	handler.ServeHTTP(prime, httptest.NewRequest(http.MethodGet, "/api/v1/inspection-cases/RIG-AUDIT-CACHE/audit", nil))
	if prime.Code != http.StatusOK {
		t.Fatalf("首次审计查询失败: %d %s", prime.Code, prime.Body.String())
	}

	tamperDB, err := sql.Open("sqlite", "file:"+databasePath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tamperDB.Exec(`UPDATE audit_events SET actor='被替换的责任人' WHERE sequence=1`); err != nil {
		tamperDB.Close()
		t.Fatal(err)
	}
	if err := tamperDB.Close(); err != nil {
		t.Fatal(err)
	}

	afterTamper := httptest.NewRecorder()
	handler.ServeHTTP(afterTamper, httptest.NewRequest(http.MethodGet, "/api/v1/inspection-cases/RIG-AUDIT-CACHE/audit", nil))
	if afterTamper.Code != http.StatusInternalServerError {
		t.Fatalf("同长度审计轨迹被篡改后仍以 %d 公开为有效: %s", afterTamper.Code, afterTamper.Body.String())
	}
}
