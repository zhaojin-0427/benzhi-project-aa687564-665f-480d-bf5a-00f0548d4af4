package crossservicecasecachestale_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"stage-rigging-clearance/internal/application"
	"stage-rigging-clearance/internal/httpapi"
	"stage-rigging-clearance/internal/repository"
)

type caseResponse struct {
	Case struct {
		Status  string `json:"status"`
		Version int64  `json:"version"`
	} `json:"case"`
}

func TestCrossServiceMutationInvalidatesCaseCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.db")
	readerStore, err := repository.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer readerStore.Close()
	writerStore, err := repository.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer writerStore.Close()

	reader := httpapi.New(application.NewService(readerStore)).Handler()
	writer := httpapi.New(application.NewService(writerStore)).Handler()
	postJSON(t, writer, "/api/v1/inspection-cases", `{"expectedVersion":0,"idempotencyKey":"cache-create-0001","actor":"检验员","role":"inspector","caseNumber":"CACHE-001","venueName":"缓存测试剧院","scope":"主舞台吊挂"}`, http.StatusCreated)

	first := getCase(t, reader, "/api/v1/inspection-cases/CACHE-001")
	if first.Case.Version != 1 || first.Case.Status != "draft" {
		t.Fatalf("首次查询状态异常: version=%d status=%s", first.Case.Version, first.Case.Status)
	}
	postJSON(t, writer, "/api/v1/inspection-cases/CACHE-001/prepare", `{"expectedVersion":1,"idempotencyKey":"cache-prepare-001","actor":"检验员","role":"inspector"}`, http.StatusOK)

	second := getCase(t, reader, "/api/v1/inspection-cases/CACHE-001")
	if second.Case.Version != 2 || second.Case.Status != "baseline_preparation" {
		t.Fatalf("另一 Service 已提交 version=2 后仍返回缓存状态: version=%d status=%s", second.Case.Version, second.Case.Status)
	}
}

func postJSON(t *testing.T, handler http.Handler, path, body string, want int) {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != want {
		t.Fatalf("POST %s 返回 %d: %s", path, response.Code, response.Body.String())
	}
}

func getCase(t *testing.T, handler http.Handler, path string) caseResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("GET %s 返回 %d: %s", path, response.Code, response.Body.String())
	}
	var result caseResponse
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}
