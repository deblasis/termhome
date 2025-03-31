package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

// LogLevel defines the severity of the log message
type LogLevel int

const (
	// DEBUG level logs for detailed debugging information
	DEBUG LogLevel = iota
	// INFO level logs for general operational information
	INFO
	// WARN level logs for warning situations that might require attention
	WARN
	// ERROR level logs for error conditions that should be addressed
	ERROR
	// FATAL level logs for severe errors that cause the program to terminate
	FATAL
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Logger is the main logging entity
type Logger struct {
	level  LogLevel
	logger *log.Logger
	writer io.Writer
}

// Options configures the logger
type Options struct {
	// Level is the minimum log level to output
	Level LogLevel
	// LogToFile determines if logs should be written to a file
	LogToFile bool
	// LogDir is the directory where log files will be stored
	LogDir string
	// LogFileName is the name of the log file
	LogFileName string
	// MaxSize is the maximum size in megabytes of the log file before it gets rotated
	MaxSize int
	// MaxBackups is the maximum number of old log files to retain
	MaxBackups int
	// MaxAge is the maximum number of days to retain old log files
	MaxAge int
	// Compress determines if the rotated log files should be compressed
	Compress bool
}

// DefaultOptions returns the default logger options
func DefaultOptions() Options {
	return Options{
		Level:       INFO,
		LogToFile:   true,
		LogDir:      ".",
		LogFileName: "termdash.log",
		MaxSize:     5,
		MaxBackups:  3,
		MaxAge:      30,
		Compress:    true,
	}
}

// New creates a new logger with the specified options
func New(opts Options) *Logger {
	var writer io.Writer

	if opts.LogToFile {
		// Configure lumberjack log rotation
		logPath := filepath.Join(opts.LogDir, opts.LogFileName)
		fileLogger := &lumberjack.Logger{
			Filename:   logPath,
			MaxSize:    opts.MaxSize,
			MaxBackups: opts.MaxBackups,
			MaxAge:     opts.MaxAge,
			Compress:   opts.Compress,
		}

		writer = fileLogger
	} else {
		// Create a no-op writer if file logging is disabled
		writer = io.Discard
	}

	logger := log.New(writer, "", log.Ldate|log.Ltime|log.Lshortfile)

	return &Logger{
		level:  opts.Level,
		logger: logger,
		writer: writer,
	}
}

// SetLevel sets the minimum log level
func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

// GetLevel returns the current log level
func (l *Logger) GetLevel() LogLevel {
	return l.level
}

// Debug logs a message at DEBUG level
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.level <= DEBUG {
		l.log(DEBUG, format, args...)
	}
}

// Info logs a message at INFO level
func (l *Logger) Info(format string, args ...interface{}) {
	if l.level <= INFO {
		l.log(INFO, format, args...)
	}
}

// Warn logs a message at WARN level
func (l *Logger) Warn(format string, args ...interface{}) {
	if l.level <= WARN {
		l.log(WARN, format, args...)
	}
}

// Error logs a message at ERROR level
func (l *Logger) Error(format string, args ...interface{}) {
	if l.level <= ERROR {
		l.log(ERROR, format, args...)
	}
}

// Fatal logs a message at FATAL level and terminates the program
func (l *Logger) Fatal(format string, args ...interface{}) {
	if l.level <= FATAL {
		l.log(FATAL, format, args...)
		os.Exit(1)
	}
}

// log formats and outputs the log message
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	prefix := fmt.Sprintf("[%s] ", level.String())
	msg := fmt.Sprintf(format, args...)
	l.logger.Output(3, prefix+msg)
}

// Global logger instance for package-level functions
var globalLogger = New(DefaultOptions())

// SetGlobalLogger sets the global logger instance
func SetGlobalLogger(logger *Logger) {
	globalLogger = logger
}

// SetGlobalLogLevel sets the log level of the global logger
func SetGlobalLogLevel(level LogLevel) {
	globalLogger.SetLevel(level)
}

// Debug logs a message at DEBUG level through the global logger
func Debug(format string, args ...interface{}) {
	globalLogger.Debug(format, args...)
}

// Info logs a message at INFO level through the global logger
func Info(format string, args ...interface{}) {
	globalLogger.Info(format, args...)
}

// Warn logs a message at WARN level through the global logger
func Warn(format string, args ...interface{}) {
	globalLogger.Warn(format, args...)
}

// Error logs a message at ERROR level through the global logger
func Error(format string, args ...interface{}) {
	globalLogger.Error(format, args...)
}

// Fatal logs a message at FATAL level through the global logger and exits
func Fatal(format string, args ...interface{}) {
	globalLogger.Fatal(format, args...)
}

// ReplaceStdLogger replaces the standard library logger with our custom logger
func ReplaceStdLogger() {
	log.SetOutput(globalLogger.writer)
	log.SetFlags(0) // Remove default flags as our logger already adds them
	log.SetPrefix("")
}

// ParseLogLevel converts a string to a LogLevel
func ParseLogLevel(level string) LogLevel {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN":
		return WARN
	case "ERROR":
		return ERROR
	case "FATAL":
		return FATAL
	default:
		return INFO
	}
}
