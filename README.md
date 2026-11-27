# WorkChatBot

基于 Go 的企业微信 AI 机器人。通过 WebSocket 接收企微消息，交由本地部署的 Ollama 大模型处理，并将结果流式回复给用户。

## 功能特性

- **企微 WebSocket 接入** — 基于 `wecom-aibot-go-sdk`，内置自动认证、心跳与断线重连
- **本地大模型推理** — 通过 Ollama API 调用本地模型（默认 `qwen3:14b`），数据不出内网
- **并发 worker pool** — 带缓冲的有界任务队列 + N 个 goroutine 并发处理，队列满时快速降级提示
- **流式回复** — 使用 `ReplyStream` 将模型回答流式返回给用户
- **任务异常隔离** — 每个任务独立 `recover`，单个失败不影响整体服务
- **结构化日志** — 自研 `logTool`，支持 `date | level | goroutine-id | caller | message` 格式，可切换 JSON、按天轮转、错误分流到 stderr
- **优雅关闭** — 监听 SIGINT/SIGTERM，停止投递、等待任务完成、断开连接
- **配置驱动** — 所有参数通过 `.env` 配置，附 `.env.example` 模板

## 技术栈

| 组件 | 选型 |
|---|---|
| 语言 | Go 1.27.2 |
| 企微 SDK | `github.com/go-sphere/wecom-aibot-go-sdk` v1.0.6 |
| 模型调用 | `github.com/ollama/ollama/api` v0.40.17 |
| 配置加载 | `godotenv` + `caarlos0/env/v11` |
| 日志 | 自研 `logTool`（标准库实现） |

## 项目结构

```
WorkChatBot/
├── main.go              # 入口：worker pool、消息回调、优雅关闭
├── getEnv/              # 环境变量解析与校验
├── toolSDK/             # Ollama 调用封装、聊天选项
├── logTool/             # 结构化日志模块
├── .env.example         # 配置模板
└── logs/                # 日志输出目录
```

## 快速开始

### 1. 准备 Ollama

确保本地或内网已有 Ollama 服务并拉取模型：

```bash
ollama pull qwen3:14b
```

### 2. 配置环境变量

```bash
cp .env.example .env
```

编辑 `.env`，至少填写以下三项：

```env
WE_COM_BOT_ID="你的企微机器人ID"
WE_COM_BOT_SECRET="你的企微机器人密钥"
OLLAMA_URL="http://你的Ollama地址:11434"
```

其余参数（模型名、超时、协程数、日志配置）按需调整。

### 3. 运行

```bash
go run .
```

或编译后运行：

```bash
go build -o WorkChatBot.exe .
./WorkChatBot.exe
```

## 配置项说明

| 变量 | 说明 | 默认值 |
|---|---|---|
| `WE_COM_BOT_ID` | 企微机器人 ID | 必填 |
| `WE_COM_BOT_SECRET` | 企微机器人密钥 | 必填 |
| `OLLAMA_URL` | Ollama 服务地址 | 必填 |
| `OLLAMA_TIMEOUT` | 请求超时 | `10m` |
| `KEEP_ALIVE` | 模型驻留时间 | `60m` |
| `TEXT_MODEL_NAME` | 文本模型名 | `qwen3:14b` |
| `IMAGE_MODEL_NAME` | 图像模型名 | `qwen2.5vl:7b` |
| `THINK` | 是否开启思考模式 | `true` |
| `MODEL_OPTIONS` | 模型参数（JSON） | `{"temperature":0.7,"num_ctx":8192}` |
| `MODEL_SYSTEM_CONTENT` | 系统提示词 | 见 `.env.example` |
| `WORKER_COUNT` | 并发 worker 数 | `4` |
| `CHAT_JOB_COUNT` | 任务队列容量 | `10` |
| `LOG_LEVEL` | 日志级别 | `DEBUG` |
| `LOG_FILE` | 日志文件路径，留空只输出控制台 | `logs/app.log` |
| `LOG_FORMAT` | `text` / `json` | `text` |
| `LOG_ROTATE` | `daily` / `none` | `daily` |

## 日志示例

```
[main] 2026-11-27 09:34:12.123 | INFO  | g1   | main.go:23       | Starting WorkChatBot...
[toolSDK] 2026-11-27 09:34:15.456 | INFO  | g6   | chatOllama.go:73 | 耗时 | 总耗时=1523.4ms 模型加载=0.0ms 提示词计算=...
```

## 安全说明

- `.env` 含机器人密钥等敏感信息，**已加入 `.gitignore`，请勿提交**
- 仓库中仅保留 `.env.example` 模板，不含真实密钥
- 首次使用请从模板复制并填入自己的配置

## License

MIT
