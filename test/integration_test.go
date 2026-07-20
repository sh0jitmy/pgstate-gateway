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

//go:build integration

package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/sh0jitmy/pgstate-gateway/internal/api"
	"github.com/sh0jitmy/pgstate-gateway/internal/config"
	"github.com/sh0jitmy/pgstate-gateway/internal/logger"
	"github.com/sh0jitmy/pgstate-gateway/internal/model"
	"github.com/sh0jitmy/pgstate-gateway/internal/repository/postgres"
	"github.com/sh0jitmy/pgstate-gateway/internal/service"
	"github.com/sh0jitmy/pgstate-gateway/migrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	pgmodule "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupPostgresContainer(ctx context.Context, t *testing.T) (*pgmodule.PostgresContainer, string) {
	pgContainer, err := pgmodule.Run(ctx,
		"postgres:15-alpine",
		pgmodule.WithDatabase("tfstate"),
		pgmodule.WithUsername("terraform"),
		pgmodule.WithPassword("password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	return pgContainer, connStr
}

func applyMigrations(t *testing.T, dsn string) {
	d, err := iofs.New(migrations.FS, ".")
	require.NoError(t, err)

	m, err := migrate.NewWithSourceInstance("iofs", d, dsn)
	require.NoError(t, err)
	defer m.Close()

	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		require.NoError(t, err)
	}
}

func TestIntegration(t *testing.T) {
	ctx := context.Background()

	// Start Postgres Testcontainer
	_, connStr := func() (*pgmodule.PostgresContainer, string) {
		t.Log("Starting Postgres Testcontainer...")
		return setupPostgresContainer(ctx, t)
	}()

	applyMigrations(t, connStr)

	// Create configuration manager
	cfg := &config.Config{
		Server: config.ServerConfig{
			Listen:     ":8080",
			ListenHTTP: ":80",
		},
		HTTPS: config.HTTPSConfig{
			Enabled: false,
		},
		Auth: config.AuthConfig{
			Basic: config.BasicAuthConfig{
				Username: "terraform",
				Password: "secret-password",
			},
			BearerTokens: []string{"token1"},
		},
		Logging: config.LoggingConfig{
			Level: "info",
		},
		Security: config.SecurityConfig{
			RateLimit:   1000, // Make high to avoid rate limiting other tests
			MaxBodySize: 52428800,
		},
	}
	require.NoError(t, logger.Init("info"))
	mgr := config.NewManager(cfg)

	// Parse database configs from connection string
	// connStr is of format: postgres://terraform:password@host:port/tfstate?sslmode=disable
	dbCfg := &config.DatabaseConfig{
		Host:            "localhost",
		Port:            5432,
		User:            "terraform",
		Password:        "password",
		Database:        "tfstate",
		SSLMode:         "disable",
		MaxOpenConns:    10,
		MaxIdleConns:    10,
		ConnMaxLifetime: 10 * time.Minute,
	}

	// We overwrite host and port based on container actual host/port mapping
	u, err := url.Parse(connStr)
	require.NoError(t, err)

	host, portStr, err := net.SplitHostPort(u.Host)
	require.NoError(t, err)

	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	dbCfg.Host = host
	dbCfg.Port = port

	// Connect to database
	db, err := postgres.NewDB(ctx, dbCfg)
	require.NoError(t, err)
	defer db.Close()

	stateRepo := postgres.NewStateRepository(db.Pool)
	lockRepo := postgres.NewLockRepository(db.Pool)

	stateSvc := service.NewStateService(stateRepo)
	lockSvc := service.NewLockService(lockRepo)

	pingFunc := func(c context.Context) error {
		return db.Pool.Ping(c)
	}

	handler := api.NewHandler(stateSvc, lockSvc, pingFunc)
	router := api.SetupRouter(mgr, handler)

	ts := httptest.NewServer(router)
	defer ts.Close()

	client := ts.Client()
	if tr, ok := client.Transport.(*http.Transport); ok {
		tr.DisableKeepAlives = true
	}

	t.Run("Get Non-existent State returns 404", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, ts.URL+"/state/nonexistent", nil)
		require.NoError(t, err)
		req.SetBasicAuth("terraform", "secret-password")

		resp, err := client.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("POST and GET State succeeds", func(t *testing.T) {
		stateJSON := `{"version":4,"serial":2,"lineage":"xyz"}`
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/state/workspace1", bytes.NewBufferString(stateJSON))
		require.NoError(t, err)
		req.SetBasicAuth("terraform", "secret-password")
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Fetch back
		reqGet, err := http.NewRequest(http.MethodGet, ts.URL+"/state/workspace1", nil)
		require.NoError(t, err)
		reqGet.SetBasicAuth("terraform", "secret-password")

		respGet, err := client.Do(reqGet)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, respGet.StatusCode)

		var buf bytes.Buffer
		_, err = buf.ReadFrom(respGet.Body)
		require.NoError(t, err)
		assert.JSONEq(t, stateJSON, buf.String())
	})

	t.Run("Authentication Verification", func(t *testing.T) {
		// Wrong basic auth password
		req, err := http.NewRequest(http.MethodGet, ts.URL+"/state/workspace1", nil)
		require.NoError(t, err)
		req.SetBasicAuth("terraform", "wrong-password")
		resp, err := client.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		// Correct bearer auth
		reqBearer, err := http.NewRequest(http.MethodGet, ts.URL+"/state/workspace1", nil)
		require.NoError(t, err)
		reqBearer.Header.Set("Authorization", "Bearer token1")
		respBearer, err := client.Do(reqBearer)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, respBearer.StatusCode)
	})

	t.Run("Locking and Mismatch UNLOCK Verification", func(t *testing.T) {
		lockData := &model.Lock{
			LockID:    "lock-a",
			Operation: "write",
			Who:       "tester",
			Version:   "1.9",
		}
		body, err := json.Marshal(lockData)
		require.NoError(t, err)

		// LOCK request
		reqLock, err := http.NewRequest("LOCK", ts.URL+"/state/workspace1", bytes.NewReader(body))
		require.NoError(t, err)
		reqLock.Header.Set("Authorization", "Bearer token1")

		respLock, err := client.Do(reqLock)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, respLock.StatusCode)

		// Concurrent LOCK request on same workspace should fail
		lockData2 := &model.Lock{
			LockID:    "lock-b",
			Operation: "write",
			Who:       "tester2",
			Version:   "1.9",
		}
		body2, err := json.Marshal(lockData2)
		require.NoError(t, err)

		reqLock2, err := http.NewRequest("LOCK", ts.URL+"/state/workspace1", bytes.NewReader(body2))
		require.NoError(t, err)
		reqLock2.Header.Set("Authorization", "Bearer token1")

		respLock2, err := client.Do(reqLock2)
		require.NoError(t, err)
		assert.Equal(t, http.StatusLocked, respLock2.StatusCode)

		// Response should contain the current lock info (lock-a)
		var collLock model.Lock
		err = json.NewDecoder(respLock2.Body).Decode(&collLock)
		require.NoError(t, err)
		assert.Equal(t, "lock-a", collLock.LockID)

		// UNLOCK with mismatched ID should fail
		reqUnlockWrong, err := http.NewRequest("UNLOCK", ts.URL+"/state/workspace1", bytes.NewReader(body2))
		require.NoError(t, err)
		reqUnlockWrong.Header.Set("Authorization", "Bearer token1")

		respUnlockWrong, err := client.Do(reqUnlockWrong)
		require.NoError(t, err)
		assert.Equal(t, http.StatusConflict, respUnlockWrong.StatusCode)

		// UNLOCK with correct ID should succeed
		reqUnlock, err := http.NewRequest("UNLOCK", ts.URL+"/state/workspace1", bytes.NewReader(body))
		require.NoError(t, err)
		reqUnlock.Header.Set("Authorization", "Bearer token1")

		respUnlock, err := client.Do(reqUnlock)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, respUnlock.StatusCode)
	})

	t.Run("Concurrent Lock Acquisition Race Test", func(t *testing.T) {
		const numGoroutines = 20
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		successCount := 0
		var countMu sync.Mutex

		// We do not need to pre-marshal the outer lockData since each goroutine marshals its own unique ID.

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				// Duplicate request payload with custom ID to make it unique per client if needed
				lData := &model.Lock{
					LockID:    fmt.Sprintf("lock-%d", id),
					Operation: "write",
					Who:       "tester-race",
					Version:   "1.9",
				}
				b, _ := json.Marshal(lData)

				req, _ := http.NewRequest("LOCK", ts.URL+"/state/workspace-race", bytes.NewReader(b))
				req.Header.Set("Authorization", "Bearer token1")

				resp, err := client.Do(req)
				if err == nil {
					if resp.StatusCode == http.StatusOK {
						countMu.Lock()
						successCount++
						countMu.Unlock()
					}
					_ = resp.Body.Close()
				}
			}(i)
		}

		wg.Wait()

		// Exactly one client should have successfully acquired the lock
		assert.Equal(t, 1, successCount)

		// Clean up the race lock using Exec to avoid leaking a connection
		_, _ = db.Pool.Exec(ctx, "DELETE FROM locks WHERE workspace = 'workspace-race'")
	})

	t.Run("DELETE State succeeds", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete, ts.URL+"/state/workspace1", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer token1")

		resp, err := client.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Fetch back should return 404
		reqGet, err := http.NewRequest(http.MethodGet, ts.URL+"/state/workspace1", nil)
		require.NoError(t, err)
		reqGet.Header.Set("Authorization", "Bearer token1")

		respGet, err := client.Do(reqGet)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, respGet.StatusCode)
	})
}
