package domain

import (
	"strings"
	"time"
)

func NewInspectionCase(caseNumber, venueName, scope string, now time.Time) (*InspectionCase, error) {
	caseNumber = strings.ToUpper(NormalizeText(caseNumber))
	venueName = NormalizeText(venueName)
	scope = NormalizeText(scope)
	if caseNumber == "" || len(caseNumber) > 64 {
		return nil, NewRuleError(CodeValidation, "档案编号不能为空且不得超过 64 个字符")
	}
	if venueName == "" || len(venueName) > 160 {
		return nil, NewRuleError(CodeValidation, "场馆名称不能为空且不得超过 160 个字符")
	}
	if scope == "" || len(scope) > 1000 {
		return nil, NewRuleError(CodeValidation, "检验范围不能为空且不得超过 1000 个字符")
	}
	now = now.UTC()
	return &InspectionCase{
		ID: NewID("case"), CaseNumber: caseNumber, VenueName: venueName, Scope: scope,
		Status: StatusDraft, Version: 1, CreatedAt: now, UpdatedAt: now,
		Assets: []RiggingAsset{}, Tests: []LoadTestRecord{}, Defects: []Defect{},
	}, nil
}

func (c *InspectionCase) PrepareBaseline(now time.Time) error {
	if c.Status != StatusDraft {
		return NewRuleError(CodeInvalidState, "只有草稿档案可以进入基线准备态")
	}
	c.Status = StatusBaselinePreparation
	c.advance(now)
	return nil
}

func (c *InspectionCase) advance(now time.Time) {
	c.Version++
	c.UpdatedAt = now.UTC()
}

func (c *InspectionCase) ensureMutable() error {
	if c.Status == StatusFrozen || c.Status == StatusCertified {
		return NewRuleError(CodeInvalidState, "报告冻结后禁止改写设备、测试和整改记录")
	}
	return nil
}

func (c *InspectionCase) AssetByID(id string) (*RiggingAsset, error) {
	for index := range c.Assets {
		if c.Assets[index].ID == id {
			return &c.Assets[index], nil
		}
	}
	return nil, NewRuleError(CodeNotFound, "受检设备不存在")
}

func (c *InspectionCase) DefectByID(id string) (*Defect, error) {
	for index := range c.Defects {
		if c.Defects[index].ID == id {
			return &c.Defects[index], nil
		}
	}
	return nil, NewRuleError(CodeNotFound, "缺陷不存在")
}
