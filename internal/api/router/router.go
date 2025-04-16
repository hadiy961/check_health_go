package router

import (
	"CheckHealthDO/internal/api/handlers"
	"CheckHealthDO/internal/api/router/routes/mariadb"
	"CheckHealthDO/internal/api/router/routes/server"
	"CheckHealthDO/internal/api/router/routes/websocket"
	"CheckHealthDO/internal/monitoring/server/cpu"
	"CheckHealthDO/internal/monitoring/server/disk"
	"CheckHealthDO/internal/monitoring/server/memory"
	"CheckHealthDO/internal/monitoring/server/sysinfo"
	mariadbMonitor "CheckHealthDO/internal/monitoring/services/mariadb"
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Router encapsulates the HTTP router functionality
type Router struct {
	config        *config.Config
	engine        *gin.Engine
	serverHandler *handlers.ServerHandler
	dbHandler     *handlers.DatabaseHandler

	// Monitors
	monitors struct {
		mariaDB *mariadbMonitor.Monitor
		cpu     *cpu.Monitor
		memory  *memory.Monitor
		sysInfo *sysinfo.Monitor
		disk    *disk.Monitor
	}
}

// New creates a new router instance with the given configuration
func New(cfg *config.Config, mariaDBMonitor *mariadbMonitor.Monitor, cpuMonitor *cpu.Monitor, memoryMonitor *memory.Monitor, sysInfoMonitor *sysinfo.Monitor, diskMonitor *disk.Monitor) *Router {
	// Configure gin mode based on config
	if cfg.Logs.Level != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()

	// Create handlers
	serverHandler := handlers.NewServerHandler(cfg)
	dbHandler := handlers.NewDatabaseHandler(cfg)

	r := &Router{
		config:        cfg,
		engine:        engine,
		serverHandler: serverHandler,
		dbHandler:     dbHandler,
	}

	// Store monitors
	r.monitors.mariaDB = mariaDBMonitor
	r.monitors.cpu = cpuMonitor
	r.monitors.memory = memoryMonitor
	r.monitors.sysInfo = sysInfoMonitor
	r.monitors.disk = diskMonitor

	return r
}

// Initialize sets up the router with middlewares and routes
func (r *Router) Initialize() *Router {
	// Apply essential middleware only
	r.engine.Use(gin.Recovery())
	r.engine.Use(LoggerMiddleware())

	// Register API routes
	r.registerAPIRoutes()

	// Register WebSocket routes
	r.registerWebSocketRoutes()

	// Add a simple root endpoint for API health check
	r.registerRootAPIEndpoint()

	// Log all registered routes for debugging
	for _, route := range r.engine.Routes() {
		logger.Debug("Registered route",
			logger.String("method", route.Method),
			logger.String("path", route.Path))
	}

	return r
}

// registerAPIRoutes registers all API-specific routes
func (r *Router) registerAPIRoutes() {
	// Register server routes
	server.RegisterRoutes(r.engine, r.serverHandler)

	// Register MariaDB routes if monitor is available
	if r.monitors.mariaDB != nil {
		mariadb.RegisterRoutes(r.engine, r.config, r.monitors.mariaDB)
	}
}

// registerWebSocketRoutes registers all WebSocket routes
func (r *Router) registerWebSocketRoutes() {
	websocket.RegisterWebSocketRoutes(
		r.engine,
		r.monitors.cpu,
		r.monitors.mariaDB,
		r.monitors.memory,
		r.monitors.sysInfo,
		r.monitors.disk,
	)
}

// registerRootAPIEndpoint provides a simple API health check endpoint
func (r *Router) registerRootAPIEndpoint() {
	r.engine.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"app":     "CheckHealthDO API",
			"version": "1.0",
		})
	})

	// API status endpoint
	r.engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
		})
	})
}

// Engine returns the underlying gin engine
func (r *Router) Engine() *gin.Engine {
	return r.engine
}

// ServeHTTP implements the http.Handler interface
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.engine.ServeHTTP(w, req)
}

// LoggerMiddleware creates a middleware for logging HTTP requests
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip logging for WebSocket connections
		if c.Request.Header.Get("Upgrade") == "websocket" {
			c.Next()
			return
		}

		// Process request
		c.Next()

		// Log after request
		logger.Info("HTTP Request",
			logger.String("method", c.Request.Method),
			logger.String("path", c.Request.URL.Path),
			logger.Int("status", c.Writer.Status()),
			logger.String("client_ip", c.ClientIP()),
		)
	}
}

// Start starts the HTTP server
func (r *Router) Start() {
	addr := fmt.Sprintf("%s:%d", r.config.Server.Host, r.config.Server.Port)
	logger.Info("Starting HTTP server", logger.String("address", addr))

	if err := r.engine.Run(addr); err != nil {
		logger.Error("Failed to start HTTP server", logger.String("error", err.Error()))
	}
}
