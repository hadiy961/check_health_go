package signal

import (
	"CheckHealthDO/internal/api/router"
	"CheckHealthDO/internal/app"
	"CheckHealthDO/internal/pkg/logger"
	"os"
	"os/signal"
	"syscall"
)

// HandleSignals sets up signal handling for graceful shutdown
func HandleSignals(application *app.Application, builder *router.Builder) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	for {
		sig := <-sigChan
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			logger.Info("Received termination signal, shutting down...",
				logger.String("signal", sig.String()))

			// Shutdown all resources
			builder.Shutdown()
			application.Shutdown()
			os.Exit(0)
		case syscall.SIGHUP:
			logger.Info("Received SIGHUP signal, triggering config reload...")
			// The watcher will handle the actual reload
		}
	}
}
