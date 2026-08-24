package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"stage-rigging-clearance/internal/domain"
)

type selfcheckCaseEnvelope struct {
	Case domain.InspectionCase `json:"case"`
}

func runSelfcheck(address string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	baseURL := "http://" + address
	caseNumber := "SELF-CHECK-" + time.Now().UTC().Format("20060102150405.000000000")
	meta := func(version int64, sequence int, role, actor string) map[string]any {
		return map[string]any{"expectedVersion": version, "idempotencyKey": fmt.Sprintf("selfcheck-%02d-%d", sequence, time.Now().UnixNano()), "role": role, "actor": actor}
	}
	create := meta(0, 1, "inspector", "检验员-自检")
	create["caseNumber"], create["venueName"], create["scope"] = caseNumber, "本智自检剧场", "主舞台一号吊挂系统"
	aggregate, err := selfcheckPost(client, baseURL+"/api/v1/inspection-cases", create, http.StatusCreated)
	if err != nil {
		return err
	}
	path := baseURL + "/api/v1/inspection-cases/" + caseNumber
	aggregate, err = selfcheckPost(client, path+"/prepare", meta(aggregate.Version, 2, "inspector", "检验员-自检"), http.StatusOK)
	if err != nil {
		return err
	}
	assetCommand := meta(aggregate.Version, 3, "inspector", "检验员-自检")
	assetCommand["asset"] = map[string]any{"assetCode": "BAT-SELF-01", "assetType": "batten", "ratedLoadKg": 1000,
		"brakeDistanceLimitMm": 80, "limitDeviceRequired": true}
	aggregate, err = selfcheckPost(client, path+"/assets", assetCommand, http.StatusOK)
	if err != nil {
		return err
	}
	assetID := aggregate.Assets[0].ID
	aggregate, err = selfcheckPost(client, path+"/baseline/lock", meta(aggregate.Version, 4, "inspector", "检验员-自检"), http.StatusOK)
	if err != nil {
		return err
	}
	tests := []map[string]any{
		{"assetId": assetID, "testKind": "static_load", "appliedLoadKg": 1250, "brakeDistanceMm": 0, "limitTriggered": false},
		{"assetId": assetID, "testKind": "dynamic_load", "appliedLoadKg": 1100, "brakeDistanceMm": 0, "limitTriggered": false},
		{"assetId": assetID, "testKind": "brake_distance", "appliedLoadKg": 1000, "brakeDistanceMm": 60, "limitTriggered": false},
		{"assetId": assetID, "testKind": "limit_trigger", "appliedLoadKg": 0, "brakeDistanceMm": 0, "limitTriggered": true},
	}
	for index, test := range tests {
		command := meta(aggregate.Version, 5+index, "inspector", "检验员-自检")
		command["test"] = test
		aggregate, err = selfcheckPost(client, path+"/tests", command, http.StatusOK)
		if err != nil {
			return err
		}
	}
	if err := selfcheckGet(client, path+"/tests/coverage", `"completionRate":1`); err != nil {
		return err
	}
	aggregate, err = selfcheckPost(client, path+"/review/submit", meta(aggregate.Version, 9, "inspector", "检验员-自检"), http.StatusOK)
	if err != nil {
		return err
	}
	approve := meta(aggregate.Version, 10, "independent_reviewer", "独立复核员-自检")
	approve["comment"] = "测试覆盖和证据满足安全门槛"
	aggregate, err = selfcheckPost(client, path+"/review/approve", approve, http.StatusOK)
	if err != nil {
		return err
	}
	aggregate, err = selfcheckPost(client, path+"/report/freeze", meta(aggregate.Version, 11, "independent_reviewer", "独立复核员-自检"), http.StatusOK)
	if err != nil {
		return err
	}
	aggregate, err = selfcheckPost(client, path+"/certificate/issue", meta(aggregate.Version, 12, "independent_reviewer", "独立复核员-自检"), http.StatusOK)
	if err != nil {
		return err
	}
	if aggregate.Status != domain.StatusCertified || aggregate.Certificate == nil || !domain.VerifyCertificate(aggregate.Certificate) {
		return fmt.Errorf("最终档案或复役凭据状态无效")
	}
	if err := selfcheckGet(client, path+"/audit", `"valid":true`); err != nil {
		return err
	}
	if err := selfcheckGet(client, path+"/certificate", `"valid":true`); err != nil {
		return err
	}
	carried := map[string]any{"certificateNumber": aggregate.Certificate.CertificateNumber,
		"certificateId": aggregate.Certificate.ID, "caseNumber": aggregate.CaseNumber,
		"reportDigest": aggregate.Certificate.ReportDigest, "issuedAt": aggregate.Certificate.IssuedAt,
		"verificationDigest": aggregate.Certificate.VerificationDigest}
	if err := selfcheckPostContains(client, path+"/certificate/verify", carried, `"valid":true`); err != nil {
		return err
	}
	return selfcheckGet(client, path, `"version":12`)
}

func selfcheckPost(client *http.Client, url string, payload any, expectedStatus int) (domain.InspectionCase, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return domain.InspectionCase{}, err
	}
	response, err := client.Post(url, "application/json", bytes.NewReader(encoded))
	if err != nil {
		return domain.InspectionCase{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return domain.InspectionCase{}, err
	}
	if response.StatusCode != expectedStatus {
		return domain.InspectionCase{}, fmt.Errorf("POST %s 返回 %d: %s", url, response.StatusCode, body)
	}
	var envelope selfcheckCaseEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return domain.InspectionCase{}, err
	}
	return envelope.Case, nil
}

func selfcheckGet(client *http.Client, url, required string) error {
	response, err := client.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK || !bytes.Contains(body, []byte(required)) {
		return fmt.Errorf("GET %s 校验失败: %d %s", url, response.StatusCode, body)
	}
	return nil
}

func selfcheckPostContains(client *http.Client, url string, payload any, required string) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	response, err := client.Post(url, "application/json", bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK || !bytes.Contains(body, []byte(required)) {
		return fmt.Errorf("POST %s 校验失败: %d %s", url, response.StatusCode, body)
	}
	return nil
}
