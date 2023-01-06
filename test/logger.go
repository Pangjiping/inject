package test

import "log"

type Logger struct {
	logger log.Logger
}

func NewDebugLogger() *Logger {
	return &Logger{
		logger: log.Logger{},
	}
}

func (l *Logger) Debugf(format string, v ...interface{}) {
	l.logger.Printf(format, v)
}
