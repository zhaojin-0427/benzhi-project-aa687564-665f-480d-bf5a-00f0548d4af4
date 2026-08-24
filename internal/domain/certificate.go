package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

func (c *InspectionCase) IssueCertificate(actor string, now time.Time) (*ReturnToServiceCertificate, error) {
	if c.Status != StatusFrozen || c.Report == nil {
		return nil, NewRuleError(CodeInvalidState, "只有冻结报告可以签发复役凭据")
	}
	if c.Certificate != nil {
		return nil, NewRuleError(CodeInvalidState, "该冻结报告已签发复役凭据")
	}
	if err := VerifyReport(c.Report); err != nil {
		return nil, err
	}
	now = now.UTC()
	number := fmt.Sprintf("RTS-%s-%s", now.Format("20060102"), c.ID[len(c.ID)-8:])
	id := NewID("cert")
	payload := fmt.Sprintf("%s\n%s\n%s\n%s\n%s", id, c.CaseNumber, c.Report.Digest, number, now.Format(time.RFC3339Nano))
	digest := sha256.Sum256([]byte(payload))
	certificate := &ReturnToServiceCertificate{ID: id, CaseID: c.ID, CaseNumber: c.CaseNumber, ReportDigest: c.Report.Digest,
		CertificateNumber: number, IssuedBy: NormalizeText(actor), IssuedAt: now,
		VerificationDigest: hex.EncodeToString(digest[:])}
	c.Certificate = certificate
	c.Status = StatusCertified
	c.advance(now)
	return certificate, nil
}

func VerifyCertificate(certificate *ReturnToServiceCertificate) bool {
	if certificate == nil {
		return false
	}
	binding := certificate.CaseNumber
	if binding == "" {
		binding = certificate.CaseID
	}
	return CertificateDigest(certificate.ID, binding, certificate.ReportDigest,
		certificate.CertificateNumber, certificate.IssuedAt) == certificate.VerificationDigest
}

func CertificateDigest(id, caseBinding, reportDigest, number string, issuedAt time.Time) string {
	payload := fmt.Sprintf("%s\n%s\n%s\n%s\n%s", id, caseBinding, reportDigest, number,
		issuedAt.UTC().Format(time.RFC3339Nano))
	digest := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(digest[:])
}
