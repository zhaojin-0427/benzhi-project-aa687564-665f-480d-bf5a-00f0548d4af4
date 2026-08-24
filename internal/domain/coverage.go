package domain

type CoverageStatus string

const (
	CoverageNotTested                CoverageStatus = "not_tested"
	CoverageLatestPass               CoverageStatus = "latest_pass"
	CoverageLatestFail               CoverageStatus = "latest_fail"
	CoverageAwaitingDefectResolution CoverageStatus = "awaiting_defect_resolution"
)

type CoverageCell struct {
	TestKind          TestKind       `json:"testKind"`
	Status            CoverageStatus `json:"status"`
	LatestRecordID    string         `json:"latestRecordId,omitempty"`
	LatestResult      TestResult     `json:"latestResult,omitempty"`
	AttemptCount      int            `json:"attemptCount"`
	BlockingDefectIDs []string       `json:"blockingDefectIds"`
}

type AssetCoverage struct {
	AssetID        string         `json:"assetId"`
	AssetCode      string         `json:"assetCode"`
	Cells          []CoverageCell `json:"tests"`
	Completed      int            `json:"completed"`
	Total          int            `json:"total"`
	CompletionRate float64        `json:"completionRate"`
}

type CoverageMatrix struct {
	Assets          []AssetCoverage `json:"assets"`
	Completed       int             `json:"completed"`
	Total           int             `json:"total"`
	CompletionRate  float64         `json:"completionRate"`
	BlockingReasons []string        `json:"blockingReasons"`
}

var requiredTestKinds = []TestKind{TestStatic, TestDynamic, TestBrake, TestLimit}

func (c *InspectionCase) TestCoverage() CoverageMatrix {
	matrix := CoverageMatrix{Assets: make([]AssetCoverage, 0, len(c.Assets)), Total: len(c.Assets) * len(requiredTestKinds)}
	for _, asset := range c.Assets {
		row := AssetCoverage{AssetID: asset.ID, AssetCode: asset.AssetCode, Total: len(requiredTestKinds)}
		for _, kind := range requiredTestKinds {
			cell := CoverageCell{TestKind: kind, Status: CoverageNotTested, BlockingDefectIDs: []string{}}
			var latest *LoadTestRecord
			for index := range c.Tests {
				record := &c.Tests[index]
				if record.AssetID == asset.ID && record.TestKind == kind {
					cell.AttemptCount++
					latest = record
				}
			}
			if latest != nil {
				cell.LatestRecordID, cell.LatestResult = latest.ID, latest.Result
				cell.Status = CoverageLatestFail
				if latest.Result == ResultPass {
					cell.Status = CoverageLatestPass
				}
				row.Completed++
				matrix.Completed++
			}
			for _, defect := range c.Defects {
				if defect.AssetID != asset.ID || defect.Status == DefectResolved || defect.SourceRecordID == "" {
					continue
				}
				source, err := c.testByID(defect.SourceRecordID)
				if err == nil && source.TestKind == kind {
					cell.BlockingDefectIDs = append(cell.BlockingDefectIDs, defect.ID)
				}
			}
			if len(cell.BlockingDefectIDs) > 0 {
				cell.Status = CoverageAwaitingDefectResolution
			}
			row.Cells = append(row.Cells, cell)
		}
		row.CompletionRate = fraction(row.Completed, row.Total)
		matrix.Assets = append(matrix.Assets, row)
	}
	matrix.CompletionRate = fraction(matrix.Completed, matrix.Total)
	for _, row := range matrix.Assets {
		for _, cell := range row.Cells {
			switch cell.Status {
			case CoverageNotTested:
				matrix.BlockingReasons = append(matrix.BlockingReasons, "设备 "+row.AssetCode+" 缺少 "+string(cell.TestKind)+" 测试")
			case CoverageLatestFail:
				matrix.BlockingReasons = append(matrix.BlockingReasons, "设备 "+row.AssetCode+" 的 "+string(cell.TestKind)+" 最近测试不合格")
			case CoverageAwaitingDefectResolution:
				matrix.BlockingReasons = append(matrix.BlockingReasons, "设备 "+row.AssetCode+" 的 "+string(cell.TestKind)+" 存在未闭环缺陷")
			}
		}
	}
	return matrix
}

func fraction(value, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(value) / float64(total)
}
