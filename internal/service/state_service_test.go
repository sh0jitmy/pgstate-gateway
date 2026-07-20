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

	"github.com/sh0jitmy/pgstate-gateway/internal/model"
	"github.com/sh0jitmy/pgstate-gateway/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStateRepo struct {
	mu     sync.RWMutex
	states map[string]*model.State
}

func (m *mockStateRepo) Get(ctx context.Context, ws string) (*model.State, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if s, ok := m.states[ws]; ok {
		return s, nil
	}
	return nil, nil
}

func (m *mockStateRepo) Update(ctx context.Context, s *model.State) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states[s.Workspace] = s
	return nil
}

func (m *mockStateRepo) Delete(ctx context.Context, ws string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.states, ws)
	return nil
}

func TestStateService(t *testing.T) {
	t.Parallel()

	repo := &mockStateRepo{states: make(map[string]*model.State)}
	svc := service.NewStateService(repo)

	t.Run("Update and Get State", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		body := []byte(`{"serial": 5, "version": 4}`)

		err := svc.Update(ctx, "dev", body)
		require.NoError(t, err)

		state, err := svc.Get(ctx, "dev")
		require.NoError(t, err)
		assert.NotNil(t, state)
		assert.Equal(t, int64(5), state.Serial)
		assert.Equal(t, body, state.State)
	})

	t.Run("Get Non-existent State", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		state, err := svc.Get(ctx, "prod")
		require.NoError(t, err)
		assert.Nil(t, state)
	})

	t.Run("Delete State", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		repo.mu.Lock()
		repo.states["prod"] = &model.State{Workspace: "prod", State: []byte("{}"), Serial: 1}
		repo.mu.Unlock()

		err := svc.Delete(ctx, "prod")
		require.NoError(t, err)

		state, err := svc.Get(ctx, "prod")
		require.NoError(t, err)
		assert.Nil(t, state)
	})
}
