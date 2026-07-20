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
	"strconv"
	"time"

	"github.com/sh0jitmy/pgstate-gateway/internal/metrics"
)

func Metrics() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			sw := &statusWriter{ResponseWriter: w}

			next.ServeHTTP(sw, r)

			duration := time.Since(start).Seconds()

			path := r.URL.Path
			if len(path) > 7 && path[:7] == "/state/" {
				path = "/state/{workspace}"
			}

			statusStr := strconv.Itoa(sw.status)
			if sw.status == 0 {
				statusStr = "200"
			}

			metrics.RequestsTotal.WithLabelValues(r.Method, path, statusStr).Inc()
			metrics.RequestDuration.WithLabelValues(r.Method, path, statusStr).Observe(duration)
		})
	}
}
