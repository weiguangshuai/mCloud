package logger

import (
	"log"
	"strings"
	"sync/atomic"
)

const (
	levelDebug int32 = 1
	levelInfo  int32 = 2
)

var logLevel atomic.Int32

func init() {
	logLevel.Store(levelInfo)
}

func SetLevel(level string) {
	if strings.EqualFold(strings.TrimSpace(level), "debug") {
		logLevel.Store(levelDebug)
		return
	}

	logLevel.Store(levelInfo)
}

func IsDebugEnabled() bool {
	return logLevel.Load() == levelDebug
}

func Debugf(format string, v ...any) {
	if !IsDebugEnabled() {
		return
	}

	log.Printf("[DEBUG] "+format, v...)
}
