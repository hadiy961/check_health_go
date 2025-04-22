package cpu

import (
	"CheckHealthDO/internal/alerts"
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"fmt"
	"sync"
	"time"
)

// SummaryReporter handles periodic summary reports for CPU usage
type SummaryReporter struct {
	monitor           *Monitor
	config            *config.Config
	warningEvents     int
	criticalEvents    int
	peakCPUUsage      float64
	lastReportTime    time.Time
	mutex             sync.Mutex
	reportingInterval time.Duration
}

// NewSummaryReporter creates a new summary reporter for CPU
func NewSummaryReporter(monitor *Monitor, cfg *config.Config) *SummaryReporter {
	// Default to daily reports
	interval := 24 * time.Hour

	return &SummaryReporter{
		monitor:           monitor,
		config:            cfg,
		warningEvents:     0,
		criticalEvents:    0,
		peakCPUUsage:      0,
		lastReportTime:    time.Now(),
		reportingInterval: interval,
	}
}

// RecordEvent records CPU events for later summarization
func (s *SummaryReporter) RecordEvent(info *CPUInfo) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Track statistics for summary
	if info.Usage > s.peakCPUUsage {
		s.peakCPUUsage = info.Usage
	}

	switch info.CPUStatus {
	case "warning":
		s.warningEvents++
	case "critical":
		s.criticalEvents++
	}

	// Check if it's time to send a report
	if time.Since(s.lastReportTime) >= s.reportingInterval {
		s.sendSummaryReport()
	}
}

// sendSummaryReport sends a summary report via email
func (s *SummaryReporter) sendSummaryReport() {
	// Reset last report time first to prevent duplicate reports
	s.lastReportTime = time.Now()

	// Create summary content
	serverInfo := alerts.GetServerInfoForAlert()

	// Add summary table
	tableRows := []alerts.TableRow{
		{Label: "Reporting Period", Value: fmt.Sprintf("Last %d hours", int(s.reportingInterval.Hours()))},
		{Label: "Warning Events", Value: fmt.Sprintf("%d", s.warningEvents)},
		{Label: "Critical Events", Value: fmt.Sprintf("%d", s.criticalEvents)},
		{Label: "Peak CPU Usage", Value: fmt.Sprintf("%.2f%%", s.peakCPUUsage)},
	}

	tableHTML := alerts.CreateTable(tableRows)

	// Generate HTML using the normal style
	styles := alerts.DefaultStyles()
	style := styles[alerts.AlertTypeNormal]

	message := alerts.CreateAlertHTML(
		alerts.AlertTypeNormal,
		style,
		"CPU USAGE SUMMARY REPORT",
		false,
		tableHTML,
		serverInfo,
		"<p>This is an automated summary of CPU usage activity.</p>",
	)

	// Get email manager and send
	emailManager := s.monitor.GetNotificationManagers()
	if err := emailManager.SendEmail("CPU Usage Summary Report", message); err != nil {
		logger.Error("Failed to send CPU summary report",
			logger.String("error", err.Error()))
	}

	// Reset counters after sending report
	s.warningEvents = 0
	s.criticalEvents = 0
	s.peakCPUUsage = 0
}
