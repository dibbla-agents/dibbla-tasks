// Package output provides formatters that convert step execution events
// into structured output. The primary implementation is the GitHub Actions
// workflow commands format, but other formatters (callback, plain text)
// are available for different use cases.
//
// All formatters implement model.Formatter.
package output
