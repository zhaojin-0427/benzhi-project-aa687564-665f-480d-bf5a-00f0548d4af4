package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"stage-rigging-clearance/internal/domain"
)

func NewEvent(caseID, actor, role, command string, beforeVersion, afterVersion int64, resultDigest string, previous *domain.AuditEvent, now time.Time) domain.AuditEvent {
	sequence := int64(1)
	previousHash := ""
	if previous != nil {
		sequence = previous.Sequence + 1
		previousHash = previous.Hash
	}
	event := domain.AuditEvent{CaseID: caseID, Sequence: sequence, Actor: domain.NormalizeText(actor),
		Role: role, Command: command, BeforeVersion: beforeVersion, AfterVersion: afterVersion,
		OccurredAt: now.UTC(), ResultDigest: resultDigest, PreviousHash: previousHash}
	event.Hash = Hash(event)
	return event
}

func Hash(event domain.AuditEvent) string {
	payload := fmt.Sprintf("%s\n%d\n%s\n%s\n%s\n%d\n%d\n%s\n%s\n%s",
		event.CaseID, event.Sequence, event.Actor, event.Role, event.Command,
		event.BeforeVersion, event.AfterVersion, event.OccurredAt.UTC().Format(time.RFC3339Nano),
		event.ResultDigest, event.PreviousHash)
	digest := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(digest[:])
}
