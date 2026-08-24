package domain

import (
	"encoding/hex"
	"strings"
	"time"
)

const (
	CertificateValid                = "valid"
	CertificateNotFound             = "certificate_not_found"
	CertificateContentDigestInvalid = "content_digest_invalid"
	CertificateCaseMismatch         = "case_mismatch"
	CertificateReportMismatch       = "report_digest_mismatch"
)

type CarriedCertificate struct {
	CertificateNumber  string    `json:"certificateNumber"`
	CertificateID      string    `json:"certificateId"`
	CaseNumber         string    `json:"caseNumber"`
	ReportDigest       string    `json:"reportDigest"`
	IssuedAt           time.Time `json:"issuedAt"`
	VerificationDigest string    `json:"verificationDigest"`
}

type CertificateVerification struct {
	Valid                    bool     `json:"valid"`
	Reason                   string   `json:"reason"`
	Reasons                  []string `json:"reasons"`
	CertificateNumberMatched bool     `json:"certificateNumberMatched"`
	CertificateIDMatched     bool     `json:"certificateIdMatched"`
	CaseNumberMatched        bool     `json:"caseNumberMatched"`
	ReportDigestMatched      bool     `json:"reportDigestMatched"`
	IssuedAtMatched          bool     `json:"issuedAtMatched"`
	ContentDigestValid       bool     `json:"contentDigestValid"`
}

func NormalizeCarriedCertificate(value CarriedCertificate) (CarriedCertificate, error) {
	value.CertificateNumber = NormalizeText(value.CertificateNumber)
	value.CertificateID = NormalizeText(value.CertificateID)
	value.CaseNumber = strings.ToUpper(NormalizeText(value.CaseNumber))
	value.ReportDigest = strings.ToLower(NormalizeText(value.ReportDigest))
	value.VerificationDigest = strings.ToLower(NormalizeText(value.VerificationDigest))
	value.IssuedAt = value.IssuedAt.UTC()
	if value.CertificateNumber == "" || len(value.CertificateNumber) > 128 ||
		value.CertificateID == "" || len(value.CertificateID) > 128 ||
		value.CaseNumber == "" || len(value.CaseNumber) > 64 || value.IssuedAt.IsZero() {
		return value, NewRuleError(CodeValidation, "凭据编号、凭据 ID、档案编号和签发时间均为必填项")
	}
	if !strings.HasPrefix(value.CertificateNumber, "RTS-") || !strings.HasPrefix(value.CertificateID, "cert_") {
		return value, NewRuleError(CodeValidation, "凭据编号或凭据 ID 格式无效")
	}
	if !validHexDigest(value.ReportDigest) || !validHexDigest(value.VerificationDigest) {
		return value, NewRuleError(CodeValidation, "报告摘要和验证摘要必须是 64 位十六进制字符串")
	}
	return value, nil
}

func VerifyCarriedCertificate(pathCaseNumber string, carried CarriedCertificate, aggregate *InspectionCase) CertificateVerification {
	result := CertificateVerification{Reason: CertificateValid, Reasons: []string{}}
	if aggregate == nil || aggregate.Certificate == nil {
		result.Valid = false
		result.Reason = CertificateNotFound
		result.Reasons = append(result.Reasons, CertificateNotFound)
		return result
	}
	certificate := aggregate.Certificate
	result.CertificateNumberMatched = certificate.CertificateNumber == carried.CertificateNumber
	result.CertificateIDMatched = certificate.ID == carried.CertificateID
	result.CaseNumberMatched = aggregate.CaseNumber == strings.ToUpper(NormalizeText(pathCaseNumber)) &&
		aggregate.CaseNumber == carried.CaseNumber
	result.ReportDigestMatched = aggregate.Report != nil && aggregate.Report.Digest == carried.ReportDigest &&
		certificate.ReportDigest == carried.ReportDigest
	result.IssuedAtMatched = certificate.IssuedAt.Equal(carried.IssuedAt)
	caseBinding := carried.CaseNumber
	if certificate.CaseNumber == "" {
		caseBinding = aggregate.ID
	}
	result.ContentDigestValid = CertificateDigest(carried.CertificateID, caseBinding, carried.ReportDigest,
		carried.CertificateNumber, carried.IssuedAt) == carried.VerificationDigest
	if !result.CaseNumberMatched {
		result.Reasons = append(result.Reasons, CertificateCaseMismatch)
	}
	if !result.ReportDigestMatched {
		result.Reasons = append(result.Reasons, CertificateReportMismatch)
	}
	if !result.ContentDigestValid || !result.CertificateIDMatched || !result.IssuedAtMatched || !result.CertificateNumberMatched {
		result.Reasons = append(result.Reasons, CertificateContentDigestInvalid)
	}
	result.Valid = len(result.Reasons) == 0
	if !result.Valid {
		result.Reason = result.Reasons[0]
	}
	return result
}

func validHexDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
