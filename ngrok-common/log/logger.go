package log

import log "github.com/alecthomas/log4go"

var root log.Logger = make(log.Logger)

func LogTo(target, levelName string) {
	var writer log.LogWriter = nil
	switch target {
	case "stdout":
		writer = log.NewConsoleLogWriter()
	case "none":
	// no logging
	default:
		writer = log.NewFileLogWriter(target, true)
	}

	if writer != nil {
		var level = log.DEBUG
		switch levelName {
		case "FINEST":
			level = log.FINEST
		case "FINE":
			level = log.FINE
		case "DEBUG":
			level = log.DEBUG
		case "TRACE":
			level = log.TRACE
		case "INFO":
			level = log.INFO
		case "WARNING":
			level = log.WARNING
		case "ERROR":
			level = log.ERROR
		case "CRITICAL":
			level = log.CRITICAL
		default:
			level = log.DEBUG
		}
		root.AddFilter("log", level, writer)
	}
}

type Logger interface {
	AddLogPrefix(prefix string)
	ClearLogPrefixes()
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{}) error
	Error(format string, args ...interface{}) error
}

func Debug(format string, args ...interface{}) {
	root.Debug(format, args...)
}

func Info(format string, args ...interface{}) {
	root.Info(format, args...)
}

func Warn(format string, args ...interface{}) error {
	return root.Warn(format, args...)
}

func Error(format string, args ...interface{}) error {
	return root.Error(format, args)
}
