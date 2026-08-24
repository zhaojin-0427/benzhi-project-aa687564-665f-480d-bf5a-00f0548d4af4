package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"stage-rigging-clearance/internal/application"
	"stage-rigging-clearance/internal/domain"
)

func (a *API) HandleCaseCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		query, err := parseWorkQueueQuery(r)
		if err != nil {
			handleError(w, err)
			return
		}
		runAction(w, func() (*application.Result, error) { return a.service.GetWorkQueue(r.Context(), query) })
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
		return
	}
	var command application.CreateCaseCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	result, err := a.service.CreateCase(r.Context(), command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, result.StatusCode, result.Body)
}

func parseWorkQueueQuery(r *http.Request) (application.WorkQueueQuery, error) {
	if len(r.URL.RawQuery) > 4096 {
		return application.WorkQueueQuery{}, domain.NewRuleError(domain.CodeValidation, "查询字符串不得超过 4096 个字符")
	}
	values := r.URL.Query()
	allowed := map[string]bool{"status": true, "venueName": true, "updatedFrom": true,
		"updatedTo": true, "highestSeverity": true, "minSeverity": true, "limit": true, "cursor": true}
	for key := range values {
		if !allowed[key] {
			return application.WorkQueueQuery{}, domain.NewRuleError(domain.CodeValidation, "不支持的查询参数: %s", key)
		}
	}
	severity := values.Get("highestSeverity")
	if legacy := values.Get("minSeverity"); severity == "" {
		severity = legacy
	} else if legacy != "" && legacy != severity {
		return application.WorkQueueQuery{}, domain.NewRuleError(domain.CodeValidation,
			"highestSeverity 与 minSeverity 不得冲突")
	}
	query := application.WorkQueueQuery{VenueName: values.Get("venueName"),
		HighestSeverity: domain.Severity(severity), Cursor: values.Get("cursor")}
	for _, raw := range values["status"] {
		for _, value := range strings.Split(raw, ",") {
			if strings.TrimSpace(value) != "" {
				query.Statuses = append(query.Statuses, domain.CaseStatus(strings.TrimSpace(value)))
			}
		}
	}
	if value := values.Get("limit"); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil {
			return query, domain.NewRuleError(domain.CodeValidation, "limit 必须是整数")
		}
		query.Limit = limit
	}
	for name, target := range map[string]**time.Time{"updatedFrom": &query.UpdatedFrom, "updatedTo": &query.UpdatedTo} {
		if value := values.Get(name); value != "" {
			parsed, err := time.Parse(time.RFC3339, value)
			if err != nil {
				return query, domain.NewRuleError(domain.CodeValidation, "%s 必须使用 RFC3339 时间", name)
			}
			*target = &parsed
		}
	}
	return query, nil
}
