//
// Copyright (C) 2025 IOTech Ltd
//

package log

// NopeLogger is a no-operation logger that implements the Logger interface
type NopeLogger struct {
}

// NewNopeLogger creates a new instance of NopeLogger
func NewNopeLogger() Logger {
	return NopeLogger{}
}

// SetLogLevel simulates setting a log severity level
func (lc NopeLogger) SetLogLevel(_ string) error {
	return nil
}

// LogLevel returns the current log level setting
func (lc NopeLogger) LogLevel() string {
	return ""
}

// Info simulates logging an entry at the INFO severity level
func (lc NopeLogger) Info(_ string, _ ...any) {
}

// Debug simulates logging an entry at the DEBUG severity level
func (lc NopeLogger) Debug(_ string, _ ...any) {
}

// Error simulates logging an entry at the ERROR severity level
func (lc NopeLogger) Error(_ string, _ ...any) {
}

// Trace simulates logging an entry at the TRACE severity level
func (lc NopeLogger) Trace(_ string, _ ...any) {
}

// Warn simulates logging an entry at the WARN severity level
func (lc NopeLogger) Warn(_ string, _ ...any) {
}

// Infof simulates logging an formatted message at the INFO severity level
func (lc NopeLogger) Infof(_ string, _ ...any) {
}

// Debugf simulates logging an formatted message at the DEBUG severity level
func (lc NopeLogger) Debugf(_ string, _ ...any) {
}

// Errorf simulates logging an formatted message at the ERROR severity level
func (lc NopeLogger) Errorf(_ string, _ ...any) {
}

// Tracef simulates logging an formatted message at the TRACE severity level
func (lc NopeLogger) Tracef(_ string, _ ...any) {
}

// Warnf simulates logging an formatted message at the WARN severity level
func (lc NopeLogger) Warnf(_ string, _ ...any) {
}
