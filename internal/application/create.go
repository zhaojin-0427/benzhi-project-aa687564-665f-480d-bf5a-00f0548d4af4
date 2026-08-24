package application

import (
	"context"
	"strings"

	"stage-rigging-clearance/internal/audit"
	"stage-rigging-clearance/internal/domain"
	"stage-rigging-clearance/internal/repository"
)

func (s *Service) CreateCase(ctx context.Context, cmd CreateCaseCommand) (*Result, error) {
	if err := validateMeta(cmd.CommandMeta, RoleInspector); err != nil {
		return nil, err
	}
	if cmd.ExpectedVersion != 0 {
		return nil, domain.NewRuleError(domain.CodeValidation, "建档 expectedVersion 必须为 0")
	}
	commandFingerprint, err := fingerprint("create_case", cmd)
	if err != nil {
		return nil, err
	}
	lock := s.lockFor(cmd.CaseNumber)
	lock.Lock()
	defer lock.Unlock()
	if replay, err := s.replay(ctx, cmd.CommandMeta, commandFingerprint); replay != nil || err != nil {
		return replay, err
	}
	now := s.now().UTC()
	aggregate, err := domain.NewInspectionCase(cmd.CaseNumber, cmd.VenueName, cmd.Scope, now)
	if err != nil {
		return nil, err
	}
	body, err := marshal(caseEnvelope{Case: aggregate})
	if err != nil {
		return nil, err
	}
	event := audit.NewEvent(aggregate.ID, cmd.Actor, cmd.Role, "create_case", 0, aggregate.Version,
		responseDigest(body), nil, now)
	idem := repository.IdempotencyRecord{Key: strings.TrimSpace(cmd.IdempotencyKey), CaseNumber: aggregate.CaseNumber,
		Command: "create_case", Fingerprint: commandFingerprint, StatusCode: 201, Response: body, CreatedAt: now}
	if err := s.store.Commit(ctx, 0, aggregate, idem, event); err != nil {
		return nil, err
	}
	return s.persistedResult(ctx, cmd.IdempotencyKey)
}
