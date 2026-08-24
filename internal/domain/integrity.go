package domain

// ValidateIntegrity 校验聚合从持久化存储重建后的关联关系和状态不变量。
func (c *InspectionCase) ValidateIntegrity() error {
	if c.ID == "" || c.CaseNumber == "" || c.Version < 1 || c.CreatedAt.IsZero() || c.UpdatedAt.IsZero() {
		return NewRuleError(CodeIntegrity, "档案基础字段不完整")
	}
	if c.UpdatedAt.Before(c.CreatedAt) {
		return NewRuleError(CodeIntegrity, "档案更新时间早于创建时间")
	}
	if !validStatus(c.Status) {
		return NewRuleError(CodeIntegrity, "档案状态 %s 无效", c.Status)
	}
	assets := make(map[string]RiggingAsset, len(c.Assets))
	codes := make(map[string]bool, len(c.Assets))
	for _, asset := range c.Assets {
		if asset.ID == "" || asset.CaseID != c.ID {
			return NewRuleError(CodeIntegrity, "设备与档案关联无效")
		}
		if codes[asset.AssetCode] {
			return NewRuleError(CodeIntegrity, "设备编号 %s 重复", asset.AssetCode)
		}
		if !validAssetType(asset.AssetType) || asset.RatedLoadKg <= 0 || asset.BrakeDistanceLimitMm <= 0 {
			return NewRuleError(CodeIntegrity, "设备 %s 的基线字段无效", asset.AssetCode)
		}
		assets[asset.ID], codes[asset.AssetCode] = asset, true
	}
	tests := make(map[string]LoadTestRecord, len(c.Tests))
	for _, record := range c.Tests {
		if record.ID == "" || record.CaseID != c.ID {
			return NewRuleError(CodeIntegrity, "测试记录与档案关联无效")
		}
		if _, ok := assets[record.AssetID]; !ok {
			return NewRuleError(CodeIntegrity, "测试记录 %s 引用了不存在的设备", record.ID)
		}
		if _, exists := tests[record.ID]; exists {
			return NewRuleError(CodeIntegrity, "测试记录 %s 重复", record.ID)
		}
		if !validTestKind(record.TestKind) || (record.Result != ResultPass && record.Result != ResultFail) {
			return NewRuleError(CodeIntegrity, "测试记录 %s 的类型或结论无效", record.ID)
		}
		tests[record.ID] = record
	}
	for _, record := range c.Tests {
		if record.RetestOfRecordID == "" {
			continue
		}
		original, ok := tests[record.RetestOfRecordID]
		if !ok || original.Result != ResultFail || original.AssetID != record.AssetID || original.TestKind != record.TestKind {
			return NewRuleError(CodeIntegrity, "复测记录 %s 的原记录关联无效", record.ID)
		}
		for _, other := range c.Tests {
			if other.ID != record.ID && other.RetestOfRecordID == record.RetestOfRecordID {
				return NewRuleError(CodeIntegrity, "失败记录 %s 存在多个直接复测", record.RetestOfRecordID)
			}
		}
	}
	defects := make(map[string]bool, len(c.Defects))
	for _, defect := range c.Defects {
		if defect.ID == "" || defect.CaseID != c.ID {
			return NewRuleError(CodeIntegrity, "缺陷与档案关联无效")
		}
		if defects[defect.ID] {
			return NewRuleError(CodeIntegrity, "缺陷 %s 重复", defect.ID)
		}
		if _, ok := assets[defect.AssetID]; !ok {
			return NewRuleError(CodeIntegrity, "缺陷 %s 引用了不存在的设备", defect.ID)
		}
		if defect.SourceRecordID != "" {
			if _, ok := tests[defect.SourceRecordID]; !ok {
				return NewRuleError(CodeIntegrity, "缺陷 %s 引用了不存在的测试", defect.ID)
			}
		}
		if !validSeverity(defect.Severity) || !validDefectStatus(defect.Status) {
			return NewRuleError(CodeIntegrity, "缺陷 %s 的级别或状态无效", defect.ID)
		}
		if defect.Status == DefectResolved && defect.ResolvedAt == nil {
			return NewRuleError(CodeIntegrity, "已解决缺陷 %s 缺少解决时间", defect.ID)
		}
		for index, evidence := range defect.EvidenceVersions {
			if evidence.Version != index+1 || evidence.SubmittedBy == "" || evidence.SubmittedAt.IsZero() || evidence.Content == "" {
				return NewRuleError(CodeIntegrity, "缺陷 %s 的证据版本历史无效", defect.ID)
			}
		}
		for _, decision := range defect.ReviewDecisions {
			if decision.EvidenceVersion < 1 || decision.EvidenceVersion > len(defect.EvidenceVersions) ||
				decision.Reviewer == "" || decision.DecidedAt.IsZero() {
				return NewRuleError(CodeIntegrity, "缺陷 %s 的逐项复核历史无效", defect.ID)
			}
		}
		if defect.AcceptedEvidenceVersion < 0 || defect.AcceptedEvidenceVersion > len(defect.EvidenceVersions) {
			return NewRuleError(CodeIntegrity, "缺陷 %s 接受的证据版本无效", defect.ID)
		}
		if defect.Status == DefectResolved && len(defect.EvidenceVersions) > 0 && (defect.AcceptedEvidenceVersion == 0 ||
			defect.AcceptedEvidenceVersion != len(defect.EvidenceVersions)) {
			return NewRuleError(CodeIntegrity, "已解决缺陷 %s 未绑定被接受的最新证据", defect.ID)
		}
		defects[defect.ID] = true
	}
	if c.Status != StatusDraft && c.Status != StatusBaselinePreparation {
		if len(c.Assets) == 0 {
			return NewRuleError(CodeIntegrity, "已锁定档案没有设备")
		}
		for _, asset := range c.Assets {
			if asset.BaselineLockedAt == nil {
				return NewRuleError(CodeIntegrity, "设备 %s 缺少基线锁定时间", asset.AssetCode)
			}
		}
	}
	if c.Status == StatusReviewed && (c.Review == nil || !c.Review.Approved) {
		return NewRuleError(CodeIntegrity, "复核通过状态缺少通过决定")
	}
	if c.Status == StatusFrozen || c.Status == StatusCertified {
		if c.Review == nil || !c.Review.Approved {
			return NewRuleError(CodeIntegrity, "冻结档案缺少独立复核结论")
		}
		if err := VerifyReport(c.Report); err != nil {
			return err
		}
	} else if c.Report != nil {
		return NewRuleError(CodeIntegrity, "未冻结状态包含冻结报告")
	}
	if c.Status == StatusCertified {
		if c.Certificate == nil || !VerifyCertificate(c.Certificate) {
			return NewRuleError(CodeIntegrity, "已签发状态的复役凭据无效")
		}
		if c.Certificate.CaseID != c.ID || (c.Certificate.CaseNumber != "" && c.Certificate.CaseNumber != c.CaseNumber) ||
			c.Certificate.ReportDigest != c.Report.Digest {
			return NewRuleError(CodeIntegrity, "复役凭据未绑定当前冻结报告")
		}
	} else if c.Certificate != nil {
		return NewRuleError(CodeIntegrity, "未签发状态包含复役凭据")
	}
	return nil
}

func validStatus(value CaseStatus) bool {
	switch value {
	case StatusDraft, StatusBaselinePreparation, StatusTesting, StatusPendingReview,
		StatusReturned, StatusReviewed, StatusFrozen, StatusCertified:
		return true
	default:
		return false
	}
}

func validDefectStatus(value DefectStatus) bool {
	return value == DefectOpen || value == DefectRemediated || value == DefectResolved
}
