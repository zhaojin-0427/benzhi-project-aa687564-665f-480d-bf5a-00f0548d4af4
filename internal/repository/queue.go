package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"stage-rigging-clearance/internal/domain"
)

type CaseFilter struct {
	Statuses        []domain.CaseStatus
	VenueName       string
	UpdatedFrom     *time.Time
	UpdatedTo       *time.Time
	HighestSeverity domain.Severity
	Snapshot        time.Time
}

type CasePageCursor struct {
	UpdatedAt  time.Time
	CaseNumber string
}

type CaseQueryResult struct {
	Cases          []*domain.InspectionCase
	HasMore        bool
	StatusCounts   map[domain.CaseStatus]int
	SeverityCounts map[domain.Severity]int
}

func (s *Store) QueryCases(ctx context.Context, filter CaseFilter, cursor *CasePageCursor, limit int) (CaseQueryResult, error) {
	where, args := caseWhere(filter)
	pageWhere, pageArgs := where, append([]any(nil), args...)
	if cursor != nil {
		pageWhere += " AND (i.updated_at < ? OR (i.updated_at = ? AND i.case_number > ?))"
		formatted := formatTime(cursor.UpdatedAt)
		pageArgs = append(pageArgs, formatted, formatted, cursor.CaseNumber)
	}
	pageArgs = append(pageArgs, limit+1)
	rows, err := s.db.QueryContext(ctx, `SELECT i.aggregate_json FROM inspection_cases i WHERE `+pageWhere+
		` ORDER BY i.updated_at DESC, i.case_number ASC LIMIT ?`, pageArgs...)
	if err != nil {
		return CaseQueryResult{}, err
	}
	cases := []*domain.InspectionCase{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			rows.Close()
			return CaseQueryResult{}, err
		}
		var aggregate domain.InspectionCase
		if err := json.Unmarshal(raw, &aggregate); err != nil {
			rows.Close()
			return CaseQueryResult{}, fmt.Errorf("重建工作队列档案: %w", err)
		}
		if err := aggregate.ValidateIntegrity(); err != nil {
			rows.Close()
			return CaseQueryResult{}, err
		}
		cases = append(cases, &aggregate)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return CaseQueryResult{}, err
	}
	if err := rows.Close(); err != nil {
		return CaseQueryResult{}, err
	}
	result := CaseQueryResult{Cases: cases, StatusCounts: map[domain.CaseStatus]int{},
		SeverityCounts: map[domain.Severity]int{}}
	if len(result.Cases) > limit {
		result.HasMore = true
		result.Cases = result.Cases[:limit]
	}
	statusRows, err := s.db.QueryContext(ctx, `SELECT i.status, COUNT(*) FROM inspection_cases i WHERE `+where+` GROUP BY i.status`, args...)
	if err != nil {
		return CaseQueryResult{}, err
	}
	for statusRows.Next() {
		var status domain.CaseStatus
		var count int
		if err := statusRows.Scan(&status, &count); err != nil {
			statusRows.Close()
			return CaseQueryResult{}, err
		}
		result.StatusCounts[status] = count
	}
	if err := statusRows.Err(); err != nil {
		statusRows.Close()
		return CaseQueryResult{}, err
	}
	if err := statusRows.Close(); err != nil {
		return CaseQueryResult{}, err
	}
	severityRows, err := s.db.QueryContext(ctx, `SELECT d.severity, COUNT(*) FROM defects d
		JOIN inspection_cases i ON i.id=d.case_id WHERE `+where+` AND d.status<>? GROUP BY d.severity`, append(args, domain.DefectResolved)...)
	if err != nil {
		return CaseQueryResult{}, err
	}
	for severityRows.Next() {
		var severity domain.Severity
		var count int
		if err := severityRows.Scan(&severity, &count); err != nil {
			severityRows.Close()
			return CaseQueryResult{}, err
		}
		result.SeverityCounts[severity] = count
	}
	if err := severityRows.Err(); err != nil {
		severityRows.Close()
		return CaseQueryResult{}, err
	}
	return result, severityRows.Close()
}

func caseWhere(filter CaseFilter) (string, []any) {
	clauses := []string{"i.updated_at <= ?"}
	args := []any{formatTime(filter.Snapshot)}
	if len(filter.Statuses) > 0 {
		placeholders := make([]string, len(filter.Statuses))
		for index, status := range filter.Statuses {
			placeholders[index] = "?"
			args = append(args, status)
		}
		clauses = append(clauses, "i.status IN ("+strings.Join(placeholders, ",")+")")
	}
	if filter.VenueName != "" {
		clauses = append(clauses, "instr(lower(i.venue_name), lower(?)) > 0")
		args = append(args, filter.VenueName)
	}
	if filter.UpdatedFrom != nil {
		clauses = append(clauses, "i.updated_at >= ?")
		args = append(args, formatTime(*filter.UpdatedFrom))
	}
	if filter.UpdatedTo != nil {
		clauses = append(clauses, "i.updated_at <= ?")
		args = append(args, formatTime(*filter.UpdatedTo))
	}
	if filter.HighestSeverity != "" {
		clauses = append(clauses, `EXISTS (SELECT 1 FROM defects risk WHERE risk.case_id=i.id
			AND risk.status<>'resolved' AND risk.severity=?)`)
		args = append(args, filter.HighestSeverity)
		higher := []domain.Severity{}
		if filter.HighestSeverity == domain.SeverityMajor {
			higher = append(higher, domain.SeverityCritical)
		} else if filter.HighestSeverity == domain.SeverityMinor {
			higher = append(higher, domain.SeverityMajor, domain.SeverityCritical)
		}
		if len(higher) > 0 {
			placeholders := make([]string, len(higher))
			for index, severity := range higher {
				placeholders[index] = "?"
				args = append(args, severity)
			}
			clauses = append(clauses, `NOT EXISTS (SELECT 1 FROM defects risk WHERE risk.case_id=i.id
				AND risk.status<>'resolved' AND risk.severity IN (`+strings.Join(placeholders, ",")+"))")
		}
	}
	return strings.Join(clauses, " AND "), args
}
