package memory

import (
	"CheckHealthDO/internal/alerts"
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"fmt"
	"sync"
	"time"
)

// SummaryReporter handles periodic summary reports
type SummaryReporter struct {
	monitor           *Monitor
	config            *config.Config
	warningEvents     int
	criticalEvents    int
	peakMemoryUsage   float64
	lastReportTime    time.Time
	mutex             sync.Mutex
	reportingInterval time.Duration
}

// NewSummaryReporter creates a new summary reporter
func NewSummaryReporter(monitor *Monitor, cfg *config.Config) *SummaryReporter {
	// Default to daily reports
	interval := 24 * time.Hour

	return &SummaryReporter{
		monitor:           monitor,
		config:            cfg,
		warningEvents:     0,
		criticalEvents:    0,
		peakMemoryUsage:   0,
		lastReportTime:    time.Now(),
		reportingInterval: interval,
	}
}

// RecordEvent records memory events for later summarization
func (s *SummaryReporter) RecordEvent(info *MemoryInfo) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Track statistics for summary
	if info.UsedMemoryPercentage > s.peakMemoryUsage {
		s.peakMemoryUsage = info.UsedMemoryPercentage
	}

	switch info.MemoryStatus {
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
		{Label: "Peak Memory Usage", Value: fmt.Sprintf("%.2f%%", s.peakMemoryUsage)},
	}

	// Add current memory status
	if currentInfo := s.monitor.GetLastMemoryInfo(); currentInfo != nil {
		tableRows = append(tableRows, alerts.TableRow{
			Label: "Current Memory Status",
			Value: fmt.Sprintf("%s (%.2f%%)", currentInfo.MemoryStatus, currentInfo.UsedMemoryPercentage),
		})
	}

	tableHTML := alerts.CreateTable(tableRows)

	// Generate HTML using the normal style
	styles := alerts.DefaultStyles()
	style := styles[alerts.AlertTypeNormal]

	// Add recommendation based on collected data
	var recommendation string
	if s.criticalEvents > 5 {
		recommendation = `
		<div style="background-color: #f2dede; border-left: 5px solid #d9534f; padding: 10px; margin: 10px 0;">
			<p><b>URGENT ACTION RECOMMENDED:</b> Multiple critical memory events detected. 
			Consider increasing system memory, identifying memory leaks, or optimizing application memory usage.</p>
		</div>`
	} else if s.warningEvents > 10 {
		recommendation = `
		<div style="background-color: #fcf8e3; border-left: 5px solid #faebcc; padding: 10px; margin: 10px 0;">
			<p><b>ACTION RECOMMENDED:</b> Frequent memory warnings detected.
			Monitor memory-intensive applications and consider optimization if the trend continues.</p>
		</div>`
	} else {
		recommendation = `
		<div style="background-color: #dff0d8; border-left: 5px solid #3c763d; padding: 10px; margin: 10px 0;">
			<p><b>SYSTEM HEALTHY:</b> Memory usage appears to be within normal parameters.</p>
		</div>`
	}

	message := alerts.CreateAlertHTML(
		alerts.AlertTypeNormal,
		style,
		"MEMORY USAGE SUMMARY REPORT",
		false,
		tableHTML,
		serverInfo,
		"<p>This is an automated summary of memory usage activity.</p>"+recommendation,
	)

	// Get email manager and send
	emailManager := s.monitor.GetNotificationManagers()
	if err := emailManager.SendEmail("Memory Usage Summary Report", message); err != nil {
		logger.Error("Failed to send memory summary report",
			logger.String("error", err.Error()))
	}

	// Reset counters after sending report
	s.warningEvents = 0
	s.criticalEvents = 0
	s.peakMemoryUsage = 0
}
