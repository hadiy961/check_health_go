package handlers

import (
	"CheckHealthDO/internal/monitoring/server"
	"CheckHealthDO/internal/monitoring/server/cpu"
	"CheckHealthDO/internal/monitoring/server/disk"
	"CheckHealthDO/internal/monitoring/server/memory"
	"CheckHealthDO/internal/monitoring/server/sysinfo"
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ServerHandler contains handlers for server-related endpoints
type ServerHandler struct {
	config *config.Config
}

// NewServerHandler creates a new server handler
func NewServerHandler(cfg *config.Config) *ServerHandler {
	return &ServerHandler{
		config: cfg,
	}
}

// GetServerMetrics handles requests to get all server metrics
func (h *ServerHandler) GetServerMetrics(c *gin.Context) {
	// Pass the config argument to GetServerAllInfo
	metrics, err := server.GetServerAllInfo(h.config)
	if err != nil {
		logger.Error("Failed to get server metrics",
			logger.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get server metrics",
		})
		return
	}

	c.JSON(http.StatusOK, metrics)
}

func (h *ServerHandler) GetSystemInfoHandler(c *gin.Context) {
	info, err := sysinfo.GetSystemInfo()
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, info)
}

// GetServerInfo handles general server information requests
// This is a convenience method that returns system information
func (h *ServerHandler) GetServerInfo(c *gin.Context) {
	info, err := sysinfo.GetSystemInfo()
	if err != nil {
		logger.Error("Failed to get system info",
			logger.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get system information",
		})
		return
	}
	c.JSON(http.StatusOK, info)
}

// GetCPUInfo handles the CPU information endpoint
func (h *ServerHandler) GetCPUInfo(c *gin.Context) {
	// Use CPU module directly with thresholds from configuration
	info, err := cpu.GetCPUInfo(
		h.config.Monitoring.CPU.WarningThreshold,
		h.config.Monitoring.CPU.CriticalThreshold,
	)
	if err != nil {
		logger.Error("Failed to get CPU info",
			logger.String("error", err.Error()))
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, info)
}

// GetMemoryInfo handles the memory information endpoint
func (h *ServerHandler) GetMemoryInfo(c *gin.Context) {
	info, err := memory.GetMemoryInfo(
		h.config.Monitoring.Memory.WarningThreshold,
		h.config.Monitoring.Memory.CriticalThreshold,
	)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, info)
}

// GetDiskInfo handles requests to get disk information
func (h *ServerHandler) GetDiskInfo(c *gin.Context) {
	// Pass monitored paths from config to GetStorageInfo
	storageInfos, totalStorage, err := disk.GetStorageInfo()
	if err != nil {
		logger.Error("Failed to get disk information",
			logger.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get disk information",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"partitions":    storageInfos,
		"total_storage": totalStorage,
	})
}
