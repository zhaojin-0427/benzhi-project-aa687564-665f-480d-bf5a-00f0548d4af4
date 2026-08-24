package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"stage-rigging-clearance/internal/domain"
)

// validateMaterializedView ensures the queryable relational records still describe
// the same immutable facts as the aggregate snapshot. A mismatch indicates manual
// database modification or an interrupted migration and must block future writes.
func (s *Store) validateMaterializedView(ctx context.Context, aggregate *domain.InspectionCase) error {
	counts := []struct {
		table    string
		expected int
	}{
		{table: "rigging_assets", expected: len(aggregate.Assets)},
		{table: "load_test_records", expected: len(aggregate.Tests)},
		{table: "defects", expected: len(aggregate.Defects)},
	}
	for _, item := range counts {
		var actual int
		query := "SELECT COUNT(*) FROM " + item.table + " WHERE case_id = ?"
		if err := s.db.QueryRowContext(ctx, query, aggregate.ID).Scan(&actual); err != nil {
			return err
		}
		if actual != item.expected {
			return fmt.Errorf("档案 %s 的 %s 明细数量不一致: 快照=%d, 明细=%d",
				aggregate.CaseNumber, item.table, item.expected, actual)
		}
	}
	if err := s.validateAssets(ctx, aggregate); err != nil {
		return err
	}
	if err := s.validateTests(ctx, aggregate); err != nil {
		return err
	}
	if err := s.validateRetestLinks(ctx, aggregate); err != nil {
		return err
	}
	if err := s.validateDefects(ctx, aggregate); err != nil {
		return err
	}
	if err := s.validateDefectHistory(ctx, aggregate); err != nil {
		return err
	}
	return s.validateFrozenArtifacts(ctx, aggregate)
}

func (s *Store) validateRetestLinks(ctx context.Context, aggregate *domain.InspectionCase) error {
	expected := map[string]string{}
	for _, record := range aggregate.Tests {
		if record.RetestOfRecordID != "" {
			expected[record.ID] = record.RetestOfRecordID
		}
	}
	rows, err := s.db.QueryContext(ctx, `SELECT record_id, original_record_id FROM test_retest_links WHERE case_id=?`, aggregate.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	seen := 0
	for rows.Next() {
		var id, original string
		if err := rows.Scan(&id, &original); err != nil {
			return err
		}
		if expected[id] != original {
			return fmt.Errorf("复测关联 %s 与档案快照不一致", id)
		}
		seen++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if seen != len(expected) {
		return fmt.Errorf("复测关联数量与档案快照不一致")
	}
	return nil
}

func (s *Store) validateDefectHistory(ctx context.Context, aggregate *domain.InspectionCase) error {
	evidenceCount, decisionCount := 0, 0
	for _, defect := range aggregate.Defects {
		evidenceCount += len(defect.EvidenceVersions)
		decisionCount += len(defect.ReviewDecisions)
		for _, evidence := range defect.EvidenceVersions {
			var submittedBy, submittedAt, content string
			err := s.db.QueryRowContext(ctx, `SELECT submitted_by, submitted_at, content
				FROM defect_evidence_versions WHERE defect_id=? AND version=?`, defect.ID, evidence.Version).
				Scan(&submittedBy, &submittedAt, &content)
			if err != nil || submittedBy != evidence.SubmittedBy || submittedAt != formatTime(evidence.SubmittedAt) || content != evidence.Content {
				return fmt.Errorf("缺陷 %s 的证据版本 %d 与档案快照不一致", defect.ID, evidence.Version)
			}
		}
		for index, decision := range defect.ReviewDecisions {
			var evidenceVersion int
			var reviewer, comment, decidedAt string
			var accepted bool
			err := s.db.QueryRowContext(ctx, `SELECT evidence_version, reviewer, accepted, comment, decided_at
				FROM defect_review_decisions WHERE defect_id=? AND sequence=?`, defect.ID, index+1).
				Scan(&evidenceVersion, &reviewer, &accepted, &comment, &decidedAt)
			if err != nil || evidenceVersion != decision.EvidenceVersion || reviewer != decision.Reviewer ||
				accepted != decision.Accepted || comment != decision.Comment || decidedAt != formatTime(decision.DecidedAt) {
				return fmt.Errorf("缺陷 %s 的逐项复核决定 %d 与档案快照不一致", defect.ID, index+1)
			}
		}
	}
	for table, expected := range map[string]int{"defect_evidence_versions": evidenceCount, "defect_review_decisions": decisionCount} {
		var actual int
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+" WHERE case_id=?", aggregate.ID).Scan(&actual); err != nil {
			return err
		}
		if actual != expected {
			return fmt.Errorf("档案 %s 的 %s 历史数量不一致", aggregate.CaseNumber, table)
		}
	}
	return nil
}

func (s *Store) validateAssets(ctx context.Context, aggregate *domain.InspectionCase) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id, asset_code, asset_type, rated_load_kg,
		brake_distance_limit_mm, limit_device_required FROM rigging_assets WHERE case_id=? ORDER BY id`, aggregate.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	expected := make(map[string]domain.RiggingAsset, len(aggregate.Assets))
	for _, asset := range aggregate.Assets {
		expected[asset.ID] = asset
	}
	for rows.Next() {
		var id, code string
		var assetType domain.AssetType
		var rated, distance float64
		var required bool
		if err := rows.Scan(&id, &code, &assetType, &rated, &distance, &required); err != nil {
			return err
		}
		asset, ok := expected[id]
		if !ok || asset.AssetCode != code || asset.AssetType != assetType ||
			asset.RatedLoadKg != rated || asset.BrakeDistanceLimitMm != distance || asset.LimitDeviceRequired != required {
			return fmt.Errorf("设备明细 %s 与档案快照不一致", id)
		}
	}
	return rows.Err()
}

func (s *Store) validateTests(ctx context.Context, aggregate *domain.InspectionCase) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id, asset_id, test_kind, applied_load_kg,
		brake_distance_mm, limit_triggered, result FROM load_test_records WHERE case_id=? ORDER BY id`, aggregate.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	expected := make(map[string]domain.LoadTestRecord, len(aggregate.Tests))
	for _, record := range aggregate.Tests {
		expected[record.ID] = record
	}
	for rows.Next() {
		var id, assetID string
		var kind domain.TestKind
		var applied, distance float64
		var triggered bool
		var result domain.TestResult
		if err := rows.Scan(&id, &assetID, &kind, &applied, &distance, &triggered, &result); err != nil {
			return err
		}
		record, ok := expected[id]
		if !ok || record.AssetID != assetID || record.TestKind != kind || record.AppliedLoadKg != applied ||
			record.BrakeDistanceMm != distance || record.LimitTriggered != triggered || record.Result != result {
			return fmt.Errorf("测试明细 %s 与档案快照不一致", id)
		}
	}
	return rows.Err()
}

func (s *Store) validateDefects(ctx context.Context, aggregate *domain.InspectionCase) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id, asset_id, severity, description, status,
		remediation_evidence FROM defects WHERE case_id=? ORDER BY id`, aggregate.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	expected := make(map[string]domain.Defect, len(aggregate.Defects))
	for _, defect := range aggregate.Defects {
		expected[defect.ID] = defect
	}
	for rows.Next() {
		var id, assetID, description, evidence string
		var severity domain.Severity
		var status domain.DefectStatus
		if err := rows.Scan(&id, &assetID, &severity, &description, &status, &evidence); err != nil {
			return err
		}
		defect, ok := expected[id]
		if !ok || defect.AssetID != assetID || defect.Severity != severity || defect.Description != description ||
			defect.Status != status || defect.RemediationEvidence != evidence {
			return fmt.Errorf("缺陷明细 %s 与档案快照不一致", id)
		}
	}
	return rows.Err()
}

func (s *Store) validateFrozenArtifacts(ctx context.Context, aggregate *domain.InspectionCase) error {
	var reportDigest, reportContent string
	reportErr := s.db.QueryRowContext(ctx, "SELECT digest, content FROM frozen_reports WHERE case_id=?", aggregate.ID).Scan(&reportDigest, &reportContent)
	if aggregate.Report == nil {
		if reportErr == nil {
			return fmt.Errorf("未冻结档案存在报告明细")
		}
		if reportErr != sql.ErrNoRows {
			return reportErr
		}
	} else if reportErr != nil || reportDigest != aggregate.Report.Digest || reportContent != aggregate.Report.Content {
		return fmt.Errorf("冻结报告明细与档案快照不一致")
	}
	var certificateID, reportRef, certificateNumber, issuedAt, verificationDigest string
	certificateErr := s.db.QueryRowContext(ctx, `SELECT id, report_digest, certificate_number, issued_at, verification_digest
		FROM certificates WHERE case_id=?`, aggregate.ID).Scan(&certificateID, &reportRef, &certificateNumber, &issuedAt, &verificationDigest)
	if aggregate.Certificate == nil {
		if certificateErr == nil {
			return fmt.Errorf("未签发档案存在凭据明细")
		}
		if certificateErr != sql.ErrNoRows {
			return certificateErr
		}
	} else if certificateErr != nil || certificateID != aggregate.Certificate.ID ||
		reportRef != aggregate.Certificate.ReportDigest || certificateNumber != aggregate.Certificate.CertificateNumber ||
		!sameStoredTime(issuedAt, aggregate.Certificate.IssuedAt) || verificationDigest != aggregate.Certificate.VerificationDigest {
		return fmt.Errorf("复役凭据明细与档案快照不一致")
	}
	return nil
}

func sameStoredTime(raw string, expected time.Time) bool {
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	return err == nil && parsed.Equal(expected)
}
