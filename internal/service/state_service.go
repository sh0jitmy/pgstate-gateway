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
	"encoding/json"

	"github.com/sh0jitmy/pgstate-gateway/internal/model"
	"github.com/sh0jitmy/pgstate-gateway/internal/repository"
)

type StateService interface {
	Get(ctx context.Context, workspace string) (*model.State, error)
	Update(ctx context.Context, workspace string, body []byte) error
	Delete(ctx context.Context, workspace string) error
}

type stateService struct {
	repo repository.StateRepository
}

func NewStateService(repo repository.StateRepository) StateService {
	return &stateService{repo: repo}
}

func (s *stateService) Get(ctx context.Context, workspace string) (*model.State, error) {
	return s.repo.Get(ctx, workspace)
}

type stateHeader struct {
	Serial int64 `json:"serial"`
}

func (s *stateService) Update(ctx context.Context, workspace string, body []byte) error {
	var header stateHeader
	// If unmarshalling fails, default serial to 0
	_ = json.Unmarshal(body, &header)

	state := &model.State{
		Workspace: workspace,
		State:     body,
		Serial:    header.Serial,
	}

	return s.repo.Update(ctx, state)
}

func (s *stateService) Delete(ctx context.Context, workspace string) error {
	return s.repo.Delete(ctx, workspace)
}
