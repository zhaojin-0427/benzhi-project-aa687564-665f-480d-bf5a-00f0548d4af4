package application

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"stage-rigging-clearance/internal/domain"
	"stage-rigging-clearance/internal/repository"
)

func TestVersionConflictIdempotencyAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflow.db")
	store, err := repository.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(store)
	ctx := context.Background()
	create := CreateCaseCommand{CommandMeta: CommandMeta{ExpectedVersion: 0, IdempotencyKey: "create-case-0001", Actor: "检验员", Role: RoleInspector},
		CaseNumber: "APP-001", VenueName: "测试剧院", Scope: "吊挂系统"}
	first, err := service.CreateCase(ctx, create)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := service.CreateCase(ctx, create)
	if err != nil {
		t.Fatal(err)
	}
	if string(first.Body) != string(replay.Body) {
		t.Fatal("幂等重放响应不一致")
	}
	prepare := CaseCommand{CommandMeta: CommandMeta{ExpectedVersion: 1, IdempotencyKey: "prepare-case-001", Actor: "检验员", Role: RoleInspector}, CaseNumber: "APP-001"}
	if _, err := service.PrepareBaseline(ctx, prepare); err != nil {
		t.Fatal(err)
	}
	prepare.IdempotencyKey = "prepare-case-002"
	if _, err := service.PrepareBaseline(ctx, prepare); domain.ErrorCodeOf(err) != domain.CodeConflict {
		t.Fatalf("期望版本冲突，得到 %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := repository.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	aggregate, err := reopened.LoadCase(ctx, "APP-001")
	if err != nil {
		t.Fatal(err)
	}
	if aggregate.Status != domain.StatusBaselinePreparation || aggregate.Version != 2 {
		t.Fatalf("恢复状态错误: %#v", aggregate)
	}
	events, err := reopened.LoadAudit(ctx, aggregate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("审计事件数=%d", len(events))
	}
	var envelope map[string]any
	if err := json.Unmarshal(first.Body, &envelope); err != nil {
		t.Fatal(err)
	}
}

func TestBatchRegistrationAndStableWorkQueuePaging(t *testing.T) {
	store, err := repository.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := NewService(store)
	fixed := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixed }
	ctx := context.Background()
	for index, number := range []string{"QUEUE-003", "QUEUE-001", "QUEUE-002"} {
		_, err := service.CreateCase(ctx, CreateCaseCommand{CommandMeta: CommandMeta{ExpectedVersion: 0,
			IdempotencyKey: "queue-create-000" + string(rune('1'+index)), Actor: "检验员", Role: RoleInspector},
			CaseNumber: number, VenueName: "分页剧院", Scope: "吊挂系统"})
		if err != nil {
			t.Fatal(err)
		}
	}
	first, err := service.GetWorkQueue(ctx, WorkQueueQuery{VenueName: "分页", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	var page1 workQueueEnvelope
	if err := json.Unmarshal(first.Body, &page1); err != nil {
		t.Fatal(err)
	}
	if len(page1.Items) != 2 || page1.Items[0].CaseNumber != "QUEUE-001" || page1.Pagination.NextCursor == "" {
		t.Fatalf("第一页不稳定: %s", first.Body)
	}
	second, err := service.GetWorkQueue(ctx, WorkQueueQuery{VenueName: "分页", Limit: 2,
		Cursor: page1.Pagination.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	var page2 workQueueEnvelope
	if err := json.Unmarshal(second.Body, &page2); err != nil {
		t.Fatal(err)
	}
	if len(page2.Items) != 1 || page2.Items[0].CaseNumber != "QUEUE-003" || page2.Pagination.NextCursor != "" {
		t.Fatalf("第二页不稳定: %s", second.Body)
	}

	prepare := CaseCommand{CommandMeta: CommandMeta{ExpectedVersion: 1, IdempotencyKey: "batch-prepare-001",
		Actor: "检验员", Role: RoleInspector}, CaseNumber: "QUEUE-001"}
	if _, err := service.PrepareBaseline(ctx, prepare); err != nil {
		t.Fatal(err)
	}
	batch := AddAssetsBatchCommand{CommandMeta: CommandMeta{ExpectedVersion: 2, IdempotencyKey: "batch-assets-0001",
		Actor: "检验员", Role: RoleInspector}, CaseNumber: "QUEUE-001", Assets: []domain.AssetInput{
		{AssetCode: " bat-01 ", AssetType: domain.AssetBatten, RatedLoadKg: 1000, BrakeDistanceLimitMm: 50},
		{AssetCode: "win-02", AssetType: domain.AssetWinch, RatedLoadKg: 800, BrakeDistanceLimitMm: 40},
	}}
	result, err := service.AddAssetsBatch(ctx, batch)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := service.AddAssetsBatch(ctx, batch)
	if err != nil || string(result.Body) != string(replay.Body) {
		t.Fatalf("批量幂等重放失败: %v", err)
	}
	aggregate, err := store.LoadCase(ctx, "QUEUE-001")
	if err != nil || aggregate.Version != 3 || len(aggregate.Assets) != 2 {
		t.Fatalf("批量持久化错误: %#v %v", aggregate, err)
	}
}

func TestRoleAuthorization(t *testing.T) {
	store, err := repository.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := NewService(store)
	_, err = service.CreateCase(context.Background(), CreateCaseCommand{CommandMeta: CommandMeta{
		ExpectedVersion: 0, IdempotencyKey: "unauthorized-01", Actor: "维护员", Role: RoleMaintenance},
		CaseNumber: "AUTH-001", VenueName: "剧院", Scope: "吊杆"})
	if domain.ErrorCodeOf(err) != domain.CodeForbidden {
		t.Fatalf("期望权限错误，得到 %v", err)
	}
}
