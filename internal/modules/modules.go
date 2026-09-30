// Package modules is the bot's module framework: the Module, Command, Callback
// and Cron types a feature package declares, the Registry that Build assembles
// from the MODULES selection, and the dispatcher that Install wires into the
// Telegram bot with visibility-based authorization.
//
// The module catalog (name → Factory) does not live here. Concrete modules are
// subpackages (internal/modules/util, /misc, …) that import this package, so
// keeping the catalog here would create an import cycle. The composition root
// in cmd/server owns it instead, and tests pass their own catalog into Build.
package modules
