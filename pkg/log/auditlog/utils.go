package auditlog

import "log/slog"

const (
	// appKey is the key for the application name attribute
	appKey = "app"
)

// coverageLevels returns an array of the possible coverage levels in order from most to least verbose
func coverageLevels() []string {
	return []string{
		FullCoverage,
		AdvancedCoverage,
		BaseCoverage}
}

// isValidCoverageLevel checks if the given coverage level is valid
func isValidCoverageLevel(l string) bool {
	for _, name := range coverageLevels() {
		if name == l {
			return true
		}
	}
	return false
}

// replaceAttr is called to rewrite each non-group attribute before it is logged, which is an option of slog.HandlerOptions
func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	// Remove ts and msg keys from the default log entry
	if a.Key == slog.TimeKey || a.Key == slog.MessageKey {
		return slog.Attr{}
	}

	// Customize output string of the level key
	if a.Key == slog.LevelKey {
		// Handle custom level values
		level := a.Value.Any().(slog.Level)

		switch {
		case level == FullCoverageLevel:
			a.Value = slog.StringValue(FullCoverage)
		case level == AdvancedCoverageLevel:
			a.Value = slog.StringValue(AdvancedCoverage)
		case level == BaseCoverageLevel:
			a.Value = slog.StringValue(BaseCoverage)
		default:
			a.Value = slog.StringValue(BaseCoverage)
		}
	}

	return a
}
