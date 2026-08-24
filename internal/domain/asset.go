package domain

import (
	"strings"
	"time"
)

type AssetInput struct {
	AssetCode            string    `json:"assetCode"`
	AssetType            AssetType `json:"assetType"`
	RatedLoadKg          float64   `json:"ratedLoadKg"`
	BrakeDistanceLimitMm float64   `json:"brakeDistanceLimitMm"`
	LimitDeviceRequired  bool      `json:"limitDeviceRequired"`
}

func (c *InspectionCase) AddAsset(input AssetInput, now time.Time) (*RiggingAsset, error) {
	assets, err := c.AddAssetsBatch([]AssetInput{input}, now)
	if err != nil {
		return nil, err
	}
	return assets[0], nil
}

func (c *InspectionCase) AddAssetsBatch(inputs []AssetInput, now time.Time) ([]*RiggingAsset, error) {
	if err := c.ensureMutable(); err != nil {
		return nil, err
	}
	if c.Status != StatusBaselinePreparation {
		return nil, NewRuleError(CodeInvalidState, "只能在基线准备态登记设备")
	}
	if len(inputs) < 1 || len(inputs) > 100 {
		return nil, NewRuleError(CodeValidation, "批量登记必须包含 1 到 100 台设备")
	}
	existing := make(map[string]bool, len(c.Assets))
	for _, asset := range c.Assets {
		existing[asset.AssetCode] = true
	}
	normalized := make([]AssetInput, len(inputs))
	batch := make(map[string]bool, len(inputs))
	for index, input := range inputs {
		input.AssetCode = strings.ToUpper(NormalizeText(input.AssetCode))
		if err := validateAssetInput(input); err != nil {
			return nil, NewRuleError(CodeValidation, "assets[%d]: %v", index, err)
		}
		if batch[input.AssetCode] {
			return nil, NewRuleError(CodeValidation, "assets[%d]: 批次内设备编号 %s 规范化后重复", index, input.AssetCode)
		}
		if existing[input.AssetCode] {
			return nil, NewRuleError(CodeValidation, "assets[%d]: 设备编号 %s 已存在", index, input.AssetCode)
		}
		batch[input.AssetCode] = true
		normalized[index] = input
	}
	start := len(c.Assets)
	for _, input := range normalized {
		c.Assets = append(c.Assets, RiggingAsset{ID: NewID("asset"), CaseID: c.ID, AssetCode: input.AssetCode,
			AssetType: input.AssetType, RatedLoadKg: input.RatedLoadKg,
			BrakeDistanceLimitMm: input.BrakeDistanceLimitMm, LimitDeviceRequired: input.LimitDeviceRequired})
	}
	c.advance(now)
	result := make([]*RiggingAsset, len(normalized))
	for index := range normalized {
		result[index] = &c.Assets[start+index]
	}
	return result, nil
}

func validateAssetInput(input AssetInput) error {
	if input.AssetCode == "" || len(input.AssetCode) > 64 {
		return NewRuleError(CodeValidation, "设备编号不能为空且不得超过 64 个字符")
	}
	if !validAssetType(input.AssetType) {
		return NewRuleError(CodeValidation, "设备类型无效")
	}
	if input.RatedLoadKg <= 0 || input.RatedLoadKg > 1000000 {
		return NewRuleError(CodeValidation, "额定载荷必须在 0 到 1000000 kg 之间")
	}
	if input.BrakeDistanceLimitMm <= 0 || input.BrakeDistanceLimitMm > 10000 {
		return NewRuleError(CodeValidation, "制动距离阈值必须在 0 到 10000 mm 之间")
	}
	return nil
}

func (c *InspectionCase) LockBaseline(now time.Time) error {
	if c.Status != StatusBaselinePreparation {
		return NewRuleError(CodeInvalidState, "只能在基线准备态锁定安全基线")
	}
	if len(c.Assets) == 0 {
		return NewRuleError(CodeValidation, "至少登记一台设备后才能锁定安全基线")
	}
	timestamp := now.UTC()
	for index := range c.Assets {
		asset := &c.Assets[index]
		if asset.AssetCode == "" || asset.RatedLoadKg <= 0 || asset.BrakeDistanceLimitMm <= 0 {
			return NewRuleError(CodeValidation, "设备 %s 的安全基线不完整", asset.AssetCode)
		}
		asset.BaselineLockedAt = &timestamp
	}
	c.Status = StatusTesting
	c.advance(now)
	return nil
}

func validAssetType(value AssetType) bool {
	switch value {
	case AssetBatten, AssetWinch, AssetWireRope, AssetBrake, AssetLimitDevice:
		return true
	default:
		return false
	}
}
