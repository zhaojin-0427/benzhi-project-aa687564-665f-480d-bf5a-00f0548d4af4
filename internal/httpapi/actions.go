package httpapi

import (
	"net/http"

	"stage-rigging-clearance/internal/application"
	"stage-rigging-clearance/internal/domain"
)

type actionFunc func() (*application.Result, error)

func runAction(w http.ResponseWriter, action actionFunc) {
	result, err := action()
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, result.StatusCode, result.Body)
}

func (a *API) HandlePrepareBaseline(w http.ResponseWriter, r *http.Request, caseNumber string) {
	var cmd application.CaseCommand
	if !decodeCommand(w, r, &cmd) {
		return
	}
	cmd.CaseNumber = caseNumber
	runAction(w, func() (*application.Result, error) { return a.service.PrepareBaseline(r.Context(), cmd) })
}

func (a *API) HandleAddAsset(w http.ResponseWriter, r *http.Request, caseNumber string) {
	var cmd application.AddAssetCommand
	if !decodeCommand(w, r, &cmd) {
		return
	}
	cmd.CaseNumber = caseNumber
	runAction(w, func() (*application.Result, error) { return a.service.AddAsset(r.Context(), cmd) })
}

func (a *API) HandleAddAssetsBatch(w http.ResponseWriter, r *http.Request, caseNumber string) {
	var cmd application.AddAssetsBatchCommand
	if !decodeCommand(w, r, &cmd) {
		return
	}
	cmd.CaseNumber = caseNumber
	runAction(w, func() (*application.Result, error) { return a.service.AddAssetsBatch(r.Context(), cmd) })
}

func (a *API) HandleLockBaseline(w http.ResponseWriter, r *http.Request, caseNumber string) {
	var cmd application.CaseCommand
	if !decodeCommand(w, r, &cmd) {
		return
	}
	cmd.CaseNumber = caseNumber
	runAction(w, func() (*application.Result, error) { return a.service.LockBaseline(r.Context(), cmd) })
}

func (a *API) HandleRecordTest(w http.ResponseWriter, r *http.Request, caseNumber string) {
	var cmd application.RecordTestCommand
	if !decodeCommand(w, r, &cmd) {
		return
	}
	cmd.CaseNumber = caseNumber
	runAction(w, func() (*application.Result, error) { return a.service.RecordTest(r.Context(), cmd) })
}

func (a *API) HandleAddDefect(w http.ResponseWriter, r *http.Request, caseNumber string) {
	var cmd application.AddDefectCommand
	if !decodeCommand(w, r, &cmd) {
		return
	}
	cmd.CaseNumber = caseNumber
	runAction(w, func() (*application.Result, error) { return a.service.AddObservedDefect(r.Context(), cmd) })
}

func (a *API) HandleRemediateDefect(w http.ResponseWriter, r *http.Request, caseNumber, defectID string) {
	var cmd application.RemediateDefectCommand
	if !decodeCommand(w, r, &cmd) {
		return
	}
	cmd.CaseNumber, cmd.DefectID = caseNumber, defectID
	runAction(w, func() (*application.Result, error) { return a.service.RemediateDefect(r.Context(), cmd) })
}

func (a *API) HandleReviewDefect(w http.ResponseWriter, r *http.Request, caseNumber, defectID string) {
	var cmd application.ReviewDefectCommand
	if !decodeCommand(w, r, &cmd) {
		return
	}
	cmd.CaseNumber, cmd.DefectID = caseNumber, defectID
	runAction(w, func() (*application.Result, error) { return a.service.ReviewDefect(r.Context(), cmd) })
}

func (a *API) HandleVerifyCertificate(w http.ResponseWriter, r *http.Request, caseNumber string) {
	var carried domain.CarriedCertificate
	if !decodeCommand(w, r, &carried) {
		return
	}
	runAction(w, func() (*application.Result, error) {
		return a.service.VerifyCertificate(r.Context(), caseNumber, carried)
	})
}

func (a *API) HandleSubmitReview(w http.ResponseWriter, r *http.Request, caseNumber string) {
	var cmd application.CaseCommand
	if !decodeCommand(w, r, &cmd) {
		return
	}
	cmd.CaseNumber = caseNumber
	runAction(w, func() (*application.Result, error) { return a.service.SubmitReview(r.Context(), cmd) })
}

func (a *API) HandleReturnReview(w http.ResponseWriter, r *http.Request, caseNumber string) {
	var cmd application.ReviewCommand
	if !decodeCommand(w, r, &cmd) {
		return
	}
	cmd.CaseNumber = caseNumber
	runAction(w, func() (*application.Result, error) { return a.service.ReturnReview(r.Context(), cmd) })
}

func (a *API) HandleApproveReview(w http.ResponseWriter, r *http.Request, caseNumber string) {
	var cmd application.ReviewCommand
	if !decodeCommand(w, r, &cmd) {
		return
	}
	cmd.CaseNumber = caseNumber
	runAction(w, func() (*application.Result, error) { return a.service.ApproveReview(r.Context(), cmd) })
}

func (a *API) HandleFreezeReport(w http.ResponseWriter, r *http.Request, caseNumber string) {
	var cmd application.CaseCommand
	if !decodeCommand(w, r, &cmd) {
		return
	}
	cmd.CaseNumber = caseNumber
	runAction(w, func() (*application.Result, error) { return a.service.FreezeReport(r.Context(), cmd) })
}

func (a *API) HandleIssueCertificate(w http.ResponseWriter, r *http.Request, caseNumber string) {
	var cmd application.CaseCommand
	if !decodeCommand(w, r, &cmd) {
		return
	}
	cmd.CaseNumber = caseNumber
	runAction(w, func() (*application.Result, error) { return a.service.IssueCertificate(r.Context(), cmd) })
}

func decodeCommand(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := decodeJSON(w, r, target); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", err.Error())
		return false
	}
	return true
}
