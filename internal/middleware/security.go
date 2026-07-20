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
	"context"
	"net/http"
	"time"

	"github.com/sh0jitmy/pgstate-gateway/internal/config"
)

func SecurityHeaders(mgr *config.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cfg := mgr.Get()

			// Secure HTTP headers
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Content-Security-Policy", "default-src 'none'")

			if cfg.HTTPS.Enabled {
				// HSTS header: 2 years, include subdomains, preload
				w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
			}

			// CORS handling
			if cfg.Security.CORS.Enabled {
				origins := cfg.Security.CORS.AllowedOrigins
				origin := r.Header.Get("Origin")
				allowed := false

				for _, o := range origins {
					if o == "*" || o == origin {
						allowed = true
						if o == "*" {
							w.Header().Set("Access-Control-Allow-Origin", "*")
						} else {
							w.Header().Set("Access-Control-Allow-Origin", origin)
						}
						break
					}
				}

				if allowed {
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, LOCK, UNLOCK, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
					if r.Method == http.MethodOptions {
						w.WriteHeader(http.StatusNoContent)
						return
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func MaxBodySize(mgr *config.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cfg := mgr.Get()
			r.Body = http.MaxBytesReader(w, r.Body, cfg.Security.MaxBodySize)
			next.ServeHTTP(w, r)
		})
	}
}

func Timeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
