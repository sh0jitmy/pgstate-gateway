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

package logger

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

var (
	Log         *slog.Logger
	logLevelVar = new(slog.LevelVar)
)

func redactAttr(groups []string, a slog.Attr) slog.Attr {
	// Re-key time to timestamp and format consistently
	if a.Key == slog.TimeKey && len(groups) == 0 {
		return slog.String("timestamp", a.Value.Time().Format("2006-01-02T15:04:05.000Z07:00"))
	}
	keyLower := strings.ToLower(a.Key)
	if keyLower == "password" || keyLower == "token" || keyLower == "secret" || keyLower == "authorization" {
		return slog.String(a.Key, "[REDACTED]")
	}
	return a
}

func Init(levelStr string) error {
	level, err := parseLevel(levelStr)
	if err != nil {
		return err
	}
	logLevelVar.Set(level)

	opts := &slog.HandlerOptions{
		Level:       logLevelVar,
		AddSource:   true,
		ReplaceAttr: redactAttr,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	Log = slog.New(handler)
	return nil
}

func SetLevel(levelStr string) error {
	level, err := parseLevel(levelStr)
	if err != nil {
		return err
	}
	logLevelVar.Set(level)
	return nil
}

func parseLevel(levelStr string) (slog.Level, error) {
	switch strings.ToLower(levelStr) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid log level: %s", levelStr)
	}
}
