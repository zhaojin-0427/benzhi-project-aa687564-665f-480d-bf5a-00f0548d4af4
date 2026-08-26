package canceled_read_recovery_context_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"stage-rigging-clearance/internal/application"
	"stage-rigging-clearance/internal/repository"

	_ "modernc.org/sqlite"
)

type observedContext struct {
	context.Context
	doneObserved chan struct{}
	errObserved  chan struct{}
	doneOnce     sync.Once
	errOnce      sync.Once
}

func (c *observedContext) Done() <-chan struct{} {
	c.doneOnce.Do(func() { close(c.doneObserved) })
	return c.Context.Done()
}

func (c *observedContext) Err() error {
	err := c.Context.Err()
	if err != nil {
		c.errOnce.Do(func() { close(c.errObserved) })
	}
	return err
}

func TestCanceledCaseReadIsNotRevivedByRecovery(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "inspection.db")
	store, err := repository.Open(databasePath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	service := application.NewService(store)
	_, err = service.CreateCase(context.Background(), application.CreateCaseCommand{
		CommandMeta: application.CommandMeta{ExpectedVersion: 0, IdempotencyKey: "create-context-case",
			Actor: "inspector-a", Role: application.RoleInspector},
		CaseNumber: "CTX-RECOVERY-001", VenueName: "取消传播剧场", Scope: "舞台吊挂系统",
	})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}

	blocker, err := sql.Open("sqlite", "file:"+databasePath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("open blocker: %v", err)
	}
	defer blocker.Close()
	lockConnection, err := blocker.Conn(context.Background())
	if err != nil {
		t.Fatalf("acquire blocker connection: %v", err)
	}
	defer lockConnection.Close()
	if _, err := lockConnection.ExecContext(context.Background(), "BEGIN EXCLUSIVE"); err != nil {
		t.Fatalf("begin exclusive transaction: %v", err)
	}
	locked := true
	defer func() {
		if locked {
			_, _ = lockConnection.ExecContext(context.Background(), "ROLLBACK")
		}
	}()

	baseContext, cancel := context.WithCancel(context.Background())
	requestContext := &observedContext{Context: baseContext, doneObserved: make(chan struct{}),
		errObserved: make(chan struct{})}
	type outcome struct {
		result *application.Result
		err    error
	}
	completed := make(chan outcome, 1)
	go func() {
		result, readErr := service.GetCase(requestContext, "CTX-RECOVERY-001")
		completed <- outcome{result: result, err: readErr}
	}()

	<-requestContext.doneObserved
	cancel()
	<-requestContext.errObserved
	if _, err := lockConnection.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatalf("release exclusive transaction: %v", err)
	}
	locked = false

	read := <-completed
	if read.err == nil {
		t.Fatalf("canceled read unexpectedly returned a successful result: %s", read.result.Body)
	}
	if !errors.Is(read.err, context.Canceled) {
		t.Fatalf("canceled read lost context error: %v", read.err)
	}
}
