package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"
)

type reportDocument struct {
	CaseNumber string           `json:"caseNumber"`
	VenueName  string           `json:"venueName"`
	Scope      string           `json:"scope"`
	Assets     []RiggingAsset   `json:"assets"`
	Tests      []LoadTestRecord `json:"tests"`
	Defects    []Defect         `json:"defects"`
	Review     *ReviewDecision  `json:"review"`
}

func (c *InspectionCase) FreezeReport(actor string, now time.Time) (*FrozenReport, error) {
	if c.Status != StatusReviewed || c.Review == nil || !c.Review.Approved {
		return nil, NewRuleError(CodeInvalidState, "只有独立复核通过的档案可以冻结报告")
	}
	document := reportDocument{CaseNumber: c.CaseNumber, VenueName: c.VenueName, Scope: c.Scope,
		Assets: append([]RiggingAsset(nil), c.Assets...), Tests: append([]LoadTestRecord(nil), c.Tests...),
		Defects: append([]Defect(nil), c.Defects...), Review: c.Review}
	sort.Slice(document.Assets, func(i, j int) bool { return document.Assets[i].ID < document.Assets[j].ID })
	sort.Slice(document.Tests, func(i, j int) bool { return document.Tests[i].ID < document.Tests[j].ID })
	sort.Slice(document.Defects, func(i, j int) bool { return document.Defects[i].ID < document.Defects[j].ID })
	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, NewRuleError(CodeIntegrity, "无法规范化检验报告: %v", err)
	}
	digest := sha256.Sum256(encoded)
	report := &FrozenReport{Digest: hex.EncodeToString(digest[:]), Content: string(encoded),
		FrozenBy: NormalizeText(actor), FrozenAt: now.UTC(), Version: c.Version + 1}
	c.Report = report
	c.Status = StatusFrozen
	c.advance(now)
	return report, nil
}

func VerifyReport(report *FrozenReport) error {
	if report == nil || report.Content == "" || report.Digest == "" {
		return NewRuleError(CodeIntegrity, "冻结报告缺失")
	}
	digest := sha256.Sum256([]byte(report.Content))
	if hex.EncodeToString(digest[:]) != report.Digest {
		return NewRuleError(CodeIntegrity, "冻结报告摘要校验失败")
	}
	return nil
}
