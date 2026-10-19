# GoAgent

一个基于 Go、Eino 和 Gin 构建的可扩展 AI Agent 服务。项目提供流式对话、AIOps 诊断、文件知识库检索、工具调用、会话管理以及上下文状态查询等能力，并通过 Docker Compose 集成 Go 服务（含前端静态资源）与 Chroma 向量数据库。

> 项目当前仍处于持续开发阶段。README 会尽量按照当前代码库的实际实现进行说明，未完成的功能会明确标注。

## 功能概览

- **流式智能对话**：通过 ReAct Agent 调用已注册工具，并以流式方式返回执行过程和回答。
- **AIOps 诊断**：提供面向运维问题的 Agent 入口；`health_check` 无需参数，查询后端运行环境的 CPU、内存、磁盘和负载，不提供按业务服务过滤或可用性探测。
- **工具调用**：支持 Bash 执行、文件读写、知识库检索、指标查询以及子 Agent 代理等工具。
- **安全控制**：对 Bash 命令进行敏感操作拦截，并通过工作区限制文件访问范围。
- **知识库检索**：结合 Embedding 模型与 Chroma/Milvus Lite 服务实现文档向量检索。
- **会话管理**：支持创建、查询、删除对话，并将会话数据持久化到本地 `data/conversations` 目录。
- **权限响应**：当工具执行需要用户确认时，可通过权限接口返回确认结果；“始终允许”默认只作用于当前对话，显式选择后才写入全局 `permission.json`。
- **输出截断保护**：模型达到最大输出上限时保留已生成内容并标注截断，不执行该轮不完整的工具调用。
- **请求级执行一致性**：一次请求固定使用准备好的对话模型、摘要模型和 Token 预算；切换模型影响后续请求。Slot 与历史组装后统一预检压缩，工具循环最多执行 20 个工具轮次，再禁用工具请求最终回答。
- **上下文监控**：提供当前上下文使用量查询，避免超过模型上下文窗口。
- **统一入口**：Go 同时托管 Web 静态资源与 `/api/*`，前端构建产物由同一端口提供，不再依赖独立 Nginx 容器。

## 技术栈

- **后端**：Go 1.25、Gin、Eino
- **Agent 编排**：CloudWeGo Eino ReAct
- **大语言模型**：兼容 OpenAI API 格式的模型服务
- **Embedding**：兼容 OpenAI API 格式的 Embedding 服务
- **向量数据库**：Chroma 服务（基于 Milvus Lite）
- **前端**：Vue 3、Vite、TypeScript（Node.js 20 构建）
- **部署**：Docker、Docker Compose

## 项目结构

```text
GoAgent/
├── cmd/
│   └── main.go                 # 服务入口
├── internal/
│   ├── config/                 # 配置加载与校验
│   ├── handler/                # HTTP Handler 与路由
│   ├── middleware/             # CORS、Session 等中间件
│   ├── service/                # Agent、文件、会话等业务服务
│   ├── store/                  # 会话等数据存储
│   ├── tools/                  # 工具注册与工具实现
│   ├── commands/               # 斜杠命令
│   └── metrics/                # 指标采集
├── chroma/                     # Chroma/Milvus Lite 向量服务
├── frontend/                   # Web 前端
├── docs/                       # 项目文档与架构说明
├── examples/                   # 示例
├── config.example.yaml         # 本地配置模板
├── config.docker.yaml          # Docker 配置
├── docker-compose.yml          # 一键启动配置
└── Dockerfile                  # 后端镜像构建文件
```

更详细的代码现状和模块说明参见 [`docs/architecture/README.md`](docs/architecture/README.md)。

## 快速开始

### 方式一：Docker Compose（推荐）

1. 克隆仓库并进入项目目录：

```bash
git clone https://github.com/LennyFace24/GoAgent.git
cd GoAgent
```

2. 配置服务参数与对话模型：

编辑 `config.docker.yaml` 中的 Embedding 地址、Key、模型名以及 `session.secret_key`；对话模型不再从 YAML 的 `llm` 段读取。

若项目根目录还没有 `model.json`，从模板复制后填写实际模型信息：

```bash
cp model.example.json model.json
```

在 `model.json` 中配置 `default` 和 `models`，每个模型的 `id` 应是提供方接受的模型名称，另填写 `base_url`、`api_key` 等参数。已有本地 `model.json` 可直接复用，不必覆盖。

Compose 将 `model.json` 只读挂载到后端 `/app/model.json`；文件必须提前存在，不会自动创建。修改文件后需重启后端生效，不需要重新构建前端或镜像。容器中的 `localhost` 指容器本身，模型端点必须是容器可访问的地址。

3. 启动所有服务：

```bash
docker compose up -d --build
```

从旧版（含独立 `frontend` 容器）升级时，先用 `docker compose down` 清掉旧容器，或执行 `docker compose up -d --build --remove-orphans`，避免旧 Nginx 容器继续占用 80 端口。

4. 访问服务：

- Web 前端：`http://localhost`（映射到后端 8080）
- 后端 API：`http://localhost:8080`
- 向量服务：`http://localhost:8088`

镜像构建阶段会执行前端 `npm ci && npm run build`，产物复制到 `/app/frontend/dist`，由 Go 在同一端口托管页面和 `/api/*`，不再启动独立 Nginx 容器。

查看日志：

```bash
docker compose logs -f backend
docker compose logs -f chromadb
```

停止服务：

```bash
docker compose down
```

如需同时删除持久化数据，请谨慎执行：

```bash
docker compose down -v
```

### 方式二：本地统一启动（Go 托管前端）

环境要求：

- Go 1.25 或更高版本
- Node.js 20 或更高版本及 npm（仅安装依赖、构建前端或使用 Vite 开发时需要）
- 一个兼容 OpenAI API 的 LLM 服务
- 一个兼容 OpenAI API 的 Embedding 服务
- 可选：本地运行 Chroma/Milvus Lite 服务

复制配置模板并填写配置：

```bash
cp config.example.yaml config.yaml
```

另需在项目根目录准备 `model.json`：若尚不存在，可复制 `model.example.json` 并填写模型的 `id`、`base_url`、`api_key` 等参数；已有配置不要覆盖。模型配置由 `internal/modelmgr` 加载。

首次安装依赖并构建前端，然后启动 Go：

```bash
npm --prefix frontend ci
npm --prefix frontend run build
go mod download
go run ./cmd
```

打开 `http://127.0.0.1:8080`。Go 在同一端口提供页面和 `/api/*` 接口，不需要再运行 Vite。默认监听地址由 `config.yaml` 中的 `server.host` 和 `server.port` 决定。

注意：

- 所有命令均从项目根目录执行；配置、Prompt 和 `frontend/dist` 目前仍按工作目录定位。
- `go run ./cmd` 不会自动运行 npm。修改 Vue 源码后，需重新执行前端构建；修改 Go 代码后需重启 Go。
- 首页、`/index.html` 和 `/assets/*` 只读取构建目录中的文件；不存在的 API、资源或页面返回 404，不回退到首页，也不公开项目源码与配置文件。
- 未找到前端首页时，Go 会打印提示并仅启动 API，保留纯后端及 Vite 开发方式；其他构建目录错误会使启动失败。
- 模型服务、Embedding、Chroma 和远程沙箱等依赖仍需按所用功能配置；统一入口不等于全部依赖离线或自动启动。

需要前端热更新时，仍可使用两个终端：

```bash
# 终端一：Go API
go run ./cmd

# 终端二：Vite 前端
npm --prefix frontend run dev
```

Vite 开发界面默认是 `http://localhost:3000`，通过原有代理访问本机 `8080` 端口。Docker Compose 同样使用 Go 统一入口，镜像内已包含前端构建产物，不再维护独立 Nginx 部署。

如果需要单独启动向量服务：

```bash
cd chroma
pip install -r requirements.txt
python main.py
```

向量服务默认监听 `http://localhost:8088`。

## 配置说明

`config.example.yaml`：

```yaml
server:
  host: 127.0.0.1
  port: 8080

embedding:
  base_url: https://your-embedding.com/v1
  api_key: your-api-key
  model: your-embedding-model

session:
  secret_key: replace-with-a-random-secret

workspace:
  root: /path/to/workspace

sandbox:
  endpoint: http://localhost:8081
```

对话模型使用独立的 `model.json`（参见 `model.example.json`），不使用 YAML 的 `llm` 段。`default` 指向 `models` 中一个条目的 `id`；`context_window`、`max_completion_tokens` 和 `safety_margin_tokens` 也放在对应模型条目中。`embedding` 则仍保留在应用 YAML 中，供知识库向量化使用。

注意事项：

- 不要将真实 API Key、Session Secret 或其他敏感信息提交到 Git 仓库。
- `session.secret_key` 不能为空，建议使用足够长度的随机字符串。
- `workspace.root` 用于限制文件工具可访问的工作区范围。
- 使用 Docker Compose 时，`config.docker.yaml` 挂载为 `/app/config.yaml`，独立的 `model.json` 只读挂载为 `/app/model.json`；向量数据和会话数据分别保存在 Docker volume 中。

## API 接口

后端 API 默认前缀为 `/api`。

### Agent 与权限

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/chat_stream` | 普通流式 Agent 对话 |
| `POST` | `/api/ai_ops` | AIOps 流式诊断 |
| `POST` | `/api/permission/response` | 返回工具执行权限确认结果 |

### 知识库与会话

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/upload_file` | 上传文件并处理到知识库（同名文件覆盖旧向量） |
| `GET` | `/api/documents` | 列出已入库文件名 |
| `DELETE` | `/api/documents/:name` | 删除指定文件名的全部向量 |
| `POST` | `/api/search` | 检索知识库内容 |
| `GET` | `/api/conversations` | 获取会话列表 |
| `POST` | `/api/conversation` | 创建会话，可传 `title` 指定名称 |
| `GET` | `/api/conversation/:id` | 获取指定会话 |
| `PATCH` | `/api/conversation/:id` | 修改会话名称（`{"title":"..."}`，1–100 字符） |
| `DELETE` | `/api/conversation/:id` | 删除指定会话 |

### 系统信息

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/metrics` | 获取运行指标 |
| `GET` | `/api/commands` | 获取可用斜杠命令 |
| `GET` | `/api/model` | 获取当前模型名称 |
| `GET` | `/api/context/status?conversation_id=...` | 获取指定对话的上下文使用状态；`/context` 为兼容路径 |

### 请求示例

创建会话：

```bash
curl -X POST http://localhost:8080/api/conversation \
  -H 'Content-Type: application/json' \
  -d '{}'
```

获取当前模型：

```bash
curl http://localhost:8080/api/model
```

检索知识库：

```bash
curl -X POST http://localhost:8080/api/search \
  -H 'Content-Type: application/json' \
  -d '{"query":"如何部署 GoAgent"}'
```

> 流式对话和文件上传的完整请求字段请以 `internal/handler` 中的 Handler 定义以及前端调用方式为准。项目会继续补充 OpenAPI 文档和更完整的请求/响应示例。

## 数据与持久化

- 会话数据：`data/conversations`
- Docker 会话 volume：`goagent-data`
- Docker 向量数据 volume：`chroma-data`
- 本地运行时请确保进程对工作区和数据目录具有必要的读写权限。

## 当前状态与已知限制

当前已实现的主要能力包括流式 Agent 对话、AIOps 入口、工具注册、文件知识库、会话管理、权限响应、上下文监控，以及 MCP Server 工具接入。

MCP 接入支持：

- 从 `GOAGENT_CONFIG` 指定目录或用户配置目录加载 `mcp_servers.json`。
- 仅支持 stdio、SSE、Streamable HTTP 三种 MCP 传输，远程服务优先使用 Streamable HTTP。
- 在启动时发现远程工具，将其 JSON Schema 转换为 Eino 工具定义，并在 Agent Loop 中转发调用。
- 工具名使用 `mcp__<server>__<tool>`；超长、特殊字符及歧义名称附加稳定哈希，保证模型允许的字符集与 64 字符长度限制。
- MCP 调用复用现有 `allow / ask / deny` 权限链路；未匹配规则时询问用户，批准后才发送 `tools/call`。
- 启动发现失败会清理已连接会话；正常退出信号会停止 HTTP 服务并关闭 MCP 子进程。

以下能力仍在完善中：

- 任务依赖调度系统目前仍是骨架，尚未形成完整的任务编排能力。
- 多 Agent 团队协作尚未实现完整的 Planner/Executor 协同流程。
- 没有独立可切换的 plan 模式；AIOps 中的“计划/执行/重规划”属于提示词流程。连续权限拒绝只提示确认执行范围，不会切换运行模式。
- 长期记忆、MCP Server 配置管理 API 以及运行时无损热替换尚未完成。
- 当前 SDK 会将 `structuredContent` 中的 JSON 数字解码为 `float64`；超过安全整数范围的标识符应由 MCP Server 返回字符串。工具调用入参使用 `json.Number` 保留精度。
- API 请求和响应模型、鉴权策略以及生产环境部署文档仍需要进一步完善。

### MCP 配置

设置 `GOAGENT_CONFIG` 为配置目录，并在其中放置 `mcp_servers.json`，例如：

```json
{
  "version": "1.0",
  "servers": {
    "remote": {
      "enabled": true,
      "transport": "streamable-http",
      "url": "http://127.0.0.1:9000/mcp"
    }
  }
}
```

将 URL 替换为实际 MCP Server 地址，重启 GoAgent 后生效。配置详细说明见 [`docs/mcp-configuration.md`](docs/mcp-configuration.md)。


## 开发与构建

当前前端入口为 `frontend/src/main.ts` → `App.vue`，消息与历史由 `useChat.ts` 管理。旧消息组合函数、未挂载的工具面板、Vue emit 教学示例和独立 Markdown/LaTeX 诊断脚本已清理；自动测试保留在 `*.test.ts` 与 Go `*_test.go` 中。Skill 从目录中的 `SKILL.md` 发现，不再维护静态路由索引。

格式化代码：

```bash
gofmt -w cmd internal
```

运行测试：

```bash
go test ./...
```

构建演示用程序（Windows，从项目根目录执行）：

```powershell
npm --prefix frontend run build
go build -o bin/goagent.exe ./cmd
.\bin\goagent.exe
```

前两条命令负责构建，最后一条才是启动；后续演示可直接执行最后一条。运行时保留 `frontend/dist`、配置和 Prompt 文件，当前不是嵌入前端的单文件发行包。Linux/macOS 可构建为 `go build -o bin/goagent ./cmd` 并执行 `./bin/goagent`。

构建 Docker 镜像（多阶段构建，自动编译前端并打包进运行时镜像）：

```bash
docker build -t goagent .
```

## 安全建议

GoAgent 可以调用 Shell 和文件相关工具，请不要直接将服务暴露到公网。生产环境建议至少：

1. 配置反向代理和 HTTPS。
2. 限制工作区目录，避免授予不必要的文件权限。
3. 使用独立、低权限的运行用户。
4. 为工具调用增加明确的用户确认流程。
5. 妥善保管 API Key、Session Secret 和其他配置密钥。
6. 在部署前审查允许执行的命令及其参数。

## 贡献

欢迎提交 Issue 和 Pull Request。在提交代码前，请确保：

- 代码已通过 `gofmt`。
- 相关测试能够通过。
- 新增配置、API 或工具已同步更新文档。
- 不提交密钥、个人数据和本地运行产物。

## License

MIT
