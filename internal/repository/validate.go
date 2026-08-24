package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"stage-rigging-clearance/internal/audit"
	"stage-rigging-clearance/internal/domain"
)

func (s *Store) Validate(ctx context.Context) error {
	var version int
	if err := s.db.QueryRowContext(ctx, "SELECT version FROM schema_meta WHERE id=1").Scan(&version); err != nil {
		return err
	}
	if version != schemaVersion {
		return fmt.Errorf("schemaVersion 不匹配")
	}
	var violations int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_foreign_key_check").Scan(&violations); err != nil {
		return err
	}
	if violations != 0 {
		return fmt.Errorf("检测到 %d 个外键关联错误", violations)
	}
	rows, err := s.db.QueryContext(ctx, "SELECT id, aggregate_json FROM inspection_cases ORDER BY id")
	if err != nil {
		return err
	}
	type snapshot struct {
		id  string
		raw []byte
	}
	snapshots := []snapshot{}
	for rows.Next() {
		var caseID string
		var raw []byte
		if err := rows.Scan(&caseID, &raw); err != nil {
			rows.Close()
			return err
		}
		snapshots = append(snapshots, snapshot{id: caseID, raw: append([]byte(nil), raw...)})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, saved := range snapshots {
		caseID, raw := saved.id, saved.raw
		var aggregate domain.InspectionCase
		if err := json.Unmarshal(raw, &aggregate); err != nil {
			return fmt.Errorf("档案 %s 快照损坏: %w", caseID, err)
		}
		if err := aggregate.ValidateIntegrity(); err != nil {
			return fmt.Errorf("档案 %s 的聚合完整性无效: %w", caseID, err)
		}
		if err := s.validateMaterializedView(ctx, &aggregate); err != nil {
			return err
		}
		events, err := s.LoadAudit(ctx, caseID)
		if err != nil {
			return err
		}
		if err := audit.Verify(events); err != nil {
			return fmt.Errorf("档案 %s 审计链无效: %w", caseID, err)
		}
		if len(events) == 0 || events[len(events)-1].AfterVersion != aggregate.Version {
			return fmt.Errorf("档案 %s 的审计版本与聚合版本不一致", caseID)
		}
	}
	return nil
}
