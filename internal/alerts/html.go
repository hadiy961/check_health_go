package alerts

import (
	"fmt"
	"strings"
)

// TableRow represents a single row in an alert table
type TableRow struct {
	Label string
	Value string
}

// CreateTable generates HTML table from a set of table rows
func CreateTable(rows []TableRow) string {
	var tableRows strings.Builder

	for _, row := range rows {
		tableRows.WriteString(fmt.Sprintf(`
	<tr>
		<td>%s</td>
		<td>%s</td>
	</tr>`, row.Label, row.Value))
	}

	return fmt.Sprintf(`
<table>
	<tr>
		<th>Metric</th>
		<th>Value</th>
	</tr>%s
</table>`, tableRows.String())
}

// CreateStatusLine creates a styled status line for alerts
func CreateStatusLine(statusClass, statusText string) string {
	return fmt.Sprintf(`<p><b>Current Status:</b> <span class="%s">%s</span></p>`,
		statusClass, statusText)
}

// ServerInfo contains server information to be included in alerts
type ServerInfo struct {
	Hostname        string
	IPAddress       string
	Platform        string
	PlatformVersion string
	KernelVersion   string
	Uptime          string
}

// CreateAlertHTML creates a standardized HTML alert message
func CreateAlertHTML(
	alertType AlertType,
	style AlertStyle,
	title string,
	statusChanged bool,
	tableContent string,
	serverInfo *ServerInfo,
	additionalContent string) string {

	// Handle status note
	var statusNote string
	if statusChanged && alertType != AlertTypeNormal {
		statusNote = fmt.Sprintf(`<div class="note" style="border-left-color: %s"><b>NOTE:</b> This is a status change alert.</div>`, style.BorderColor)
	}

	// Format server info section if provided
	serverInfoHTML := ""
	if serverInfo != nil {
		serverInfoHTML = fmt.Sprintf(`
    <div class="section">
        <h3 class="section-title">Server Information</h3>
        <table>
            <tr>
                <th>Attribute</th>
                <th>Value</th>
            </tr>
            <tr>
                <td>Hostname</td>
                <td>%s</td>
            </tr>
            <tr>
                <td>IP Address</td>
                <td>%s</td>
            </tr>
            <tr>
                <td>Operating System</td>
                <td>%s %s</td>
            </tr>
            <tr>
                <td>Kernel Version</td>
                <td>%s</td>
            </tr>
            <tr>
                <td>System Uptime</td>
                <td>%s</td>
            </tr>
        </table>
    </div>`,
			serverInfo.Hostname,
			serverInfo.IPAddress,
			serverInfo.Platform, serverInfo.PlatformVersion,
			serverInfo.KernelVersion,
			serverInfo.Uptime,
		)
	}

	// Format the HTML
	return fmt.Sprintf(`
<html>
<head>
    <style>
        %s
        .container { border: 1px solid %s; }
        .header { background-color: %s; }
        .%s { color: %s; font-weight: bold; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h2>%s</h2>
        </div>
        <div class="content">
            <p>Dear Team,</p>
            <p>This is an automated alert from the CheckHealthDO services. Please review the following details:</p>
            
            %s
            
            %s
            %s
            %s
        </div>
    </div>
</body>
</html>`,
		CommonStyles,
		style.BorderColor,
		style.HeaderColor,
		style.StatusColorClass,
		style.BorderColor,
		title,
		tableContent,
		additionalContent,
		statusNote,
		serverInfoHTML,
	)
}
