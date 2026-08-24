package domain

import "time"

func (c *InspectionCase) AddObservedDefect(assetID string, severity Severity, description string, now time.Time) (*Defect, error) {
	if err := c.ensureMutable(); err != nil {
		return nil, err
	}
	if c.Status != StatusTesting && c.Status != StatusReturned {
		return nil, NewRuleError(CodeInvalidState, "当前状态不允许登记人工观察缺陷")
	}
	if _, err := c.AssetByID(assetID); err != nil {
		return nil, err
	}
	if !validSeverity(severity) {
		return nil, NewRuleError(CodeValidation, "缺陷严重级别无效")
	}
	description = NormalizeText(description)
	if description == "" || len(description) > 1000 {
		return nil, NewRuleError(CodeValidation, "缺陷描述不能为空且不得超过 1000 个字符")
	}
	defect := Defect{ID: NewID("defect"), CaseID: c.ID, AssetID: assetID,
		Severity: severity, Description: description, Status: DefectOpen}
	c.Defects = append(c.Defects, defect)
	c.advance(now)
	return &c.Defects[len(c.Defects)-1], nil
}

func (c *InspectionCase) RemediateDefect(id, evidence string, now time.Time) (*Defect, error) {
	return c.RemediateDefectBy(id, evidence, "未指定维护负责人", now)
}

func (c *InspectionCase) RemediateDefectBy(id, evidence, submittedBy string, now time.Time) (*Defect, error) {
	if err := c.ensureMutable(); err != nil {
		return nil, err
	}
	if c.Status != StatusTesting && c.Status != StatusReturned {
		return nil, NewRuleError(CodeInvalidState, "当前状态不允许提交整改证据")
	}
	defect, err := c.DefectByID(id)
	if err != nil {
		return nil, err
	}
	if defect.Status == DefectResolved {
		return nil, NewRuleError(CodeInvalidState, "已解决缺陷不能重复整改")
	}
	evidence = NormalizeText(evidence)
	if evidence == "" || len(evidence) > 2000 {
		return nil, NewRuleError(CodeValidation, "整改证据不能为空且不得超过 2000 个字符")
	}
	if NormalizeText(submittedBy) == "" {
		return nil, NewRuleError(CodeValidation, "整改证据提交人不能为空")
	}
	if len(defect.EvidenceVersions) > 0 && NormalizeText(defect.EvidenceVersions[len(defect.EvidenceVersions)-1].Content) == evidence {
		return nil, NewRuleError(CodeValidation, "整改证据与上一版本内容相同")
	}
	version := len(defect.EvidenceVersions) + 1
	defect.EvidenceVersions = append(defect.EvidenceVersions, RemediationEvidenceVersion{
		Version: version, SubmittedBy: NormalizeText(submittedBy), SubmittedAt: now.UTC(), Content: evidence,
	})
	defect.RemediationEvidence = evidence
	defect.Status = DefectRemediated
	defect.ReviewComment = ""
	defect.AcceptedEvidenceVersion = 0
	c.advance(now)
	return defect, nil
}

func validSeverity(value Severity) bool {
	return value == SeverityMinor || value == SeverityMajor || value == SeverityCritical
}
