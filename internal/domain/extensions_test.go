package domain

import (
	"testing"
	"time"
)

func TestAddAssetsBatchIsAtomicAndAdvancesOnce(t *testing.T) {
	now := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
	inspectionCase, err := NewInspectionCase("batch-001", "批量测试剧院", "吊挂系统", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := inspectionCase.PrepareBaseline(now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	inputs := []AssetInput{
		{AssetCode: " bat-01 ", AssetType: AssetBatten, RatedLoadKg: 1000, BrakeDistanceLimitMm: 50, LimitDeviceRequired: true},
		{AssetCode: "win-02", AssetType: AssetWinch, RatedLoadKg: 800, BrakeDistanceLimitMm: 40},
		{AssetCode: "rope-03", AssetType: AssetWireRope, RatedLoadKg: 500, BrakeDistanceLimitMm: 30},
	}
	assets, err := inspectionCase.AddAssetsBatch(inputs, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 3 || inspectionCase.Version != 3 || assets[0].AssetCode != "BAT-01" {
		t.Fatalf("批量结果无效: version=%d assets=%#v", inspectionCase.Version, assets)
	}
	beforeVersion, beforeCount := inspectionCase.Version, len(inspectionCase.Assets)
	_, err = inspectionCase.AddAssetsBatch([]AssetInput{
		{AssetCode: "new-01", AssetType: AssetBrake, RatedLoadKg: 100, BrakeDistanceLimitMm: 10},
		{AssetCode: " NEW-01 ", AssetType: AssetBrake, RatedLoadKg: 100, BrakeDistanceLimitMm: 10},
	}, now.Add(3*time.Minute))
	if ErrorCodeOf(err) != CodeValidation || inspectionCase.Version != beforeVersion || len(inspectionCase.Assets) != beforeCount {
		t.Fatalf("失败批次改变了聚合: err=%v version=%d count=%d", err, inspectionCase.Version, len(inspectionCase.Assets))
	}
}

func TestRetestCoverageAndVersionedDefectReview(t *testing.T) {
	now := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	inspectionCase, _ := NewInspectionCase("retest-001", "复测剧院", "吊杆", now)
	_ = inspectionCase.PrepareBaseline(now.Add(time.Minute))
	asset, err := inspectionCase.AddAsset(AssetInput{AssetCode: "bat-01", AssetType: AssetBatten,
		RatedLoadKg: 1000, BrakeDistanceLimitMm: 50, LimitDeviceRequired: true}, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	_ = inspectionCase.LockBaseline(now.Add(3 * time.Minute))
	failure, defect, err := inspectionCase.RecordTest(TestInput{AssetID: asset.ID, TestKind: TestBrake,
		BrakeDistanceMm: 80, RecordedBy: "检验员甲"}, now.Add(4*time.Minute))
	if err != nil || defect == nil {
		t.Fatalf("未形成失败证据: %v", err)
	}
	retest, _, err := inspectionCase.RecordTest(TestInput{AssetID: asset.ID, TestKind: TestBrake,
		BrakeDistanceMm: 40, RetestOfRecordID: failure.ID, RecordedBy: "检验员甲"}, now.Add(5*time.Minute))
	if err != nil || retest.Result != ResultPass {
		t.Fatalf("合格复测失败: %#v %v", retest, err)
	}
	matrix := inspectionCase.TestCoverage()
	if matrix.Assets[0].Cells[2].Status != CoverageAwaitingDefectResolution || len(inspectionCase.Tests) != 2 {
		t.Fatalf("覆盖矩阵未保留原失败缺陷: %#v", matrix.Assets[0].Cells[2])
	}
	for index, input := range []TestInput{
		{AssetID: asset.ID, TestKind: TestStatic, AppliedLoadKg: 1250, RecordedBy: "检验员甲"},
		{AssetID: asset.ID, TestKind: TestDynamic, AppliedLoadKg: 1100, RecordedBy: "检验员甲"},
		{AssetID: asset.ID, TestKind: TestLimit, LimitTriggered: true, RecordedBy: "检验员甲"},
	} {
		if _, _, err := inspectionCase.RecordTest(input, now.Add(time.Duration(6+index)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := inspectionCase.RemediateDefectBy(defect.ID, "第一版整改证据", "维护负责人", now.Add(9*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := inspectionCase.SubmitForReview(now.Add(10 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := inspectionCase.ReviewDefect(defect.ID, "复核员乙", false, "缺少现场照片", now.Add(11*time.Minute)); err != nil {
		t.Fatal(err)
	}
	version := inspectionCase.Version
	if err := inspectionCase.SubmitForReview(now.Add(12 * time.Minute)); ErrorCodeOf(err) != CodeInvalidState || inspectionCase.Version != version {
		t.Fatalf("未补证据不应重新提交: %v", err)
	}
	if _, err := inspectionCase.RemediateDefectBy(defect.ID, "第二版整改证据及现场照片", "维护负责人", now.Add(13*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := inspectionCase.SubmitForReview(now.Add(14 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := inspectionCase.ReviewDefect(defect.ID, "复核员乙", true, "材料完整", now.Add(15*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := inspectionCase.ApproveReview("复核员乙", "整体通过", now.Add(16*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if inspectionCase.Defects[0].Status != DefectResolved || len(inspectionCase.Defects[0].EvidenceVersions) != 2 ||
		len(inspectionCase.Defects[0].ReviewDecisions) != 2 || inspectionCase.Defects[0].AcceptedEvidenceVersion != 2 {
		t.Fatalf("证据和决定历史无效: %#v", inspectionCase.Defects[0])
	}
	if _, err := inspectionCase.FreezeReport("复核员乙", now.Add(17*time.Minute)); err != nil {
		t.Fatal(err)
	}
	certificate, err := inspectionCase.IssueCertificate("复核员乙", now.Add(18*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	carried := CarriedCertificate{CertificateNumber: certificate.CertificateNumber, CertificateID: certificate.ID,
		CaseNumber: inspectionCase.CaseNumber, ReportDigest: certificate.ReportDigest,
		IssuedAt: certificate.IssuedAt, VerificationDigest: certificate.VerificationDigest}
	verification := VerifyCarriedCertificate(inspectionCase.CaseNumber, carried, inspectionCase)
	if !verification.Valid {
		t.Fatalf("原始凭据核验失败: %#v", verification)
	}
	carried.ReportDigest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	verification = VerifyCarriedCertificate(inspectionCase.CaseNumber, carried, inspectionCase)
	if verification.Valid || verification.Reason != CertificateReportMismatch {
		t.Fatalf("篡改报告摘要未被区分: %#v", verification)
	}
}
