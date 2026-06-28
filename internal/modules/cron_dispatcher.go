package modules

import (
	"context"
	"errors"
	"fmt"
)

// ErrCronNotFound is returned when name addresses an unregistered cron.
var ErrCronNotFound = errors.New("cron not found")

// DispatchScheduled runs the cron registered under name with the per-module
// prefixed Deps the registry stored at Build time. Returns ErrCronNotFound if
// no module owns that name — a scheduler firing an unknown name is a
// configuration bug worth surfacing to the caller.
//
// The handler runs synchronously in the calling goroutine; ctx propagates
// the scheduler's per-fire cancellation/timeout.
func DispatchScheduled(ctx context.Context, name string, reg *Registry) error {
	cron, ok := reg.Cron(name)
	if !ok {
		return fmt.Errorf("%w: %q", ErrCronNotFound, name)
	}
	deps, ok := reg.CronDeps(name)
	if !ok {
		// Should be impossible: Build always co-registers cron and cronDeps.
		return fmt.Errorf("modules: cron %q has no registered deps (registry corruption)", name)
	}
	return cron.Handler(ctx, deps)
}
