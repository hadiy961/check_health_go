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
	monitor            *Monitor
	config             *config.Config
	warningEvents      int
	criticalEvents     int
	peakCPUUsage       float64
	lastReportTime     time.Time
	mutex              sync.Mutex
	reportingInterval  time.Duration
	highUsageDurations map[string]time.Duration // Track how long CPU spent in different usage ranges
	lastCheckTime      time.Time
	lastStatus         string
}

// NewSummaryReporter creates a new summary reporter for CPU
func NewSummaryReporter(monitor *Monitor, cfg *config.Config) *SummaryReporter {
	// Default to daily reports
	interval := 24 * time.Hour

	return &SummaryReporter{
		monitor:            monitor,
		config:             cfg,
		warningEvents:      0,
		criticalEvents:     0,
		peakCPUUsage:       0,
		lastReportTime:     time.Now(),
		reportingInterval:  interval,
		highUsageDurations: make(map[string]time.Duration),
		lastCheckTime:      time.Now(),
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

	// Track durations in different states
	now := time.Now()
	if !s.lastCheckTime.IsZero() && s.lastStatus != "" {
		duration := now.Sub(s.lastCheckTime)
		s.highUsageDurations[s.lastStatus] += duration
	}
	s.lastCheckTime = now
	s.lastStatus = info.CPUStatus

	// Count event types
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

	// Calculate percentage of time spent in each state
	totalDuration := s.reportingInterval
	normalDuration := totalDuration - s.highUsageDurations["warning"] - s.highUsageDurations["critical"]
	if normalDuration < 0 {
		normalDuration = 0
	}

	normalPct := float64(normalDuration) / float64(totalDuration) * 100
	warningPct := float64(s.highUsageDurations["warning"]) / float64(totalDuration) * 100
	criticalPct := float64(s.highUsageDurations["critical"]) / float64(totalDuration) * 100

	// Add summary table
	tableRows := []alerts.TableRow{
		{Label: "Reporting Period", Value: fmt.Sprintf("Last %d hours", int(s.reportingInterval.Hours()))},
		{Label: "Warning Events", Value: fmt.Sprintf("%d", s.warningEvents)},
		{Label: "Critical Events", Value: fmt.Sprintf("%d", s.criticalEvents)},
		{Label: "Peak CPU Usage", Value: fmt.Sprintf("%.2f%%", s.peakCPUUsage)},
		{Label: "Time in Normal State", Value: fmt.Sprintf("%.1f%% (%.1f hours)", normalPct, float64(normalDuration.Hours()))},
		{Label: "Time in Warning State", Value: fmt.Sprintf("%.1f%% (%.1f hours)", warningPct, float64(s.highUsageDurations["warning"].Hours()))},
		{Label: "Time in Critical State", Value: fmt.Sprintf("%.1f%% (%.1f hours)", criticalPct, float64(s.highUsageDurations["critical"].Hours()))},
	}

	// Add trend information if available
	trend, percentChange := s.monitor.getCPUTrend()
	tableRows = append(tableRows, alerts.TableRow{
		Label: "CPU Usage Trend",
		Value: fmt.Sprintf("%s (%.1f%% change)", trend, percentChange),
	})

	// Add current CPU status
	if currentInfo := s.monitor.GetLastCPUInfo(); currentInfo != nil {
		tableRows = append(tableRows, alerts.TableRow{
			Label: "Current CPU Status",
			Value: fmt.Sprintf("%s (%.2f%%)", currentInfo.CPUStatus, currentInfo.Usage),
		})
	}

	tableHTML := alerts.CreateTable(tableRows)

	// Generate HTML using the normal style
	styles := alerts.DefaultStyles()
	style := styles[alerts.AlertTypeNormal]

	// Add recommendation based on collected data
	var recommendation string
	if s.criticalEvents > 5 || criticalPct > 10 {
		recommendation = `
		<div style="background-color: #f2dede; border-left: 5px solid #d9534f; padding: 10px; margin: 10px 0;">
			<p><b>URGENT ACTION RECOMMENDED:</b> Multiple critical CPU events detected or significant time spent in critical state.</p>
			<p><b>Recommendations:</b></p>
			<ul>
				<li>Investigate high CPU usage causes using tools like top, htop, or ps</li>
				<li>Consider upgrading CPU resources if consistently overloaded</li>
				<li>Optimize resource-intensive applications</li>
				<li>Review and adjust scheduled tasks to balance system load</li>
				<li>Check for runaway processes or malware that might be consuming resources</li>
			</ul>
		</div>`
	} else if s.warningEvents > 10 || warningPct > 20 {
		recommendation = `
		<div style="background-color: #fcf8e3; border-left: 5px solid #faebcc; padding: 10px; margin: 10px 0;">
			<p><b>ACTION RECOMMENDED:</b> Frequent CPU warnings detected or significant time spent in warning state.</p>
			<p><b>Recommendations:</b></p>
			<ul>
				<li>Monitor CPU-intensive applications</li>
				<li>Review application performance and consider optimization</li>
				<li>Check if CPU usage pattern correlates with specific scheduled jobs</li>
				<li>Consider load balancing or distributing workloads if applicable</li>
			</ul>
		</div>`
	} else {
		recommendation = `
		<div style="background-color: #dff0d8; border-left: 5px solid #3c763d; padding: 10px; margin: 10px 0;">
			<p><b>SYSTEM HEALTHY:</b> CPU usage appears to be within normal parameters.</p>
			<p>The system has maintained healthy CPU usage levels during the reporting period.</p>
		</div>`
	}

	// Get information about frequently used processes if available
	processInfo := ""
	if currentInfo := s.monitor.GetLastCPUInfo(); currentInfo != nil {
		topProcs := getTopCPUProcesses(5)
		if len(topProcs) > 0 {
			procList := "<ul>\n"
			for _, proc := range topProcs {
				procList += fmt.Sprintf("<li>%s</li>\n", proc)
			}
			procList += "</ul>"

			processInfo = fmt.Sprintf(`
			<div style="background-color: #f5f5f5; border-left: 5px solid #5bc0de; padding: 10px; margin: 10px 0;">
				<p><b>TOP CPU CONSUMERS:</b> The following processes are currently using the most CPU:</p>
				%s
				<p><i>Note: This represents a snapshot at report generation time and may change.</i></p>
			</div>`, procList)
		}
	}

	message := alerts.CreateAlertHTML(
		alerts.AlertTypeNormal,
		style,
		"CPU USAGE SUMMARY REPORT",
		false,
		tableHTML,
		serverInfo,
		"<p>This is an automated summary of CPU usage activity during the reporting period.</p>"+recommendation+processInfo,
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
	s.highUsageDurations = make(map[string]time.Duration)
}
