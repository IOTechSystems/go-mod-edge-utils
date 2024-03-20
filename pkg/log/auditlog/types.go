//
// Copyright (C) 2024 IOTech Ltd
//

package auditlog

import "log/slog"

// Constants of coverage level which can be used to label and group audit log by their coverage level.
const (
	BaseCoverage     = "BASE"
	AdvancedCoverage = "ADVANCED"
	FullCoverage     = "FULL"
)

// Leverage the slog level to define the coverage levels for easily setting the level and filtering.
// The higher the level, the more general the event.
// Those pre-defined levels can be found in the slog package (https://pkg.go.dev/log/slog#Level).
const (
	FullCoverageLevel     = slog.Level(-8)
	AdvancedCoverageLevel = slog.Level(2)
	BaseCoverageLevel     = slog.Level(12)
)

// slogLevelFromCoverageLevel returns the slog level for the given coverage level.
func slogLevelFromString(coverageLevel string) slog.Level {
	switch coverageLevel {
	case BaseCoverage:
		return BaseCoverageLevel
	case AdvancedCoverage:
		return AdvancedCoverageLevel
	case FullCoverage:
		return FullCoverageLevel
	default:
		return BaseCoverageLevel
	}
}

// ActionType is a categorical identifier used to give high-level insight as to the action type.
type ActionType string

// Constant ActionType identifiers which can be used to label and group audit log by their action type.
const (
	ActionTypeCreate   ActionType = "CREATE"
	ActionTypeDelete   ActionType = "DELETE"
	ActionTypeDownload ActionType = "DOWNLOAD"
	ActionTypeLogin    ActionType = "LOGIN"
	ActionTypeLogout   ActionType = "LOGOUT"
	ActionTypeUnknown  ActionType = "UNKNOWN"
	ActionTypeUpdate   ActionType = "UPDATE"
	ActionTypeUpload   ActionType = "UPLOAD"
	ActionTypeView     ActionType = "VIEW"
)

// isValidActionType checks if the given action type is valid.
func isValidActionType(a ActionType) bool {
	switch a {
	case ActionTypeCreate, ActionTypeDelete, ActionTypeDownload, ActionTypeLogin, ActionTypeLogout, ActionTypeUpdate, ActionTypeUpload, ActionTypeView, ActionTypeUnknown:
		return true
	default:
		return false
	}
}

// Severity is a categorical identifier used to give high-level insight as to the severity type.
type Severity string

// Constant Severity identifiers which can be used to label and group audit log by their severity.
const (
	SeverityCritical Severity = "CRITICAL"
	SeverityNormal   Severity = "NORMAL"
	SeverityMinor    Severity = "MINOR"
)

// Logger defines the interface for logging operations.
type Logger interface {
	// LogBase adds an audit log entry to the log writer with base coverage level
	LogBase(severity Severity, actor string, action ActionType, description string, details any)
	// LogAdvanced adds an audit log entry to the log writer with advanced coverage level
	LogAdvanced(severity Severity, actor string, action ActionType, description string, details any)
	// LogFull adds an audit log entry to the log writer with full coverage level
	LogFull(severity Severity, actor string, action ActionType, description string, details any)
}
