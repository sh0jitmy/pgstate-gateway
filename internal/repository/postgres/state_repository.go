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

package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sh0jitmy/pgstate-gateway/internal/metrics"
	"github.com/sh0jitmy/pgstate-gateway/internal/model"
	"github.com/sh0jitmy/pgstate-gateway/internal/repository"
)

type StateRepository struct {
	pool *pgxpool.Pool
}

func NewStateRepository(pool *pgxpool.Pool) repository.StateRepository {
	return &StateRepository{pool: pool}
}

func (r *StateRepository) Get(ctx context.Context, workspace string) (*model.State, error) {
	start := time.Now()
	var err error
	defer func() {
		status := "success"
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			status = "error"
		}
		metrics.DBQueryDuration.WithLabelValues("GetState", status).Observe(time.Since(start).Seconds())
	}()

	query := `SELECT workspace, state, serial, updated_at FROM states WHERE workspace = $1`

	var s model.State
	err = r.pool.QueryRow(ctx, query, workspace).Scan(&s.Workspace, &s.State, &s.Serial, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &s, nil
}

func (r *StateRepository) Update(ctx context.Context, state *model.State) error {
	start := time.Now()
	var err error
	defer func() {
		status := "success"
		if err != nil {
			status = "error"
		}
		metrics.DBQueryDuration.WithLabelValues("UpdateState", status).Observe(time.Since(start).Seconds())
	}()

	query := `
		INSERT INTO states (workspace, state, serial, updated_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
		ON CONFLICT (workspace)
		DO UPDATE SET state = EXCLUDED.state, serial = EXCLUDED.serial, updated_at = CURRENT_TIMESTAMP
	`

	_, err = r.pool.Exec(ctx, query, state.Workspace, state.State, state.Serial)
	return err
}

func (r *StateRepository) Delete(ctx context.Context, workspace string) error {
	start := time.Now()
	var err error
	defer func() {
		status := "success"
		if err != nil {
			status = "error"
		}
		metrics.DBQueryDuration.WithLabelValues("DeleteState", status).Observe(time.Since(start).Seconds())
	}()

	query := `DELETE FROM states WHERE workspace = $1`
	_, err = r.pool.Exec(ctx, query, workspace)
	return err
}
