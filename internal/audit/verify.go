package audit

import (
	"fmt"

	"stage-rigging-clearance/internal/domain"
)

func Verify(events []domain.AuditEvent) error {
	previousHash := ""
	for index, event := range events {
		expectedSequence := int64(index + 1)
		if event.Sequence != expectedSequence {
			return fmt.Errorf("审计序号不连续: 期望 %d，实际 %d", expectedSequence, event.Sequence)
		}
		if event.PreviousHash != previousHash {
			return fmt.Errorf("审计事件 %d 的前序摘要不匹配", event.Sequence)
		}
		if Hash(event) != event.Hash {
			return fmt.Errorf("审计事件 %d 的摘要无效", event.Sequence)
		}
		if event.AfterVersion <= event.BeforeVersion {
			return fmt.Errorf("审计事件 %d 的版本变化无效", event.Sequence)
		}
		previousHash = event.Hash
	}
	return nil
}
