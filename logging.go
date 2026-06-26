package main

import (
	"fmt"
	"io"
	"log"
	"os"
)

type logLevel int

const (
	levelDebug logLevel = iota
	levelInfo
	levelWarn
	levelError
)

func (l logLevel) String() string {
	switch l {
	case levelDebug:
		return "[DEBUG] "
	case levelInfo:
		return "[INFO]  "
	case levelWarn:
		return "[WARN]  "
	case levelError:
		return "[ERROR] "
	default:
		return ""
	}
}

type levelLogger struct {
	logger    *log.Logger
	level     logLevel
	isEnabled bool
}

var gLog levelLogger

func init() {
	gLog = levelLogger{
		logger:    log.New(io.Discard, "", log.LstdFlags),
		level:     levelDebug,
		isEnabled: false,
	}
}

func setupLog(path string) (*os.File, error) {
	gLog.isEnabled = false
	if path == "" {
		return nil, nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		return nil, err
	}
	gLog.logger.SetOutput(f)
	gLog.isEnabled = true
	return f, nil
}

func logp(l logLevel, v ...any) {
	if !gLog.isEnabled || l < gLog.level {
		return
	}
	if err := gLog.logger.Output(3, l.String()+fmt.Sprint(v...)); err != nil {
		panic(err)
	}
}

func logf(l logLevel, format string, args ...any) {
	if !gLog.isEnabled || l < gLog.level {
		return
	}
	if err := gLog.logger.Output(3, l.String()+fmt.Sprintf(format, args...)); err != nil {
		panic(err)
	}
}

func debugp(v ...any) { logp(levelDebug, v...) }
func infop(v ...any)  { logp(levelInfo, v...) }
func warnp(v ...any)  { logp(levelWarn, v...) }
func errorp(v ...any) { logp(levelError, v...) }

func debugf(format string, args ...any) { logf(levelDebug, format, args...) }
func infof(format string, args ...any)  { logf(levelInfo, format, args...) }
func warnf(format string, args ...any)  { logf(levelWarn, format, args...) }
func errorf(format string, args ...any) { logf(levelError, format, args...) }
