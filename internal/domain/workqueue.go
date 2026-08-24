package domain

type CaseWorkSummary struct {
	CaseNumber          string     `json:"caseNumber"`
	VenueName           string     `json:"venueName"`
	Status              CaseStatus `json:"status"`
	Version             int64      `json:"version"`
	UpdatedAt           string     `json:"updatedAt"`
	AssetCount          int        `json:"assetCount"`
	TestCoverageRate    float64    `json:"testCoverageRate"`
	OpenDefectCount     int        `json:"openDefectCount"`
	PendingReviewCount  int        `json:"pendingReviewDefectCount"`
	HighestRiskSeverity string     `json:"highestRiskSeverity,omitempty"`
	BlockingReasons     []string   `json:"blockingReasons"`
	CertificateIssued   bool       `json:"certificateIssued"`
}

func (c *InspectionCase) WorkSummary() CaseWorkSummary {
	matrix := c.TestCoverage()
	result := CaseWorkSummary{CaseNumber: c.CaseNumber, VenueName: c.VenueName, Status: c.Status,
		Version: c.Version, UpdatedAt: c.UpdatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		AssetCount: len(c.Assets), TestCoverageRate: matrix.CompletionRate,
		BlockingReasons: append([]string(nil), matrix.BlockingReasons...), CertificateIssued: c.Certificate != nil}
	highest := 0
	for _, defect := range c.Defects {
		if defect.Status == DefectResolved {
			continue
		}
		if defect.Status == DefectOpen {
			result.OpenDefectCount++
		} else {
			result.PendingReviewCount++
		}
		if rank := severityRank(defect.Severity); rank > highest {
			highest = rank
			result.HighestRiskSeverity = string(defect.Severity)
		}
	}
	if result.OpenDefectCount > 0 {
		result.BlockingReasons = append(result.BlockingReasons, "存在未提交整改证据的缺陷")
	}
	if result.PendingReviewCount > 0 {
		result.BlockingReasons = append(result.BlockingReasons, "存在等待逐项复核的整改证据")
	}
	return result
}

func severityRank(value Severity) int {
	switch value {
	case SeverityMinor:
		return 1
	case SeverityMajor:
		return 2
	case SeverityCritical:
		return 3
	default:
		return 0
	}
}
