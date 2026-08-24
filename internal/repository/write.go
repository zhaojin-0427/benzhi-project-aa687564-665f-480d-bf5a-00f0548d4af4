package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"stage-rigging-clearance/internal/domain"
)

func (s *Store) Commit(ctx context.Context, expectedVersion int64, aggregate *domain.InspectionCase, idem IdempotencyRecord, event domain.AuditEvent) error {
	if s.readOnly {
		return domain.NewRuleError(domain.CodeIntegrity, "存储处于只读保护状态")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if existing, err := findIdempotencyTx(ctx, tx, idem.Key); err != nil {
		return err
	} else if existing != nil {
		if existing.Fingerprint == idem.Fingerprint {
			return nil
		}
		return domain.NewRuleError(domain.CodeIdempotencyReuse, "idempotencyKey 已用于不同请求")
	}
	encoded, err := json.Marshal(aggregate)
	if err != nil {
		return err
	}
	if expectedVersion == 0 {
		_, err = tx.ExecContext(ctx, `INSERT INTO inspection_cases
			(id, case_number, venue_name, scope, status, version, created_at, updated_at, aggregate_json)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`, aggregate.ID, aggregate.CaseNumber, aggregate.VenueName,
			aggregate.Scope, aggregate.Status, aggregate.Version, formatTime(aggregate.CreatedAt),
			formatTime(aggregate.UpdatedAt), encoded)
		if err != nil {
			if isUniqueError(err) {
				return domain.NewRuleError(domain.CodeConflict, "档案编号或幂等键已存在")
			}
			return err
		}
	} else {
		result, err := tx.ExecContext(ctx, `UPDATE inspection_cases SET venue_name=?, scope=?, status=?,
			version=?, updated_at=?, aggregate_json=? WHERE id=? AND version=?`, aggregate.VenueName,
			aggregate.Scope, aggregate.Status, aggregate.Version, formatTime(aggregate.UpdatedAt), encoded,
			aggregate.ID, expectedVersion)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil || rows != 1 {
			return domain.NewRuleError(domain.CodeConflict, "expectedVersion 与当前版本不一致")
		}
	}
	if err := replaceDetails(ctx, tx, aggregate); err != nil {
		return err
	}
	if err := insertAudit(ctx, tx, event); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO idempotency_records
		(key, case_number, command, fingerprint, status_code, response, created_at) VALUES(?, ?, ?, ?, ?, ?, ?)`,
		idem.Key, aggregate.CaseNumber, idem.Command, idem.Fingerprint, idem.StatusCode, idem.Response,
		formatTime(idem.CreatedAt))
	if err != nil {
		return err
	}
	return tx.Commit()
}

func replaceDetails(ctx context.Context, tx *sql.Tx, aggregate *domain.InspectionCase) error {
	for _, table := range []string{"test_retest_links", "defect_review_decisions", "defect_evidence_versions",
		"load_test_records", "defects", "rigging_assets", "frozen_reports", "certificates"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE case_id = ?", aggregate.ID); err != nil {
			return err
		}
	}
	for _, asset := range aggregate.Assets {
		var locked any
		if asset.BaselineLockedAt != nil {
			locked = formatTime(*asset.BaselineLockedAt)
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO rigging_assets VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
			asset.ID, asset.CaseID, asset.AssetCode, asset.AssetType, asset.RatedLoadKg,
			asset.BrakeDistanceLimitMm, asset.LimitDeviceRequired, locked)
		if err != nil {
			return err
		}
	}
	for _, record := range aggregate.Tests {
		_, err := tx.ExecContext(ctx, `INSERT INTO load_test_records VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			record.ID, record.CaseID, record.AssetID, record.TestKind, record.AppliedLoadKg,
			record.BrakeDistanceMm, record.LimitTriggered, record.Result, record.FailureReason,
			record.RecordedBy, formatTime(record.RecordedAt))
		if err != nil {
			return err
		}
	}
	for _, record := range aggregate.Tests {
		if record.RetestOfRecordID == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO test_retest_links VALUES(?, ?, ?)`,
			record.ID, aggregate.ID, record.RetestOfRecordID); err != nil {
			return err
		}
	}
	for _, defect := range aggregate.Defects {
		var resolved any
		if defect.ResolvedAt != nil {
			resolved = formatTime(*defect.ResolvedAt)
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO defects VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			defect.ID, defect.CaseID, defect.AssetID, nullable(defect.SourceRecordID), defect.Severity,
			defect.Description, defect.Status, defect.RemediationEvidence, defect.ReviewComment, resolved)
		if err != nil {
			return err
		}
		for _, evidence := range defect.EvidenceVersions {
			if _, err := tx.ExecContext(ctx, `INSERT INTO defect_evidence_versions VALUES(?, ?, ?, ?, ?, ?)`,
				defect.ID, aggregate.ID, evidence.Version, evidence.SubmittedBy,
				formatTime(evidence.SubmittedAt), evidence.Content); err != nil {
				return err
			}
		}
		for index, decision := range defect.ReviewDecisions {
			if _, err := tx.ExecContext(ctx, `INSERT INTO defect_review_decisions VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
				defect.ID, aggregate.ID, index+1, decision.EvidenceVersion, decision.Reviewer,
				decision.Accepted, decision.Comment, formatTime(decision.DecidedAt)); err != nil {
				return err
			}
		}
	}
	if aggregate.Report != nil {
		r := aggregate.Report
		_, err := tx.ExecContext(ctx, `INSERT INTO frozen_reports VALUES(?, ?, ?, ?, ?, ?)`, aggregate.ID,
			r.Digest, r.Content, r.FrozenBy, formatTime(r.FrozenAt), r.Version)
		if err != nil {
			return err
		}
	}
	if aggregate.Certificate != nil {
		c := aggregate.Certificate
		_, err := tx.ExecContext(ctx, `INSERT INTO certificates VALUES(?, ?, ?, ?, ?, ?, ?)`, c.ID,
			c.CaseID, c.ReportDigest, c.CertificateNumber, c.IssuedBy, formatTime(c.IssuedAt), c.VerificationDigest)
		if err != nil {
			return err
		}
	}
	return nil
}

func insertAudit(ctx context.Context, tx *sql.Tx, event domain.AuditEvent) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO audit_events VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.CaseID, event.Sequence, event.Actor, event.Role, event.Command, event.BeforeVersion,
		event.AfterVersion, formatTime(event.OccurredAt), event.ResultDigest, event.PreviousHash, event.Hash)
	return err
}

func findIdempotencyTx(ctx context.Context, tx *sql.Tx, key string) (*IdempotencyRecord, error) {
	var value IdempotencyRecord
	err := tx.QueryRowContext(ctx, `SELECT key, fingerprint FROM idempotency_records WHERE key=?`, key).
		Scan(&value.Key, &value.Fingerprint)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &value, err
}

func formatTime(value time.Time) string {
	return value.UTC().Format("2006-01-02T15:04:05.000000000Z07:00")
}
func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func isUniqueError(err error) bool {
	return err != nil && (contains(err.Error(), "UNIQUE") || contains(err.Error(), "constraint failed"))
}
func contains(value, part string) bool {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return true
		}
	}
	return false
}
