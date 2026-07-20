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

package middleware

import (
	"net/http"
	"strings"

	"github.com/sh0jitmy/pgstate-gateway/internal/config"
)

func Auth(mgr *config.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cfg := mgr.Get()

			hasBasicConfig := cfg.Auth.Basic.Username != "" && cfg.Auth.Basic.Password != ""
			hasBearerConfig := len(cfg.Auth.BearerTokens) > 0

			// If no authentication is configured, allow request
			if !hasBasicConfig && !hasBearerConfig {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				w.Header().Set("WWW-Authenticate", `Basic realm="Terraform State Gateway"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Try Basic Auth first if configured
			if hasBasicConfig && strings.HasPrefix(strings.ToLower(authHeader), "basic ") {
				username, password, ok := r.BasicAuth()
				if ok && username == cfg.Auth.Basic.Username && password == cfg.Auth.Basic.Password {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Try Bearer Token next if configured
			if hasBearerConfig && strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				token := strings.TrimSpace(authHeader[7:])
				for _, t := range cfg.Auth.BearerTokens {
					if token == t {
						next.ServeHTTP(w, r)
						return
					}
				}
			}

			w.Header().Set("WWW-Authenticate", `Basic realm="Terraform State Gateway"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		})
	}
}
