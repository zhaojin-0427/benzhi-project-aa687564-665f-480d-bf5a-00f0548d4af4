package httpapi

import (
	"encoding/json"
	"net/http"

	"stage-rigging-clearance/internal/domain"
)

type problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

func handleError(w http.ResponseWriter, err error) {
	code := domain.ErrorCodeOf(err)
	status := http.StatusInternalServerError
	title := "服务器内部错误"
	switch code {
	case domain.CodeValidation:
		status, title = http.StatusBadRequest, "请求校验失败"
	case domain.CodeNotFound:
		status, title = http.StatusNotFound, "资源不存在"
	case domain.CodeConflict, domain.CodeIdempotencyReuse:
		status, title = http.StatusConflict, "请求冲突"
	case domain.CodeInvalidState:
		status, title = http.StatusUnprocessableEntity, "业务状态不允许"
	case domain.CodeForbidden:
		status, title = http.StatusForbidden, "角色权限不足"
	case domain.CodeIntegrity:
		status, title = http.StatusInternalServerError, "数据完整性错误"
	}
	detail := err.Error()
	if status == http.StatusInternalServerError && code == domain.CodeIntegrity && detail == "" {
		detail = "服务无法完成请求"
	}
	writeProblemFull(w, status, string(code), title, detail)
}

func writeProblem(w http.ResponseWriter, status int, code, detail string) {
	writeProblemFull(w, status, code, http.StatusText(status), detail)
}

func writeProblemFull(w http.ResponseWriter, status int, code, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	body, _ := json.Marshal(problem{Type: "urn:stage-rigging-clearance:problem:" + code,
		Title: title, Status: status, Code: code, Detail: detail})
	writeJSON(w, status, body)
}

func methodNotAllowed(w http.ResponseWriter, methods ...string) {
	for index, method := range methods {
		if index > 0 {
			w.Header().Add("Allow", method)
		} else {
			w.Header().Set("Allow", method)
		}
	}
	writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不受支持")
}
