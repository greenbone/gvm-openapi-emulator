// SPDX-FileCopyrightText: 2026 Greenbone AG
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/greenbone/gvm-openapi-emulator/config"

	"github.com/sirupsen/logrus"
)

// CustomFormatter embeds logrus.TextFormatter and adds the version
type CustomFormatter struct {
	logrus.TextFormatter
	Version string
}

func (f *CustomFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	// ANSI colors
	colorReset := "\033[0m"
	colorCyan := "\033[36m"
	colorGreen := "\033[32m"
	colorYellow := "\033[33m"
	colorRed := "\033[31m"
	colorBlue := "\033[34m"

	// Timestamp
	timestamp := entry.Time.Format("2006-01-02T15:04:05.000Z07:00")

	levelColor := ""
	switch entry.Level {
	case logrus.DebugLevel:
		levelColor = colorBlue
	case logrus.InfoLevel:
		levelColor = colorGreen
	case logrus.WarnLevel:
		levelColor = colorYellow
	case logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel:
		levelColor = colorRed
	default:
		levelColor = colorReset
	}
	level := fmt.Sprintf("%s%s%s", levelColor, entry.Level.String(), colorReset)

	caller := ""
	if entry.HasCaller() {
		caller = fmt.Sprintf("%s:%d", filepath.Base(entry.Caller.File), entry.Caller.Line)
	}

	var keys []string
	for k := range entry.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fields := ""
	for _, k := range keys {
		v := entry.Data[k]
		fields += fmt.Sprintf(" %s%s%s=%v", colorCyan, k, colorReset, v)
	}

	// Final log line
	logLine := fmt.Sprintf(
		"%s [%s] [Version: %s] %s %s%s\n",
		timestamp, level, f.Version, caller, entry.Message, fields,
	)

	return []byte(logLine), nil
}

var log *logrus.Logger

func init() {
	log = logrus.New()

	// set filename to logs
	log.SetReportCaller(true)

	log.SetFormatter(&CustomFormatter{
		TextFormatter: logrus.TextFormatter{
			FullTimestamp: true,
		},
		Version: "dev", // todo we may need to add proper version info
	})

	log.SetOutput(os.Stdout)
	logLvl := getLogLvl(strings.ToLower(config.Envs.LogLevel))
	log.SetLevel(logLvl)
}

func GetLogger() *logrus.Logger {
	return log
}

func getLogLvl(logLvl string) logrus.Level {
	switch logLvl {
	case "debug":
		return logrus.DebugLevel
	case "info":
		return logrus.InfoLevel
	case "warn":
		return logrus.WarnLevel
	case "error":
		return logrus.ErrorLevel
	}
	return logrus.InfoLevel
}
