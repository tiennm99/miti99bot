package server

import "net/http"

// New builds the application's HTTP handler. The only route is:
//
//	GET / → health (Coolify container monitor; not publicly routed)
//
// There is no /webhook route: Telegram updates arrive via long polling
// (cmd/server runs b.Start), so the bot needs no public inbound ingress.
// There is no /cron route either: crons fire from the in-process scheduler
// (internal/cron), so cron has no HTTP surface to expose or protect.
// Anything else is 404. All routes pass through LogRequests so every request
// emits a structured `req` log line.
func New() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", HealthHandler())
	return LogRequests(mux)
}
