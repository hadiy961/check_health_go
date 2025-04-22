package cpu

import (
	"CheckHealthDO/internal/alerts"
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"fmt"
	"strings"
	"time"

	"github.com/shirou/gopsutil/load"
)

// AlertHandler handles CPU alerts
type AlertHandler struct {
	monitor               *Monitor
	handler               *alerts.Handler
	lastWarningAlertTime  time.Time
	lastCriticalAlertTime time.Time
	lastNormalAlertTime   time.Time
	warningCount          int           // Track consecutive warnings
	warningEscalation     int           // Number of warnings before escalating
	pendingWarnings       []CPUInfo     // For collecting multiple warnings
	aggregationInterval   time.Duration // How long to collect alerts before sending
	lastAggregationTime   time.Time     // When we last sent an aggregated alert
	// Add new anti-spam fields similar to memory monitor
	warningThrottleWindow time.Duration // Only send one warning per this window
	criticalThrottleCount int           // Send critical alerts only after this many consecutive critical events
	currentCriticalCount  int           // Counter for current consecutive critical events
	maxWarningsPerDay     int           // Maximum number of warning emails per day
	warningsSentToday     int           // Counter for warnings sent today
	lastDayReset          time.Time     // When we last reset the daily counter
	lastInfo              *CPUInfo      // Last CPU info for comparison
}

// NewAlertHandler creates a new alert handler
func NewAlertHandler(monitor *Monitor) *AlertHandler {
	// Get config to read throttling settings
	cfg := monitor.GetConfigPtr()

	// Set defaults
	criticalThrottleCount := 3
	warningEscalation := 5
	maxWarningsPerDay := 5
	aggregationInterval := 5 * time.Minute
	warningThrottleWindow := 30 * time.Minute

	// Use config values if available
	if cfg != nil && cfg.Notifications.Throttling.Enabled {
		if cfg.Notifications.Throttling.CriticalThreshold > 0 {
			criticalThrottleCount = cfg.Notifications.Throttling.CriticalThreshold
		}
		if cfg.Notifications.Throttling.MaxWarningsPerDay > 0 {
			maxWarningsPerDay = cfg.Notifications.Throttling.MaxWarningsPerDay
		}
		if cfg.Notifications.Throttling.AggregationPeriod > 0 {
			aggregationInterval = time.Duration(cfg.Notifications.Throttling.AggregationPeriod) * time.Minute
		}
	}

	return &AlertHandler{
		monitor:              monitor,
		handler:              alerts.NewHandler(monitor, nil),
		lastWarningAlertTime: time.Time{},
		warningCount:         0,
		warningEscalation:    warningEscalation, // Only notify after consecutive warnings
		pendingWarnings:      make([]CPUInfo, 0),
		aggregationInterval:  aggregationInterval, // Use from config
		lastAggregationTime:  time.Now(),
		// Anti-spam settings
		warningThrottleWindow: warningThrottleWindow,
		criticalThrottleCount: criticalThrottleCount, // Use from config
		currentCriticalCount:  0,
		maxWarningsPerDay:     maxWarningsPerDay, // Use from config
		warningsSentToday:     0,
		lastDayReset:          time.Now(),
		lastInfo:              nil,
	}
}

// HandleWarningAlert handles warning level CPU alerts
func (a *AlertHandler) HandleWarningAlert(info *CPUInfo, statusChanged bool) {
	var counter *int = &a.handler.SuppressedWarningCount

	// Check if we need to reset the daily counter
	now := time.Now()
	if now.YearDay() != a.lastDayReset.YearDay() || now.Year() != a.lastDayReset.Year() {
		a.warningsSentToday = 0
		a.lastDayReset = now
	}

	// Get config with proper type assertion to determine cooldown
	configInterface := a.monitor.GetConfig()
	cfg, ok := configInterface.(*config.Config)

	// Default cooldown of 5 minutes if can't get config
	cooldownPeriod := 300
	if ok && cfg.Notifications.Throttling.Enabled {
		cooldownPeriod = cfg.Notifications.Throttling.CooldownPeriod
	}

	// Log the attempted alert regardless of whether it's throttled
	if statusChanged {
		logger.Info("CPU entered warning state",
			logger.Float64("usage_percent", info.Usage),
			logger.String("status", info.CPUStatus),
			logger.String("timestamp", time.Now().Format(time.RFC3339)),
			logger.Bool("notification_will_be_sent", time.Since(a.lastWarningAlertTime) >= time.Duration(cooldownPeriod)*time.Second))
	}

	// Reset critical counter when we get a warning
	a.currentCriticalCount = 0

	// If status changed from critical to warning, handle it differently
	if statusChanged && a.lastInfo != nil && a.lastInfo.CPUStatus == "critical" {
		// This is an improvement, just log it but don't send notification to reduce spam
		logger.Info("CPU improved from critical to warning state",
			logger.Float64("usage_percent", info.Usage))
		a.lastInfo = info
		return
	}

	// Throttle based on our custom window - only one warning alert per warningThrottleWindow
	if !a.lastWarningAlertTime.IsZero() && time.Since(a.lastWarningAlertTime) < a.warningThrottleWindow {
		logger.Debug("Suppressing CPU warning notification due to throttle window",
			logger.Int("minutes_since_last", int(time.Since(a.lastWarningAlertTime).Minutes())),
			logger.Int("throttle_window_minutes", int(a.warningThrottleWindow.Minutes())))
		*counter++
		return
	}

	// Enforce daily maximum
	if a.warningsSentToday >= a.maxWarningsPerDay {
		logger.Info("Daily CPU warning notification limit reached",
			logger.Int("max_warnings_per_day", a.maxWarningsPerDay),
			logger.Int("warnings_sent_today", a.warningsSentToday))
		return
	}

	// If status changed, send immediately (unless already handled above)
	if statusChanged {
		// Reset counter on status change
		a.warningCount = 0
		a.lastInfo = info

		// Send normal notification for status changes
		a.sendWarningNotification(info, statusChanged, "")
		return
	}

	// For non-status change warnings, use escalation
	a.warningCount++
	a.lastInfo = info

	// Collect for aggregation
	a.pendingWarnings = append(a.pendingWarnings, *info)

	// Only send aggregated alert if enough time has passed
	if time.Since(a.lastAggregationTime) >= a.aggregationInterval && len(a.pendingWarnings) > 0 {
		// Create an aggregated message
		a.sendAggregatedWarningAlert()
		return
	}

	// Only send notification if we hit escalation threshold
	if a.warningCount < a.warningEscalation {
		logger.Debug("CPU warning suppressed due to escalation policy",
			logger.Int("warning_count", a.warningCount),
			logger.Int("escalation_threshold", a.warningEscalation))
		return
	}

	// Add escalation information to the alert
	escalationNote := fmt.Sprintf(`
	<p><b>Note:</b> This alert was sent after %d consecutive warnings.</p>`,
		a.warningCount)

	// Send the alert with escalation information
	a.sendWarningNotification(info, statusChanged, escalationNote)
}

// sendWarningNotification sends a warning notification for CPU issues
func (a *AlertHandler) sendWarningNotification(info *CPUInfo, statusChanged bool, additionalNote string) {
	// Increase the warning sent counter
	a.warningsSentToday++

	// For warning alert, check if we should throttle using the handler's method
	var counter *int = &a.handler.SuppressedWarningCount
	if a.handler.ShouldThrottleAlert(statusChanged, counter, alerts.AlertTypeWarning) {
		return
	}

	// Record the time we're sending this warning
	a.lastWarningAlertTime = time.Now()

	// Get server information using the common utility function
	serverInfo := alerts.GetServerInfoForAlert()

	// Create table content for CPU info
	tableContent := a.createCPUTableContent(info)

	// Base content for warning
	additionalContent := `<p><b>Recommendation:</b> Please monitor the system closely if this condition persists.</p>`

	// Add any additional note if provided
	if additionalNote != "" {
		additionalContent += additionalNote
	}

	// Add daily warning count information
	additionalContent += fmt.Sprintf(`
	<p><small>This is warning notification %d of %d allowed per day.</small></p>`,
		a.warningsSentToday, a.maxWarningsPerDay)

	// Get trend information directly from the monitor
	trend, percentChange := a.monitor.getCPUTrend()

	// Customize additional content based on trend
	if strings.Contains(trend, "increasing") {
		trendHTML := fmt.Sprintf(`
		<div style="background-color: #fcf8e3; border-left: 5px solid #faebcc; padding: 10px; margin: 10px 0;">
			<p><b>TREND ALERT:</b> CPU usage is %s (%.1f%% change over monitoring period).</p>
			<p>This suggests increasing system load that may require investigation.</p>
			<p><b>Possible causes:</b></p>
			<ul>
				<li>Running resource-intensive applications</li>
				<li>Background processes consuming CPU</li>
				<li>Scheduled tasks like backups or indexing</li>
				<li>Runaway processes or potential malware</li>
			</ul>
		</div>`, trend, percentChange)
		additionalContent += trendHTML
	}

	// Add system load information if available
	if loadAvg, err := getSystemLoadAvg(); err == nil && len(loadAvg) >= 3 {
		additionalContent += fmt.Sprintf(`
		<div style="background-color: #f5f5f5; border-left: 5px solid #ddd; padding: 10px; margin: 10px 0;">
			<p><b>SYSTEM LOAD AVERAGE:</b> 1-min: %.2f, 5-min: %.2f, 15-min: %.2f</p>
			<p>System load indicates the number of processes waiting for CPU time.</p>
		</div>`, loadAvg[0], loadAvg[1], loadAvg[2])
	}

	// Get style for this alert type
	style := a.handler.GetAlertStyle(alerts.AlertTypeWarning)

	// Generate HTML
	message := alerts.CreateAlertHTML(
		alerts.AlertTypeWarning,
		style,
		"CPU WARNING ALERT",
		statusChanged,
		tableContent,
		serverInfo,
		additionalContent,
	)

	// Send notification
	a.handler.SendNotifications("CPU Warning", message, "warning")
	a.monitor.UpdateLastAlertTime()

	// Reset warning count after sending
	a.warningCount = 0

	logger.Info("Sent CPU warning notification",
		logger.Float64("usage_percent", info.Usage),
		logger.Int("warnings_sent_today", a.warningsSentToday),
		logger.Int("max_per_day", a.maxWarningsPerDay))
}

// HandleCriticalAlert handles critical level CPU alerts
func (a *AlertHandler) HandleCriticalAlert(info *CPUInfo, statusChanged bool) {
	// Increment critical event counter
	a.currentCriticalCount++

	var counter *int = &a.handler.SuppressedCriticalCount

	// Get config with proper type assertion to determine cooldown
	configInterface := a.monitor.GetConfig()
	cfg, ok := configInterface.(*config.Config)

	// Default cooldown of 5 minutes if can't get config
	cooldownPeriod := 300
	if ok && cfg.Notifications.Throttling.Enabled {
		cooldownPeriod = cfg.Notifications.Throttling.CooldownPeriod
	}

	// Log the attempted alert regardless of whether it's throttled
	if statusChanged {
		logger.Info("CPU entered critical state",
			logger.Float64("usage_percent", info.Usage),
			logger.String("status", info.CPUStatus),
			logger.String("timestamp", time.Now().Format(time.RFC3339)),
			logger.Int("consecutive_critical_events", a.currentCriticalCount),
			logger.Int("threshold_for_alert", a.criticalThrottleCount),
			logger.Bool("notification_will_be_sent", time.Since(a.lastCriticalAlertTime) >= time.Duration(cooldownPeriod)*time.Second))
	}

	// Store current info for comparison in next cycle
	a.lastInfo = info

	// For critical events, require consecutive occurrences before alerting
	// Unless this is a status change from normal directly to critical
	if !statusChanged && a.currentCriticalCount < a.criticalThrottleCount {
		logger.Info("Suppressing CPU critical alert until threshold reached",
			logger.Int("current_count", a.currentCriticalCount),
			logger.Int("threshold", a.criticalThrottleCount))
		return
	}

	// Apply throttling even for status changes
	if !a.lastCriticalAlertTime.IsZero() {
		sinceLastCritical := time.Since(a.lastCriticalAlertTime)
		if sinceLastCritical < time.Duration(cooldownPeriod)*time.Second {
			logger.Debug("Suppressing CPU critical notification due to cooldown",
				logger.Int("seconds_since_last", int(sinceLastCritical.Seconds())),
				logger.Int("cooldown_period", cooldownPeriod))
			*counter++
			return
		}
	}

	// Get server information using the common utility function
	serverInfo := alerts.GetServerInfoForAlert()

	// Create table content for CPU info
	tableContent := a.createCPUTableContent(info)

	// Additional content for critical with detailed recommendation
	additionalContent := `
	<div style="background-color: #d9534f; color: white; padding: 10px; text-align: center; margin: 20px 0;">
		<h3>IMMEDIATE ACTION REQUIRED!</h3>
	</div>
	
	<div style="background-color: #f2dede; border-left: 5px solid #d9534f; padding: 10px; margin: 10px 0;">
		<p><b>Recommended Actions:</b></p>
		<ol>
			<li>Identify CPU-intensive processes using 'top' or 'htop' commands</li>
			<li>Check for runaway processes that may need to be terminated</li>
			<li>If appropriate, restart resource-intensive services</li>
			<li>Verify that system cooling is functioning properly</li>
			<li>Check for recent system changes that may have caused this spike</li>
		</ol>
	</div>`

	// Add load average information
	if loadAvg, err := getSystemLoadAvg(); err == nil && len(loadAvg) >= 3 {
		criticalLoad := false
		processorCount := info.ProcessorCount
		if processorCount == 0 {
			processorCount = info.Cores
		}

		// Load is critical if 1-minute average exceeds number of cores
		if loadAvg[0] > float64(processorCount) {
			criticalLoad = true
		}

		loadStatus := "NORMAL"
		loadColor := "#5cb85c" // green
		if criticalLoad {
			loadStatus = "CRITICAL"
			loadColor = "#d9534f" // red
		} else if loadAvg[0] > float64(processorCount)*0.7 {
			loadStatus = "WARNING"
			loadColor = "#f0ad4e" // yellow
		}

		additionalContent += fmt.Sprintf(`
		<div style="background-color: #f2dede; border-left: 5px solid %s; padding: 10px; margin: 10px 0;">
			<p><b>SYSTEM LOAD:</b> 1-min: %.2f, 5-min: %.2f, 15-min: %.2f <span style="color: %s; font-weight: bold;">(%s)</span></p>
			<p>Load average above processor count (%d) indicates CPU saturation and performance degradation.</p>
		</div>`, loadColor, loadAvg[0], loadAvg[1], loadAvg[2], loadColor, loadStatus, processorCount)
	}

	// Get style for this alert type
	style := a.handler.GetAlertStyle(alerts.AlertTypeCritical)

	// Generate HTML
	message := alerts.CreateAlertHTML(
		alerts.AlertTypeCritical,
		style,
		"CRITICAL CPU ALERT",
		statusChanged,
		tableContent,
		serverInfo,
		additionalContent,
	)

	// Send notification
	a.handler.SendNotifications("CRITICAL CPU Alert", message, "critical")
	a.monitor.UpdateLastAlertTime()
	a.lastCriticalAlertTime = time.Now()

	// Reset the counter after alert is sent
	a.currentCriticalCount = 0

	logger.Info("Sent critical CPU alert",
		logger.Float64("usage_percent", info.Usage))
}

// HandleNormalAlert handles notifications when CPU returns to normal state
func (a *AlertHandler) HandleNormalAlert(info *CPUInfo, statusChanged bool) {
	// Reset counters when returning to normal
	a.warningCount = 0
	a.currentCriticalCount = 0

	// Store current info for comparison in next cycle
	a.lastInfo = info

	// Only send notification if the status has changed from critical to normal
	// Don't send notifications for warning->normal transitions to reduce spam
	if !statusChanged || (a.lastInfo != nil && a.lastInfo.CPUStatus != "critical") {
		return
	}

	// Get config with proper type assertion to determine cooldown
	configInterface := a.monitor.GetConfig()
	cfg, ok := configInterface.(*config.Config)

	// Default cooldown of 5 minutes if can't get config
	cooldownPeriod := 300
	if ok && cfg.Notifications.Throttling.Enabled {
		cooldownPeriod = cfg.Notifications.Throttling.CooldownPeriod
	}

	// Log the return to normal regardless of whether notification is throttled
	logger.Info("CPU returned to normal state",
		logger.Float64("usage_percent", info.Usage),
		logger.String("status", info.CPUStatus),
		logger.String("timestamp", time.Now().Format(time.RFC3339)),
		logger.Bool("notification_will_be_sent", time.Since(a.lastNormalAlertTime) >= time.Duration(cooldownPeriod)*time.Second))

	// Apply throttling even for normal status
	if !a.lastNormalAlertTime.IsZero() {
		sinceLastNormal := time.Since(a.lastNormalAlertTime)
		if sinceLastNormal < time.Duration(cooldownPeriod)*time.Second {
			logger.Debug("Suppressing CPU normal notification due to cooldown",
				logger.Int("seconds_since_last", int(sinceLastNormal.Seconds())),
				logger.Int("cooldown_period", cooldownPeriod))
			return
		}
	}

	// Get server information using the common utility function
	serverInfo := alerts.GetServerInfoForAlert()

	// Create table content for CPU info
	tableContent := a.createCPUTableContent(info)

	// Additional content for normal
	additionalContent := `
	<div style="background-color: #dff0d8; color: #3c763d; padding: 10px; margin: 20px 0; text-align: center; border-radius: 5px;">
		<p>System CPU usage has returned to normal parameters.</p>
	</div>
	
	<div style="background-color: #f5f5f5; border-left: 5px solid #5cb85c; padding: 10px; margin: 10px 0;">
		<p><b>RESOLUTION SUMMARY:</b></p>
		<p>The CPU usage has stabilized. If you took any actions to reduce the load, they appear to have been successful.</p>
		<p>Continue monitoring the system for any recurring issues.</p>
	</div>`

	// Get style for this alert type
	style := a.handler.GetAlertStyle(alerts.AlertTypeNormal)

	// Generate HTML
	message := alerts.CreateAlertHTML(
		alerts.AlertTypeNormal,
		style,
		"CPU STATUS NORMALIZED",
		statusChanged,
		tableContent,
		serverInfo,
		additionalContent,
	)

	// Send notification
	a.handler.SendNotifications("CPU Status Normalized", message, "info")
	a.monitor.UpdateLastAlertTime()
	a.lastNormalAlertTime = time.Now()

	logger.Info("Sent CPU normalized notification",
		logger.Float64("usage_percent", info.Usage))
}

// Helper method to create CPU-specific table content
func (a *AlertHandler) createCPUTableContent(info *CPUInfo) string {
	// Get style for alert
	style := a.handler.GetAlertStyle(alerts.AlertTypeWarning)

	// Create status line
	statusLine := alerts.CreateStatusLine(
		style.StatusColorClass,
		style.StatusText,
	)

	// Create table rows
	tableRows := []alerts.TableRow{
		{Label: "Usage Percentage", Value: fmt.Sprintf("%.2f%%", info.Usage)},
		{Label: "Model", Value: info.ModelName},
		{Label: "Cores", Value: fmt.Sprintf("%d", info.Cores)},
		{Label: "Threads", Value: fmt.Sprintf("%d", info.Threads)},
	}

	// Add processor count if available
	if info.ProcessorCount > 0 {
		tableRows = append(tableRows, alerts.TableRow{
			Label: "Physical CPUs",
			Value: fmt.Sprintf("%d", info.ProcessorCount),
		})
	}

	// Add clock speed information
	tableRows = append(tableRows, alerts.TableRow{
		Label: "Clock Speed",
		Value: fmt.Sprintf("%.2f GHz", info.ClockSpeed),
	})

	// Add temperature if available
	if info.Temperature > 0 {
		tempStatus := "Normal"
		if info.Temperature > 85 {
			tempStatus = "Critical"
		} else if info.Temperature > 75 {
			tempStatus = "Warning"
		}

		tableRows = append(tableRows, alerts.TableRow{
			Label: "Temperature",
			Value: fmt.Sprintf("%.1f°C (%s)", info.Temperature, tempStatus),
		})
	}

	// Add CPU time distribution
	userPct := 0.0
	sysPct := 0.0
	idlePct := 0.0

	if times, ok := info.CPUTimes["user"]; ok {
		userPct = times
	}
	if times, ok := info.CPUTimes["system"]; ok {
		sysPct = times
	}
	if times, ok := info.CPUTimes["idle"]; ok {
		idlePct = times
	}

	tableRows = append(tableRows, alerts.TableRow{
		Label: "Usage Distribution",
		Value: fmt.Sprintf("User: %.1f%%, System: %.1f%%, Idle: %.1f%%", userPct, sysPct, idlePct),
	})

	// Add trend information directly from the monitor
	trend, percentChange := a.monitor.getCPUTrend()
	tableRows = append(tableRows, alerts.TableRow{
		Label: "CPU Trend",
		Value: fmt.Sprintf("%s (%.1f%% change)", trend, percentChange),
	})

	// Create the table HTML
	tableHTML := alerts.CreateTable(tableRows)

	// Return the complete content
	return statusLine + tableHTML
}

// sendAggregatedWarningAlert sends a single warning alert that summarizes multiple warnings
func (a *AlertHandler) sendAggregatedWarningAlert() {
	if len(a.pendingWarnings) == 0 {
		return
	}

	// Increase the warning sent counter
	a.warningsSentToday++

	// Find highest CPU usage from collected warnings
	highestUsage := float64(0)
	var worstCPUInfo *CPUInfo
	for i, info := range a.pendingWarnings {
		if info.Usage > highestUsage {
			highestUsage = info.Usage
			worstCPUInfo = &a.pendingWarnings[i]
		}
	}

	// Create a summary message
	additionalContent := fmt.Sprintf(`
    <p><b>Aggregated Warning:</b> %d CPU warnings detected in the last %d minutes.</p>
    <p>Highest CPU usage was %.2f%%</p>
    <p><b>Recommendation:</b> Please monitor the system closely if this condition persists.</p>
	<p><small>This is warning notification %d of %d allowed per day.</small></p>
    
    <div style="background-color: #fcf8e3; border-left: 5px solid #faebcc; padding: 10px; margin: 10px 0;">
        <p><b>TREND SUMMARY:</b> Persistent CPU warnings may indicate:</p>
        <ul>
            <li>Undersized infrastructure for current workload</li>
            <li>Application optimization opportunities</li>
            <li>Background processes consuming resources</li>
            <li>Potential need for workload distribution or scaling</li>
        </ul>
    </div>`,
		len(a.pendingWarnings),
		int(a.aggregationInterval.Minutes()),
		highestUsage,
		a.warningsSentToday,
		a.maxWarningsPerDay)

	// Get server information using the common utility function
	serverInfo := alerts.GetServerInfoForAlert()

	// Create table content for CPU info
	tableContent := a.createCPUTableContent(worstCPUInfo)

	// Get style for this alert type
	style := a.handler.GetAlertStyle(alerts.AlertTypeWarning)

	// Generate HTML
	message := alerts.CreateAlertHTML(
		alerts.AlertTypeWarning,
		style,
		"AGGREGATED CPU WARNING ALERT",
		false,
		tableContent,
		serverInfo,
		additionalContent,
	)

	// Send notification
	a.handler.SendNotifications("CPU Warning Summary", message, "warning")
	a.monitor.UpdateLastAlertTime()

	// Update tracking state
	a.lastAggregationTime = time.Now()
	a.pendingWarnings = make([]CPUInfo, 0) // Clear pending warnings

	// Record the time we're sending this warning
	a.lastWarningAlertTime = time.Now()

	// Reset warning count after sending aggregated alert
	a.warningCount = 0

	logger.Info("Sent aggregated CPU warning notification",
		logger.Int("warning_count", len(a.pendingWarnings)),
		logger.Float64("highest_usage", highestUsage),
		logger.Int("warnings_sent_today", a.warningsSentToday),
		logger.Int("max_per_day", a.maxWarningsPerDay))
}

// Helper function to get system load average
func getSystemLoadAvg() ([]float64, error) {
	loadInfo, err := load.Avg()
	if err != nil {
		return nil, err
	}
	return []float64{loadInfo.Load1, loadInfo.Load5, loadInfo.Load15}, nil
}
