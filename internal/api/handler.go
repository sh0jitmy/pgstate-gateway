// Copyright 2026 [Copyright Holder]
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Author: [YOUR_NAME]

package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sh0jitmy/pgstate-gateway/internal/config"
	"github.com/sh0jitmy/pgstate-gateway/internal/logger"
	"github.com/sh0jitmy/pgstate-gateway/internal/metrics"
	"github.com/sh0jitmy/pgstate-gateway/internal/middleware"
	"github.com/sh0jitmy/pgstate-gateway/internal/model"
	"github.com/sh0jitmy/pgstate-gateway/internal/service"
	"go.uber.org/zap"
)

type Handler struct {
	stateService service.StateService
	lockService  service.LockService
	pingFunc     func(context.Context) error
}

func init() {
	chi.RegisterMethod("LOCK")
	chi.RegisterMethod("UNLOCK")
}

func NewHandler(stateService service.StateService, lockService service.LockService, pingFunc func(context.Context) error) *Handler {
	return &Handler{
		stateService: stateService,
		lockService:  lockService,
		pingFunc:     pingFunc,
	}
}

func SetupRouter(mgr *config.Manager, h *Handler) http.Handler {
	r := chi.NewRouter()

	// Rate limiter middleware
	limiter := middleware.NewIPRateLimiter(mgr)

	// Apply global middlewares
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(middleware.MaxBodySize(mgr))
	r.Use(middleware.SecurityHeaders(mgr))
	r.Use(middleware.RequestLogger())
	r.Use(middleware.Metrics())
	r.Use(limiter.Handler())

	// Core API endpoints
	r.Route("/state/{workspace}", func(sr chi.Router) {
		sr.Use(middleware.Auth(mgr))
		sr.Get("/", h.GetState)
		sr.Post("/", h.PostState)
		sr.Delete("/", h.DeleteState)
		sr.MethodFunc("LOCK", "/", h.LockState)
		sr.MethodFunc("UNLOCK", "/", h.UnlockState)
	})

	// Public observability endpoints
	r.Get("/healthz", h.Healthz)
	r.Handle("/metrics", promhttp.Handler())

	// Security: bind pprof separately only on localhost (R-2.16)
	// Handled at main.go entry point on localhost:6060.

	return r
}

func (h *Handler) GetState(w http.ResponseWriter, r *http.Request) {
	workspace := chi.URLParam(r, "workspace")
	if workspace == "" {
		http.Error(w, "Workspace is required", http.StatusBadRequest)
		return
	}

	state, err := h.stateService.Get(r.Context(), workspace)
	if err != nil {
		logger.Log.Error("Failed to fetch state", zap.String("workspace", workspace), zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if state == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	// #nosec G705
	_, _ = w.Write(state.State)
}

func (h *Handler) PostState(w http.ResponseWriter, r *http.Request) {
	workspace := chi.URLParam(r, "workspace")
	if workspace == "" {
		http.Error(w, "Workspace is required", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Log.Error("Failed to read state body", zap.String("workspace", workspace), zap.Error(err))
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	err = h.stateService.Update(r.Context(), workspace, body)
	if err != nil {
		logger.Log.Error("Failed to save state", zap.String("workspace", workspace), zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) DeleteState(w http.ResponseWriter, r *http.Request) {
	workspace := chi.URLParam(r, "workspace")
	if workspace == "" {
		http.Error(w, "Workspace is required", http.StatusBadRequest)
		return
	}

	err := h.stateService.Delete(r.Context(), workspace)
	if err != nil {
		logger.Log.Error("Failed to delete state", zap.String("workspace", workspace), zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) LockState(w http.ResponseWriter, r *http.Request) {
	workspace := chi.URLParam(r, "workspace")
	if workspace == "" {
		http.Error(w, "Workspace is required", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	var lockReq model.Lock
	if unmarshalErr := json.Unmarshal(body, &lockReq); unmarshalErr != nil {
		http.Error(w, "Invalid Lock JSON", http.StatusBadRequest)
		return
	}

	lockReq.Workspace = workspace
	lockReq.Created = time.Now()

	acquired, err := h.lockService.Acquire(r.Context(), &lockReq)
	if err != nil {
		logger.Log.Error("Failed to acquire lock", zap.String("workspace", workspace), zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if !acquired {
		metrics.LockConflicts.Inc()
		currentLock, err := h.lockService.Get(r.Context(), workspace)
		if err != nil {
			logger.Log.Error("Failed to retrieve current lock after collision", zap.String("workspace", workspace), zap.Error(err))
			http.Error(w, "Locked", http.StatusLocked)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusLocked)
		if currentLock != nil {
			_ = json.NewEncoder(w).Encode(currentLock)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) UnlockState(w http.ResponseWriter, r *http.Request) {
	workspace := chi.URLParam(r, "workspace")
	if workspace == "" {
		http.Error(w, "Workspace is required", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	var lockReq model.Lock
	if unmarshalErr := json.Unmarshal(body, &lockReq); unmarshalErr != nil {
		http.Error(w, "Invalid Lock JSON", http.StatusBadRequest)
		return
	}

	released, err := h.lockService.Release(r.Context(), workspace, lockReq.LockID)
	if err != nil {
		logger.Log.Error("Failed to release lock", zap.String("workspace", workspace), zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if !released {
		// Mismatch lock ID or lock does not exist
		currentLock, err := h.lockService.Get(r.Context(), workspace)
		if err != nil {
			http.Error(w, "Conflict", http.StatusConflict)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		if currentLock != nil {
			_ = json.NewEncoder(w).Encode(currentLock)
		} else {
			_, _ = w.Write([]byte(`{"message": "Lock not found"}`))
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.pingFunc(ctx); err != nil {
		logger.Log.Error("Healthcheck failed (DB Ping error)", zap.Error(err))
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status": "error", "message": "database ping failed"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status": "ok"}`))
}
