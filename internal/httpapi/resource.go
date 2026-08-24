package httpapi

import (
	"net/http"
	"net/url"
	"strings"
)

func (a *API) HandleCaseResource(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/inspection-cases/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeProblem(w, http.StatusNotFound, "not_found", "档案编号缺失")
		return
	}
	caseNumber, err := url.PathUnescape(parts[0])
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_path", "档案编号编码无效")
		return
	}
	if len(parts) == 1 {
		a.HandleGetCase(w, r, caseNumber)
		return
	}
	a.dispatchAction(w, r, caseNumber, parts[1:])
}

func (a *API) dispatchAction(w http.ResponseWriter, r *http.Request, caseNumber string, parts []string) {
	if r.Method == http.MethodGet && len(parts) == 1 {
		switch parts[0] {
		case "audit":
			a.HandleGetAudit(w, r, caseNumber)
			return
		case "certificate":
			a.HandleGetCertificate(w, r, caseNumber)
			return
		}
	}
	if r.Method == http.MethodGet && len(parts) == 2 && parts[0] == "tests" && parts[1] == "coverage" {
		a.HandleGetTestCoverage(w, r, caseNumber)
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
		return
	}
	switch {
	case len(parts) == 1 && parts[0] == "prepare":
		a.HandlePrepareBaseline(w, r, caseNumber)
	case len(parts) == 1 && parts[0] == "assets":
		a.HandleAddAsset(w, r, caseNumber)
	case len(parts) == 2 && parts[0] == "assets" && parts[1] == "batch":
		a.HandleAddAssetsBatch(w, r, caseNumber)
	case len(parts) == 2 && parts[0] == "baseline" && parts[1] == "lock":
		a.HandleLockBaseline(w, r, caseNumber)
	case len(parts) == 1 && parts[0] == "tests":
		a.HandleRecordTest(w, r, caseNumber)
	case len(parts) == 1 && parts[0] == "defects":
		a.HandleAddDefect(w, r, caseNumber)
	case len(parts) == 3 && parts[0] == "defects" && parts[2] == "remediation":
		a.HandleRemediateDefect(w, r, caseNumber, parts[1])
	case len(parts) == 3 && parts[0] == "defects" && parts[2] == "review":
		a.HandleReviewDefect(w, r, caseNumber, parts[1])
	case len(parts) == 2 && parts[0] == "review" && parts[1] == "submit":
		a.HandleSubmitReview(w, r, caseNumber)
	case len(parts) == 2 && parts[0] == "review" && parts[1] == "return":
		a.HandleReturnReview(w, r, caseNumber)
	case len(parts) == 2 && parts[0] == "review" && parts[1] == "approve":
		a.HandleApproveReview(w, r, caseNumber)
	case len(parts) == 2 && parts[0] == "report" && parts[1] == "freeze":
		a.HandleFreezeReport(w, r, caseNumber)
	case len(parts) == 2 && parts[0] == "certificate" && parts[1] == "issue":
		a.HandleIssueCertificate(w, r, caseNumber)
	case len(parts) == 2 && parts[0] == "certificate" && (parts[1] == "verify" || parts[1] == "verification"):
		a.HandleVerifyCertificate(w, r, caseNumber)
	default:
		writeProblem(w, http.StatusNotFound, "not_found", "操作路由不存在")
	}
}

func (a *API) HandleGetTestCoverage(w http.ResponseWriter, r *http.Request, caseNumber string) {
	result, err := a.service.GetTestCoverage(r.Context(), caseNumber)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, result.StatusCode, result.Body)
}

func (a *API) HandleGetCase(w http.ResponseWriter, r *http.Request, caseNumber string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	result, err := a.service.GetCase(r.Context(), caseNumber)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, result.StatusCode, result.Body)
}

func (a *API) HandleGetAudit(w http.ResponseWriter, r *http.Request, caseNumber string) {
	result, err := a.service.GetAudit(r.Context(), caseNumber)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, result.StatusCode, result.Body)
}

func (a *API) HandleGetCertificate(w http.ResponseWriter, r *http.Request, caseNumber string) {
	result, err := a.service.GetCertificate(r.Context(), caseNumber)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, result.StatusCode, result.Body)
}
