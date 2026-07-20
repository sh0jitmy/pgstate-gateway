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
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	Log         *zap.Logger
	atomicLevel zap.AtomicLevel
)

type redactingCore struct {
	zapcore.Core
}

func newRedactingCore(c zapcore.Core) zapcore.Core {
	return &redactingCore{Core: c}
}

func (rc *redactingCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	redactFields(fields)
	return rc.Core.Write(ent, fields)
}

func (rc *redactingCore) With(fields []zapcore.Field) zapcore.Core {
	redactFields(fields)
	return &redactingCore{Core: rc.Core.With(fields)}
}

func redactFields(fields []zapcore.Field) {
	for i := range fields {
		keyLower := strings.ToLower(fields[i].Key)
		if keyLower == "password" || keyLower == "token" || keyLower == "secret" || keyLower == "authorization" {
			fields[i].Type = zapcore.StringType
			fields[i].String = "[REDACTED]"
			fields[i].Interface = nil
			fields[i].Integer = 0
		}
	}
}

func Init(levelStr string) error {
	level, err := parseLevel(levelStr)
	if err != nil {
		return err
	}
	atomicLevel = zap.NewAtomicLevelAt(level)

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		atomicLevel,
	)

	Log = zap.New(newRedactingCore(core), zap.AddCaller())
	return nil
}

func SetLevel(levelStr string) error {
	level, err := parseLevel(levelStr)
	if err != nil {
		return err
	}
	atomicLevel.SetLevel(level)
	return nil
}

func parseLevel(levelStr string) (zapcore.Level, error) {
	switch strings.ToLower(levelStr) {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn", "warning":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("invalid log level: %s", levelStr)
	}
}
