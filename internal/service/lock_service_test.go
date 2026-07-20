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

package service_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/sh0jitmy/pgstate-gateway/internal/model"
	"github.com/sh0jitmy/pgstate-gateway/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLockRepo struct {
	mu    sync.RWMutex
	locks map[string]*model.Lock
}

func (m *mockLockRepo) Get(ctx context.Context, ws string) (*model.Lock, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if l, ok := m.locks[ws]; ok {
		return l, nil
	}
	return nil, nil
}

func (m *mockLockRepo) Acquire(ctx context.Context, lock *model.Lock) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.locks[lock.Workspace]; exists {
		return false, nil
	}
	m.locks[lock.Workspace] = lock
	return true, nil
}

func (m *mockLockRepo) Release(ctx context.Context, ws string, lockID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, exists := m.locks[ws]
	if !exists || l.LockID != lockID {
		return false, nil
	}
	delete(m.locks, ws)
	return true, nil
}

func TestLockService(t *testing.T) {
	t.Parallel()

	repo := &mockLockRepo{locks: make(map[string]*model.Lock)}
	svc := service.NewLockService(repo)

	t.Run("Acquire and Get Lock", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		lock := &model.Lock{
			Workspace: "dev",
			LockID:    "lock-123",
			Operation: "write",
			Who:       "user@host",
			Version:   "1.9.0",
			Created:   time.Now(),
		}

		acquired, err := svc.Acquire(ctx, lock)
		require.NoError(t, err)
		assert.True(t, acquired)

		got, err := svc.Get(ctx, "dev")
		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Equal(t, "lock-123", got.LockID)
	})

	t.Run("Acquire Colliding Lock", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		repo.mu.Lock()
		repo.locks["prod"] = &model.Lock{
			Workspace: "prod",
			LockID:    "lock-original",
		}
		repo.mu.Unlock()

		lock := &model.Lock{
			Workspace: "prod",
			LockID:    "lock-new",
		}

		acquired, err := svc.Acquire(ctx, lock)
		require.NoError(t, err)
		assert.False(t, acquired)
	})

	t.Run("Release Lock Success", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		repo.mu.Lock()
		repo.locks["test"] = &model.Lock{
			Workspace: "test",
			LockID:    "lock-to-release",
		}
		repo.mu.Unlock()

		released, err := svc.Release(ctx, "test", "lock-to-release")
		require.NoError(t, err)
		assert.True(t, released)

		got, err := svc.Get(ctx, "test")
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("Release Lock Mismatch ID", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		repo.mu.Lock()
		repo.locks["test-mismatch"] = &model.Lock{
			Workspace: "test-mismatch",
			LockID:    "lock-original",
		}
		repo.mu.Unlock()

		released, err := svc.Release(ctx, "test-mismatch", "lock-wrong")
		require.NoError(t, err)
		assert.False(t, released)

		got, err := svc.Get(ctx, "test-mismatch")
		require.NoError(t, err)
		assert.NotNil(t, got)
	})
}
