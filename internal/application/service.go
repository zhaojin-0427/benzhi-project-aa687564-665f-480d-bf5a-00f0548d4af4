package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"stage-rigging-clearance/internal/audit"
	"stage-rigging-clearance/internal/domain"
	"stage-rigging-clearance/internal/repository"
)

type Service struct {
	store *repository.Store
	now   func() time.Time
	locks sync.Map
}

func NewService(store *repository.Store) *Service {
	return &Service{store: store, now: time.Now}
}

func (s *Service) lockFor(caseNumber string) *sync.Mutex {
	value, _ := s.locks.LoadOrStore(strings.ToUpper(strings.TrimSpace(caseNumber)), &sync.Mutex{})
	return value.(*sync.Mutex)
}

func validateMeta(meta CommandMeta, allowedRoles ...string) error {
	if meta.ExpectedVersion < 0 {
		return domain.NewRuleError(domain.CodeValidation, "expectedVersion 不得为负数")
	}
	if len(strings.TrimSpace(meta.IdempotencyKey)) < 8 || len(meta.IdempotencyKey) > 128 {
		return domain.NewRuleError(domain.CodeValidation, "idempotencyKey 长度必须在 8 到 128 个字符之间")
	}
	if strings.TrimSpace(meta.Actor) == "" {
		return domain.NewRuleError(domain.CodeValidation, "actor 不能为空")
	}
	for _, role := range allowedRoles {
		if meta.Role == role {
			return nil
		}
	}
	return domain.NewRuleError(domain.CodeForbidden, "角色 %s 无权执行该命令", meta.Role)
}

func fingerprint(command string, value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(append([]byte(command+"\n"), encoded...))
	return hex.EncodeToString(digest[:]), nil
}

func responseDigest(body []byte) string {
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

func (s *Service) replay(ctx context.Context, meta CommandMeta, commandFingerprint string) (*Result, error) {
	record, err := s.store.FindIdempotency(ctx, strings.TrimSpace(meta.IdempotencyKey))
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, nil
	}
	if record.Fingerprint != commandFingerprint {
		return nil, domain.NewRuleError(domain.CodeIdempotencyReuse, "idempotencyKey 已用于不同请求")
	}
	return &Result{StatusCode: record.StatusCode, Body: append([]byte(nil), record.Response...)}, nil
}

type mutation func(*domain.InspectionCase, time.Time) error

type resultMutation func(*domain.InspectionCase, time.Time) (any, error)

func (s *Service) mutate(ctx context.Context, caseNumber, command string, meta CommandMeta, input any, fn mutation) (*Result, error) {
	return s.mutateResult(ctx, caseNumber, command, meta, input,
		func(aggregate *domain.InspectionCase, now time.Time) (any, error) {
			if err := fn(aggregate, now); err != nil {
				return nil, err
			}
			return caseEnvelope{Case: aggregate}, nil
		})
}

func (s *Service) mutateResult(ctx context.Context, caseNumber, command string, meta CommandMeta, input any, fn resultMutation) (*Result, error) {
	commandFingerprint, err := fingerprint(command, struct {
		CaseNumber string `json:"caseNumber"`
		Input      any    `json:"input"`
	}{CaseNumber: strings.ToUpper(strings.TrimSpace(caseNumber)), Input: input})
	if err != nil {
		return nil, err
	}
	lock := s.lockFor(caseNumber)
	lock.Lock()
	defer lock.Unlock()
	if replay, err := s.replay(ctx, meta, commandFingerprint); replay != nil || err != nil {
		return replay, err
	}
	aggregate, err := s.store.LoadCase(ctx, strings.ToUpper(strings.TrimSpace(caseNumber)))
	if err != nil {
		return nil, err
	}
	if aggregate.Version != meta.ExpectedVersion {
		return nil, domain.NewRuleError(domain.CodeConflict, "expectedVersion=%d，当前版本=%d", meta.ExpectedVersion, aggregate.Version)
	}
	before := aggregate.Version
	now := s.now().UTC()
	response, err := fn(aggregate, now)
	if err != nil {
		return nil, err
	}
	body, err := marshal(response)
	if err != nil {
		return nil, err
	}
	events, err := s.store.LoadAudit(ctx, aggregate.ID)
	if err != nil {
		return nil, err
	}
	var previous *domain.AuditEvent
	if len(events) > 0 {
		previous = &events[len(events)-1]
	}
	event := audit.NewEvent(aggregate.ID, meta.Actor, meta.Role, command, before, aggregate.Version,
		responseDigest(body), previous, now)
	idem := repository.IdempotencyRecord{Key: strings.TrimSpace(meta.IdempotencyKey), CaseNumber: aggregate.CaseNumber,
		Command: command, Fingerprint: commandFingerprint, StatusCode: 200, Response: body, CreatedAt: now}
	if err := s.store.Commit(ctx, before, aggregate, idem, event); err != nil {
		return nil, err
	}
	return s.persistedResult(ctx, meta.IdempotencyKey)
}

func (s *Service) persistedResult(ctx context.Context, key string) (*Result, error) {
	record, err := s.store.FindIdempotency(ctx, strings.TrimSpace(key))
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, domain.NewRuleError(domain.CodeIntegrity, "原子提交后缺少幂等响应")
	}
	return &Result{StatusCode: record.StatusCode, Body: append([]byte(nil), record.Response...)}, nil
}
