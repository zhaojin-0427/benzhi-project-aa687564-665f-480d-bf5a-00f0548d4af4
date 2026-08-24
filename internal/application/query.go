package application

import (
	"context"
	"strings"

	"stage-rigging-clearance/internal/audit"
	"stage-rigging-clearance/internal/domain"
)

type cachedCaseView struct {
	body []byte
}

func (s *Service) GetCase(ctx context.Context, caseNumber string) (*Result, error) {
	normalized := strings.ToUpper(strings.TrimSpace(caseNumber))
	if cached, ok := s.caseViews.Load(normalized); ok {
		view := cached.(cachedCaseView)
		return &Result{StatusCode: 200, Body: append([]byte(nil), view.body...)}, nil
	}
	aggregate, err := s.store.LoadCase(ctx, normalized)
	if err != nil {
		return nil, err
	}
	body, err := marshal(caseEnvelope{Case: aggregate})
	if err != nil {
		return nil, err
	}
	s.caseViews.Store(normalized, cachedCaseView{body: append([]byte(nil), body...)})
	return &Result{StatusCode: 200, Body: append([]byte(nil), body...)}, nil
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
