package domain

import "time"

func (c *InspectionCase) SubmitForReview(now time.Time) error {
	if c.Status != StatusTesting && c.Status != StatusReturned {
		return NewRuleError(CodeInvalidState, "只有测试中或已退回档案可以提交复核")
	}
	if err := c.validateTestCoverage(); err != nil {
		return err
	}
	for _, defect := range c.Defects {
		if defect.Status == DefectOpen || len(defect.EvidenceVersions) == 0 {
			return NewRuleError(CodeInvalidState, "缺陷 %s 尚未提交整改证据", defect.ID)
		}
		if len(defect.ReviewDecisions) > 0 {
			last := defect.ReviewDecisions[len(defect.ReviewDecisions)-1]
			if !last.Accepted && last.EvidenceVersion == len(defect.EvidenceVersions) {
				return NewRuleError(CodeInvalidState, "缺陷 %s 被驳回后必须追加新证据", defect.ID)
			}
		}
	}
	c.Status = StatusPendingReview
	c.Review = nil
	c.advance(now)
	return nil
}

func (c *InspectionCase) ReturnReview(reviewer, comment string, now time.Time) error {
	if c.Status != StatusPendingReview {
		return NewRuleError(CodeInvalidState, "只有待复核档案可以退回")
	}
	comment = NormalizeText(comment)
	if len(comment) < 3 || len(comment) > 1000 {
		return NewRuleError(CodeValidation, "退回理由长度必须在 3 到 1000 个字符之间")
	}
	c.Status = StatusReturned
	c.Review = &ReviewDecision{Reviewer: NormalizeText(reviewer), Comment: comment, Approved: false, At: now.UTC()}
	c.advance(now)
	return nil
}

func (c *InspectionCase) ApproveReview(reviewer, comment string, now time.Time) error {
	if c.Status != StatusPendingReview {
		return NewRuleError(CodeInvalidState, "只有待复核档案可以确认通过")
	}
	for index := range c.Defects {
		defect := &c.Defects[index]
		latest := len(defect.EvidenceVersions)
		if defect.Status != DefectRemediated || latest == 0 || defect.AcceptedEvidenceVersion != latest {
			return NewRuleError(CodeInvalidState, "缺陷 %s 的最新证据尚未逐项接受", defect.ID)
		}
	}
	for index := range c.Defects {
		defect := &c.Defects[index]
		resolvedAt := now.UTC()
		defect.Status = DefectResolved
		defect.ReviewComment = NormalizeText(comment)
		defect.ResolvedAt = &resolvedAt
	}
	c.Status = StatusReviewed
	c.Review = &ReviewDecision{Reviewer: NormalizeText(reviewer), Comment: NormalizeText(comment), Approved: true, At: now.UTC()}
	c.advance(now)
	return nil
}

func (c *InspectionCase) ReviewDefect(id, reviewer string, accepted bool, comment string, now time.Time) (*Defect, error) {
	if c.Status != StatusPendingReview {
		return nil, NewRuleError(CodeInvalidState, "只有待复核档案可以执行逐项复核")
	}
	defect, err := c.DefectByID(id)
	if err != nil {
		return nil, err
	}
	latest := len(defect.EvidenceVersions)
	if latest == 0 || defect.Status != DefectRemediated {
		return nil, NewRuleError(CodeInvalidState, "缺陷尚无可复核的最新整改证据")
	}
	comment = NormalizeText(comment)
	if !accepted && (len(comment) < 3 || len(comment) > 1000) {
		return nil, NewRuleError(CodeValidation, "驳回理由长度必须在 3 到 1000 个字符之间")
	}
	if accepted && len(comment) > 1000 {
		return nil, NewRuleError(CodeValidation, "复核意见不得超过 1000 个字符")
	}
	if len(defect.ReviewDecisions) > 0 {
		last := defect.ReviewDecisions[len(defect.ReviewDecisions)-1]
		if last.EvidenceVersion == latest {
			return nil, NewRuleError(CodeInvalidState, "该证据版本已经完成逐项复核")
		}
	}
	decision := DefectReviewDecision{EvidenceVersion: latest, Reviewer: NormalizeText(reviewer),
		Accepted: accepted, Comment: comment, DecidedAt: now.UTC()}
	defect.ReviewDecisions = append(defect.ReviewDecisions, decision)
	defect.ReviewComment = comment
	if accepted {
		defect.AcceptedEvidenceVersion = latest
	} else {
		defect.AcceptedEvidenceVersion = 0
		c.Status = StatusReturned
		c.Review = &ReviewDecision{Reviewer: NormalizeText(reviewer), Comment: comment, Approved: false, At: now.UTC()}
	}
	c.advance(now)
	return defect, nil
}

func (c *InspectionCase) validateTestCoverage() error {
	if len(c.Assets) == 0 {
		return NewRuleError(CodeValidation, "档案没有受检设备")
	}
	for _, asset := range c.Assets {
		kinds := map[TestKind]bool{}
		for _, record := range c.Tests {
			if record.AssetID == asset.ID {
				kinds[record.TestKind] = true
			}
		}
		for _, required := range []TestKind{TestStatic, TestDynamic, TestBrake, TestLimit} {
			if !kinds[required] {
				return NewRuleError(CodeValidation, "设备 %s 缺少 %s 测试", asset.AssetCode, required)
			}
		}
	}
	return nil
}
