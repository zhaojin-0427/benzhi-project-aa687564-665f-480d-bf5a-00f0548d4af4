package concurrentloadcasestatementinit_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"stage-rigging-clearance/internal/application"
	"stage-rigging-clearance/internal/httpapi"
	"stage-rigging-clearance/internal/repository"
)

func TestConcurrentCaseReadsPublishStatementSafely(t *testing.T) {
	store, err := repository.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	service := application.NewService(store)
	_, err = service.CreateCase(context.Background(), application.CreateCaseCommand{
		CommandMeta: application.CommandMeta{
			ExpectedVersion: 0,
			IdempotencyKey:  "statement-create-0001",
			Actor:           "并发检验员",
			Role:            application.RoleInspector,
		},
		CaseNumber: "STMT-RACE-001",
		VenueName:  "并发初始化剧院",
		Scope:      "吊挂系统",
	})
	if err != nil {
		t.Fatal(err)
	}

	handler := httpapi.New(service).Handler()
	start := make(chan struct{})
	errors := make(chan error, 64)
	var ready sync.WaitGroup
	var finished sync.WaitGroup
	ready.Add(64)
	finished.Add(64)
	for index := 0; index < 64; index++ {
		go func() {
			defer finished.Done()
			request := httptest.NewRequest(http.MethodGet,
				"/api/v1/inspection-cases/STMT-RACE-001", nil)
			recorder := httptest.NewRecorder()
			ready.Done()
			<-start
			handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK {
				errors <- fmt.Errorf("GET status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		}()
	}
	ready.Wait()
	close(start)
	finished.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}
