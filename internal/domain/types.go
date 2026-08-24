package domain

import "time"

type CaseStatus string

const (
	StatusDraft               CaseStatus = "draft"
	StatusBaselinePreparation CaseStatus = "baseline_preparation"
	StatusTesting             CaseStatus = "testing"
	StatusPendingReview       CaseStatus = "pending_review"
	StatusReturned            CaseStatus = "returned"
	StatusReviewed            CaseStatus = "reviewed"
	StatusFrozen              CaseStatus = "frozen"
	StatusCertified           CaseStatus = "certified"
)

type AssetType string

const (
	AssetBatten      AssetType = "batten"
	AssetWinch       AssetType = "winch"
	AssetWireRope    AssetType = "wire_rope"
	AssetBrake       AssetType = "brake"
	AssetLimitDevice AssetType = "limit_device"
)

type TestKind string

const (
	TestStatic  TestKind = "static_load"
	TestDynamic TestKind = "dynamic_load"
	TestBrake   TestKind = "brake_distance"
	TestLimit   TestKind = "limit_trigger"
)

type TestResult string

const (
	ResultPass TestResult = "pass"
	ResultFail TestResult = "fail"
)

type Severity string

const (
	SeverityMinor    Severity = "minor"
	SeverityMajor    Severity = "major"
	SeverityCritical Severity = "critical"
)

type DefectStatus string

const (
	DefectOpen       DefectStatus = "open"
	DefectRemediated DefectStatus = "remediated"
	DefectResolved   DefectStatus = "resolved"
)

type InspectionCase struct {
	ID          string                      `json:"id"`
	CaseNumber  string                      `json:"caseNumber"`
	VenueName   string                      `json:"venueName"`
	Scope       string                      `json:"scope"`
	Status      CaseStatus                  `json:"status"`
	Version     int64                       `json:"version"`
	CreatedAt   time.Time                   `json:"createdAt"`
	UpdatedAt   time.Time                   `json:"updatedAt"`
	Assets      []RiggingAsset              `json:"assets"`
	Tests       []LoadTestRecord            `json:"tests"`
	Defects     []Defect                    `json:"defects"`
	Review      *ReviewDecision             `json:"review,omitempty"`
	Report      *FrozenReport               `json:"report,omitempty"`
	Certificate *ReturnToServiceCertificate `json:"certificate,omitempty"`
}

type RiggingAsset struct {
	ID                   string     `json:"id"`
	CaseID               string     `json:"caseId"`
	AssetCode            string     `json:"assetCode"`
	AssetType            AssetType  `json:"assetType"`
	RatedLoadKg          float64    `json:"ratedLoadKg"`
	BrakeDistanceLimitMm float64    `json:"brakeDistanceLimitMm"`
	LimitDeviceRequired  bool       `json:"limitDeviceRequired"`
	BaselineLockedAt     *time.Time `json:"baselineLockedAt,omitempty"`
}

type LoadTestRecord struct {
	ID               string     `json:"id"`
	CaseID           string     `json:"caseId"`
	AssetID          string     `json:"assetId"`
	TestKind         TestKind   `json:"testKind"`
	AppliedLoadKg    float64    `json:"appliedLoadKg"`
	BrakeDistanceMm  float64    `json:"brakeDistanceMm"`
	LimitTriggered   bool       `json:"limitTriggered"`
	Result           TestResult `json:"result"`
	FailureReason    string     `json:"failureReason,omitempty"`
	RecordedBy       string     `json:"recordedBy"`
	RecordedAt       time.Time  `json:"recordedAt"`
	RetestOfRecordID string     `json:"retestOfRecordId,omitempty"`
}

type RemediationEvidenceVersion struct {
	Version     int       `json:"version"`
	SubmittedBy string    `json:"submittedBy"`
	SubmittedAt time.Time `json:"submittedAt"`
	Content     string    `json:"content"`
}

type DefectReviewDecision struct {
	EvidenceVersion int       `json:"evidenceVersion"`
	Reviewer        string    `json:"reviewer"`
	Accepted        bool      `json:"accepted"`
	Comment         string    `json:"comment,omitempty"`
	DecidedAt       time.Time `json:"decidedAt"`
}

type Defect struct {
	ID                      string                       `json:"id"`
	CaseID                  string                       `json:"caseId"`
	AssetID                 string                       `json:"assetId"`
	SourceRecordID          string                       `json:"sourceRecordId,omitempty"`
	Severity                Severity                     `json:"severity"`
	Description             string                       `json:"description"`
	Status                  DefectStatus                 `json:"status"`
	RemediationEvidence     string                       `json:"remediationEvidence,omitempty"`
	ReviewComment           string                       `json:"reviewComment,omitempty"`
	ResolvedAt              *time.Time                   `json:"resolvedAt,omitempty"`
	EvidenceVersions        []RemediationEvidenceVersion `json:"evidenceVersions"`
	ReviewDecisions         []DefectReviewDecision       `json:"reviewDecisions"`
	AcceptedEvidenceVersion int                          `json:"acceptedEvidenceVersion,omitempty"`
}

type ReviewDecision struct {
	Reviewer string    `json:"reviewer"`
	Comment  string    `json:"comment,omitempty"`
	Approved bool      `json:"approved"`
	At       time.Time `json:"at"`
}

type FrozenReport struct {
	Digest   string    `json:"digest"`
	Content  string    `json:"content"`
	FrozenBy string    `json:"frozenBy"`
	FrozenAt time.Time `json:"frozenAt"`
	Version  int64     `json:"version"`
}

type ReturnToServiceCertificate struct {
	ID                 string    `json:"id"`
	CaseID             string    `json:"caseId"`
	CaseNumber         string    `json:"caseNumber"`
	ReportDigest       string    `json:"reportDigest"`
	CertificateNumber  string    `json:"certificateNumber"`
	IssuedBy           string    `json:"issuedBy"`
	IssuedAt           time.Time `json:"issuedAt"`
	VerificationDigest string    `json:"verificationDigest"`
}

type AuditEvent struct {
	CaseID        string    `json:"caseId"`
	Sequence      int64     `json:"sequence"`
	Actor         string    `json:"actor"`
	Role          string    `json:"role"`
	Command       string    `json:"command"`
	BeforeVersion int64     `json:"beforeVersion"`
	AfterVersion  int64     `json:"afterVersion"`
	OccurredAt    time.Time `json:"occurredAt"`
	ResultDigest  string    `json:"resultDigest"`
	PreviousHash  string    `json:"previousHash"`
	Hash          string    `json:"hash"`
}
