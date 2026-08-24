package application

import (
	"encoding/json"
	"time"

	"stage-rigging-clearance/internal/domain"
)

const (
	RoleInspector   = "inspector"
	RoleMaintenance = "maintenance_manager"
	RoleReviewer    = "independent_reviewer"
)

type CommandMeta struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
	Actor           string `json:"actor"`
	Role            string `json:"role"`
}

type CreateCaseCommand struct {
	CommandMeta
	CaseNumber string `json:"caseNumber"`
	VenueName  string `json:"venueName"`
	Scope      string `json:"scope"`
}

type CaseCommand struct {
	CommandMeta
	CaseNumber string `json:"-"`
}

type AddAssetCommand struct {
	CommandMeta
	CaseNumber string            `json:"-"`
	Asset      domain.AssetInput `json:"asset"`
}

type AddAssetsBatchCommand struct {
	CommandMeta
	CaseNumber string              `json:"-"`
	Assets     []domain.AssetInput `json:"assets"`
}

type RecordTestCommand struct {
	CommandMeta
	CaseNumber string           `json:"-"`
	Test       domain.TestInput `json:"test"`
}

type AddDefectCommand struct {
	CommandMeta
	CaseNumber  string          `json:"-"`
	AssetID     string          `json:"assetId"`
	Severity    domain.Severity `json:"severity"`
	Description string          `json:"description"`
}

type RemediateDefectCommand struct {
	CommandMeta
	CaseNumber string `json:"-"`
	DefectID   string `json:"-"`
	Evidence   string `json:"evidence"`
}

type ReviewCommand struct {
	CommandMeta
	CaseNumber string `json:"-"`
	Comment    string `json:"comment"`
}

type ReviewDefectCommand struct {
	CommandMeta
	CaseNumber string `json:"-"`
	DefectID   string `json:"-"`
	Accepted   bool   `json:"accepted"`
	Comment    string `json:"comment"`
}

type WorkQueueQuery struct {
	Statuses        []domain.CaseStatus
	VenueName       string
	UpdatedFrom     *time.Time
	UpdatedTo       *time.Time
	HighestSeverity domain.Severity
	Limit           int
	Cursor          string
}

type Result struct {
	StatusCode int
	Body       []byte
}

type caseEnvelope struct {
	Case *domain.InspectionCase `json:"case"`
}

type auditEnvelope struct {
	CaseNumber string              `json:"caseNumber"`
	Valid      bool                `json:"valid"`
	Events     []domain.AuditEvent `json:"events"`
}

type certificateEnvelope struct {
	CaseNumber  string                             `json:"caseNumber"`
	Valid       bool                               `json:"valid"`
	Certificate *domain.ReturnToServiceCertificate `json:"certificate"`
}

type batchAssetResult struct {
	ID                  string `json:"id"`
	AssetCode           string `json:"assetCode"`
	NormalizedAssetCode string `json:"normalizedAssetCode"`
	Result              string `json:"result"`
}

type batchAssetEnvelope struct {
	CaseNumber    string             `json:"caseNumber"`
	Results       []batchAssetResult `json:"results"`
	AddedCount    int                `json:"addedCount"`
	LatestVersion int64              `json:"latestVersion"`
}

type coverageEnvelope struct {
	CaseNumber string                `json:"caseNumber"`
	Matrix     domain.CoverageMatrix `json:"matrix"`
}

func marshal(value any) ([]byte, error) { return json.Marshal(value) }
