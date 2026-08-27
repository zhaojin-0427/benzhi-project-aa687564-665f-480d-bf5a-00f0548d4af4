package concurrentqueryviewbuffer_test

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"testing"

	"stage-rigging-clearance/internal/application"
	"stage-rigging-clearance/internal/repository"
)

func TestConcurrentQueriesKeepSerializationStateIsolated(t *testing.T) {
	previousProcs := runtime.GOMAXPROCS(4)
	t.Cleanup(func() { runtime.GOMAXPROCS(previousProcs) })

	store, err := repository.Open(t.TempDir() + "/queries.db")
	if err != nil {
		t.Fatalf("打开测试存储: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service := application.NewService(store)
	ctx := context.Background()

	const caseCount = 64
	for index := 0; index < caseCount; index++ {
		_, err := service.CreateCase(ctx, application.CreateCaseCommand{
			CommandMeta: application.CommandMeta{
				ExpectedVersion: 0,
				IdempotencyKey:  fmt.Sprintf("concurrent-query-case-%03d", index),
				Actor:           "并发复现检验员",
				Role:            application.RoleInspector,
			},
			CaseNumber: fmt.Sprintf("RIG-CONCURRENT-%03d", index),
			VenueName:  "并发查询剧场",
			Scope:      "验证并发只读响应的序列化状态隔离",
		})
		if err != nil {
			t.Fatalf("创建第 %d 个档案: %v", index, err)
		}
	}

	start := make(chan struct{})
	failures := make(chan error, 12)
	var workers sync.WaitGroup
	for worker := 0; worker < 6; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			for attempt := 0; attempt < 20; attempt++ {
				result, err := service.GetWorkQueue(ctx, application.WorkQueueQuery{Limit: 100})
				if err != nil {
					failures <- fmt.Errorf("工作队列查询失败: %w", err)
					return
				}
				var response struct {
					Items []json.RawMessage `json:"items"`
				}
				if err := json.Unmarshal(result.Body, &response); err != nil {
					failures <- fmt.Errorf("工作队列响应不是完整 JSON: %w", err)
					return
				}
				if len(response.Items) != caseCount {
					failures <- fmt.Errorf("工作队列响应串线: items=%d", len(response.Items))
					return
				}
			}
		}()
	}
	for worker := 0; worker < 6; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			for attempt := 0; attempt < 20; attempt++ {
				result, err := service.GetTestCoverage(ctx, "RIG-CONCURRENT-000")
				if err != nil {
					failures <- fmt.Errorf("覆盖率查询失败: %w", err)
					return
				}
				var response struct {
					CaseNumber string `json:"caseNumber"`
				}
				if err := json.Unmarshal(result.Body, &response); err != nil {
					failures <- fmt.Errorf("覆盖率响应不是完整 JSON: %w", err)
					return
				}
				if response.CaseNumber != "RIG-CONCURRENT-000" {
					failures <- fmt.Errorf("覆盖率响应串线: caseNumber=%q", response.CaseNumber)
					return
				}
			}
		}()
	}

	close(start)
	workers.Wait()
	close(failures)
	for failure := range failures {
		t.Error(failure)
	}
}
