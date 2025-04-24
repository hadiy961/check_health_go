package startup

import (
	"CheckHealthDO/internal/api/router"
	"CheckHealthDO/internal/app"
)

// StartServer initializes and starts the HTTP server
func StartServer(application *app.Application) *router.Builder {
	// Get configuration
	config := application.GetConfig()

	// Create builder that internally manages all monitors
	builder := router.NewBuilder(config).
		WithAllRoutes() // This already calls Initialize()

	// Start HTTP server in a goroutine
	go builder.Start()

	return builder
}
