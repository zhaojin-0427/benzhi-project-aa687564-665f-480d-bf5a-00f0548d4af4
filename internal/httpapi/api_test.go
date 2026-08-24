package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"stage-rigging-clearance/internal/application"
	"stage-rigging-clearance/internal/repository"
)

func TestCreateAndQueryCaseAPI(t *testing.T) {
	store, err := repository.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	handler := New(application.NewService(store)).Handler()
	body := []byte(`{"expectedVersion":0,"idempotencyKey":"http-create-0001","actor":"检验员","role":"inspector","caseNumber":"HTTP-001","venueName":"城市剧院","scope":"主舞台"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/inspection-cases", bytes.NewReader(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("建档响应=%d: %s", response.Code, response.Body.String())
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/inspection-cases/HTTP-001", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"caseNumber":"HTTP-001"`)) {
		t.Fatalf("查询失败: %d %s", response.Code, response.Body.String())
	}
}

func TestProblemDetailsAndStrictJSON(t *testing.T) {
	store, err := repository.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	handler := New(application.NewService(store)).Handler()
	body := []byte(`{"expectedVersion":0,"unknown":true}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/inspection-cases", bytes.NewReader(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || response.Header().Get("Content-Type") != "application/problem+json; charset=utf-8" {
		t.Fatalf("问题详情错误: %d %s", response.Code, response.Body.String())
	}
}
