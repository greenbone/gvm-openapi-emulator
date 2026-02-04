// SPDX-FileCopyrightText: 2026 Greenbone AG
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package logger_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ozgen/gvm-openapi-emulator/logger"

	"github.com/sirupsen/logrus"
)

var FixedTime = time.Date(2025, time.March, 1, 22, 36, 14, 889828078, time.UTC)

func TestCustomFormatter(t *testing.T) {

	custom := &logger.CustomFormatter{

		Version: "0.1-test",
	}
	entry := &logrus.Entry{
		Logger:  logrus.New(),
		Time:    FixedTime,
		Level:   logrus.InfoLevel,
		Message: "Test message",
		Data: map[string]interface{}{
			"user": "admin",
			"id":   42,
		},
	}

	formatted, err := custom.Format(entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := string(formatted)
	if !strings.Contains(output, "Test message") {
		t.Errorf("Expected log message to contain 'Test message', got: %s", output)
	}
	if !strings.Contains(output, "Version: 0.1-test") {
		t.Errorf("Expected version to appear in log, got: %s", output)
	}
}

func TestGetLoggerOutput(t *testing.T) {
	log := logger.GetLogger()

	tmpFile, err := os.CreateTemp("", "test-logger-*.log")
	if err != nil {
		t.Fatalf("could not create temp log file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	log.SetOutput(tmpFile)
	log.SetLevel(logrus.InfoLevel)
	log.Info("integrations logger test")

	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("could not read log output: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "integrations logger test") {
		t.Errorf("Expected log output to contain test message, got: %s", content)
	}
	if !strings.Contains(content, "Version:") {
		t.Errorf("Expected log output to contain version info, got: %s", content)
	}
}
