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

从 [GitHub Releases 下载预编译版本](../../releases)，或使用 Go 1.27.2 从源码构建。

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
| `components` | String 数组 | 否 | 要监控的组件名数组；已知异常优先，否则存在 Unknown 就保持 Unknown。省略或使用 `["none"]` 表示 page 级状态。 |
| `component` | String | 否 | 旧版单组件兼容字段。仅在 `components` 为空时使用。 |

```json
"components": ["API Service", "Web Chat Service"]
```

右键悬浮 monitor 菜单项时，只展开所选组件中状态为 `Degraded`、`Outage` 或 `Unknown` 的组件及其详情。旧的单组件配置继续兼容。

默认 monitor 包括 OpenAI、Claude、Google AI、DeepSeek 和 Z.ai。新生成的配置统一使用 `components`；DeepSeek 默认选择 `API Service` 和 `Web Chat Service`，其余支持组件的默认 monitor 使用 `["none"]` 保持 page 级状态。

### 支持的组件

`apiget` monitor 因技术原因不支持组件过滤。

| Provider | 组件 |
| --- | --- |
| OpenAI (`openai`) | 见下方按官方状态页分组的推荐配置值。 |
| Anthropic (`statuspage`) | `claude.ai`, `Claude Console (platform.claude.com)`, `Claude API (api.anthropic.com)`, `Claude Code`, `Claude Cowork`, `Claude for Government` |
| Google (`google`) | `api`, `multimodal live api`, `google ai studio` |
| DeepSeek (`deepseek`) | `Pro API`, `Flash API`, `Web Chat Service`；`API Service` 兼容聚合全部 API。 |
| Kimi (`statuspage`) | `Kimi`, `Website`, `Open API`, `API Service`, `Open Platform Portal`, `SaaS`, `Sign In / Sign Up`, `File uploads`, `Search`, `Model`, `Vision Model`, `Thinking Model`, `Text Model`, `Research Model`, `K2 Model`, `Agentic Model` |
| Minimax (`statuspage`) | `Large Language Models (LLM)`, `Text-to-Speech`, `Video Generation`, `Music Generation` |

#### OpenAI 组件

OpenAI 支持 `分组/组件` 限定名，但唯一组件推荐直接写组件名。两个 `Login` 推荐分别写为 `APIs/Login` 或 `ChatGPT/Login`；未限定的 `Login` 会聚合两者的最差状态。例如 `Codex/Codex API` 受支持，但推荐简写为 `Codex API`。

| 官方分组 | `components` 推荐配置值 |
| --- | --- |
| APIs | `Chat Completions`, `Responses`, `Agents`, `Decisions`, `Fine-tuning`, `Embeddings`, `Images`, `Batch`, `Audio`, `Moderations`, `Realtime`, `Files`, `APIs/Login` |
| ChatGPT | `Conversations`, `ChatGPT/Login`, `ChatGPT Work`, `Codex in ChatGPT Desktop`, `Compliance API`, `Search`, `File uploads`, `Voice mode`, `GPTs`, `Image Generation`, `Deep Research`, `Agent`, `dots`, `Space`, `Sites`, `Connectors/Apps` |
| Codex | `Codex Web`, `Codex API`, `CLI`, `VS Code extension` |
| FedRAMP | `FedRAMP` |
| Ads Platform | `Ads Manager`, `Ads API` |
| 未列入当前分组 | `ChatGPT Atlas`（使用未限定名） |

OpenAI 分组和组件名称于 2026-10-10 从 [OpenAI 官方状态代理 JSON](https://status.openai.com/proxy/status.openai.com) 实际核对：`summary.components` 有 37 个组件，`summary.structure.items[].group` 列出其中 36 个的分组。相对旧列表，新增 `Agents`、`Decisions`、`dots`、`Space`，`Sora` 已不存在；`ChatGPT Atlas` 仍在组件索引中，但当前没有分组记录，因此不能使用 `ChatGPT/ChatGPT Atlas`。

配置使用组件名或已有分组的 `分组/组件`，忽略大小写和两端空白，不按模型版本或子串匹配。组件索引来自 `summary.components`，受影响状态通过 `summary.affected_components[].component_id/status` 关联；没有受影响记录的有效组件视为正常。不存在的名称返回 `Unknown`，不会自动替换成其他组件。列表及分组仍可能继续变化。

#### Google 组件及返回字段

2026-10-10 实际调用 [Google AI Studio 状态页](https://aistudio.google.com/status) 使用的 `MakerSuiteService/ListIncidentsHistory` 接口，返回 63 条历史事件，组件 ID 仍为 `1/2/3`；本次所有事件的最后更新均为已解决，现有三个配置值解析正常。

| `components` 配置值 | 接口组件 ID | 监控范围 |
| --- | --- | --- |
| `api` | 1 | Gemini API |
| `multimodal live api` | 2 | Live API |
| `google ai studio` | 3 | Google AI Studio |

这些是客户端支持的配置值，忽略大小写和两端空白，不是模型名称，也不是从事件标题做关键词匹配。接口返回位置数组：`raw[0][0]` 是事件列表；每条事件的 `[1]` 是标题、`[2]` 是严重度（1=性能下降，2=中断）、`[3]` 是状态更新数组、`[5]` 是受影响组件 ID 数组。最后更新的 `[0]` 为 4 时表示已解决；组件过滤读取 `[5]`，而不是标题或旧的单值字段 `[4]`。本次未发现匹配 ID 或数组结构变化；该页面内部接口将来仍可能改变，无法解释的数据不会被当作正常；已确认的异常仍保留。

### DeepSeek 说明

`deepseek` provider 优先使用 `https://status.deepseek.com/feed.rss`。如果标准 Go HTTP client 被状态站断连，会使用 Chrome-like uTLS 指纹重试。若 RSS 仍不可用，API 聚合与网页组件会直接探测服务：

| 组件 | fallback 探测地址 |
| --- | --- |
| `API Service` | `https://api.deepseek.com/v1/models` |
| `Web Chat Service` | `https://chat.deepseek.com` |

直接探测只能推断 `Normal`、`Outage` 或 `Unknown`，不能可靠判断 `Degraded`；RSS 仍是优先数据源。API 地址的可达性不能分别证明 Pro 和 Flash 模型健康。

2026-10-10 实际获取的 RSS 有 33 条事件，均为 `resolved`。`description` 的 `Affected components` 包含带版本号的 Pro API、Flash API、对话服务，以及上传文件和搜索服务。解析按服务角色识别独立词 `Pro`／`Flash`，不匹配品牌前缀或模型版本；`Chat Service`、`Chatservice` 和中文对话标签均可识别。服务归属读取 `Affected components`，不从笼统的“网页/API”标题推断。

要分别显示三类服务，可以设置：

```json
"components": ["Pro API", "Flash API", "Web Chat Service"]
```

`API Service` 继续聚合全部 API，原配置无需迁移。仅 Flash 的事件不会影响 Pro；同家族不同模型版本归入同一服务角色。旧事件若只写“API 服务”而未区分家族，API 聚合项保留异常，Pro／Flash 显示 `Unknown` 并说明未细分；无法识别的活动服务标签保留诊断，已确认的异常不被丢弃。部分中断（`Partially Unavailable`／`partial outage`／“部分中断”）判为 `Degraded`，明确完全不可用判为 `Outage`；`monitoring` 仍是未解决状态。

RSS 不可用时，`Pro API`／`Flash API` 返回 `Unknown`，不会把通用网关可达性当作模型健康；旧 `API Service` 和网页组件继续使用上表的直接探测。RSS 是事件历史，正常表示未发现已知未解决事件，不等于实际验证模型推理可用。将来版本号改变无需更新配置；服务角色词本身改变则仍需重新核对。

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

Download pre-built binaries from [GitHub Releases](../../releases), or build from source with Go 1.27.2.

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
| `components` | String array | No | Component names aggregated by one monitor; confirmed faults win, otherwise any Unknown keeps the result Unknown. Omit it or use `["none"]` for page-level status. |
| `component` | String | No | Legacy single-component compatibility field, used only when `components` is empty. |

```json
"components": ["API Service", "Web Chat Service"]
```

Hovering the monitor's right-click menu item expands only selected components whose status is `Degraded`, `Outage`, or `Unknown`, including their details. Existing single-component configurations remain compatible.

Default monitors include OpenAI, Claude, Google AI, DeepSeek, and Z.ai. Newly generated configurations consistently use `components`; DeepSeek selects `API Service` and `Web Chat Service` by default, while the other component-aware defaults use `["none"]` for page-level status.

### Supported Components

Component filtering is not supported for `apiget` monitors.

| Provider | Components |
| --- | --- |
| OpenAI (`openai`) | See the recommended values grouped by the official status page below. |
| Anthropic (`statuspage`) | `claude.ai`, `Claude Console (platform.claude.com)`, `Claude API (api.anthropic.com)`, `Claude Code`, `Claude Cowork`, `Claude for Government` |
| Google (`google`) | `api`, `multimodal live api`, `google ai studio` |
| DeepSeek (`deepseek`) | `Pro API`, `Flash API`, `Web Chat Service`; `API Service` combines all APIs for compatibility. |
| Kimi (`statuspage`) | `Kimi`, `Website`, `Open API`, `API Service`, `Open Platform Portal`, `SaaS`, `Sign In / Sign Up`, `File uploads`, `Search`, `Model`, `Vision Model`, `Thinking Model`, `Text Model`, `Research Model`, `K2 Model`, `Agentic Model` |
| Minimax (`statuspage`) | `Large Language Models (LLM)`, `Text-to-Speech`, `Video Generation`, `Music Generation` |

#### OpenAI Components

OpenAI accepts qualified `Group/Component` names, while unique components are best configured by component name alone. The two `Login` components are best written as `APIs/Login` or `ChatGPT/Login`; unqualified `Login` aggregates all matching components using the same uncertainty-preserving rule. For example, `Codex/Codex API` is supported, but `Codex API` is the recommended shorthand.

| Official group | Recommended `components` values |
| --- | --- |
| APIs | `Chat Completions`, `Responses`, `Agents`, `Decisions`, `Fine-tuning`, `Embeddings`, `Images`, `Batch`, `Audio`, `Moderations`, `Realtime`, `Files`, `APIs/Login` |
| ChatGPT | `Conversations`, `ChatGPT/Login`, `ChatGPT Work`, `Codex in ChatGPT Desktop`, `Compliance API`, `Search`, `File uploads`, `Voice mode`, `GPTs`, `Image Generation`, `Deep Research`, `Agent`, `dots`, `Space`, `Sites`, `Connectors/Apps` |
| Codex | `Codex Web`, `Codex API`, `CLI`, `VS Code extension` |
| FedRAMP | `FedRAMP` |
| Ads Platform | `Ads Manager`, `Ads API` |
| Not in the current grouping | `ChatGPT Atlas` (unqualified name only) |

Verified on 2026-10-10 against the [official OpenAI status proxy JSON](https://status.openai.com/proxy/status.openai.com): `summary.components` contains 37 components, while `summary.structure.items[].group` groups 36 of them. Since the previous list, `Agents`, `Decisions`, `dots`, and `Space` were added and `Sora` disappeared. `ChatGPT Atlas` remains in the component index but has no current group entry, so `ChatGPT/ChatGPT Atlas` does not match.

Configuration matches component names or existing `Group/Component` names, ignoring case and surrounding whitespace; it does not match model versions or substrings. Components come from `summary.components`; affected states join through `summary.affected_components[].component_id/status`. Valid components without an affected-state entry are treated as normal. Missing names return `Unknown` and are not automatically replaced. Names and groups may change again.

#### Google Components and Response Fields

On 2026-10-10, the `MakerSuiteService/ListIncidentsHistory` endpoint used by the [Google AI Studio status page](https://aistudio.google.com/status) returned 63 historical incidents. Component IDs remain `1/2/3`; all latest updates in this capture were resolved, and the three existing configuration values parsed successfully.

| `components` value | Response component ID | Scope |
| --- | --- | --- |
| `api` | 1 | Gemini API |
| `multimodal live api` | 2 | Live API |
| `google ai studio` | 3 | Google AI Studio |

These are supported client configuration values, ignoring case and surrounding whitespace, rather than model names or incident-title keywords. The response uses positional arrays: `raw[0][0]` is the incident list; incident `[1]` is the title, `[2]` severity (1=degraded, 2=outage), `[3]` the update list, and `[5]` the affected component-ID array. The last update's `[0]` equals 4 when resolved. Filtering uses `[5]`, not the title or the older single-value field `[4]`. No ID or array-layout change was found in this capture; the internal page endpoint may still change; uninterpretable data is not assumed healthy, while confirmed faults remain visible.

### DeepSeek Notes

The `deepseek` provider prefers `https://status.deepseek.com/feed.rss`. If the standard Go HTTP client is disconnected by the status host, it retries with a Chrome-like uTLS fingerprint. If RSS is still unavailable, aggregate API and web components use direct probes:

| Component | Fallback probe |
| --- | --- |
| `API Service` | `https://api.deepseek.com/v1/models` |
| `Web Chat Service` | `https://chat.deepseek.com` |

Direct probes can infer `Normal`, `Outage`, or `Unknown`, but not reliable `Degraded`; RSS remains the preferred source. API endpoint reachability cannot establish Pro and Flash model health independently.

The RSS captured on 2026-10-10 contained 33 incidents, all resolved. Its description's `Affected components` lists versioned Pro API and Flash API labels, chat, file upload, and search services. Parsing identifies the standalone service-role words `Pro`/`Flash`, without matching brand prefixes or model versions; `Chat Service`, `Chatservice`, and Chinese chat labels are recognized. Impact comes from `Affected components`, rather than broad Web/API incident titles.

To display the three services independently, configure:

```json
"components": ["Pro API", "Flash API", "Web Chat Service"]
```

`API Service` still aggregates all APIs, preserving existing configurations. A Flash-only incident does not affect Pro; different model versions in the same family share a service role. Older incidents that only identify a general API service preserve the aggregate fault but leave Pro/Flash `Unknown` with a diagnostic. Unrecognized active service labels retain diagnostics without discarding confirmed faults. Partial outages (`Partially Unavailable`, `partial outage`, or Chinese partial-outage wording) map to `Degraded`; explicit full unavailability maps to `Outage`. `monitoring` remains unresolved.

When RSS is unavailable, `Pro API`/`Flash API` return `Unknown`; a reachable gateway is not treated as model health. Legacy `API Service` and web chat retain the direct probes above. RSS is incident history: normal means no known unresolved incident, rather than verified inference availability. Version-number changes need no configuration update; changes to the service-role wording still require review.

[Back to language links](#isllmalive)
