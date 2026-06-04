package tray

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"isllmalive/internal/providers"
	"isllmalive/internal/status"

	"github.com/getlantern/systray"
)

var (
	monitorItems    []*systray.MenuItem
	monitorUrls     = make([]string, 20) // store URLs for the 20 slots
	btnRefresh      *systray.MenuItem
	btnToggleNotify *systray.MenuItem
	btnConfig       *systray.MenuItem
	btnDiagnostics  *systray.MenuItem
	btnQuit         *systray.MenuItem

	// Callbacks to main app
	OnRefresh      func()
	OnToggleNotify func()
	OnConfig       func()
	OnDiagnostics  func()
	OnQuit         func()
)

// Init starts the tray application blocking loop.
func Init(onReady, onExit func()) {
	systray.Run(onReady, onExit)
}

// SetupMenu initializes the static parts of the tray menu.
func SetupMenu(onRefresh, onToggleNotify, onConfig, onDiagnostics, onQuit func()) {
	OnRefresh = onRefresh
	OnToggleNotify = onToggleNotify
	OnConfig = onConfig
	OnDiagnostics = onDiagnostics
	OnQuit = onQuit

	for i := 0; i < 20; i++ {
		item := systray.AddMenuItem("", "")
		item.Hide()
		monitorItems = append(monitorItems, item)

		// Spawn a goroutine to handle clicks for this specific index
		go func(idx int, it *systray.MenuItem) {
			for range it.ClickedCh {
				url := monitorUrls[idx]
				if url != "" {
					openBrowser(url)
				}
			}
		}(i, item)
	}

	systray.AddSeparator()
	btnRefresh = systray.AddMenuItem("Refresh Now", "Manual Update")
	btnToggleNotify = systray.AddMenuItem("🔔 Notifications: On", "Toggle Notifications")
	btnConfig = systray.AddMenuItem("Open Config", "Open Config")
	btnDiagnostics = systray.AddMenuItem("Open Diagnostics", "Open Diagnostics")
	systray.AddSeparator()
	btnQuit = systray.AddMenuItem("Quit", "Quit")

	go func() {
		for {
			select {
			case <-btnRefresh.ClickedCh:
				if OnRefresh != nil {
					OnRefresh()
				}
			case <-btnToggleNotify.ClickedCh:
				if OnToggleNotify != nil {
					OnToggleNotify()
				}
			case <-btnConfig.ClickedCh:
				if OnConfig != nil {
					OnConfig()
				}
			case <-btnDiagnostics.ClickedCh:
				if OnDiagnostics != nil {
					OnDiagnostics()
				}
			case <-btnQuit.ClickedCh:
				if OnQuit != nil {
					OnQuit()
				}
				systray.Quit()
			}
		}
	}()
}

// openBrowser opens the specified URL in the default browser of the user.
func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		fmt.Printf("failed to open browser: %v\n", err)
	}
}

// TranslateMenu updates the static menu items based on language.
func TranslateMenu(lang string) {
	if btnRefresh == nil {
		return
	}
	if strings.ToLower(lang) == "zh-cn" {
		btnRefresh.SetTitle("立即刷新")
		btnConfig.SetTitle("打开配置文件")
		btnDiagnostics.SetTitle("打开诊断信息")
		btnQuit.SetTitle("退出")
	} else {
		btnRefresh.SetTitle("Refresh Now")
		btnConfig.SetTitle("Open Config")
		btnDiagnostics.SetTitle("Open Diagnostics")
		btnQuit.SetTitle("Quit")
	}
}

// UpdateToggleNotifyState updates the state of the toggle notification button.
func UpdateToggleNotifyState(enabled bool, lang string) {
	if btnToggleNotify == nil {
		return
	}
	isZh := strings.ToLower(lang) == "zh-cn"
	if enabled {
		if isZh {
			btnToggleNotify.SetTitle("🔔 系统通知: 开")
		} else {
			btnToggleNotify.SetTitle("🔔 Notifications: On")
		}
	} else {
		if isZh {
			btnToggleNotify.SetTitle("🔕 系统通知: 关")
		} else {
			btnToggleNotify.SetTitle("🔕 Notifications: Off")
		}
	}
}

// translateStatus returns the localized string for a status.
func translateStatus(s status.Status, lang string) string {
	if strings.ToLower(lang) == "zh-cn" {
		switch s {
		case status.Normal:
			return "正常"
		case status.Degraded:
			return "服务降级"
		case status.Outage:
			return "宕机"
		default:
			return "未知状态"
		}
	}
	return s.String()
}

// Update refreshes the tray icon, tooltip, and menu items based on current results.
func Update(results []providers.MonitorResult, lang string) {
	maxSeverity := status.Unknown
	for _, res := range results {
		if res.Status > maxSeverity {
			maxSeverity = res.Status
		}
	}

	colorHex := "#9E9E9E" // default Unknown
	switch maxSeverity {
	case status.Normal:
		colorHex = "#69B72A"
	case status.Degraded:
		colorHex = "#F0E442"
	case status.Outage:
		colorHex = "#D50000"
	}

	iconBytes, err := GenerateIcon(colorHex)
	if err == nil {
		systray.SetIcon(iconBytes)
	}

	var anomalyMsgs []string
	for _, res := range results {
		if res.Status != status.Normal {
			anomalyMsgs = append(anomalyMsgs, fmt.Sprintf("%s: %s", res.Name, monitorDetail(res, lang)))
		}
	}

	if len(anomalyMsgs) == 0 {
		if strings.ToLower(lang) == "zh-cn" {
			systray.SetTooltip("全部正常")
		} else {
			systray.SetTooltip("All Operational")
		}
	} else {
		systray.SetTooltip(strings.Join(anomalyMsgs, " | "))
	}

	for i, item := range monitorItems {
		if i < len(results) {
			res := results[i]
			emoji := "🚫" // Unknown
			switch res.Status {
			case status.Normal:
				emoji = "✅" // Normal
			case status.Degraded:
				emoji = "⚠️" // Degraded
			case status.Outage:
				emoji = "❌" // Outage
			}
			item.SetTitle(fmt.Sprintf("%s %s: %s", emoji, res.Name, translateStatus(res.Status, lang)))
			item.SetTooltip(monitorTooltip(res, lang))
			monitorUrls[i] = res.StatusPage
			item.Show()
		} else {
			item.SetTooltip("")
			monitorUrls[i] = ""
			item.Hide()
		}
	}
}

func monitorDetail(res providers.MonitorResult, lang string) string {
	detail := res.Message
	if detail == "" {
		detail = translateStatus(res.Status, lang)
	}
	if res.Err != nil {
		errText := res.Err.Error()
		if detail == "" {
			detail = errText
		} else if !strings.Contains(detail, errText) {
			detail = fmt.Sprintf("%s (%s)", detail, errText)
		}
	}
	return detail
}

func monitorTooltip(res providers.MonitorResult, lang string) string {
	lines := []string{
		res.Name,
		fmt.Sprintf("Status: %s", translateStatus(res.Status, lang)),
	}
	if res.Type != "" {
		lines = append(lines, "Type: "+res.Type)
	}
	if res.Endpoint != "" {
		lines = append(lines, "Endpoint: "+res.Endpoint)
	}
	if res.Component != "" {
		lines = append(lines, "Component: "+res.Component)
	}
	if detail := monitorDetail(res, lang); detail != "" {
		lines = append(lines, "Detail: "+detail)
	}
	if res.CheckedAt.IsZero() {
		lines = append(lines, "Checked: unknown")
	} else {
		lines = append(lines, "Checked: "+res.CheckedAt.Format("2006-01-02 15:04:05 MST"))
	}
	if res.StatusPage != "" {
		lines = append(lines, "Status page: "+res.StatusPage)
	}
	return strings.Join(lines, "\n")
}
