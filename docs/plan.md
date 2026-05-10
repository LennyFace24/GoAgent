# GoAgent — 项目文档

## 1. 项目概述

GoAgent 是一个面向企业级运维场景的智能问答与诊断平台。通过 LLM 工具调用（Tool Calling）实现知识检索增强生成（RAG）和自动化故障诊断，降低人工 OnCall 成本。

**核心特性：**
- 基于 RAG 的智能问答：上传文档 → 分片 → 向量化 → 语义检索 → LLM 生成回答
- ReAct 编排：显式工具调用循环（调用 → 执行 → 注入结果 → 继续生成），最多 20 轮，todo 提醒机制保障任务可追踪
- 多对话管理：每个 session 支持多个独立对话，前端侧边栏新建/切换/删除
- 多轮对话持久化：基于 JSONL 的会话历史，按 session/conversation 组织，跨进程重启保留
- 流式输出：SSE（Server-Sent Events）逐 token 推送，前端实时渲染
- AIOps 智能诊断：Planner-Executor-Replanner 模式，自动执行健康检查、日志分析、知识检索
- 工具体系：8 个内置工具，覆盖文件读写、bash 执行、系统监控、知识检索、待办管理
- Docker 一键部署：Go 后端 + Vue 前端 + ChromaDB + Prometheus，支持生产环境运行
- Vue 3 调试前端：对话式界面，支持 Markdown 渲染，深浅色主题跟随系统

---

## 2. 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│                     Vue 3 Frontend (:80/:3000)              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  ChatView (3 modes: chat / chat_stream / ai_ops)       │ │
│  │  Sidebar: 对话列表 / 新建对话 / 文件上传 / 文档搜索      │ │
│  └────────────────────────────────────────────────────────┘ │
│                     │ Nginx / Vite Proxy                    │
└─────────────────────┼───────────────────────────────────────┘
                      │
┌─────────────────────┼───────────────────────────────────────┐
│              Go Backend (Gin, :8080)                         │
│  ┌──────────────────┼──────────────────────────────────┐    │
│  │  Handler Layer   │                                  │    │
│  │  ┌────────────┐  │  ┌────────────┐  ┌────────────┐ │    │
│  │  │  /chat     │  │  │/chat_stream│  │  /ai_ops   │ │    │
│  │  │/conversations│ │  │/upload_file│  │  /search   │ │    │
│  │  │/conversation│  │  └────────────┘  └────────────┘ │    │
│  │  └────────────┘  │                                  │    │
│  └──────────────────┼──────────────────────────────────┘    │
│  ┌──────────────────┼──────────────────────────────────┐    │
│  │  Service Layer   │                                  │    │
│  │  ┌────────────┐  │  ┌────────────┐  ┌────────────┐ │    │
│  │  │ChatService │  │  │ChatStream  │  │AIOpsService│ │    │
│  │  │            │  │  │  Service   │  │            │ │    │
│  │  └─────┬──────┘  │  └─────┬──────┘  └─────┬──────┘ │    │
│  │        │         │        │               │        │    │
│  │  ┌─────┴─────────┴────────┴───────────────┴──────┐ │    │
│  │  │       ReAct Loop (max 20 turns)               │ │    │
│  │  │  LLM Stream → ToolCall → Execute → todo check │ │    │
│  │  └───────────────────────────────────────────────┘ │    │
│  └───────────────────────────────────────────────────┘    │
│  ┌───────────────────────────────────────────────────────┐ │
│  │  Tools Layer (ToolHandler 统一管理)                    │ │
│  │  bash / read_file / write_file / edit_file            │ │
│  │  health_check / knowledge_search                      │ │
│  │  write_todo / read_todo                               │ │
│  └───────────────────┬───────────────────────────────────┘ │
│  ┌───────────────────┴───────────────────────────────────┐ │
│  │  Store Layer                                          │ │
│  │  ConversationStore (JSONL 多对话)                      │ │
│  │  ChromaStore (HTTP)                                   │ │
│  └─────────────────────────────┬─────────────────────────┘ │
└────────────────────────────────┼───────────────────────────┘
                                 │
┌────────────────────────────────┼───────────────────────────┐
│         Python ChromaDB Service (FastAPI, :8088)            │
│         PersistentClient → ChromaDB 向量存储                 │
└────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────┐
│         Prometheus (:9090) + Node Exporter (宿主机 :9100)    │
│         系统指标采集 → health_check 工具查询                   │
└────────────────────────────────────────────────────────────┘
```

**Docker 服务编排（docker-compose.yml）：**

| 服务 | 技术栈 | 端口 | 职责 |
|---|--------|------|------|
| frontend | Vue 3 + Nginx | 80 | 对话 UI、文件上传、SSE 流式渲染、API 反向代理 |
| backend | Go + Gin + cloudwego/eino | 8080 | HTTP API、LLM 编排、工具调用、会话持久化 |
| chromadb | Python + FastAPI + ChromaDB | 8088 | 向量存储、语义检索 |
| prometheus | Prometheus | 9090 | 时序数据采集存储，供 health_check 查询 |
| node_exporter | 宿主机独立进程 | 9100 | 主机指标采集（CPU/内存/磁盘/负载） |

---

## 3. API 参考

### 3.1 POST /chat — 非流式问答

**请求：**
```json
{"message": "什么是 RAG？", "conversation_id": "conv_1715151200000"}
```

`conversation_id` 可选，默认 `"default"`。

**响应：**
```json
{"reply": "RAG（Retrieval-Augmented Generation）是一种..."}
```

**行为：** 同步阻塞，等待 LLM 完整生成后一次性返回。最多 3 轮工具调用，超过后强制生成最终回答。超时 120s。

---

### 3.2 POST /chat_stream — 流式问答 (SSE)

**请求：**
```json
{"message": "什么是 RAG？", "conversation_id": "conv_1715151200000"}
```

`conversation_id` 可选，默认 `"default"`。

**响应（SSE 流）：**
```
data: {"content":"RAG"}
data: {"content":"（Retrieval-Augmented"}
data: {"content":" Generation）"}
data: {"content":"是一种"}
...
data: "done"
```

**行为：** 异步流式，LLM 每生成一个 token 立即推送。最多 20 轮工具调用，每 5 轮未调用 todo 工具则注入提醒。超时 120s。

---

### 3.3 POST /ai_ops — 智能运维诊断 (SSE)

**请求：**
```json
{"message": "生产环境 API /order/create 响应时间从 200ms 飙升到 3s，错误率 12%", "conversation_id": "conv_1715151200000"}
```

**响应（SSE 流）：** 与 `/chat_stream` 格式相同。LLM 按 system prompt 中的 Plan-Execute-Replanner-Report 流程自动输出阶段化内容。

**行为：** 最多 20 轮工具调用，超过后强制生成诊断报告。超时 180s。

---

### 3.4 POST /upload_file — 文档上传

**请求：** `multipart/form-data`，字段名 `file`，支持 `.txt` / `.md` / `.json` / `.csv`。

**响应：**
```json
{"status": "ok", "message": "File uploaded and processed successfully", "file": "doc.md"}
```

**处理流程：** 保存临时文件 → 加载文档 → 按 Markdown 标题（`#`/`##`/`###`）分片 → Embedding 向量化 → 写入 ChromaDB `documents` collection。

---

### 3.5 POST /search — 文档检索

**请求：**
```json
{"query": "Go Context 用法", "top_k": 5}
```

`top_k` 可选，默认 5。

**响应：**
```json
{"status": "ok", "results": ["文档片段1...", "文档片段2...", ...]}
```

**行为：** 将 query 文本向量化后在 ChromaDB 中做语义相似度搜索，返回最相关的文档片段。

---

### 3.6 GET /conversations — 获取对话列表

**请求：** 无请求体，通过 session cookie 识别会话。

**响应：**
```json
{
  "conversations": [
    {"id": "conv_1715151200000", "title": "什么是 RAG？", "created_at": "2026-05-08T15:00:00Z", "updated_at": "2026-05-08T15:30:00Z"},
    ...
  ]
}
```

**行为：** 返回当前 session 下所有对话的元数据，按更新时间倒序排列。

---

### 3.7 POST /conversation — 创建新对话

**请求：**
```json
{"title": "新对话"}
```

`title` 可选，默认 `"新对话"`。

**响应：**
```json
{"conversation": {"id": "conv_1715151200000", "title": "新对话", "created_at": "...", "updated_at": "..."}}
```

---

### 3.8 GET /conversation/:id — 获取对话历史

**响应：**
```json
{
  "messages": [
    {"role": "user", "content": "什么是 RAG？"},
    {"role": "assistant", "content": "RAG 是一种..."}
  ]
}
```

---

### 3.9 DELETE /conversation/:id — 删除对话

**响应：**
```json
{"message": "已删除"}
```

---

## 4. 工具参考

LLM 可调用的工具通过 `cloudwego/eino` 的 `utils.InferTool` 注册，使用 JSON Tag 结构体定义输入 schema。所有工具通过 `ToolHandler` 统一创建和管理，服务层通过 `toolHandler.Tools()` 获取完整工具列表。

| 工具名 | 描述 | 输入参数 | 实现状态 | 来源文件 |
|--------|------|----------|----------|----------|
| `bash` | 在服务器上执行 bash 命令 | `command` (string, 必填), `timeout` (int, 可选, 默认30, 最大120) | 真实实现 | `bash_tool.go` |
| `read_file` | 读取本地文件内容，带行号 | `path` (string, 必填), `offset` (int, 可选), `limit` (int, 可选, 默认2000) | 真实实现 | `file_tools.go` |
| `write_file` | 创建或覆盖文件 | `path` (string, 必填), `content` (string, 必填) | 真实实现 | `file_tools.go` |
| `edit_file` | 精确替换文件中的文本 | `path` (string, 必填), `old_text` (string, 必填), `new_text` (string, 必填) | 真实实现 | `file_tools.go` |
| `health_check` | 检查系统健康状态（CPU/内存/磁盘/负载） | `service` (string, 可选) | 真实实现 (Prometheus) | `diagnostic_tool.go` |
| `knowledge_search` | 从知识库中搜索相关文档 | `query` (string, 必填), `top_k` (int, 可选, 默认 5) | 真实实现 (ChromaDB) | `knowledge_tool.go` |
| `write_todo` | 将待办事项文档写入文件 | `path` (string, 必填), `content` (string, 必填, Markdown 格式) | 真实实现 | `todo_tool.go` |
| `read_todo` | 读取待办事项文件查看进度 | `path` (string, 必填), `offset` (int, 可选), `limit` (int, 可选) | 真实实现 | `todo_tool.go` |

**安全机制（bash 工具）：**
- 危险命令拦截：`rm -rf /`、`sudo`、`shutdown` 等 14 种危险模式
- 输出截断：最大 100KB / 1000 行
- 超时限制：最大 120 秒
- 审计日志：所有命令记录到 `data/bash_audit.log`

**安全机制（文件工具）：**
- 路径遍历防护：禁止 `..`
- 敏感目录拦截：`/etc/`、`/sys/`、`/proc/`、`/dev/`、`/boot/`、`/root/.ssh`
- 读取上限：200KB / 5000 行
- 写入上限：1MB

**工具调用流程：**
1. LLM 返回 `ToolCalls`（包含工具名和参数 JSON）
2. 框架按工具名匹配 `tool.InvokableTool` 接口
3. 调用 `InvokableRun(ctx, arguments)` 执行
4. 将返回值作为 `ToolMessage` 注入消息上下文
5. LLM 继续生成（可能再次调用工具，循环最多 N 轮）

---

## 5. 会话与对话管理

**Session 中间件：**
- `gin-contrib/sessions` + `cookie.NewStore`：Session 数据加密存储在客户端 Cookie 中
- `SessionIDMiddleware`：首次请求时生成 UUID 作为 `session_id`，后续请求从 Cookie 读取
- Cookie 名：`goagent_session`

**多对话持久化：**
- 目录结构：`data/conversations/<session_id>/<conversation_id>.jsonl`
- 每个对话独立文件，每行一个 JSON 对象：`{"role": "user", "content": "..."}`
- 对话元数据：`<conversation_id>.meta.json`，包含 id、title、created_at、updated_at
- `SaveMessages`：`O_APPEND|O_CREATE|O_WRONLY` 追加写入，同时更新 meta 的 updated_at
- `LoadHistory`：`bufio.Scanner` 逐行解析
- `ListConversations`：读取 session 目录下所有 `.meta.json`，按 updated_at 倒序
- 零外部依赖，纯标准库实现

**多轮对话上下文构建：**
```
[system prompt] + [历史消息...] + [当前用户消息] → LLM
```
每次请求都从 JSONL 文件加载完整历史，注入 LLM 上下文。对话在 LLM 的 `max_completion_tokens` 限制内自动截断（由 LLM API 处理）。

**Todo 提醒机制：**
- 每轮检测是否调用了 `write_todo` 或 `read_todo`
- 连续 5 轮未调用 → 在 tool result 中注入系统提醒
- 计数器在提醒后归零，重新开始累积

---

## 6. ReAct 编排策略

### 6.1 Chat / Chat Stream — 显式工具调用循环

```
用户消息
    │
    ▼
for turn = 0..19:
    ├── LLM.Stream(messages)  + stream.Copy(2) 分流
    ├── 一份流推送给前端，一份拼接检测 ToolCalls
    ├── 检查 ToolCalls
    │   ├── 无 ToolCall → 返回最终回答，结束
    │   └── 有 ToolCall → 逐个执行工具
    │       ├── 检测是否包含 write_todo / read_todo
    │       ├── 工具执行失败 → 将错误信息作为 tool result 返回（不中断）
    │       └── 注入 ToolMessage，继续循环
    ├── todo 检查
    │   ├── 调用了 todo → 计数器归零
    │   └── 未调用 → 计数器+1，累积 5 轮注入提醒
    │
    └── (turn == 19 仍调用工具 → fallback: 注入 UserMessage 强制回答)
```

**关键设计决策：**
- 不使用 Eino 框架的 `ChatModelAgent`（IoC 有向无环图），改用手写 `for` 循环（命令式编排）
- 原因：Eino Agent 将"是否继续调用工具"的决策委托给 LLM，但 LLM 没有元认知能力（不知道何时该停），导致无限循环
- 方案：显式限制轮次（max 20），超限后注入 UserMessage 强制模型转向回答
- 工具执行失败不中断对话，错误信息作为 tool result 返回给 LLM 自行处理

### 6.2 AIOps — Planner-Executor-Replanner

与 Chat 使用相同的编排逻辑，区别仅在于：
- System prompt 定义了四阶段工作流（Plan/Execute/Replan/Report），阶段划分由 LLM 自行在文本中输出，后端不结构化解析
- 超时 180s（Chat 为 120s）

---

## 7. 配置参考

**文件：** `config.yaml`（程序根目录，不提交到 Git）

```yaml
server:
  host: 0.0.0.0            # 监听地址（Docker 中需 0.0.0.0）
  port: "8080"             # 监听端口
llm:
  base_url: https://...    # OpenAI 兼容 API 地址
  api_key: sk-xxx          # API 密钥
  model: mimo-v2.5-pro     # 模型名称
  max_completion_tokens: 8192  # 最大输出 token 数
embedding:
  base_url: https://...    # Embedding API 地址
  api_key: sk-xxx          # Embedding API 密钥
  model: Qwen3-Embedding-8B  # Embedding 模型名
prometheus:
  url: http://prometheus:9090  # Prometheus API 地址（Docker 内用服务名）
chromadb:
  url: http://chromadb:8088    # ChromaDB API 地址（Docker 内用服务名）
```

`config.yaml.example` 提供模板，不含真实密钥。

---

## 8. 前端

**技术栈：** Vue 3 + Vite + marked + DOMPurify

**结构：**
```
frontend/
├── index.html
├── vite.config.js          # 开发服务器 + API 代理
├── package.json
└── src/
    ├── main.js
    ├── App.vue              # 根组件：侧边栏 + ChatView
    └── components/
        └── ChatView.vue     # 主聊天界面
```

**功能：**
- **多对话管理**：侧边栏对话列表，支持新建/切换/删除对话，自动以用户第一条消息作为标题
- **三种对话模式**：Chat Stream（SSE 流式）/ Chat（非流式）/ AI Ops（SSE 诊断），模式选择器位于输入框上方
- **消息气泡**：用户消息右对齐，AI 消息左对齐，错误消息红色高亮
- **Markdown 渲染**：AI 回复支持标题、列表、代码块、表格、引用等，使用 `marked` 解析 + `DOMPurify` 防 XSS
- **流式光标**：AI 生成期间气泡末尾显示闪烁光标
- **对话历史**：切换对话时自动加载该对话的历史消息
- **文件工具**：左侧栏集成文档上传和语义搜索
- **侧边栏可拖拽**：180px-400px 自由调整，拖到左边缘自动折叠，有展开按钮
- **主题跟随系统**：CSS 变量 + `prefers-color-scheme` 媒体查询
- **自定义滚动条**：6px 宽度，半透明圆角滑块

**API 请求：** 所有聊天请求自动携带 `conversation_id`，前端不传时默认 `"default"`。

**Vite 代理配置（开发模式）：**
```js
proxy: {
  '^/(chat|chat_stream|ai_ops|upload_file|search|history|conversations?|conversation)': {
    target: 'http://127.0.0.1:8080',
    changeOrigin: true,
  },
}
```

**Nginx 配置（生产模式）：** 前端静态文件 + API 反向代理到 `backend:8080`，正则规则与 Vite 代理一致。

---

## 9. 项目结构

```
GoAgent/
├── cmd/
│   └── main.go                    # 程序入口：加载配置、初始化中间件和路由
├── internal/
│   ├── config/
│   │   └── config.go              # YAML 配置加载（sync.Once 单例，含 prometheus/chromadb）
│   ├── middleware/
│   │   └── session.go             # Session ID 中间件（UUID 生成）
│   ├── handler/
│   │   ├── routes.go              # 路由注册 + ToolHandler 初始化
│   │   ├── chat_handler.go        # POST /chat
│   │   ├── chat_stream_handler.go # POST /chat_stream (SSE)
│   │   ├── aiops_handler.go       # POST /ai_ops (SSE)
│   │   ├── conversation_handler.go # /conversations, /conversation CRUD
│   │   └── file_handler.go        # POST /upload_file, POST /search
│   ├── service/
│   │   ├── chat_service.go        # 非流式 ReAct 编排
│   │   ├── chat_stream_service.go # 流式 ReAct 编排（max 20 轮 + todo 提醒）
│   │   ├── aiops_service.go       # AIOps Planner-Executor 编排
│   │   └── file_service.go        # 文档加载、分片、向量化、入库
│   ├── tools/
│   │   ├── tool_handler.go        # ToolHandler 统一管理所有工具
│   │   ├── bash_tool.go           # bash 工具（危险命令拦截、审计日志、输出截断）
│   │   ├── file_tools.go          # read_file / write_file / edit_file
│   │   ├── diagnostic_tool.go     # health_check（真实 Prometheus 查询）
│   │   ├── knowledge_tool.go      # knowledge_search（ChromaDB 语义检索）
│   │   └── todo_tool.go           # write_todo / read_todo（Markdown 文档）
│   └── store/
│       ├── conversation.go        # JSONL 多对话存储 + ConversationMeta
│       └── chroma.go              # ChromaDB HTTP 客户端
├── frontend/                      # Vue 3 前端（详见第 8 节）
├── chroma/                        # Python ChromaDB 向量数据库服务
├── test_docs/                     # 测试文档（RAG 测试用）
├── docs/
│   └── plan.md                    # 本文档
├── Dockerfile                     # Go 后端多阶段构建（Alpine）
├── docker-compose.yml             # 编排：frontend + backend + chromadb + prometheus
├── config.docker.yaml             # Docker 环境配置（服务名访问）
├── prometheus.yml                 # Prometheus 采集配置
├── config.yaml                    # 本地开发配置（含密钥，不提交 Git）
├── config.yaml.example            # 配置模板
├── data/conversations/            # 对话历史 JSONL 文件（不提交 Git）
└── data/bash_audit.log            # bash 命令审计日志（不提交 Git）
```

---

## 10. 实施状态

### 已完成

| 功能 | 接口/位置 | 说明 |
|------|-----------|------|
| 文档上传 → 分片 → 向量化 → 入库 | `POST /upload_file` | Markdown Header Splitter + OpenAI Embedding + ChromaDB |
| 文档语义检索 | `POST /search` | Embedding 向量化 + ChromaDB 相似度搜索 |
| 流式智能问答 (SSE + ReAct) | `POST /chat_stream` | Stream Copy(2) 分流，max 20 轮，todo 提醒机制 |
| 非流式问答 (ReAct) | `POST /chat` | 同步 Generate，完整响应一次性返回 |
| AIOps 智能诊断 (SSE) | `POST /ai_ops` | Planner-Executor-Replanner，复用 chat 编排逻辑 |
| 多对话管理 API | `GET/POST/DELETE /conversation(s)` | 创建/列表/查看/删除对话，ConversationMeta 元数据 |
| 多对话持久化 | `store/conversation.go` | JSONL 多对话，session_id/conversation_id 二级目录 |
| 会话管理 | `middleware/session.go` | Cookie-based Session，UUID 标识 |
| ToolHandler 统一工具管理 | `tools/tool_handler.go` | 8 个工具集中创建，服务层按需获取 |
| bash 工具 | `tools/bash_tool.go` | 危险命令拦截、输出截断（100KB/1000行）、审计日志、超时控制 |
| 文件读写编辑工具 | `tools/file_tools.go` | read_file / write_file / edit_file，路径安全校验 |
| health_check 真实实现 | `tools/diagnostic_tool.go` | Prometheus HTTP API 查询 CPU/内存/磁盘/负载/up 指标 |
| knowledge_search | `tools/knowledge_tool.go` | ChromaDB 语义检索 |
| 待办事项工具 | `tools/todo_tool.go` | write_todo / read_todo，Markdown 文档格式 |
| Vue 3 前端 | `frontend/` | 多对话侧边栏、Markdown 渲染、SSE 流式、可拖拽侧边栏、深浅色主题 |
| Docker 部署 | `docker-compose.yml` | 一键部署：frontend + backend + chromadb + prometheus |
| Prometheus 集成 | `prometheus.yml` + node_exporter | 真实系统指标采集，供 health_check 查询 |

### 待实现

| 功能 | 说明 |
|------|------|
| 单元测试 | handler / service / store 各层测试 |
| README | 项目根目录 README.md |
| MCP 协议集成 | 通过 Model Context Protocol 接入外部工具 |
| Milvus 向量存储 | 替换 ChromaDB，支持分布式向量检索 |
| SSE phase 结构化 | 前端区分 Plan/Execute/Replan/Report 阶段并分别展示 |
| 用户登录系统 | 用户认证，跨设备对话同步 |
| AI 自动标题 | 对话标题由 AI 根据内容自动生成 |
