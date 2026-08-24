package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"stage-rigging-clearance/internal/domain"
	"stage-rigging-clearance/internal/repository"
)

type workQueueStatistics struct {
	ByStatus       map[domain.CaseStatus]int `json:"byStatus"`
	ByOpenSeverity map[domain.Severity]int   `json:"byUnresolvedDefectSeverity"`
}

type workQueuePagination struct {
	Limit      int    `json:"limit"`
	NextCursor string `json:"nextCursor,omitempty"`
	SnapshotAt string `json:"snapshotAt"`
}

type workQueueEnvelope struct {
	Items      []domain.CaseWorkSummary `json:"items"`
	Statistics workQueueStatistics      `json:"statistics"`
	Pagination workQueuePagination      `json:"pagination"`
}

type queueCursor struct {
	UpdatedAt  time.Time `json:"updatedAt"`
	CaseNumber string    `json:"caseNumber"`
	SnapshotAt time.Time `json:"snapshotAt"`
	QueryHash  string    `json:"queryHash"`
	Checksum   string    `json:"checksum"`
}

func (s *Service) GetWorkQueue(ctx context.Context, query WorkQueueQuery) (*Result, error) {
	if err := normalizeWorkQueueQuery(&query); err != nil {
		return nil, err
	}
	queryHash, err := workQueueQueryHash(query)
	if err != nil {
		return nil, err
	}
	snapshot := s.now().UTC()
	var repositoryCursor *repository.CasePageCursor
	if query.Cursor != "" {
		cursor, err := decodeQueueCursor(query.Cursor)
		if err != nil || cursor.QueryHash != queryHash {
			return nil, domain.NewRuleError(domain.CodeValidation, "分页游标无效或与筛选条件不匹配")
		}
		snapshot = cursor.SnapshotAt
		repositoryCursor = &repository.CasePageCursor{UpdatedAt: cursor.UpdatedAt, CaseNumber: cursor.CaseNumber}
	}
	result, err := s.store.QueryCases(ctx, repository.CaseFilter{Statuses: query.Statuses,
		VenueName: query.VenueName, UpdatedFrom: query.UpdatedFrom, UpdatedTo: query.UpdatedTo,
		HighestSeverity: query.HighestSeverity, Snapshot: snapshot}, repositoryCursor, query.Limit)
	if err != nil {
		return nil, err
	}
	items := make([]domain.CaseWorkSummary, len(result.Cases))
	for index, aggregate := range result.Cases {
		items[index] = aggregate.WorkSummary()
	}
	next := ""
	if result.HasMore && len(result.Cases) > 0 {
		last := result.Cases[len(result.Cases)-1]
		next, err = encodeQueueCursor(queueCursor{UpdatedAt: last.UpdatedAt, CaseNumber: last.CaseNumber,
			SnapshotAt: snapshot, QueryHash: queryHash})
		if err != nil {
			return nil, err
		}
	}
	statuses := []domain.CaseStatus{domain.StatusDraft, domain.StatusBaselinePreparation, domain.StatusTesting,
		domain.StatusPendingReview, domain.StatusReturned, domain.StatusReviewed, domain.StatusFrozen, domain.StatusCertified}
	for _, status := range statuses {
		if _, ok := result.StatusCounts[status]; !ok {
			result.StatusCounts[status] = 0
		}
	}
	for _, severity := range []domain.Severity{domain.SeverityMinor, domain.SeverityMajor, domain.SeverityCritical} {
		if _, ok := result.SeverityCounts[severity]; !ok {
			result.SeverityCounts[severity] = 0
		}
	}
	envelope := workQueueEnvelope{Items: items,
		Statistics: workQueueStatistics{ByStatus: result.StatusCounts, ByOpenSeverity: result.SeverityCounts},
		Pagination: workQueuePagination{Limit: query.Limit, NextCursor: next, SnapshotAt: snapshot.Format(time.RFC3339Nano)}}
	body, err := marshal(envelope)
	if err != nil {
		return nil, err
	}
	return &Result{StatusCode: 200, Body: body}, nil
}

func normalizeWorkQueueQuery(query *WorkQueueQuery) error {
	if query.Limit == 0 {
		query.Limit = 20
	}
	if query.Limit < 1 || query.Limit > 100 {
		return domain.NewRuleError(domain.CodeValidation, "limit 必须在 1 到 100 之间")
	}
	query.VenueName = domain.NormalizeText(query.VenueName)
	if len(query.VenueName) > 160 {
		return domain.NewRuleError(domain.CodeValidation, "venueName 不得超过 160 个字符")
	}
	seen := map[domain.CaseStatus]bool{}
	for _, status := range query.Statuses {
		if !validQueueStatus(status) {
			return domain.NewRuleError(domain.CodeValidation, "status 枚举值无效: %s", status)
		}
		seen[status] = true
	}
	query.Statuses = query.Statuses[:0]
	for status := range seen {
		query.Statuses = append(query.Statuses, status)
	}
	sort.Slice(query.Statuses, func(i, j int) bool { return query.Statuses[i] < query.Statuses[j] })
	if query.HighestSeverity != "" && query.HighestSeverity != domain.SeverityMinor &&
		query.HighestSeverity != domain.SeverityMajor && query.HighestSeverity != domain.SeverityCritical {
		return domain.NewRuleError(domain.CodeValidation, "highestSeverity 枚举值无效")
	}
	if query.UpdatedFrom != nil {
		value := query.UpdatedFrom.UTC()
		query.UpdatedFrom = &value
	}
	if query.UpdatedTo != nil {
		value := query.UpdatedTo.UTC()
		query.UpdatedTo = &value
	}
	if query.UpdatedFrom != nil && query.UpdatedTo != nil && query.UpdatedFrom.After(*query.UpdatedTo) {
		return domain.NewRuleError(domain.CodeValidation, "updatedFrom 不得晚于 updatedTo")
	}
	return nil
}

func validQueueStatus(status domain.CaseStatus) bool {
	switch status {
	case domain.StatusDraft, domain.StatusBaselinePreparation, domain.StatusTesting, domain.StatusPendingReview,
		domain.StatusReturned, domain.StatusReviewed, domain.StatusFrozen, domain.StatusCertified:
		return true
	default:
		return false
	}
}

func workQueueQueryHash(query WorkQueueQuery) (string, error) {
	encoded, err := json.Marshal(struct {
		Statuses []domain.CaseStatus `json:"statuses"`
		Venue    string              `json:"venue"`
		From     *time.Time          `json:"from"`
		To       *time.Time          `json:"to"`
		Severity domain.Severity     `json:"severity"`
	}{query.Statuses, strings.ToLower(query.VenueName), query.UpdatedFrom, query.UpdatedTo, query.HighestSeverity})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func encodeQueueCursor(cursor queueCursor) (string, error) {
	cursor.Checksum = ""
	payload, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(append([]byte("stage-rigging-queue-v1\n"), payload...))
	cursor.Checksum = hex.EncodeToString(digest[:])
	encoded, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func decodeQueueCursor(value string) (queueCursor, error) {
	var cursor queueCursor
	if len(value) > 2048 {
		return cursor, domain.NewRuleError(domain.CodeValidation, "分页游标过长")
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || json.Unmarshal(raw, &cursor) != nil || cursor.UpdatedAt.IsZero() ||
		cursor.SnapshotAt.IsZero() || cursor.CaseNumber == "" || cursor.QueryHash == "" || cursor.Checksum == "" {
		return cursor, domain.NewRuleError(domain.CodeValidation, "分页游标格式无效")
	}
	canonical, err := json.Marshal(cursor)
	if err != nil || !bytes.Equal(canonical, raw) {
		return cursor, domain.NewRuleError(domain.CodeValidation, "分页游标格式无效")
	}
	checksum := cursor.Checksum
	cursor.Checksum = ""
	payload, err := json.Marshal(cursor)
	if err != nil {
		return cursor, err
	}
	digest := sha256.Sum256(append([]byte("stage-rigging-queue-v1\n"), payload...))
	if checksum != hex.EncodeToString(digest[:]) {
		return cursor, domain.NewRuleError(domain.CodeValidation, "分页游标校验失败")
	}
	cursor.Checksum = checksum
	return cursor, nil
}
