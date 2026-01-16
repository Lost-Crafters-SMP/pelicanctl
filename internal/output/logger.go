package output

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
)

var (
	globalLogger *slog.Logger
	logLevel     slog.Level
)

// InitLogger initializes the global logger.
func InitLogger(verbose bool, quiet bool, outputFormat OutputFormat, writer io.Writer) {
	if writer == nil {
		writer = os.Stderr
	}

	// Set log level
	switch {
	case quiet:
		logLevel = slog.LevelError
	case verbose:
		logLevel = slog.LevelDebug
	default:
		logLevel = slog.LevelInfo
	}

	// Create handler based on output format
	var handler slog.Handler
	if outputFormat == OutputFormatJSON {
		handler = slog.NewJSONHandler(writer, &slog.HandlerOptions{
			Level: logLevel,
		})
	} else {
		handler = slog.NewTextHandler(writer, &slog.HandlerOptions{
			Level: logLevel,
		})
	}

	globalLogger = slog.New(handler)
}

// GetLogger returns the global logger.
func GetLogger() *slog.Logger {
	if globalLogger == nil {
		// Default logger if not initialized
		InitLogger(false, false, OutputFormatTable, nil)
	}
	return globalLogger
}

// LogDebug logs a debug message.
func LogDebug(msg string, args ...any) {
	GetLogger().Debug(msg, args...)
}

// LogInfo logs an info message.
func LogInfo(msg string, args ...any) {
	GetLogger().Info(msg, args...)
}

// LogWarn logs a warning message.
func LogWarn(msg string, args ...any) {
	GetLogger().Warn(msg, args...)
}

// LogError logs an error message.
func LogError(msg string, args ...any) {
	GetLogger().Error(msg, args...)
}

// FormatError formats an error as JSON if needed.
func FormatError(err error) map[string]any {
	return map[string]any{
		"error": err.Error(),
	}
}

// FormatMessage formats a message as JSON if needed.
func FormatMessage(msg string) map[string]any {
	return map[string]any{
		"message": msg,
	}
}

// PrettyPrintJSON pretty prints JSON to writer.
func PrettyPrintJSON(data any, writer io.Writer) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
