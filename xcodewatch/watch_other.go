//go:build !darwin

package xcodewatch

import (
	"context"

	"github.com/dibbla-agents/dibbla-tasks/model"
)

// Watch is a no-op on non-darwin platforms.
func Watch(_ context.Context, _ model.Formatter) context.CancelFunc {
	return func() {}
}
