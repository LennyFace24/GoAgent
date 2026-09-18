# GoAgent

一个基于 Go、Eino 和 Gin 构建的可扩展 AI Agent 服务。项目提供流式对话、AIOps 诊断、文件知识库检索、工具调用、会话管理以及上下文状态查询等能力，并通过 Docker Compose 集成前端、后端和 Chroma 向量数据库。

> 项目当前仍处于持续开发阶段。README 会尽量按照当前代码库的实际实现进行说明，未完成的功能会明确标注。

## 功能概览

- **流式智能对话**：通过 ReAct Agent 调用已注册工具，并以流式方式返回执行过程和回答。
- **AIOps 诊断**：提供面向运维问题的 Agent 入口，可结合指标查询工具进行分析。
- **工具调用**：支持 Bash 执行、文件读写、知识库检索、指标查询以及子 Agent 代理等工具。
- **安全控制**：对 Bash 命令进行敏感操作拦截，并通过工作区限制文件访问范围。
- **知识库检索**：结合 Embedding 模型与 Chroma/Milvus Lite 服务实现文档向量检索。
- **会话管理**：支持创建、查询、删除对话，并将会话数据持久化到本地 `data/conversations` 目录。
- **权限响应**：当工具执行需要用户确认时，可通过权限接口返回确认结果。
- **上下文监控**：提供当前上下文使用量查询，避免超过模型上下文窗口。
- **前后端分离**：后端提供 HTTP API，前端通过 Nginx 提供 Web 界面。

## 技术栈

- **后端**：Go 1.25、Gin、Eino
- **Agent 编排**：CloudWeGo Eino ReAct
- **大语言模型**：兼容 OpenAI API 格式的模型服务
- **Embedding**：兼容 OpenAI API 格式的 Embedding 服务
- **向量数据库**：Chroma 服务（基于 Milvus Lite）
- **前端**：Node.js 20、Nginx
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

2. 编辑 Docker 配置：

```bash
cp config.docker.yaml config.local.yaml
```

然后将实际使用的配置写入 `config.docker.yaml`，至少需要设置：

- `llm.base_url`
- `llm.api_key`
- `llm.model`
- `embedding.base_url`
- `embedding.api_key`
- `embedding.model`
- `session.secret_key`

3. 启动所有服务：

```bash
docker compose up -d --build
```

4. 访问服务：

- Web 前端：`http://localhost`
- 后端 API：`http://localhost:8080`
- 向量服务：`http://localhost:8088`

查看日志：

```bash
docker compose logs -f backend
docker compose logs -f frontend
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

### 方式二：本地运行后端

环境要求：

- Go 1.25 或更高版本
- 一个兼容 OpenAI API 的 LLM 服务
- 一个兼容 OpenAI API 的 Embedding 服务
- 可选：本地运行 Chroma/Milvus Lite 服务

复制配置模板并填写配置：

```bash
cp config.example.yaml config.yaml
```

安装依赖并启动：

```bash
go mod download
go run ./cmd/main.go
```

默认监听地址为 `127.0.0.1:8080`。配置文件中的 `server.host` 和 `server.port` 可以修改监听地址。

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

llm:
  base_url: https://your-api.com/v1/
  api_key: your-api-key
  model: your-model
  context_window: 200000
  max_completion_tokens: 8192
  safety_margin_tokens: 4096

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

注意事项：

- 不要将真实 API Key、Session Secret 或其他敏感信息提交到 Git 仓库。
- `session.secret_key` 不能为空，建议使用足够长度的随机字符串。
- `workspace.root` 用于限制文件工具可访问的工作区范围。
- 使用 Docker Compose 时，后端容器通过 `config.docker.yaml` 加载配置，向量数据和会话数据分别保存在 Docker volume 中。

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
| `POST` | `/api/upload_file` | 上传文件并处理到知识库 |
| `POST` | `/api/search` | 检索知识库内容 |
| `GET` | `/api/conversations` | 获取会话列表 |
| `POST` | `/api/conversation` | ��建会话 |
| `GET` | `/api/conversation/:id` | 获取指定会话 |
| `DELETE` | `/api/conversation/:id` | 删除指定会话 |

### 系统信息

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/metrics` | 获取运行指标 |
| `GET` | `/api/commands` | 获取可用斜杠命令 |
| `GET` | `/api/model` | 获取当前模型名称 |
| `GET` | `/context` | 获取上下文使用状态 |

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

当前已实现的主要能力包括流式 Agent 对话、AIOps 入口、工具注册、文件知识库、会话管理、权限响应和上下文监控。

以下能力仍在完善中：

- 任务依赖调度系统目前仍是骨架，尚未形成完整的任务编排能力。
- 多 Agent 团队协作尚未实现完整的 Planner/Executor 协同流程。
- `todo_tool` 等部分工具可能已存在代码实现，但还未完全接入默认工具注册流程。
- 长期记忆、MCP Server 自由导入等规划能力尚未完成。
- API 请求和响应模型、鉴权策略以及生产环境部署文档仍需要进一步完善。

## 开发与构建

格式化代码：

```bash
gofmt -w cmd internal
```

运行测试：

```bash
go test ./...
```

构建后端：

```bash
go build -o goagent ./cmd/main.go
```

构建 Docker 镜像：

```bash
docker build -t goagent-backend .
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

当前仓库尚未声明明确的开源许可证。若要将项目用于分发或商业用途，请先确认仓库后续发布的许可证信息。
