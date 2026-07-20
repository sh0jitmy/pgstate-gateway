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
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/sh0jitmy/pgstate-gateway/internal/logger"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func generateRequestID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "unknown-request-id"
	}
	return hex.EncodeToString(b)
}

func RequestLogger() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()

			reqID := r.Header.Get("X-Request-ID")
			if reqID == "" {
				reqID = generateRequestID()
			}
			w.Header().Set("X-Request-ID", reqID)

			sw := &statusWriter{ResponseWriter: w}

			next.ServeHTTP(sw, r)

			latency := time.Since(startTime)

			path := r.URL.Path
			var workspace string
			if strings.HasPrefix(path, "/state/") {
				workspace = strings.TrimPrefix(path, "/state/")
			}

			// Capture OTel trace information if active
			spanCtx := trace.SpanContextFromContext(r.Context())

			fields := []zap.Field{
				zap.String("request_id", reqID),
				zap.String("method", r.Method),
				zap.String("path", path),
				zap.String("workspace", workspace),
				zap.Int("status", sw.status),
				zap.Duration("latency", latency),
				zap.String("remote_ip", getIP(r)),
				zap.String("user_agent", r.UserAgent()),
			}

			if spanCtx.IsValid() {
				fields = append(fields,
					zap.String("trace_id", spanCtx.TraceID().String()),
					zap.String("span_id", spanCtx.SpanID().String()),
				)
			}

			// If status is 5xx, log as error; if 4xx log as warn; otherwise info
			if sw.status >= 500 {
				logger.Log.Error("Request processed with server error", fields...)
			} else if sw.status >= 400 {
				logger.Log.Warn("Request processed with client error", fields...)
			} else {
				logger.Log.Info("Request processed successfully", fields...)
			}
		})
	}
}
