package mariadb

import (
	mariadbMonitor "CheckHealthDO/internal/monitoring/services/mariadb"
	"CheckHealthDO/internal/pkg/config"

	"github.com/gin-gonic/gin"
)

// Handler contains MariaDB handler functionality
type Handler struct {
	config  *config.Config
	info    *InfoHandler
	service *ServiceHandler
	status  *StatusHandler
	monitor *mariadbMonitor.Monitor // Add monitor reference
}

// NewHandler creates a new MariaDB handler
func NewHandler(cfg *config.Config) *Handler {
	h := &Handler{
		config: cfg,
	}

	// Initialize sub-handlers
	h.info = NewInfoHandler(cfg)
	h.service = NewServiceHandler(cfg)
	h.status = NewStatusHandler(cfg)

	return h
}

// SetMonitor sets the monitor reference
func (h *Handler) SetMonitor(monitor *mariadbMonitor.Monitor) {
	h.monitor = monitor
	// Pass monitor to service handler
	h.service.SetMonitor(monitor)
}

// StartService handles starting the MariaDB service
func (h *Handler) StartService(c *gin.Context) {
	h.service.StartService(c)
}

// StopService handles stopping the MariaDB service
func (h *Handler) StopService(c *gin.Context) {
	h.service.StopService(c)
}

// RestartService handles restarting the MariaDB service
func (h *Handler) RestartService(c *gin.Context) {
	h.service.RestartService(c)
}
