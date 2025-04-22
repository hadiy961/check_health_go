package memory

import (
	"CheckHealthDO/internal/alerts"
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"CheckHealthDO/internal/services/mariadb"
	"fmt"
	"strings"
	"time"

	"github.com/shirou/gopsutil/load"
)

// AlertHandler handles memory alerts
type AlertHandler struct {
	monitor               *Monitor
	handler               *alerts.Handler
	lastWarningAlertTime  time.Time
	lastCriticalAlertTime time.Time
	lastNormalAlertTime   time.Time
	warningCount          int           // Track consecutive warnings
	warningEscalation     int           // Number of warnings before escalating
	pendingWarnings       []MemoryInfo  // For collecting multiple warnings
	aggregationInterval   time.Duration // How long to collect alerts before sending
	lastAggregationTime   time.Time     // When we last sent an aggregated alert
	// Add additional fields for advanced throttling
	warningThrottleWindow time.Duration // Only send one warning per this window
	criticalThrottleCount int           // Send critical alerts only after this many consecutive critical events
	currentCriticalCount  int           // Counter for current consecutive critical events
	maxWarningsPerDay     int           // Maximum number of warning emails per day
	warningsSentToday     int           // Counter for warnings sent today
	lastDayReset          time.Time     // When we last reset the daily counter
	lastInfo              *MemoryInfo   // Last memory info for comparison
}

// NewAlertHandler creates a new alert handler
func NewAlertHandler(monitor *Monitor) *AlertHandler {
	// Get config to read throttling settings
	cfg := monitor.GetConfigPtr()

	// Set defaults
	criticalThrottleCount := 3
	warningEscalation := 10
	maxWarningsPerDay := 5
	aggregationInterval := 15 * time.Minute
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
		pendingWarnings:      make([]MemoryInfo, 0),
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

// HandleWarningAlert handles warning level memory alerts
func (a *AlertHandler) HandleWarningAlert(info *MemoryInfo, statusChanged bool) {
	var counter *int = &a.handler.SuppressedWarningCount

	// Check if we need to reset the daily counter
	now := time.Now()
	if now.YearDay() != a.lastDayReset.YearDay() || now.Year() != a.lastDayReset.Year() {
		a.warningsSentToday = 0
		a.lastDayReset = now
	}

	// Always log the event regardless of whether notification is throttled
	if statusChanged {
		logger.Info("Memory entered warning state",
			logger.Float64("usage_percent", info.UsedMemoryPercentage),
			logger.String("status", info.MemoryStatus),
			logger.String("timestamp", time.Now().Format(time.RFC3339)),
			logger.Bool("notification_will_be_sent", false))
	}

	// Reset critical counter when we get a warning
	a.currentCriticalCount = 0

	// If status changed from critical to warning, handle it differently
	if statusChanged && a.lastInfo != nil && a.lastInfo.MemoryStatus == "critical" {
		// This is an improvement, just log it but don't send notification
		logger.Info("Memory improved from critical to warning state",
			logger.Float64("usage_percent", info.UsedMemoryPercentage))
		return
	}

	// Throttle based on our custom window - only one warning alert per warningThrottleWindow
	if !a.lastWarningAlertTime.IsZero() && time.Since(a.lastWarningAlertTime) < a.warningThrottleWindow {
		logger.Debug("Suppressing memory warning notification due to throttle window",
			logger.Int("minutes_since_last", int(time.Since(a.lastWarningAlertTime).Minutes())),
			logger.Int("throttle_window_minutes", int(a.warningThrottleWindow.Minutes())))
		*counter++
		return
	}

	// Enforce daily maximum
	if a.warningsSentToday >= a.maxWarningsPerDay {
		logger.Info("Daily warning notification limit reached",
			logger.Int("max_warnings_per_day", a.maxWarningsPerDay),
			logger.Int("warnings_sent_today", a.warningsSentToday))
		return
	}

	// For non-status change warnings, use escalation
	if !statusChanged {
		a.warningCount++

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
			logger.Debug("Memory warning suppressed due to escalation policy",
				logger.Int("warning_count", a.warningCount),
				logger.Int("escalation_threshold", a.warningEscalation))
			return
		}
	}

	// Add escalation information to the alert
	escalationNote := fmt.Sprintf(`
	<p><b>Note:</b> This alert was sent after %d consecutive warnings.</p>`,
		a.warningCount)

	// Store the info for later comparison
	a.lastInfo = info

	// Send the alert with escalation information
	a.sendWarningNotification(info, statusChanged, escalationNote)
}

// sendWarningNotification sends a warning notification for memory issues
func (a *AlertHandler) sendWarningNotification(info *MemoryInfo, statusChanged bool, additionalNote string) {
	// Increase the warning sent counter
	a.warningsSentToday++

	// Record the time we're sending this warning
	a.lastWarningAlertTime = time.Now()

	// Get server information using the common utility function
	serverInfo := alerts.GetServerInfoForAlert()

	// Create table content for memory info
	tableContent := a.createMemoryTableContent(info)

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
	trend, percentChange := a.monitor.getMemoryTrend()

	// Customize additional content based on trend
	if strings.Contains(trend, "increasing") {
		trendHTML := fmt.Sprintf(`
		<div style="background-color: #fcf8e3; border-left: 5px solid #faebcc; padding: 10px; margin: 10px 0;">
			<p><b>TREND ALERT:</b> Memory usage is %s (%.1f%% change over monitoring period).</p>
			<p>This suggests a potential memory leak or growing resource usage that may require investigation.</p>
		</div>`, trend, percentChange)
		additionalContent += trendHTML
	}

	// Get style for this alert type
	style := a.handler.GetAlertStyle(alerts.AlertTypeWarning)

	// Generate HTML
	message := alerts.CreateAlertHTML(
		alerts.AlertTypeWarning,
		style,
		"MEMORY WARNING ALERT",
		statusChanged,
		tableContent,
		serverInfo,
		additionalContent,
	)

	// Send notification
	a.handler.SendNotifications("Memory Warning", message, "warning")
	a.monitor.UpdateLastAlertTime()

	// Reset warning count after sending
	a.warningCount = 0

	logger.Info("Sent memory warning notification",
		logger.Float64("usage_percent", info.UsedMemoryPercentage),
		logger.Int("warnings_sent_today", a.warningsSentToday),
		logger.Int("max_per_day", a.maxWarningsPerDay))
}

// HandleCriticalAlert handles critical level memory alerts
func (a *AlertHandler) HandleCriticalAlert(info *MemoryInfo, statusChanged bool) {
	// Increment critical event counter
	a.currentCriticalCount++

	// Get config with proper type assertion to determine cooldown
	configInterface := a.monitor.GetConfig()
	cfg, ok := configInterface.(*config.Config)

	// Default cooldown of 5 minutes if can't get config
	cooldownPeriod := 300
	if ok && cfg.Notifications.Throttling.Enabled {
		cooldownPeriod = cfg.Notifications.Throttling.CooldownPeriod
	}

	// Always log the event regardless of whether notification is throttled
	if statusChanged {
		logger.Info("Memory entered critical state",
			logger.Float64("usage_percent", info.UsedMemoryPercentage),
			logger.String("status", info.MemoryStatus),
			logger.String("timestamp", time.Now().Format(time.RFC3339)),
			logger.Int("consecutive_critical_events", a.currentCriticalCount),
			logger.Int("threshold_for_alert", a.criticalThrottleCount))
	}

	// Store current info for comparison in next cycle
	a.lastInfo = info

	// For critical events, require consecutive occurrences before alerting
	// Unless this is a status change from normal directly to critical
	if !statusChanged && a.currentCriticalCount < a.criticalThrottleCount {
		logger.Info("Suppressing critical alert until threshold reached",
			logger.Int("current_count", a.currentCriticalCount),
			logger.Int("threshold", a.criticalThrottleCount))
		return
	}

	// Apply throttling even for status changes
	if !a.lastCriticalAlertTime.IsZero() {
		sinceLastCritical := time.Since(a.lastCriticalAlertTime)
		if sinceLastCritical < time.Duration(cooldownPeriod)*time.Second {
			logger.Debug("Suppressing memory critical notification due to cooldown",
				logger.Int("seconds_since_last", int(sinceLastCritical.Seconds())),
				logger.Int("cooldown_period", cooldownPeriod))
			return
		}
	}

	// Get server information using the common utility function
	serverInfo := alerts.GetServerInfoForAlert()

	// Create table content for memory info
	tableContent := a.createMemoryTableContent(info)

	// Get config with proper type assertion
	if !ok {
		logger.Error("Failed to convert config to *config.Config in HandleCriticalAlert")
		// Use a default notification without custom recovery actions
		additionalContent := `
		<div style="background-color: #d9534f; color: white; padding: 10px; text-align: center; margin: 20px 0;">
			<h3>IMMEDIATE ACTION REQUIRED!</h3>
		</div>`

		// Get style for this alert type
		style := a.handler.GetAlertStyle(alerts.AlertTypeCritical)

		// Generate HTML
		message := alerts.CreateAlertHTML(
			alerts.AlertTypeCritical,
			style,
			"CRITICAL MEMORY ALERT",
			statusChanged,
			tableContent,
			serverInfo,
			additionalContent,
		)

		// Send notification
		a.handler.SendNotifications("CRITICAL Memory Alert", message, "critical")
		a.monitor.UpdateLastAlertTime()
		a.lastCriticalAlertTime = time.Now()
		return
	}

	// Prepare additional content for critical alerts
	additionalContent := `
	<div style="background-color: #d9534f; color: white; padding: 10px; text-align: center; margin: 20px 0;">
		<h3>IMMEDIATE ACTION REQUIRED!</h3>
	</div>`

	// Add recovery action message if MariaDB auto-restart is enabled
	if cfg.Monitoring.MariaDB.Enabled && cfg.Monitoring.MariaDB.RestartOnThreshold.Enabled {
		additionalContent += `
		<div style="background-color: #5bc0de; padding: 10px; margin: 20px 0; border-radius: 5px;">
			<h3 style="color: white; background-color: #31b0d5; padding: 5px; margin-top: 0;">AUTOMATIC RECOVERY ACTION</h3>
			<p>The system is attempting to automatically restart the MariaDB service 
			to free up memory resources and maintain stability.</p>
		</div>`
	}

	// Add system load information if available
	if loadAvg, err := getSystemLoadAvg(); err == nil && len(loadAvg) >= 3 {
		additionalContent += fmt.Sprintf(`
		<div style="background-color: #f2dede; border-left: 5px solid #d9534f; padding: 10px; margin: 10px 0;">
			<p><b>SYSTEM LOAD:</b> 1-min: %.2f, 5-min: %.2f, 15-min: %.2f</p>
			<p>This indicates the overall system pressure and may help diagnose the memory issue.</p>
		</div>`, loadAvg[0], loadAvg[1], loadAvg[2])
	}

	// Get style for this alert type
	style := a.handler.GetAlertStyle(alerts.AlertTypeCritical)

	// Generate HTML
	message := alerts.CreateAlertHTML(
		alerts.AlertTypeCritical,
		style,
		"CRITICAL MEMORY ALERT",
		statusChanged,
		tableContent,
		serverInfo,
		additionalContent,
	)

	// Send notification
	a.handler.SendNotifications("CRITICAL Memory Alert", message, "critical")
	a.monitor.UpdateLastAlertTime()

	// Perform recovery actions if configured to do so
	if cfg.Monitoring.MariaDB.RestartOnThreshold.Enabled {
		a.performRecoveryActions(info)
	}

	// Update the last critical alert time
	a.lastCriticalAlertTime = time.Now()

	// Reset the counter after alert is sent
	a.currentCriticalCount = 0

	logger.Info("Sent critical memory alert",
		logger.Float64("usage_percent", info.UsedMemoryPercentage))
}

// HandleNormalAlert handles notifications when memory returns to normal state
func (a *AlertHandler) HandleNormalAlert(info *MemoryInfo, statusChanged bool) {
	// Reset counters when returning to normal
	a.warningCount = 0
	a.currentCriticalCount = 0

	// Store current info for comparison in next cycle
	a.lastInfo = info

	// Only send notification if the status has changed from critical to normal
	// Don't send notifications for warning->normal transitions to reduce spam
	if !statusChanged || (a.lastInfo != nil && a.lastInfo.MemoryStatus != "critical") {
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
	logger.Info("Memory returned to normal state",
		logger.Float64("usage_percent", info.UsedMemoryPercentage),
		logger.String("status", info.MemoryStatus),
		logger.String("timestamp", time.Now().Format(time.RFC3339)),
		logger.Bool("notification_will_be_sent", time.Since(a.lastNormalAlertTime) >= time.Duration(cooldownPeriod)*time.Second))

	// Apply throttling even for normal status
	if !a.lastNormalAlertTime.IsZero() {
		sinceLastNormal := time.Since(a.lastNormalAlertTime)
		if sinceLastNormal < time.Duration(cooldownPeriod)*time.Second {
			logger.Debug("Suppressing memory normal notification due to cooldown",
				logger.Int("seconds_since_last", int(sinceLastNormal.Seconds())),
				logger.Int("cooldown_period", cooldownPeriod))
			return
		}
	}

	// Get server information using the common utility function
	serverInfo := alerts.GetServerInfoForAlert()

	// Create table content for memory info
	tableContent := a.createMemoryTableContent(info)

	// Additional content for normal
	additionalContent := `
	<div style="background-color: #dff0d8; color: #3c763d; padding: 10px; margin: 20px 0; text-align: center; border-radius: 5px;">
		<p>System is now operating within normal parameters.</p>
	</div>`

	// Get style for this alert type
	style := a.handler.GetAlertStyle(alerts.AlertTypeNormal)

	// Generate HTML
	message := alerts.CreateAlertHTML(
		alerts.AlertTypeNormal,
		style,
		"MEMORY STATUS NORMALIZED",
		statusChanged,
		tableContent,
		serverInfo,
		additionalContent,
	)

	// Send notification
	a.handler.SendNotifications("Memory Status Normalized", message, "info")
	a.monitor.UpdateLastAlertTime()

	// Update the last normal alert time
	a.lastNormalAlertTime = time.Now()

	logger.Info("Sent memory normalized notification",
		logger.Float64("usage_percent", info.UsedMemoryPercentage))
}

// Helper method to create memory-specific table content
func (a *AlertHandler) createMemoryTableContent(info *MemoryInfo) string {
	// Get style for alert
	style := a.handler.GetAlertStyle(alerts.AlertTypeWarning)

	// Create status line
	statusLine := alerts.CreateStatusLine(
		style.StatusColorClass,
		style.StatusText,
	)

	// Convert memory values to GB for readability
	usedMemoryGB := float64(info.UsedMemory) / 1024 / 1024 / 1024
	totalMemoryGB := float64(info.TotalMemory) / 1024 / 1024 / 1024
	freeMemoryGB := float64(info.FreeMemory) / 1024 / 1024 / 1024

	// Create table rows
	tableRows := []alerts.TableRow{
		{Label: "Usage Percentage", Value: fmt.Sprintf("%.2f%%", info.UsedMemoryPercentage)},
		{Label: "Used Memory", Value: fmt.Sprintf("%.2f GB (%.2f%%)", usedMemoryGB, info.UsedMemoryPercentage)},
		{Label: "Total Memory", Value: fmt.Sprintf("%.2f GB", totalMemoryGB)},
		{Label: "Free Memory", Value: fmt.Sprintf("%.2f GB (%.2f%%)", freeMemoryGB, info.FreeMemoryPercentage)},
	}

	// Add trend information directly from the monitor
	trend, percentChange := a.monitor.getMemoryTrend()
	tableRows = append(tableRows, alerts.TableRow{
		Label: "Memory Trend",
		Value: fmt.Sprintf("%s (%.1f%% change)", trend, percentChange),
	})

	// Add any available memory info if present
	availableMemory := GetAvailableMemory()
	if availableMemory > 0 {
		availableMemoryGB := float64(availableMemory) / 1024 / 1024 / 1024
		tableRows = append(tableRows, alerts.TableRow{
			Label: "Available Memory",
			Value: fmt.Sprintf("%.2f GB (%.2f%%)", availableMemoryGB, float64(availableMemory)/float64(info.TotalMemory)*100.0),
		})
	}

	// Add cached memory information since it's often reclaimed when needed
	if info.CachedMemory > 0 {
		cachedMemoryGB := float64(info.CachedMemory) / 1024 / 1024 / 1024
		tableRows = append(tableRows, alerts.TableRow{
			Label: "Cached Memory",
			Value: fmt.Sprintf("%.2f GB", cachedMemoryGB),
		})
	}

	// Add active/inactive memory
	if info.ActiveMemory > 0 && info.InactiveMemory > 0 {
		activeMemoryGB := float64(info.ActiveMemory) / 1024 / 1024 / 1024
		inactiveMemoryGB := float64(info.InactiveMemory) / 1024 / 1024 / 1024
		tableRows = append(tableRows, alerts.TableRow{
			Label: "Active/Inactive Memory",
			Value: fmt.Sprintf("%.2f GB / %.2f GB", activeMemoryGB, inactiveMemoryGB),
		})
	}

	// Add swap information if available
	if info.SwapTotal > 0 {
		swapUsedGB := float64(info.SwapUsed) / 1024 / 1024 / 1024
		swapTotalGB := float64(info.SwapTotal) / 1024 / 1024 / 1024
		swapPercentage := 0.0
		if info.SwapTotal > 0 {
			swapPercentage = float64(info.SwapUsed) * 100.0 / float64(info.SwapTotal)
		}

		tableRows = append(tableRows, alerts.TableRow{
			Label: "Swap Usage",
			Value: fmt.Sprintf("%.2f GB / %.2f GB (%.2f%%)", swapUsedGB, swapTotalGB, swapPercentage),
		})
	}

	// Create the table HTML
	tableHTML := alerts.CreateTable(tableRows)

	// Return the complete content
	return statusLine + tableHTML
}

// performRecoveryActions takes steps to reduce memory usage
func (a *AlertHandler) performRecoveryActions(info *MemoryInfo) {
	logger.Info("Performing memory recovery actions due to critical memory usage")
	logger.Info("This is a critical situation that requires immediate attention. The system is attempting automatic recovery.")

	// Get config with proper type assertion
	configInterface := a.monitor.GetConfig()
	cfg, ok := configInterface.(*config.Config)
	if !ok {
		logger.Error("Failed to convert config to *config.Config in performRecoveryActions")
		return
	}

	// Restart MariaDB if configured and running
	if cfg.Monitoring.MariaDB.Enabled &&
		cfg.Monitoring.MariaDB.RestartOnThreshold.Enabled {

		serviceName := cfg.Monitoring.MariaDB.ServiceName

		// Check if service is running first
		isRunning, _ := mariadb.CheckServiceStatus(serviceName, nil)
		if isRunning {
			logger.Info("Attempting to restart MariaDB service to free memory",
				logger.String("service", serviceName),
				logger.Float64("memory_usage", info.UsedMemoryPercentage))

			err := mariadb.RestartMariaDBService(serviceName)
			if err != nil {
				logger.Error("Failed to restart MariaDB service",
					logger.String("error", err.Error()))
			} else {
				logger.Info("Successfully restarted MariaDB service")
			}
		} else {
			logger.Warn("MariaDB service is not running, no restart performed",
				logger.String("service", serviceName))
		}

		// Log memory-intensive processes for additional context
		logger.Info("Consider checking for memory-intensive processes if issues persist")
	}
}

// sendAggregatedWarningAlert sends a single warning alert that summarizes multiple warnings
func (a *AlertHandler) sendAggregatedWarningAlert() {
	if len(a.pendingWarnings) == 0 {
		return
	}

	// Increase the warning sent counter
	a.warningsSentToday++

	// Find highest memory usage from collected warnings
	highestUsage := float64(0)
	var worstMemoryInfo *MemoryInfo
	for i, info := range a.pendingWarnings {
		if info.UsedMemoryPercentage > highestUsage {
			highestUsage = info.UsedMemoryPercentage
			worstMemoryInfo = &a.pendingWarnings[i]
		}
	}

	// Create a summary message
	additionalContent := fmt.Sprintf(`
    <p><b>Aggregated Warning:</b> %d memory warnings detected in the last %d minutes.</p>
    <p>Highest memory usage was %.2f%%</p>
    <p><b>Recommendation:</b> Please monitor the system closely if this condition persists.</p>
	<p><small>This is warning notification %d of %d allowed per day.</small></p>`,
		len(a.pendingWarnings),
		int(a.aggregationInterval.Minutes()),
		highestUsage,
		a.warningsSentToday,
		a.maxWarningsPerDay)

	// Get server information using the common utility function
	serverInfo := alerts.GetServerInfoForAlert()

	// Create table content for memory info
	tableContent := a.createMemoryTableContent(worstMemoryInfo)

	// Get style for this alert type
	style := a.handler.GetAlertStyle(alerts.AlertTypeWarning)

	// Generate HTML
	message := alerts.CreateAlertHTML(
		alerts.AlertTypeWarning,
		style,
		"AGGREGATED MEMORY WARNING ALERT",
		false,
		tableContent,
		serverInfo,
		additionalContent,
	)

	// Send notification
	a.handler.SendNotifications("Memory Warning Summary", message, "warning")
	a.monitor.UpdateLastAlertTime()

	// Update tracking state
	a.lastAggregationTime = time.Now()
	a.pendingWarnings = make([]MemoryInfo, 0) // Clear pending warnings

	// Record the time we're sending this warning
	a.lastWarningAlertTime = time.Now()

	// Reset warning count after sending aggregated alert
	a.warningCount = 0

	logger.Info("Sent aggregated memory warning notification",
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
