package logger

import (
	"fmt"
	"io"
	"os"
)

type LogLevel int

const (
	NONE  LogLevel = iota // Only errors and start/end messages
	STEPS                 // Show which steps are taken
	FULL                  // Show detailed information about each step
)

type Logger struct {
	level  LogLevel
	output io.Writer
}

var globalLogger *Logger

// Initialize the global logger
func init() {
	globalLogger = &Logger{
		level:  NONE,
		output: os.Stdout,
	}
}

// SetLevel sets the log level for the global logger
func SetLevel(level LogLevel) {
	globalLogger.level = level
}

// ParseLevel converts a string to a LogLevel
func ParseLevel(levelStr string) LogLevel {
	switch levelStr {
	case "full":
		return FULL
	case "steps":
		return STEPS
	case "none":
		return NONE
	default:
		return NONE
	}
}

// Error always prints (regardless of log level)
func Error(format string, args ...interface{}) {
	fmt.Fprintf(globalLogger.output, "ERROR: "+format, args...)
}

// Info prints only start/end messages and errors (always printed)
func Info(format string, args ...interface{}) {
	fmt.Fprintf(globalLogger.output, format, args...)
}

// Step prints step information (printed at STEPS and FULL levels)
func Step(format string, args ...interface{}) {
	if globalLogger.level >= STEPS {
		fmt.Fprintf(globalLogger.output, format, args...)
	}
}

// Detail prints detailed information (printed only at FULL level)
func Detail(format string, args ...interface{}) {
	if globalLogger.level >= FULL {
		fmt.Fprintf(globalLogger.output, format, args...)
	}
}

// GetLevel returns the current log level
func GetLevel() LogLevel {
	return globalLogger.level
}
