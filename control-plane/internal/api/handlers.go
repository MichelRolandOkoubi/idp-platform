package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"

	"github.com/MichelRolandOkoubi/idp-platform/control-plane/internal/orchestrator"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func (h *Handler) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		h.Logger.Error("failed to encode response", zap.Error(err))
	}
}

func (h *Handler) writeError(w http.ResponseWriter, status int, msg string) {
	h.writeJSON(w, status, map[string]string{"error": msg})
}

func (h *Handler) decode(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// ── Health ────────────────────────────────────────────────────────────────────

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]any{
		"status":  "healthy",
		"version": "0.1.0",
	})
}

// ── Deploy ────────────────────────────────────────────────────────────────────

type DeployRequest struct {
	Name      string            `json:"name" validate:"required,alphanum"`
	Image     string            `json:"image" validate:"required"`
	Replicas  int32             `json:"replicas" validate:"min=1,max=50"`
	Namespace string            `json:"namespace" validate:"required"`
	DryRun    bool              `json:"dry_run"`
	Resources ResourceSpec      `json:"resources"`
	EnvVars   map[string]string `json:"env_vars"`
	Port      *int32            `json:"port,omitempty"`
}

type ResourceSpec struct {
	CPURequest    string `json:"cpu_request"`
	CPULimit      string `json:"cpu_limit"`
	MemoryRequest string `json:"memory_request"`
	MemoryLimit   string `json:"memory_limit"`
}

func (h *Handler) Deploy(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("idp").Start(r.Context(), "Deploy")
	defer span.End()

	var req DeployRequest
	if err := h.decode(r, &req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	h.Logger.Info("deploying application",
		zap.String("name", req.Name),
		zap.String("namespace", req.Namespace),
		zap.String("image", req.Image),
		zap.Bool("dry_run", req.DryRun),
	)

	result, err := h.Orchestrator.Deploy(ctx, &orchestrator.DeploySpec{
		Name:      req.Name,
		Image:     req.Image,
		Replicas:  req.Replicas,
		Namespace: req.Namespace,
		DryRun:    req.DryRun,
		Resources: orchestrator.ResourceSpec{
			CPURequest:    req.Resources.CPURequest,
			CPULimit:      req.Resources.CPULimit,
			MemoryRequest: req.Resources.MemoryRequest,
			MemoryLimit:   req.Resources.MemoryLimit,
		},
		EnvVars: req.EnvVars,
		Port:    req.Port,
	})
	if err != nil {
		span.RecordError(err)
		h.Logger.Error("deploy failed", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, result)
}

// ── Applications ──────────────────────────────────────────────────────────────

func (h *Handler) ListApplications(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = "default"
	}

	apps, err := h.Orchestrator.ListApplications(r.Context(), namespace)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, apps)
}

func (h *Handler) GetApplication(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = "default"
	}

	app, err := h.Orchestrator.GetApplication(r.Context(), namespace, name)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, app)
}

func (h *Handler) DeleteApplication(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = "default"
	}

	if err := h.Orchestrator.DeleteApplication(r.Context(), namespace, name); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Scale(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	namespace := r.URL.Query().Get("namespace")

	var body struct {
		Replicas int32 `json:"replicas"`
	}
	if err := h.decode(r, &body); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid body")
		return
	}

	app, err := h.Orchestrator.Scale(r.Context(), namespace, name, body.Replicas)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, app)
}

func (h *Handler) GetLogs(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	namespace := r.URL.Query().Get("namespace")
	tailStr := r.URL.Query().Get("tail")
	follow := r.URL.Query().Get("follow") == "true"

	tail := int64(100)
	if tailStr != "" {
		if t, err := strconv.ParseInt(tailStr, 10, 64); err == nil {
			tail = t
		}
	}

	if err := h.Orchestrator.StreamLogs(r.Context(), w, namespace, name, tail, follow); err != nil {
		h.Logger.Error("failed to stream logs", zap.Error(err))
	}
}

// ── Env Vars ──────────────────────────────────────────────────────────────────

// Use orchestrator.EnvVar

func (h *Handler) ListEnv(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	namespace := r.URL.Query().Get("namespace")

	vars, err := h.Orchestrator.ListEnvVars(r.Context(), namespace, name)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, vars)
}

func (h *Handler) SetEnv(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	namespace := r.URL.Query().Get("namespace")

	var vars []orchestrator.EnvVar
	if err := h.decode(r, &vars); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid body")
		return
	}

	if err := h.Orchestrator.SetEnvVars(r.Context(), namespace, name, vars); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) DeleteEnv(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	namespace := r.URL.Query().Get("namespace")
	key := chi.URLParam(r, "key")

	if err := h.Orchestrator.DeleteEnvVar(r.Context(), namespace, name, key); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Cost ──────────────────────────────────────────────────────────────────────

func (h *Handler) EstimateCost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Namespace   string `json:"namespace"`
		Replicas    int32  `json:"replicas"`
		CPULimit    string `json:"cpu_limit"`
		MemoryLimit string `json:"memory_limit"`
		Region      string `json:"region"`
	}
	if err := h.decode(r, &req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid body")
		return
	}

	result, err := h.Orchestrator.EstimateCost(r.Context(), orchestrator.CostRequest{
		Name:        req.Name,
		Namespace:   req.Namespace,
		Replicas:    req.Replicas,
		CPULimit:    req.CPULimit,
		MemoryLimit: req.MemoryLimit,
		Region:      req.Region,
	})
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, result)
}

func (h *Handler) GetCostHistory(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil {
			days = n
		}
	}

	history, err := h.DB.GetCostHistory(r.Context(), namespace, days)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, history)
}

func (h *Handler) GetAnomalies(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	anomalies, err := h.DB.GetAnomalies(r.Context(), namespace)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, anomalies)
}

// ── Platform Status ───────────────────────────────────────────────────────────

func (h *Handler) PlatformStatus(w http.ResponseWriter, r *http.Request) {
	status := h.Orchestrator.PlatformStatus(r.Context())
	h.writeJSON(w, http.StatusOK, status)
}
