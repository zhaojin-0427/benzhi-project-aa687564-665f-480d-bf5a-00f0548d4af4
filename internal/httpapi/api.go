package httpapi

import (
	"net/http"
	"strings"

	"stage-rigging-clearance/internal/application"
)

type API struct {
	service *application.Service
}

func New(service *application.Service) *API { return &API{service: service} }

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", a.HandleHealth)
	mux.HandleFunc("/api/v1/inspection-cases", a.HandleCaseCollection)
	mux.HandleFunc("/api/v1/inspection-cases/", a.HandleCaseResource)
	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") && r.URL.Path != "/healthz" {
			writeProblem(w, http.StatusNotFound, "not_found", "资源不存在")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, []byte(`{"status":"ok"}`))
}
