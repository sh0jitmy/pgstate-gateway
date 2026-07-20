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

package api_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sh0jitmy/pgstate-gateway/internal/api"
	"github.com/sh0jitmy/pgstate-gateway/internal/config"
	"github.com/sh0jitmy/pgstate-gateway/internal/logger"
	"github.com/sh0jitmy/pgstate-gateway/internal/model"
	"github.com/stretchr/testify/assert"
)

// Mocks for testing handlers
type mockStateService struct {
	getFunc    func(ctx context.Context, ws string) (*model.State, error)
	updateFunc func(ctx context.Context, ws string, body []byte) error
	deleteFunc func(ctx context.Context, ws string) error
}

func (m *mockStateService) Get(ctx context.Context, ws string) (*model.State, error) {
	return m.getFunc(ctx, ws)
}
func (m *mockStateService) Update(ctx context.Context, ws string, body []byte) error {
	return m.updateFunc(ctx, ws, body)
}
func (m *mockStateService) Delete(ctx context.Context, ws string) error {
	return m.deleteFunc(ctx, ws)
}

type mockLockService struct {
	getFunc     func(ctx context.Context, ws string) (*model.Lock, error)
	acquireFunc func(ctx context.Context, lock *model.Lock) (bool, error)
	releaseFunc func(ctx context.Context, ws string, lockID string) (bool, error)
}

func (m *mockLockService) Get(ctx context.Context, ws string) (*model.Lock, error) {
	return m.getFunc(ctx, ws)
}
func (m *mockLockService) Acquire(ctx context.Context, lock *model.Lock) (bool, error) {
	return m.acquireFunc(ctx, lock)
}
func (m *mockLockService) Release(ctx context.Context, ws string, lockID string) (bool, error) {
	return m.releaseFunc(ctx, ws, lockID)
}

func init() {
	_ = logger.Init("info")
}

func setupTestRouter(h *api.Handler) http.Handler {
	cfg := &config.Config{
		Security: config.SecurityConfig{
			RateLimit:   10000,
			MaxBodySize: 52428800,
		},
	}
	mgr := config.NewManager(cfg)
	return api.SetupRouter(mgr, h)
}

func TestGetState(t *testing.T) {
	t.Parallel()

	t.Run("Success 200", func(t *testing.T) {
		t.Parallel()
		stateSvc := &mockStateService{
			getFunc: func(ctx context.Context, ws string) (*model.State, error) {
				return &model.State{Workspace: ws, State: []byte(`{"version":4}`)}, nil
			},
		}
		handler := api.NewHandler(stateSvc, nil, nil)
		router := setupTestRouter(handler)

		req := httptest.NewRequest(http.MethodGet, "/state/dev", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.JSONEq(t, `{"version":4}`, w.Body.String())
	})

	t.Run("NotFound 404", func(t *testing.T) {
		t.Parallel()
		stateSvc := &mockStateService{
			getFunc: func(ctx context.Context, ws string) (*model.State, error) {
				return nil, nil
			},
		}
		handler := api.NewHandler(stateSvc, nil, nil)
		router := setupTestRouter(handler)

		req := httptest.NewRequest(http.MethodGet, "/state/dev", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("ServerError 500", func(t *testing.T) {
		t.Parallel()
		stateSvc := &mockStateService{
			getFunc: func(ctx context.Context, ws string) (*model.State, error) {
				return nil, errors.New("db crash")
			},
		}
		handler := api.NewHandler(stateSvc, nil, nil)
		router := setupTestRouter(handler)

		req := httptest.NewRequest(http.MethodGet, "/state/dev", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestPostState(t *testing.T) {
	t.Parallel()

	t.Run("Success 200", func(t *testing.T) {
		t.Parallel()
		stateSvc := &mockStateService{
			updateFunc: func(ctx context.Context, ws string, body []byte) error {
				assert.Equal(t, "dev", ws)
				assert.JSONEq(t, `{"serial":1}`, string(body))
				return nil
			},
		}
		handler := api.NewHandler(stateSvc, nil, nil)
		router := setupTestRouter(handler)

		req := httptest.NewRequest(http.MethodPost, "/state/dev", bytes.NewBufferString(`{"serial":1}`))
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestDeleteState(t *testing.T) {
	t.Parallel()

	t.Run("Success 200", func(t *testing.T) {
		t.Parallel()
		stateSvc := &mockStateService{
			deleteFunc: func(ctx context.Context, ws string) error {
				assert.Equal(t, "dev", ws)
				return nil
			},
		}
		handler := api.NewHandler(stateSvc, nil, nil)
		router := setupTestRouter(handler)

		req := httptest.NewRequest(http.MethodDelete, "/state/dev", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestLockState(t *testing.T) {
	t.Parallel()

	t.Run("Acquire Success 200", func(t *testing.T) {
		t.Parallel()
		lockSvc := &mockLockService{
			acquireFunc: func(ctx context.Context, lock *model.Lock) (bool, error) {
				assert.Equal(t, "lock-1", lock.LockID)
				return true, nil
			},
		}
		handler := api.NewHandler(nil, lockSvc, nil)
		router := setupTestRouter(handler)

		body := `{"ID":"lock-1","Operation":"write","Who":"tester"}`
		req := httptest.NewRequest("LOCK", "/state/dev", bytes.NewBufferString(body))
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Acquire Collision 423", func(t *testing.T) {
		t.Parallel()
		lockSvc := &mockLockService{
			acquireFunc: func(ctx context.Context, lock *model.Lock) (bool, error) {
				return false, nil
			},
			getFunc: func(ctx context.Context, ws string) (*model.Lock, error) {
				return &model.Lock{LockID: "existing-lock", Who: "owner"}, nil
			},
		}
		handler := api.NewHandler(nil, lockSvc, nil)
		router := setupTestRouter(handler)

		body := `{"ID":"lock-new"}`
		req := httptest.NewRequest("LOCK", "/state/dev", bytes.NewBufferString(body))
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusLocked, w.Code)
		assert.Contains(t, w.Body.String(), "existing-lock")
	})
}

func TestUnlockState(t *testing.T) {
	t.Parallel()

	t.Run("Release Success 200", func(t *testing.T) {
		t.Parallel()
		lockSvc := &mockLockService{
			releaseFunc: func(ctx context.Context, ws string, lockID string) (bool, error) {
				assert.Equal(t, "lock-to-release", lockID)
				return true, nil
			},
		}
		handler := api.NewHandler(nil, lockSvc, nil)
		router := setupTestRouter(handler)

		body := `{"ID":"lock-to-release"}`
		req := httptest.NewRequest("UNLOCK", "/state/dev", bytes.NewBufferString(body))
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Release Conflict 409", func(t *testing.T) {
		t.Parallel()
		lockSvc := &mockLockService{
			releaseFunc: func(ctx context.Context, ws string, lockID string) (bool, error) {
				return false, nil
			},
			getFunc: func(ctx context.Context, ws string) (*model.Lock, error) {
				return &model.Lock{LockID: "other-lock", Who: "other-user"}, nil
			},
		}
		handler := api.NewHandler(nil, lockSvc, nil)
		router := setupTestRouter(handler)

		body := `{"ID":"lock-wrong"}`
		req := httptest.NewRequest("UNLOCK", "/state/dev", bytes.NewBufferString(body))
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), "other-lock")
	})
}

func TestHealthz(t *testing.T) {
	t.Parallel()

	t.Run("Success 200", func(t *testing.T) {
		t.Parallel()
		pingFunc := func(ctx context.Context) error {
			return nil
		}
		handler := api.NewHandler(nil, nil, pingFunc)
		router := setupTestRouter(handler)

		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.JSONEq(t, `{"status":"ok"}`, w.Body.String())
	})

	t.Run("DB Error 503", func(t *testing.T) {
		t.Parallel()
		pingFunc := func(ctx context.Context) error {
			return errors.New("db disconnect")
		}
		handler := api.NewHandler(nil, nil, pingFunc)
		router := setupTestRouter(handler)

		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})
}
