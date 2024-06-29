package log

import (
	"fmt"
	log "github.com/alecthomas/log4go"
)

type prefixLogger struct {
	*log.Logger
	prefix string
}

func NewPrefixLogger(prefix ...string) Logger {
	logger := &prefixLogger{Logger: &root}
	for _, p := range prefix {
		logger.AddLogPrefix(p)
	}
	return logger
}

func (p *prefixLogger) AddLogPrefix(prefix string) {
	if len(p.prefix) > 0 {
		p.prefix += " "
	}
	p.prefix += "[" + prefix + "]"
}

func (p *prefixLogger) ClearLogPrefixes() {
	p.prefix = ""
}

func (p *prefixLogger) pfx(format string) interface{} {
	return fmt.Sprintf("%s %s", p.prefix, format)
}

func (p *prefixLogger) Debug(format string, args ...interface{}) {
	p.Logger.Debug(p.pfx(format), args...)
}

func (p *prefixLogger) Info(format string, args ...interface{}) {
	p.Logger.Info(p.pfx(format), args...)
}

func (p *prefixLogger) Warn(format string, args ...interface{}) error {
	return p.Logger.Warn(p.pfx(format), args...)
}

func (p *prefixLogger) Error(format string, args ...interface{}) error {
	return p.Logger.Error(p.pfx(format), args...)
}
