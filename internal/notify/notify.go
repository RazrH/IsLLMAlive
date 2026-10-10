package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

const windowsScript = `
$ErrorActionPreference = 'Stop'
[Console]::InputEncoding = New-Object System.Text.UTF8Encoding
$data = [Console]::In.ReadToEnd() | ConvertFrom-Json
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null
$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml('<toast><visual><binding template="ToastText02"><text id="1"></text><text id="2"></text></binding></visual></toast>')
$texts = $xml.GetElementsByTagName('text')
$texts.Item(0).AppendChild($xml.CreateTextNode([string]$data.title)) | Out-Null
$texts.Item(1).AppendChild($xml.CreateTextNode([string]$data.message)) | Out-Null
$toast = New-Object Windows.UI.Notifications.ToastNotification $xml
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('IsLLMAlive').Show($toast)
`

const appleScript = `on run argv
display notification (item 2 of argv) with title (item 1 of argv)
end run`

func command(ctx context.Context, platform, title, message string) (*exec.Cmd, error) {
	var cmd *exec.Cmd
	switch platform {
	case "windows":
		data, err := json.Marshal(struct {
			Title   string `json:"title"`
			Message string `json:"message"`
		}{title, message})
		if err != nil {
			return nil, err
		}
		cmd = exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", windowsScript)
		cmd.Stdin = bytes.NewReader(data)
	case "darwin":
		cmd = exec.CommandContext(ctx, "osascript", "-e", appleScript, "--", title, message)
	case "linux":
		cmd = exec.CommandContext(ctx, "notify-send", "--", title, message)
	default:
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}
	hideWindow(cmd)
	cmd.WaitDelay = time.Second
	return cmd, nil
}

// Send treats upstream text only as data and bounds execution by the parent and five seconds.
func Send(ctx context.Context, title, message string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd, err := command(ctx, runtime.GOOS, title, message)
	if err != nil {
		return err
	}
	if err = cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("notification: %w", err)
	}
	return nil
}
