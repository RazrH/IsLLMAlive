# IsLLMAlive
Go based gadget in system tray that shows availability of LLM APIs.

## Disclaimer
1. This tool is fully vibe-coded (except from PRD). Planned with Gemini 3.1 pro (web), Sturctured by GPT 5.5 (high), coded by Gemini 3.1 pro (high).
2. The developer(s)(the human) uses this tool personally.
3. The developer(s)(both the human and the LLMs) of this tool are not responsible for any issues or damages caused by the use of this tool.
Please use the code and software at your own discretion, based on claims above.

MIT License.
User guide by Gemini 3.1 pro (high).

## 📖 User Guide (使用说明)

### 1. Installation (安装)

#### Option A: Download Pre-compiled Binaries (推荐)
You can directly download the latest executable files for Windows, macOS, and Linux from the [GitHub Releases](../../releases) page. No additional installation is required.

#### Option B: Build from Source (自行编译)
If you prefer to compile the code yourself, make sure you have Go (1.21+) installed.

**Dependencies:**
- **Windows / macOS**: No extra dependencies required.
- **Linux**: Requires GTK3 and AppIndicator development headers.
  ```bash
  sudo apt-get update && sudo apt-get install -y libgtk-3-dev libappindicator3-dev
  ```

**Build Commands:**
- **Windows**: (Adds `-H=windowsgui` to hide the CMD window in the background)
  ```bash
  go build -ldflags "-H=windowsgui -w -s" -o isllmalive.exe ./cmd/isllmalive
  ```
- **Linux / macOS**:
  ```bash
  go build -ldflags "-w -s" -o isllmalive ./cmd/isllmalive
  ```

---

### 2. Status Indicators (托盘图标与状态说明)
The system tray icon dynamically changes color based on the worst status among all your enabled API providers (Bucket Effect). Hovering your mouse over the icon will show detailed status information.

| Icon Color | Status | Description |
| :---: | :--- | :--- |
| 🟢 | **Normal (正常)** | All enabled services are operational. Tooltip displays "All Operational". |
| 🟡 | **Degraded (服务降级)** | At least one service is experiencing performance issues or partial outages. |
| 🔴 | **Outage (宕机)** | At least one service is experiencing a major outage. |
| ⚪ | **Unknown (未知)** | ALL enabled services failed to fetch status (e.g., network disconnected). |

*Note: The hover tooltip features noise-reduction. It intelligently hides healthy services and only displays the names and error reasons for services experiencing issues (Degraded, Outage, or Unknown).*

---

### 3. Usage & Operations (基本操作方法)
- **Background Execution**: The app runs silently in the background with near-zero CPU and <10MB memory usage.
- **Context Menu (右键菜单)**: Right-click the tray icon to view the status of each service individually, manually force a refresh, toggle system notifications, or open the configuration file.
- **Hot Reload (热重载)**: Clicking "Open Config" will open `config.json`. Any changes saved to this file will be applied instantly without needing to restart the application.
- **System Notifications (系统通知)**: When a service transitions between Normal and Degraded/Outage, a native desktop notification will be triggered. This can be globally turned off via the right-click menu, or disabled per-provider in `config.json`.

---

### 4. Configuration (配置指南)
The `config.json` file is automatically generated in the same directory as the executable upon first launch. You can customize the behavior by editing it directly.

#### Global Settings (系统级字段)
| Field (字段) | Type | Default | Description (说明) |
| --- | --- | --- | --- |
| `language` | String | `"en-US"` | Interface language. Currently only supports `"en-US"` and `"zh-CN"`. |
| `refresh_interval_minutes` | Int | `10` | How often to poll API statuses (in minutes). |
| `global_notify_on` | Bool | `true` | Global switch for desktop notifications. Can also be toggled via the tray menu. |

#### Provider Settings (供应商具体字段)
Inside the `monitors` array, you can define multiple providers.

| Field (字段) | Type | Required | Description (说明) |
| :--- | :---: | :---: | :--- |
| `type` | String | Yes | Provider API type. Supported: `statuspage` (for OpenAI, Anthropic), `google` (Vertex/Gemini), `deepseek`. |
| `name` | String | Yes | Display name for the tray menu and notifications. |
| `enabled` | Bool | Yes | Set to `false` to completely pause polling for this provider. |
| `notify_on` | Bool | Yes | Local notification switch. If `false`, this specific provider will never trigger notifications. |
| `status_page` | String | No | The human-readable URL to open when clicking the item in the tray menu. |
| `endpoint` | String | If `statuspage` | The API endpoint URL to fetch status from (e.g., `https://status.openai.com`). |
| `component` | String | No | Filter by specific component name (e.g., "API" or "ChatGPT"). If omitted, uses the overall page status. |
