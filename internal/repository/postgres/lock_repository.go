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

type LockRepository struct {
	pool *pgxpool.Pool
}

func NewLockRepository(pool *pgxpool.Pool) repository.LockRepository {
	return &LockRepository{pool: pool}
}

func (r *LockRepository) Get(ctx context.Context, workspace string) (*model.Lock, error) {
	start := time.Now()
	var err error
	defer func() {
		status := "success"
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			status = "error"
		}
		metrics.DBQueryDuration.WithLabelValues("GetLock", status).Observe(time.Since(start).Seconds())
	}()

	query := `
		SELECT workspace, lock_id, operation, who, info, version, created
		FROM locks
		WHERE workspace = $1
	`

	var l model.Lock
	err = r.pool.QueryRow(ctx, query, workspace).Scan(
		&l.Workspace, &l.LockID, &l.Operation, &l.Who, &l.Info, &l.Version, &l.Created,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &l, nil
}

func (r *LockRepository) Acquire(ctx context.Context, lock *model.Lock) (bool, error) {
	start := time.Now()
	var err error
	defer func() {
		status := "success"
		if err != nil {
			status = "error"
		}
		metrics.DBQueryDuration.WithLabelValues("AcquireLock", status).Observe(time.Since(start).Seconds())
	}()

	query := `
		INSERT INTO locks (workspace, lock_id, operation, who, info, version, created)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (workspace) DO NOTHING
	`

	res, err := r.pool.Exec(ctx, query,
		lock.Workspace, lock.LockID, lock.Operation, lock.Who, lock.Info, lock.Version, lock.Created,
	)
	if err != nil {
		return false, err
	}

	return res.RowsAffected() == 1, nil
}

func (r *LockRepository) Release(ctx context.Context, workspace string, lockID string) (bool, error) {
	start := time.Now()
	var err error
	defer func() {
		status := "success"
		if err != nil {
			status = "error"
		}
		metrics.DBQueryDuration.WithLabelValues("ReleaseLock", status).Observe(time.Since(start).Seconds())
	}()

	query := `
		DELETE FROM locks
		WHERE workspace = $1 AND lock_id = $2
	`

	res, err := r.pool.Exec(ctx, query, workspace, lockID)
	if err != nil {
		return false, err
	}

	return res.RowsAffected() == 1, nil
}
