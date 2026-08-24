package application

import (
	"context"
	"strings"

	"stage-rigging-clearance/internal/audit"
	"stage-rigging-clearance/internal/domain"
)

func (s *Service) GetCase(ctx context.Context, caseNumber string) (*Result, error) {
	aggregate, err := s.store.LoadCase(ctx, strings.ToUpper(strings.TrimSpace(caseNumber)))
	if err != nil {
		return nil, err
	}
	body, err := marshal(caseEnvelope{Case: aggregate})
	if err != nil {
		return nil, err
	}
	return &Result{StatusCode: 200, Body: body}, nil
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
	key := strings.ToUpper(strings.TrimSpace(caseNumber))
	if body, ok := s.cachedCertificateView(key); ok {
		return &Result{StatusCode: 200, Body: body}, nil
	}
	aggregate, err := s.store.LoadCase(ctx, key)
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
	return &Result{StatusCode: 200, Body: s.rememberCertificateView(key, body)}, nil
}

func (s *Service) cachedCertificateView(caseNumber string) ([]byte, bool) {
	s.certificateViewsMu.Lock()
	defer s.certificateViewsMu.Unlock()
	body, ok := s.certificateViews[caseNumber]
	return body, ok
}

func (s *Service) rememberCertificateView(caseNumber string, body []byte) []byte {
	s.certificateViewsMu.Lock()
	defer s.certificateViewsMu.Unlock()
	if len(body) > len(s.certificateViewBuffer) {
		s.certificateViews[caseNumber] = append([]byte(nil), body...)
		return s.certificateViews[caseNumber]
	}
	view := s.certificateViewBuffer[:len(body)]
	copy(view, body)
	s.certificateViews[caseNumber] = view
	return view
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
