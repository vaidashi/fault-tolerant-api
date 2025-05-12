package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// Logger represents a simple logger interface
type Logger interface {
	Debug(msg string, keyvals ...interface{})
	Info(msg string, keyvals ...interface{})
	Warn(msg string, keyvals ...interface{})
	Error(msg string, keyvals ...interface{})
}

type logLevel int

const (
	debugLevel logLevel = iota
	infoLevel
	warnLevel
	errorLevel
)

type simpleLogger struct {
	debugLogger *log.Logger
	infoLogger  *log.Logger
	warnLogger  *log.Logger
	errorLogger *log.Logger
	level       logLevel
}

// NewLogger creates a new logger with the specified level
func NewLogger(level string) Logger {
	var l logLevel

	switch strings.ToLower(level) {
	case "debug":
		l = debugLevel
	case "info":
		l = infoLevel
	case "warn":
		l = warnLevel
	case "error":
		l = errorLevel
	default:
		l = infoLevel
	}

	return &simpleLogger{
		debugLogger: log.New(os.Stdout, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile),
		infoLogger:  log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime),
		warnLogger:  log.New(os.Stdout, "WARN: ", log.Ldate|log.Ltime),
		errorLogger: log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile),
		level:       l,
	}
}

func (l *simpleLogger) Debug(msg string, keyvals ...interface{}) {
	if l.level <= debugLevel {
		l.debugLogger.Println(formatMsg(msg, keyvals...))
	}
}

func (l *simpleLogger) Info(msg string, keyvals ...interface{}) {
	if l.level <= infoLevel {
		l.infoLogger.Println(formatMsg(msg, keyvals...))
	}
}

func (l *simpleLogger) Warn(msg string, keyvals ...interface{}) {
	if l.level <= warnLevel {
		l.warnLogger.Println(formatMsg(msg, keyvals...))
	}
}

func (l *simpleLogger) Error(msg string, keyvals ...interface{}) {
	if l.level <= errorLevel {
		l.errorLogger.Println(formatMsg(msg, keyvals...))
	}
}

func formatMsg(msg string, keyvals ...interface{}) string {
	if len(keyvals) == 0 {
		return msg
	}

	formattedMsg := msg

	for i := 0; i < len(keyvals); i += 2 {
		var key, value string
		key = keyvals[i].(string)
		
		if i+1 < len(keyvals) {
			value = fmt.Sprintf("%v", keyvals[i+1])
		} else {
			value = "missing"
		}
		
		formattedMsg += " " + key + "=" + value
	}
	
	return formattedMsg
}