# IsLLMAlive

监测各LLM API提供商状态的轻量级托盘应用。
Lightweight system tray monitor for LLM API status.

Languages: [English](#english) | [中文](#中文)

## 中文

IsLLMAlive 是一个轻量级 Go 桌面托盘应用，用来在后台监控 LLM 服务商的 API 可用性。它只显示一个托盘图标，按已启用监控项中的最差状态变色，并且只在服务进入或恢复异常状态时发送通知。

如果你想低干扰地观察 OpenAI、Claude、Google AI、DeepSeek、Z.ai，或其他兼容状态端点，而不想一直打开各家状态页，可以使用它。

### 免责声明
1. 此工具（除 PRD 外）**完全通过 vibe coding 生成**，使用 GPT-5.5@Codex 架构和编程；使用 Gemini-3.1-pro 规划和编程。
2. 开发者（人类一方）正在个人使用这款工具。
3. 这款工具的开发者（人类和 LLM）**不对任何使用这款工具造成的后果负责**。

**请根据以上声明和你的判断自己决定是否使用这款工具。**

[MIT 许可](LICENSE)。

用户指南使用 Gemini-3.1-pro 与 GPT-5.5 (xhigh) 编写。

### 功能

- Windows、macOS、Linux 原生系统托盘应用。
- 并发轮询已启用的 monitor。
- 使用 `config.json` 作为设置界面；保存后热重载，无需重启。
- 托盘悬浮提示和右键菜单展示每个 monitor 的状态。
- 只在有意义的状态跨级变化时发送系统原生通知。
- 可从托盘菜单生成并打开 `diagnostics.txt`，用于排查 monitor 异常。

### 安装

从 [GitHub Releases 下载预编译版本](../../releases)，或使用 Go 1.24 及以上版本从源码构建。

Linux 构建需要 GTK 和 AppIndicator 头文件：

```bash
sudo apt-get update && sudo apt-get install -y pkg-config libgtk-3-dev libayatana-appindicator3-dev
```

Windows 构建：

```bash
go build -trimpath -mod=readonly -ldflags "-s -w -H=windowsgui" -o isllmalive.exe ./cmd/isllmalive
```

Linux 或 macOS 构建：

```bash
go build -trimpath -mod=readonly -ldflags "-s -w" -o isllmalive ./cmd/isllmalive
```

### 运行

启动可执行文件。首次运行时，IsLLMAlive 会在可执行文件同级目录创建 `config.json`。已有配置不会被覆盖；如果想使用新增的默认 monitor，需要删除或手动编辑现有 `config.json`。

本地标准启动路径：

```bash
go build -o isllmalive.exe ./cmd/isllmalive
.\isllmalive.exe
```

### 托盘状态

托盘图标取所有已启用 monitor 中的最差状态。颜色不是唯一信息来源；右键菜单和悬浮提示也会显示文字状态。

| 托盘信号 | Provider Status | 含义 |
| --- | --- | --- |
| 绿色 | `Normal` | 至少一个已启用 monitor 正常，且没有 Degraded 或 Outage。 |
| 黄色 | `Degraded` | 至少一个已启用 monitor 出现性能下降或局部故障。 |
| 红色 | `Outage` | 至少一个已启用 monitor 出现重大故障或维护故障。 |
| 灰色 | `Unknown` | 所有已启用 monitor 都未能获取状态，例如网络断开。 |

悬浮提示会隐藏正常 monitor，只列出需要注意的 monitor：`Degraded`、`Outage` 或 `Unknown`。

### 配置

`config.json` 包含全局配置和 `monitors` 数组。

| 字段 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `language` | String | `"en-US"` | 界面语言。支持 `"en-US"` 和 `"zh-CN"`。 |
| `refresh_interval_minutes` | Int | `10` | 轮询间隔，单位为分钟。 |
| `global_notify_on` | Bool | `true` | 全局桌面通知开关，也可在托盘菜单中切换。 |

每个 monitor 支持以下字段：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `type` | String | 是 | Provider 类型：`statuspage`、`openai`、`google`、`deepseek` 或 `apiget`。 |
| `name` | String | 是 | 托盘菜单和通知中显示的名称。 |
| `enabled` | Bool | 是 | 设为 `false` 可暂停此 monitor。 |
| `notify_on` | Bool | 是 | 单个 monitor 的通知开关。 |
| `endpoint` | String | 按 provider 而定 | 状态获取端点。`statuspage` 使用状态页根地址；`deepseek` 使用 RSS feed；`apiget` 使用 API 探测端点或 API host。 |
| `status_page` | String | 否 | 从托盘菜单打开的人类可读状态页。 |
| `component` | String | 否 | 要监控的组件名。省略或使用 `"none"` 表示 page 级状态。 |

默认 monitor 包括 OpenAI、Claude、Google AI、DeepSeek 和 Z.ai。

### 支持的组件

`apiget` monitor 因技术原因不支持组件过滤。

| Provider | 组件 |
| --- | --- |
| OpenAI (`openai`) | `Responses`, `Fine-tuning`, `Images`, `Batch`, `Moderations`, `Embeddings`, `Files`, `Login`, `File uploads`, `CLI`, `FedRAMP`, `Compliance API`, `ChatGPT Atlas`, `Realtime`, `Sora`, `Conversations`, `Agent`, `Connectors/Apps`, `Codex API`, `Deep Research`, `Search`, `GPTs`, `Image Generation`, `Audio`, `VS Code extension`, `Voice mode`, `Codex Web`, `App`, `Chat Completions` |
| Anthropic (`statuspage`) | `claude.ai`, `Claude Console (platform.claude.com)`, `Claude API (api.anthropic.com)`, `Claude Code`, `Claude Cowork`, `Claude for Government` |
| Google (`google`) | `api`, `multimodal live api`, `google ai studio` |
| DeepSeek (`deepseek`) | `API Service`, `Web Chat Service` |
| Kimi (`statuspage`) | `Kimi`, `Website`, `Open API`, `API Service`, `Open Platform Portal`, `SaaS`, `Sign In / Sign Up`, `File uploads`, `Search`, `Model`, `Vision Model`, `Thinking Model`, `Text Model`, `Research Model`, `K2 Model`, `Agentic Model` |
| Minimax (`statuspage`) | `Large Language Models (LLM)`, `Text-to-Speech`, `Video Generation`, `Music Generation` |

### DeepSeek 说明

`deepseek` provider 优先使用 `https://status.deepseek.com/feed.rss`。如果标准 Go HTTP client 被状态站断连，会使用 Chrome-like uTLS 指纹重试。若 RSS 仍不可用，则直接探测服务：

| 组件 | fallback 探测地址 |
| --- | --- |
| `API Service` | `https://api.deepseek.com/v1/models` |
| `Web Chat Service` | `https://chat.deepseek.com` |

直接探测只能推断 `Normal`、`Outage` 或 `Unknown`，不能可靠判断 `Degraded`；RSS 仍是优先数据源。

[返回语言链接](#isllmalive)


## English

IsLLMAlive is a small Go desktop app that watches LLM provider status in the background. It shows one tray icon, colors it by the worst enabled monitor status, and stays quiet unless a service moves into or out of a problem state.

Use it when you want a low-noise signal for OpenAI, Claude, Google AI, DeepSeek, Z.ai, or another compatible status endpoint without keeping provider status pages open.

### Disclaimer
1. This tool is **fully vibe-coded** except for the PRD. It was structured and coded using GPT-5.5@Codex, also planned and coded using Gemini-3.1-pro@Antigravity.
2. The human developer uses this tool personally.
3. The developers, both human and LLM, are **not responsible** for any consequences caused by using this tool.

**Please decide whether to use this code and software based on the statements above and your own judgment.**

[MIT License](LICENSE).
User guide by Gemini-3.1-pro and GPT-5.5 (xhigh).

### Features

- Native system tray app for Windows, macOS, and Linux.
- Polls enabled monitors concurrently.
- Uses `config.json` as the settings UI; saved edits hot reload without restarting.
- Shows a concise tooltip and a right-click menu with per-monitor status.
- Sends native desktop notifications only on meaningful status transitions.
- Writes `diagnostics.txt` from the tray menu when a monitor needs debugging.

### Install

Download pre-built binaries from [GitHub Releases](../../releases), or build from source with Go 1.24 or newer.

Linux builds need GTK and AppIndicator headers:

```bash
sudo apt-get update && sudo apt-get install -y pkg-config libgtk-3-dev libayatana-appindicator3-dev
```

Build on Windows:

```bash
go build -trimpath -mod=readonly -ldflags "-s -w -H=windowsgui" -o isllmalive.exe ./cmd/isllmalive
```

Build on Linux or macOS:

```bash
go build -trimpath -mod=readonly -ldflags "-s -w" -o isllmalive ./cmd/isllmalive
```

### Run

Start the executable. On first launch, IsLLMAlive creates `config.json` next to the executable. Existing config files are not overwritten; delete or edit `config.json` if you want newly added default monitors.

Standard local build path:

```bash
go build -o isllmalive.exe ./cmd/isllmalive
.\isllmalive.exe
```

### Tray Status

The tray icon uses the worst enabled monitor status. Color is not the only signal; the menu and tooltip also show status text.

| Tray signal | Provider Status | Meaning |
| --- | --- | --- |
| Green | `Normal` | At least one enabled monitor is operational, with no Degraded or Outage monitor. |
| Yellow | `Degraded` | At least one enabled monitor reports degraded performance or a partial outage. |
| Red | `Outage` | At least one enabled monitor reports a major outage or maintenance outage. |
| Gray | `Unknown` | All enabled monitors failed to fetch status, such as during network loss. |

The tooltip hides healthy monitors and lists only monitors that need attention: `Degraded`, `Outage`, or `Unknown`.

### Configuration

`config.json` contains global settings and a `monitors` array.

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `language` | String | `"en-US"` | Interface language. Supported values: `"en-US"` and `"zh-CN"`. |
| `refresh_interval_minutes` | Int | `10` | Poll interval in minutes. |
| `global_notify_on` | Bool | `true` | Global desktop notification switch. It can also be toggled from the tray menu. |

Each monitor supports these fields:

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `type` | String | Yes | Provider type: `statuspage`, `openai`, `google`, `deepseek`, or `apiget`. |
| `name` | String | Yes | Display name in the tray menu and notifications. |
| `enabled` | Bool | Yes | Set to `false` to pause this monitor. |
| `notify_on` | Bool | Yes | Per-monitor notification switch. |
| `endpoint` | String | Provider-specific | Fetch endpoint. For `statuspage`, use the status page root; for `deepseek`, use the RSS feed; for `apiget`, use the API probe endpoint or base API host. |
| `status_page` | String | No | Human-readable status page opened from the tray menu. |
| `component` | String | No | Component name to monitor. Omit it or use `"none"` for page-level status. |

Default monitors include OpenAI, Claude, Google AI, DeepSeek, and Z.ai.

### Supported Components

Component filtering is not supported for `apiget` monitors.

| Provider | Components |
| --- | --- |
| OpenAI (`openai`) | `Responses`, `Fine-tuning`, `Images`, `Batch`, `Moderations`, `Embeddings`, `Files`, `Login`, `File uploads`, `CLI`, `FedRAMP`, `Compliance API`, `ChatGPT Atlas`, `Realtime`, `Sora`, `Conversations`, `Agent`, `Connectors/Apps`, `Codex API`, `Deep Research`, `Search`, `GPTs`, `Image Generation`, `Audio`, `VS Code extension`, `Voice mode`, `Codex Web`, `App`, `Chat Completions` |
| Anthropic (`statuspage`) | `claude.ai`, `Claude Console (platform.claude.com)`, `Claude API (api.anthropic.com)`, `Claude Code`, `Claude Cowork`, `Claude for Government` |
| Google (`google`) | `api`, `multimodal live api`, `google ai studio` |
| DeepSeek (`deepseek`) | `API Service`, `Web Chat Service` |
| Kimi (`statuspage`) | `Kimi`, `Website`, `Open API`, `API Service`, `Open Platform Portal`, `SaaS`, `Sign In / Sign Up`, `File uploads`, `Search`, `Model`, `Vision Model`, `Thinking Model`, `Text Model`, `Research Model`, `K2 Model`, `Agentic Model` |
| Minimax (`statuspage`) | `Large Language Models (LLM)`, `Text-to-Speech`, `Video Generation`, `Music Generation` |

### DeepSeek Notes

The `deepseek` provider prefers `https://status.deepseek.com/feed.rss`. If the standard Go HTTP client is disconnected by the status host, it retries with a Chrome-like uTLS fingerprint. If RSS is still unavailable, it probes services directly:

| Component | Fallback probe |
| --- | --- |
| `API Service` | `https://api.deepseek.com/v1/models` |
| `Web Chat Service` | `https://chat.deepseek.com` |

Direct probes can infer `Normal`, `Outage`, or `Unknown`, but not reliable `Degraded`; RSS remains the preferred source.

[Back to language links](#isllmalive)
