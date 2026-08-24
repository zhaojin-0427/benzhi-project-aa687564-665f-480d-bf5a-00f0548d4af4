package audit

import (
	"testing"
	"time"

	"stage-rigging-clearance/internal/domain"
)

func TestAuditHashChainDetectsTampering(t *testing.T) {
	now := time.Date(2026, 8, 25, 1, 2, 3, 0, time.UTC)
	first := NewEvent("case-1", "甲", "inspector", "create_case", 0, 1, "result-a", nil, now)
	second := NewEvent("case-1", "乙", "inspector", "prepare_baseline", 1, 2, "result-b", &first, now.Add(time.Second))
	events := []domain.AuditEvent{first, second}
	if err := Verify(events); err != nil {
		t.Fatal(err)
	}
	events[0].Actor = "篡改者"
	if err := Verify(events); err == nil {
		t.Fatal("篡改后审计链仍然通过")
	}
}
