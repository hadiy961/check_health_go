package router

import (
	"CheckHealthDO/internal/monitoring/server/cpu"
	"CheckHealthDO/internal/monitoring/server/disk"
	"CheckHealthDO/internal/monitoring/server/memory"
	"CheckHealthDO/internal/monitoring/server/sysinfo"
	"CheckHealthDO/internal/monitoring/services/mariadb"
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"context"

	"github.com/gin-gonic/gin"
)

// Builder provides a fluent interface for constructing a router
type Builder struct {
	router *Router
	ctx    context.Context
	cancel context.CancelFunc

	// Monitors for lifecycle management
	monitors struct {
		mariaDB *mariadb.Monitor
		cpu     *cpu.Monitor
		memory  *memory.Monitor
		sysInfo *sysinfo.Monitor
		disk    *disk.Monitor
	}
}

// NewBuilder creates a new router builder
func NewBuilder(cfg *config.Config) *Builder {
	// Create cancellable context for monitors
	ctx, cancel := context.WithCancel(context.Background())

	// Create monitors
	cpuMonitor := createCPUMonitor(cfg)
	memoryMonitor := createMemoryMonitor(cfg)
	mariaDBMonitor := createMariaDBMonitor(cfg, ctx)
	sysInfoMonitor := createSysInfoMonitor(cfg)
	diskMonitor := createDiskMonitor(cfg)

	// Create builder
	builder := &Builder{
		router: New(cfg, mariaDBMonitor, cpuMonitor, memoryMonitor, sysInfoMonitor, diskMonitor),
		ctx:    ctx,
		cancel: cancel,
	}

	// Store monitors for lifecycle management
	builder.monitors.mariaDB = mariaDBMonitor
	builder.monitors.cpu = cpuMonitor
	builder.monitors.memory = memoryMonitor
	builder.monitors.sysInfo = sysInfoMonitor
	builder.monitors.disk = diskMonitor

	return builder
}

// createCPUMonitor creates and starts the CPU monitor
func createCPUMonitor(cfg *config.Config) *cpu.Monitor {
	monitor := cpu.NewMonitor(cfg)
	if monitor != nil {
		if err := monitor.StartMonitoring(); err != nil {
			logger.Warn("Failed to start CPU monitor", logger.String("error", err.Error()))
		} else {
			logger.Debug("Started CPU monitoring service")
		}
	}
	return monitor
}

// createMemoryMonitor creates and starts the Memory monitor
func createMemoryMonitor(cfg *config.Config) *memory.Monitor {
	monitor := memory.NewMonitor(cfg)
	if monitor != nil {
		if err := monitor.StartMonitoring(); err != nil {
			logger.Warn("Failed to start Memory monitor", logger.String("error", err.Error()))
		} else {
			logger.Debug("Started Memory monitoring service")
		}
	}
	return monitor
}

// createDiskMonitor creates and starts the Disk monitor
func createDiskMonitor(cfg *config.Config) *disk.Monitor {
	monitor := disk.NewMonitor(cfg)
	if monitor != nil {
		if err := monitor.StartMonitoring(); err != nil {
			logger.Warn("Failed to start Disk monitor", logger.String("error", err.Error()))
		} else {
			logger.Debug("Started Disk monitoring service")
		}
	}
	return monitor
}

// createSysInfoMonitor creates and starts the SysInfo monitor
func createSysInfoMonitor(cfg *config.Config) *sysinfo.Monitor {
	monitor := sysinfo.NewMonitor(cfg)
	if monitor != nil {
		go monitor.StartMonitoring()
		logger.Debug("Started SysInfo monitoring service")
	}
	return monitor
}

// createMariaDBMonitor creates and starts the MariaDB monitor
func createMariaDBMonitor(cfg *config.Config, ctx context.Context) *mariadb.Monitor {
	monitor, err := mariadb.NewMonitor(cfg)
	if err != nil {
		logger.Warn("Failed to create MariaDB monitor", logger.String("error", err.Error()))
		return nil
	}

	if monitor != nil {
		go monitor.Start(ctx)
		logger.Debug("Started MariaDB monitoring service")
	}
	return monitor
}

// WithMiddleware adds a middleware to the router
func (b *Builder) WithMiddleware(middleware gin.HandlerFunc) *Builder {
	b.router.engine.Use(middleware)
	return b
}

// WithAPIRoutes adds API routes
func (b *Builder) WithAPIRoutes() *Builder {
	b.router.registerAPIRoutes()
	return b
}

// WithWebSocketRoutes adds WebSocket routes
func (b *Builder) WithWebSocketRoutes() *Builder {
	b.router.registerWebSocketRoutes()
	return b
}

// WithAllRoutes adds all routes and initializes the router
func (b *Builder) WithAllRoutes() *Builder {
	b.router.Initialize()
	return b
}

// GetRouter returns the underlying router
func (b *Builder) GetRouter() *Router {
	return b.router
}

// Start starts the HTTP server
func (b *Builder) Start() {
	b.router.Start()
}

// Build is a convenience method that returns the Builder itself
// to maintain backward compatibility
func (b *Builder) Build() (*Builder, error) {
	return b, nil
}

// Shutdown stops all monitors
func (b *Builder) Shutdown() {
	// Cancel context to stop MariaDB monitor
	if b.cancel != nil {
		b.cancel()
	}

	// Stop all monitors explicitly
	if b.monitors.mariaDB != nil {
		b.monitors.mariaDB.Stop()
		logger.Info("Stopped MariaDB monitoring service")
	}

	if b.monitors.cpu != nil {
		b.monitors.cpu.StopMonitoring()
		logger.Info("Stopped CPU monitoring service")
	}

	if b.monitors.memory != nil {
		b.monitors.memory.StopMonitoring()
		logger.Info("Stopped Memory monitoring service")
	}

	if b.monitors.sysInfo != nil {
		b.monitors.sysInfo.StopMonitoring()
		logger.Info("Stopped SysInfo monitoring service")
	}

	if b.monitors.disk != nil {
		b.monitors.disk.StopMonitoring()
		logger.Info("Stopped Disk monitoring service")
	}
}
