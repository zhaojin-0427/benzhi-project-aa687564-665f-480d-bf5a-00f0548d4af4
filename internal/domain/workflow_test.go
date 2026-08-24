package domain

import (
	"testing"
	"time"
)

func TestInspectionWorkflowWithDefectRemediation(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	c, err := NewInspectionCase("insp-001", "中心剧院", "主舞台吊杆", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.PrepareBaseline(now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	asset, err := c.AddAsset(AssetInput{AssetCode: "B-01", AssetType: AssetBatten,
		RatedLoadKg: 1000, BrakeDistanceLimitMm: 80, LimitDeviceRequired: true}, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.LockBaseline(now.Add(3 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	tests := []TestInput{
		{AssetID: asset.ID, TestKind: TestStatic, AppliedLoadKg: 1250, RecordedBy: "检验员甲"},
		{AssetID: asset.ID, TestKind: TestDynamic, AppliedLoadKg: 1100, RecordedBy: "检验员甲"},
		{AssetID: asset.ID, TestKind: TestBrake, AppliedLoadKg: 1000, BrakeDistanceMm: 100, RecordedBy: "检验员甲"},
		{AssetID: asset.ID, TestKind: TestLimit, LimitTriggered: true, RecordedBy: "检验员甲"},
	}
	for index, input := range tests {
		_, _, err := c.RecordTest(input, now.Add(time.Duration(index+4)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(c.Defects) != 1 || c.Defects[0].Severity != SeverityCritical {
		t.Fatalf("未生成严重制动缺陷: %#v", c.Defects)
	}
	if err := c.SubmitForReview(now.Add(8 * time.Minute)); err == nil {
		t.Fatal("未整改严重缺陷不应进入复核")
	}
	if _, err := c.RemediateDefect(c.Defects[0].ID, "更换制动器并附扭矩及复测记录", now.Add(9*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := c.SubmitForReview(now.Add(10 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := c.ReturnReview("复核员乙", "请补充现场照片", now.Add(11*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := c.RemediateDefect(c.Defects[0].ID, "更换制动器，附扭矩、复测记录和现场照片", now.Add(12*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := c.SubmitForReview(now.Add(13 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ReviewDefect(c.Defects[0].ID, "复核员乙", true, "补充材料完整", now.Add(14*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := c.ApproveReview("复核员乙", "证据完整", now.Add(15*time.Minute)); err != nil {
		t.Fatal(err)
	}
	report, err := c.FreezeReport("复核员乙", now.Add(16*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyReport(report); err != nil {
		t.Fatal(err)
	}
	certificate, err := c.IssueCertificate("复核员乙", now.Add(17*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyCertificate(certificate) || c.Status != StatusCertified {
		t.Fatal("凭据校验或最终状态错误")
	}
	if _, err := c.AddObservedDefect(asset.ID, SeverityMinor, "冻结后修改", now.Add(18*time.Minute)); err == nil {
		t.Fatal("冻结后不应允许改写")
	}
}

func TestThresholdEvaluation(t *testing.T) {
	asset := RiggingAsset{RatedLoadKg: 1000, BrakeDistanceLimitMm: 50, LimitDeviceRequired: true}
	cases := []struct {
		input TestInput
		want  TestResult
	}{
		{TestInput{TestKind: TestStatic, AppliedLoadKg: 1249}, ResultFail},
		{TestInput{TestKind: TestStatic, AppliedLoadKg: 1250}, ResultPass},
		{TestInput{TestKind: TestDynamic, AppliedLoadKg: 1099}, ResultFail},
		{TestInput{TestKind: TestBrake, BrakeDistanceMm: 51}, ResultFail},
		{TestInput{TestKind: TestLimit, LimitTriggered: false}, ResultFail},
	}
	for _, tc := range cases {
		got, _ := evaluateTest(asset, tc.input)
		if got != tc.want {
			t.Fatalf("%s: got %s want %s", tc.input.TestKind, got, tc.want)
		}
	}
}
