package signal

import (
	"CheckHealthDO/internal/api/router"
	"CheckHealthDO/internal/app"
	"CheckHealthDO/internal/pkg/logger"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var (
	cleanupFuncs     []func()
	cleanupMutex     sync.Mutex
	cleanupPerformed bool
)

// RegisterCleanupFunc registers a function to be called during graceful shutdown
func RegisterCleanupFunc(fn func()) {
	cleanupMutex.Lock()
	defer cleanupMutex.Unlock()
	cleanupFuncs = append(cleanupFuncs, fn)
}

// HandleSignals sets up signal handling for graceful shutdown
func HandleSignals(application *app.Application, builder *router.Builder) {
	sigChan := make(chan os.Signal, 1)

	// Register for common termination signals
	signal.Notify(sigChan,
		syscall.SIGINT,  // Ctrl+C
		syscall.SIGTERM, // Normal termination signal
		syscall.SIGHUP)  // Terminal disconnect

	for {
		sig := <-sigChan
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			logger.Info("Received termination signal, shutting down gracefully...",
				logger.String("signal", sig.String()))

			// Use a reliable shutdown approach with timeout
			performGracefulShutdown(application, builder)
			os.Exit(0)

		case syscall.SIGHUP:
			logger.Info("Received SIGHUP signal, triggering config reload...")
			// The watcher will handle the actual reload
		}
	}
}

// performGracefulShutdown handles the graceful shutdown process with timeout
func performGracefulShutdown(application *app.Application, builder *router.Builder) {
	// Give the application 30 seconds to shut down gracefully
	shutdownTimer := time.NewTimer(30 * time.Second)
	shutdownDone := make(chan struct{})

	go func() {
		// Shutdown all resources
		builder.Shutdown()
		application.Shutdown()

		// Run all registered cleanup functions
		runCleanupFunctions()

		close(shutdownDone)
	}()

	select {
	case <-shutdownDone:
		// Shutdown completed normally
		logger.Info("Graceful shutdown completed")
	case <-shutdownTimer.C:
		// Timeout occurred
		logger.Warn("Graceful shutdown timed out after 30 seconds, forcing exit")
	}
}

// runCleanupFunctions executes all registered cleanup functions
func runCleanupFunctions() {
	cleanupMutex.Lock()
	defer cleanupMutex.Unlock()

	// Only run cleanup once
	if cleanupPerformed {
		logger.Debug("Cleanup already performed, skipping")
		return
	}

	logger.Info("Running cleanup functions...",
		logger.Int("count", len(cleanupFuncs)))

	for _, fn := range cleanupFuncs {
		fn()
	}

	cleanupPerformed = true
}
