package certificate_view_cache_alias_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"stage-rigging-clearance/internal/application"
	"stage-rigging-clearance/internal/audit"
	"stage-rigging-clearance/internal/domain"
	"stage-rigging-clearance/internal/httpapi"
	"stage-rigging-clearance/internal/repository"
)

func TestCertificateCacheKeepsResponsesIsolated(t *testing.T) {
	store, err := repository.Open(t.TempDir() + "/cache-alias.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	persistCertifiedCase(t, store, buildCertifiedCase(t, "RIG-CACHE-A", now), now, "seed-cache-a")
	persistCertifiedCase(t, store, buildCertifiedCase(t, "RIG-CACHE-B", now.Add(time.Minute)), now.Add(time.Minute), "seed-cache-b")

	handler := httpapi.New(application.NewService(store)).Handler()
	first := getCertificate(t, handler, "RIG-CACHE-A")
	second := getCertificate(t, handler, "RIG-CACHE-B")
	third := getCertificate(t, handler, "RIG-CACHE-A")
	if first.CaseNumber != "RIG-CACHE-A" || second.CaseNumber != "RIG-CACHE-B" {
		t.Fatalf("cache setup returned unexpected cases: first=%q second=%q", first.CaseNumber, second.CaseNumber)
	}
	if third.CaseNumber != "RIG-CACHE-A" {
		t.Fatalf("cached certificate response crossed case ownership boundary: got %q", third.CaseNumber)
	}
	if third.Certificate == nil || third.Certificate.CaseNumber != "RIG-CACHE-A" {
		t.Fatalf("cached certificate belongs to another case: %#v", third.Certificate)
	}
}

type certificateResponse struct {
	CaseNumber  string                             `json:"caseNumber"`
	Certificate *domain.ReturnToServiceCertificate `json:"certificate"`
}

func getCertificate(t *testing.T, handler http.Handler, caseNumber string) certificateResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/inspection-cases/"+caseNumber+"/certificate", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET certificate %s: status=%d body=%s", caseNumber, recorder.Code, recorder.Body.String())
	}
	var response certificateResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode certificate %s: %v; body=%s", caseNumber, err, recorder.Body.String())
	}
	return response
}

func buildCertifiedCase(t *testing.T, caseNumber string, now time.Time) *domain.InspectionCase {
	t.Helper()
	aggregate, err := domain.NewInspectionCase(caseNumber, "缓存隔离测试剧院", "吊挂设备年度检验", now)
	must(t, err)
	must(t, aggregate.PrepareBaseline(now.Add(time.Second)))
	asset, err := aggregate.AddAsset(domain.AssetInput{AssetCode: "BATTEN-01", AssetType: domain.AssetBatten,
		RatedLoadKg: 1000, BrakeDistanceLimitMm: 120, LimitDeviceRequired: true}, now.Add(2*time.Second))
	must(t, err)
	must(t, aggregate.LockBaseline(now.Add(3*time.Second)))
	tests := []domain.TestInput{
		{AssetID: asset.ID, TestKind: domain.TestStatic, AppliedLoadKg: 1250, RecordedBy: "inspector-a"},
		{AssetID: asset.ID, TestKind: domain.TestDynamic, AppliedLoadKg: 1100, RecordedBy: "inspector-a"},
		{AssetID: asset.ID, TestKind: domain.TestBrake, BrakeDistanceMm: 100, RecordedBy: "inspector-a"},
		{AssetID: asset.ID, TestKind: domain.TestLimit, LimitTriggered: true, RecordedBy: "inspector-a"},
	}
	for index, input := range tests {
		_, _, err = aggregate.RecordTest(input, now.Add(time.Duration(4+index)*time.Second))
		must(t, err)
	}
	must(t, aggregate.SubmitForReview(now.Add(8*time.Second)))
	must(t, aggregate.ApproveReview("reviewer-b", "全部测试满足复役门槛", now.Add(9*time.Second)))
	_, err = aggregate.FreezeReport("reviewer-b", now.Add(10*time.Second))
	must(t, err)
	_, err = aggregate.IssueCertificate("reviewer-b", now.Add(11*time.Second))
	must(t, err)
	return aggregate
}

func persistCertifiedCase(t *testing.T, store *repository.Store, aggregate *domain.InspectionCase, now time.Time, key string) {
	t.Helper()
	body, err := json.Marshal(struct {
		Case *domain.InspectionCase `json:"case"`
	}{Case: aggregate})
	must(t, err)
	digest := sha256.Sum256(body)
	event := audit.NewEvent(aggregate.ID, "seed", application.RoleReviewer, "seed_certified_case", 0,
		aggregate.Version, hex.EncodeToString(digest[:]), nil, now)
	idem := repository.IdempotencyRecord{Key: key, CaseNumber: aggregate.CaseNumber, Command: "seed_certified_case",
		Fingerprint: key, StatusCode: http.StatusCreated, Response: body, CreatedAt: now}
	must(t, store.Commit(context.Background(), 0, aggregate, idem, event))
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
