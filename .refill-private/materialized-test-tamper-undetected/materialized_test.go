package materializedtest_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	"stage-rigging-clearance/internal/application"
	"stage-rigging-clearance/internal/domain"
	"stage-rigging-clearance/internal/repository"

	_ "modernc.org/sqlite"
)

func TestOpenRejectsTamperedMaterializedTestOwner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tampered.db")
	store, err := repository.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(store)
	ctx := context.Background()

	create, err := service.CreateCase(ctx, application.CreateCaseCommand{
		CommandMeta: application.CommandMeta{ExpectedVersion: 0, IdempotencyKey: "create-owner-case", Actor: "inspector-a", Role: application.RoleInspector},
		CaseNumber:  "OWNER-CASE", VenueName: "Integrity Venue", Scope: "Integrity Scope",
	})
	if err != nil {
		t.Fatal(err)
	}
	created := decodeCase(t, create.Body)
	if _, err := service.PrepareBaseline(ctx, application.CaseCommand{CommandMeta: application.CommandMeta{
		ExpectedVersion: created.Version, IdempotencyKey: "prepare-owner-case", Actor: "inspector-a", Role: application.RoleInspector,
	}, CaseNumber: created.CaseNumber}); err != nil {
		t.Fatal(err)
	}
	added, err := service.AddAsset(ctx, application.AddAssetCommand{CommandMeta: application.CommandMeta{
		ExpectedVersion: 2, IdempotencyKey: "add-owner-asset", Actor: "inspector-a", Role: application.RoleInspector,
	}, CaseNumber: created.CaseNumber, Asset: domain.AssetInput{AssetCode: "WINCH-1", AssetType: domain.AssetWinch,
		RatedLoadKg: 100, BrakeDistanceLimitMm: 20, LimitDeviceRequired: true}})
	if err != nil {
		t.Fatal(err)
	}
	withAsset := decodeCase(t, added.Body)
	if _, err := service.LockBaseline(ctx, application.CaseCommand{CommandMeta: application.CommandMeta{
		ExpectedVersion: withAsset.Version, IdempotencyKey: "lock-owner-case", Actor: "inspector-a", Role: application.RoleInspector,
	}, CaseNumber: created.CaseNumber}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RecordTest(ctx, application.RecordTestCommand{CommandMeta: application.CommandMeta{
		ExpectedVersion: 4, IdempotencyKey: "record-owner-test", Actor: "inspector-a", Role: application.RoleInspector,
	}, CaseNumber: created.CaseNumber, Test: domain.TestInput{AssetID: withAsset.Assets[0].ID, TestKind: domain.TestStatic,
		AppliedLoadKg: 125}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	raw, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`UPDATE load_test_records SET recorded_by='intruder'`); err != nil {
		raw.Close()
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := repository.Open(path)
	if err == nil {
		reopened.Close()
		t.Fatalf("TestOpenRejectsTamperedMaterializedTestOwner: startup accepted recorded_by that disagrees with aggregate_json")
	}
}

func decodeCase(t *testing.T, body []byte) *domain.InspectionCase {
	t.Helper()
	var envelope struct {
		Case *domain.InspectionCase `json:"case"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Case == nil {
		t.Fatal("missing case")
	}
	return envelope.Case
}
