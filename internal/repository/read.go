package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"stage-rigging-clearance/internal/domain"
)

func (s *Store) LoadCase(ctx context.Context, caseNumber string) (*domain.InspectionCase, error) {
	var data []byte
	err := s.db.QueryRowContext(ctx, "SELECT aggregate_json FROM inspection_cases WHERE case_number = ?", caseNumber).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.NewRuleError(domain.CodeNotFound, "检验档案不存在")
	}
	if err != nil {
		return nil, fmt.Errorf("读取检验档案: %w", err)
	}
	var aggregate domain.InspectionCase
	if err := json.Unmarshal(data, &aggregate); err != nil {
		return nil, fmt.Errorf("重建检验档案: %w", err)
	}
	if err := aggregate.ValidateIntegrity(); err != nil {
		return nil, err
	}
	return &aggregate, nil
}

func (s *Store) FindIdempotency(ctx context.Context, key string) (*IdempotencyRecord, error) {
	var result IdempotencyRecord
	var created string
	err := s.db.QueryRowContext(ctx, `SELECT key, case_number, command, fingerprint, status_code, response, created_at
		FROM idempotency_records WHERE key = ?`, key).Scan(&result.Key, &result.CaseNumber,
		&result.Command, &result.Fingerprint, &result.StatusCode, &result.Response, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	result.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	return &result, err
}

func (s *Store) FindIdempotencyByCase(ctx context.Context, caseNumber string) ([]IdempotencyRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, case_number, command, fingerprint, status_code, response, created_at
		FROM idempotency_records WHERE case_number = ?`, caseNumber)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []IdempotencyRecord{}
	for rows.Next() {
		var record IdempotencyRecord
		var created string
		if err := rows.Scan(&record.Key, &record.CaseNumber, &record.Command, &record.Fingerprint,
			&record.StatusCode, &record.Response, &created); err != nil {
			return nil, err
		}
		parsed, err := time.Parse(time.RFC3339Nano, created)
		if err != nil {
			return nil, err
		}
		record.CreatedAt = parsed
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) LoadAudit(ctx context.Context, caseID string) ([]domain.AuditEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT case_id, sequence, actor, role, command, before_version,
		after_version, occurred_at, result_digest, previous_hash, hash FROM audit_events
		WHERE case_id = ? ORDER BY sequence`, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []domain.AuditEvent{}
	for rows.Next() {
		var event domain.AuditEvent
		var occurred string
		if err := rows.Scan(&event.CaseID, &event.Sequence, &event.Actor, &event.Role, &event.Command,
			&event.BeforeVersion, &event.AfterVersion, &occurred, &event.ResultDigest,
			&event.PreviousHash, &event.Hash); err != nil {
			return nil, err
		}
		event.OccurredAt, err = time.Parse(time.RFC3339Nano, occurred)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Store) LoadAuditByCaseNumber(ctx context.Context, caseNumber string) ([]domain.AuditEvent, error) {
	var caseID string
	err := s.db.QueryRowContext(ctx, "SELECT id FROM inspection_cases WHERE case_number = ?", caseNumber).Scan(&caseID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.LoadAudit(ctx, caseID)
}

func (s *Store) LoadCaseByCertificateNumber(ctx context.Context, certificateNumber string) (*domain.InspectionCase, error) {
	var caseNumber string
	err := s.db.QueryRowContext(ctx, `SELECT i.case_number FROM certificates c
		JOIN inspection_cases i ON i.id=c.case_id WHERE c.certificate_number=?`, certificateNumber).Scan(&caseNumber)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	aggregate, err := s.LoadCase(ctx, caseNumber)
	if err != nil {
		return nil, err
	}
	if err := s.validateMaterializedView(ctx, aggregate); err != nil {
		return nil, domain.NewRuleError(domain.CodeIntegrity, "凭据持久化完整性校验失败: %v", err)
	}
	return aggregate, nil
}
