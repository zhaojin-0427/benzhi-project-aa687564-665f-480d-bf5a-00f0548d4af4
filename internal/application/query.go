package application

import (
	"context"
	"strings"
	"time"

	"stage-rigging-clearance/internal/audit"
	"stage-rigging-clearance/internal/domain"
)

type cachedCaseView struct {
	body      []byte
	version   int64
	updatedAt time.Time
}

func (s *Service) GetCase(ctx context.Context, caseNumber string) (*Result, error) {
	normalized := strings.ToUpper(strings.TrimSpace(caseNumber))
	if cached, ok := s.caseViews.Load(normalized); ok {
		view := cached.(cachedCaseView)
		if fresh, err := s.caseViewFresh(ctx, normalized, view); err != nil {
			return nil, err
		} else if fresh {
			return &Result{StatusCode: 200, Body: append([]byte(nil), view.body...)}, nil
		}
		s.caseViews.Delete(normalized)
	}
	aggregate, err := s.store.LoadCase(ctx, normalized)
	if err != nil {
		return nil, err
	}
	body, err := marshal(caseEnvelope{Case: aggregate})
	if err != nil {
		return nil, err
	}
	s.caseViews.Store(normalized, cachedCaseView{body: append([]byte(nil), body...),
		version: aggregate.Version, updatedAt: aggregate.UpdatedAt})
	return &Result{StatusCode: 200, Body: append([]byte(nil), body...)}, nil
}

// caseViewFresh reports whether a cached case snapshot still matches the
// persisted row. A commit performed by another process or Store handle sharing
// the SQLite file advances (version, updated_at), which is detected here so the
// stale cached response is discarded and the next query reflects the persisted
// state. Errors other than not-found surface to the caller; a missing row means
// the cache cannot be trusted and the caller should attempt a fresh load that
// will report the proper not-found status.
func (s *Service) caseViewFresh(ctx context.Context, normalized string, view cachedCaseView) (bool, error) {
	version, updatedAt, err := s.store.CaseSignature(ctx, normalized)
	if err != nil {
		if domain.ErrorCodeOf(err) == domain.CodeNotFound {
			return false, nil
		}
		return false, err
	}
	return version == view.version && updatedAt.Equal(view.updatedAt), nil
}

func (s *Service) forgetCaseView(caseNumber string) {
	s.caseViews.Delete(strings.ToUpper(strings.TrimSpace(caseNumber)))
}

func (s *Service) GetAudit(ctx context.Context, caseNumber string) (*Result, error) {
	aggregate, err := s.store.LoadCase(ctx, strings.ToUpper(strings.TrimSpace(caseNumber)))
	if err != nil {
		return nil, err
	}
	events, err := s.store.LoadAudit(ctx, aggregate.ID)
	if err != nil {
		return nil, err
	}
	if err := audit.Verify(events); err != nil {
		return nil, domain.NewRuleError(domain.CodeIntegrity, "审计轨迹校验失败: %v", err)
	}
	body, err := marshal(auditEnvelope{CaseNumber: aggregate.CaseNumber, Valid: true, Events: events})
	if err != nil {
		return nil, err
	}
	return &Result{StatusCode: 200, Body: body}, nil
}

func (s *Service) GetCertificate(ctx context.Context, caseNumber string) (*Result, error) {
	aggregate, err := s.store.LoadCase(ctx, strings.ToUpper(strings.TrimSpace(caseNumber)))
	if err != nil {
		return nil, err
	}
	if aggregate.Certificate == nil {
		return nil, domain.NewRuleError(domain.CodeNotFound, "复役凭据尚未签发")
	}
	body, err := marshal(certificateEnvelope{CaseNumber: aggregate.CaseNumber,
		Valid: domain.VerifyCertificate(aggregate.Certificate), Certificate: aggregate.Certificate})
	if err != nil {
		return nil, err
	}
	return &Result{StatusCode: 200, Body: body}, nil
}

func (s *Service) GetTestCoverage(ctx context.Context, caseNumber string) (*Result, error) {
	aggregate, err := s.store.LoadCase(ctx, strings.ToUpper(strings.TrimSpace(caseNumber)))
	if err != nil {
		return nil, err
	}
	body, err := marshal(coverageEnvelope{CaseNumber: aggregate.CaseNumber, Matrix: aggregate.TestCoverage()})
	if err != nil {
		return nil, err
	}
	return &Result{StatusCode: 200, Body: body}, nil
}

func (s *Service) VerifyCertificate(ctx context.Context, pathCaseNumber string, carried domain.CarriedCertificate) (*Result, error) {
	normalized, err := domain.NormalizeCarriedCertificate(carried)
	if err != nil {
		return nil, err
	}
	aggregate, err := s.store.LoadCaseByCertificateNumber(ctx, normalized.CertificateNumber)
	if err != nil {
		return nil, err
	}
	if aggregate == nil {
		body, marshalErr := marshal(domain.CertificateVerification{Valid: false,
			Reason: domain.CertificateNotFound, Reasons: []string{domain.CertificateNotFound}})
		if marshalErr != nil {
			return nil, marshalErr
		}
		return &Result{StatusCode: 200, Body: body}, nil
	}
	verification := domain.VerifyCarriedCertificate(pathCaseNumber, normalized, aggregate)
	body, err := marshal(verification)
	if err != nil {
		return nil, err
	}
	return &Result{StatusCode: 200, Body: body}, nil
}
