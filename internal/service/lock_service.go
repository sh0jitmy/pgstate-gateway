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

package service

import (
	"context"

	"github.com/sh0jitmy/pgstate-gateway/internal/model"
	"github.com/sh0jitmy/pgstate-gateway/internal/repository"
)

type LockService interface {
	Get(ctx context.Context, workspace string) (*model.Lock, error)
	Acquire(ctx context.Context, lock *model.Lock) (bool, error)
	Release(ctx context.Context, workspace string, lockID string) (bool, error)
}

type lockService struct {
	repo repository.LockRepository
}

func NewLockService(repo repository.LockRepository) LockService {
	return &lockService{repo: repo}
}

func (s *lockService) Get(ctx context.Context, workspace string) (*model.Lock, error) {
	return s.repo.Get(ctx, workspace)
}

func (s *lockService) Acquire(ctx context.Context, lock *model.Lock) (bool, error) {
	return s.repo.Acquire(ctx, lock)
}

func (s *lockService) Release(ctx context.Context, workspace string, lockID string) (bool, error) {
	return s.repo.Release(ctx, workspace, lockID)
}
