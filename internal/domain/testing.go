package domain

import (
	"fmt"
	"time"
)

type TestInput struct {
	AssetID          string   `json:"assetId"`
	TestKind         TestKind `json:"testKind"`
	AppliedLoadKg    float64  `json:"appliedLoadKg"`
	BrakeDistanceMm  float64  `json:"brakeDistanceMm"`
	LimitTriggered   bool     `json:"limitTriggered"`
	RecordedBy       string   `json:"recordedBy"`
	RetestOfRecordID string   `json:"retestOfRecordId,omitempty"`
	OriginalRecordID string   `json:"originalRecordId,omitempty"`
}

func (c *InspectionCase) RecordTest(input TestInput, now time.Time) (*LoadTestRecord, *Defect, error) {
	if err := c.ensureMutable(); err != nil {
		return nil, nil, err
	}
	if c.Status != StatusTesting && c.Status != StatusReturned {
		return nil, nil, NewRuleError(CodeInvalidState, "当前状态不允许记录负载测试")
	}
	asset, err := c.AssetByID(input.AssetID)
	if err != nil {
		return nil, nil, err
	}
	if !validTestKind(input.TestKind) {
		return nil, nil, NewRuleError(CodeValidation, "测试类型无效")
	}
	if NormalizeText(input.RecordedBy) == "" {
		return nil, nil, NewRuleError(CodeValidation, "采集责任人不能为空")
	}
	if input.AppliedLoadKg < 0 || input.BrakeDistanceMm < 0 {
		return nil, nil, NewRuleError(CodeValidation, "原始读数不得为负数")
	}
	if input.RetestOfRecordID != "" && input.OriginalRecordID != "" && input.RetestOfRecordID != input.OriginalRecordID {
		return nil, nil, NewRuleError(CodeValidation, "originalRecordId 与 retestOfRecordId 不得冲突")
	}
	if input.RetestOfRecordID == "" {
		input.RetestOfRecordID = input.OriginalRecordID
	}
	if input.RetestOfRecordID != "" {
		original, err := c.testByID(input.RetestOfRecordID)
		if err != nil {
			return nil, nil, err
		}
		if original.AssetID != input.AssetID || original.TestKind != input.TestKind {
			return nil, nil, NewRuleError(CodeValidation, "复测必须与原记录属于同一设备和同一测试类型")
		}
		if original.Result != ResultFail {
			return nil, nil, NewRuleError(CodeValidation, "复测只能引用不合格记录")
		}
		for _, record := range c.Tests {
			if record.RetestOfRecordID == original.ID {
				return nil, nil, NewRuleError(CodeInvalidState, "原失败记录已被后续复测取代")
			}
		}
	}
	result, reason := evaluateTest(*asset, input)
	record := LoadTestRecord{
		ID: NewID("test"), CaseID: c.ID, AssetID: asset.ID, TestKind: input.TestKind,
		AppliedLoadKg: input.AppliedLoadKg, BrakeDistanceMm: input.BrakeDistanceMm,
		LimitTriggered: input.LimitTriggered, Result: result, FailureReason: reason,
		RecordedBy: NormalizeText(input.RecordedBy), RecordedAt: now.UTC(),
		RetestOfRecordID: input.RetestOfRecordID,
	}
	c.Tests = append(c.Tests, record)
	var generated *Defect
	if result == ResultFail {
		severity := SeverityMajor
		if input.TestKind == TestBrake || input.TestKind == TestLimit {
			severity = SeverityCritical
		}
		defect := Defect{ID: NewID("defect"), CaseID: c.ID, AssetID: asset.ID,
			SourceRecordID: record.ID, Severity: severity, Description: reason, Status: DefectOpen}
		c.Defects = append(c.Defects, defect)
		generated = &c.Defects[len(c.Defects)-1]
	}
	c.advance(now)
	return &c.Tests[len(c.Tests)-1], generated, nil
}

func (c *InspectionCase) testByID(id string) (*LoadTestRecord, error) {
	for index := range c.Tests {
		if c.Tests[index].ID == id {
			return &c.Tests[index], nil
		}
	}
	return nil, NewRuleError(CodeValidation, "复测引用的原记录不属于当前档案或不存在")
}

func evaluateTest(asset RiggingAsset, input TestInput) (TestResult, string) {
	switch input.TestKind {
	case TestStatic:
		required := asset.RatedLoadKg * 1.25
		if input.AppliedLoadKg < required {
			return ResultFail, fmt.Sprintf("静载读数 %.2f kg 低于基线要求 %.2f kg", input.AppliedLoadKg, required)
		}
	case TestDynamic:
		required := asset.RatedLoadKg * 1.10
		if input.AppliedLoadKg < required {
			return ResultFail, fmt.Sprintf("动载读数 %.2f kg 低于基线要求 %.2f kg", input.AppliedLoadKg, required)
		}
	case TestBrake:
		if input.BrakeDistanceMm > asset.BrakeDistanceLimitMm {
			return ResultFail, fmt.Sprintf("制动距离 %.2f mm 超过基线阈值 %.2f mm", input.BrakeDistanceMm, asset.BrakeDistanceLimitMm)
		}
	case TestLimit:
		if asset.LimitDeviceRequired && !input.LimitTriggered {
			return ResultFail, "限位装置未按基线要求触发"
		}
	}
	return ResultPass, ""
}

func validTestKind(value TestKind) bool {
	switch value {
	case TestStatic, TestDynamic, TestBrake, TestLimit:
		return true
	default:
		return false
	}
}
