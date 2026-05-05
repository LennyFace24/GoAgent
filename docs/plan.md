# GoAgent — 项目文档

## 1. 项目概述

GoAgent 是一个面向企业级运维场景的智能问答与诊断平台。通过 LLM 工具调用（Tool Calling）实现知识检索增强生成（RAG）和自动化故障诊断，降低人工 OnCall 成本。

**核心特性：**
- 基于 RAG 的智能问答：上传文档 → 分片 → 向量化 → 语义检索 → LLM 生成回答
- ReAct 编排：显式工具调用循环（调用 → 执行 → 注入结果 → 继续生成），最多 3 轮后强制回答
- 多轮对话持久化：基于 JSONL 的会话历史，跨进程重启保留
- 流式输出：SSE（Server-Sent Events）逐 token 推送，前端实时渲染
- AIOps 智能诊断：Planner-Executor-Replanner 模式，自动执行健康检查、日志分析、知识检索
- Vue 3 调试前端：对话式界面，支持 Markdown 渲染，深浅色主题跟随系统

---

## 2. 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│                     Vue 3 Frontend (:3000)                  │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  ChatView (3 modes: chat / chat_stream / ai_ops)       │ │
│  │  Sidebar: 文件上传 / 文档搜索                           │ │
│  └────────────────────────────────────────────────────────┘ │
│                     │ Vite Proxy                            │
└─────────────────────┼───────────────────────────────────────┘
                      │
┌─────────────────────┼───────────────────────────────────────┐
│              Go Backend (Gin, :8080)                         │
│  ┌──────────────────┼──────────────────────────────────┐    │
│  │  Handler Layer   │                                  │    │
│  │  ┌────────────┐  │  ┌────────────┐  ┌────────────┐ │    │
│  │  │  /chat     │  │  │/chat_stream│  │  /ai_ops   │ │    │
│  │  │  /history  │  │  │/upload_file│  │  /search   │ │    │
│  │  └────────────┘  │  └────────────┘  └────────────┘ │    │
│  └──────────────────┼──────────────────────────────────┘    │
│  ┌──────────────────┼──────────────────────────────────┐    │
│  │  Service Layer   │                                  │    │
│  │  ┌────────────┐  │  ┌────────────┐  ┌────────────┐ │    │
│  │  │ChatService │  │  │ChatStream  │  │AIOpsService│ │    │
│  │  │            │  │  │  Service   │  │            │ │    │
│  │  └─────┬──────┘  │  └─────┬──────┘  └─────┬──────┘ │    │
│  │        │         │        │               │        │    │
│  │  ┌─────┴─────────┴────────┴───────────────┴──────┐ │    │
│  │  │       ReAct / Planner-Executor Loop           │ │    │
│  │  │  LLM Generate/Stream → ToolCall → Execute     │ │    │
│  │  └───────────────────────────────────────────────┘ │    │
│  └───────────────────────────────────────────────────┘    │
│  ┌───────────────────────────────────────────────────────┐ │
│  │  Tools Layer                                          │ │
│  │  knowledge_search │ health_check │ log_analyzer       │ │
│  └───────────────────┬───────────────────────────────────┘ │
│  ┌───────────────────┴───────────────────────────────────┐ │
│  │  Store Layer                                          │ │
│  │  ConversationStore (JSONL)  │  ChromaStore (HTTP)     │ │
│  └─────────────────────────────┼─────────────────────────┘ │
└────────────────────────────────┼───────────────────────────┘
                                 │
┌────────────────────────────────┼───────────────────────────┐
│         Python ChromaDB Service (FastAPI, :8088)            │
│         PersistentClient → ChromaDB 向量存储                 │
└────────────────────────────────────────────────────────────┘
```

**三层服务：**

| 层 | 技术栈 | 端口 | 职责 |
|---|--------|------|------|
| Frontend | Vue 3 + Vite + marked + DOMPurify | 3000 | 对话 UI、文件上传、SSE 流式渲染 |
| Backend | Go + Gin + cloudwego/eino | 8080 | HTTP API、LLM 编排、工具调用、会话持久化 |
| Vector DB | Python + FastAPI + ChromaDB | 8088 | 向量存储、语义检索 |

---

## 3. API 参考

### 3.1 POST /chat — 非流式问答

**请求：**
```json
{"message": "什么是 RAG？"}
```

**响应：**
```json
{"reply": "RAG（Retrieval-Augmented Generation）是一种..."}
```

**行为：** 同步阻塞，等待 LLM 完整生成后一次性返回。最多 3 轮工具调用，超过后强制生成最终回答。超时 120s。

---

### 3.2 POST /chat_stream — 流式问答 (SSE)

**请求：**
```json
{"message": "什么是 RAG？"}
```

**响应（SSE 流）：**
```
data: {"content":"RAG"}
data: {"content":"（Retrieval-Augmented"}
data: {"content":" Generation）"}
data: {"content":"是一种"}
...
data: "done"
```

**行为：** 异步流式，LLM 每生成一个 token 立即推送。工具调用期间的中间消息（ToolCall 构造块、工具返回值）不会推送给前端，只推送最终回答。超时 120s。

---

### 3.3 POST /ai_ops — 智能运维诊断 (SSE)

**请求：**
```json
{"message": "生产环境 API /order/create 响应时间从 200ms 飙升到 3s，错误率 12%"}
```

**响应（SSE 流）：** 与 `/chat_stream` 格式相同。LLM 按 system prompt 中的 Plan-Execute-Replanner-Report 流程自动输出阶段化内容。

**行为：** 最多 5 轮工具调用，超过后强制生成诊断报告。超时 180s。配备 3 个专用工具：`health_check`、`log_analyzer`、`knowledge_search`。

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

### 3.6 GET /history — 获取对话历史

**请求：** 无请求体，通过 session cookie 识别会话。

**响应：**
```json
[
  {"role": "user", "content": "什么是 RAG？"},
  {"role": "assistant", "content": "RAG 是一种..."},
  ...
]
```

**行为：** 读取 `data/conversations/<session_id>.jsonl` 文件，返回该会话的全部对话记录。

---

## 4. 工具参考

LLM 可调用的工具通过 `cloudwego/eino` 的 `utils.InferTool` 注册，使用 JSON Tag 结构体定义输入 schema。

| 工具名 | 描述 | 输入参数 | 实现状态 | 使用者 |
|--------|------|----------|----------|--------|
| `knowledge_search` | 从知识库中搜索相关文档 | `query` (string, 必填), `top_k` (int, 可选, 默认 5) | 真实实现 (ChromaDB) | ChatService, ChatStreamService, AIOpsService |
| `health_check` | 检查指定服务的健康状态 | `service` (string, 必填) | Mock 实现 | AIOpsService |
| `log_analyzer` | 分析指定服务的错误日志 | `service` (string, 必填), `time_range` (string, 可选), `keyword` (string, 可选) | Mock 实现 | AIOpsService |

**工具调用流程：**
1. LLM 返回 `ToolCalls`（包含工具名和参数 JSON）
2. 框架按工具名匹配 `tool.InvokableTool` 接口
3. 调用 `InvokableRun(ctx, arguments)` 执行
4. 将返回值作为 `ToolMessage` 注入消息上下文
5. LLM 继续生成（可能再次调用工具，循环最多 N 轮）

---

## 5. 会话管理

**Session 中间件：**
- `gin-contrib/sessions` + `cookie.NewStore`：Session 数据加密存储在客户端 Cookie 中
- `SessionIDMiddleware`：首次请求时生成 UUID 作为 `session_id`，后续请求从 Cookie 读取
- Cookie 名：`goagent_session`

**对话持久化：**
- 每个 session 一个 JSONL 文件：`data/conversations/<session_id>.jsonl`
- 每行一个 JSON 对象：`{"role": "user", "content": "..."}` 或 `{"role": "assistant", "content": "..."}`
- `SaveMessages`：`O_APPEND|O_CREATE|O_WRONLY` 追加写入
- `LoadHistory`：`bufio.Scanner` 逐行解析
- 零外部依赖，纯标准库实现

**多轮对话上下文构建：**
```
[system prompt] + [历史消息...] + [当前用户消息] → LLM
```
每次请求都从 JSONL 文件加载完整历史，注入 LLM 上下文。对话在 LLM 的 `max_completion_tokens` 限制内自动截断（由 LLM API 处理）。

---

## 6. ReAct 编排策略

### 6.1 Chat / Chat Stream — 显式工具调用循环

```
用户消息
    │
    ▼
for turn = 0..2:
    ├── LLM.Generate/Stream(messages)
    ├── 检查 ToolCalls
    │   ├── 无 ToolCall → 返回最终回答，结束
    │   └── 有 ToolCall → 执行工具，注入 ToolMessage，继续循环
    │
    └── (turn == 2 仍调用工具 → fallback: 注入 UserMessage 强制回答)
```

**关键设计决策：**
- 不使用 Eino 框架的 `ChatModelAgent`（IoC 有向无环图），改用手写 `for` 循环（命令式编排）
- 原因：Eino Agent 将"是否继续调用工具"的决策委托给 LLM，但 LLM 没有元认知能力（不知道何时该停），导致无限循环
- 方案：显式限制轮次（max 3），超限后注入 UserMessage 强制模型转向回答

### 6.2 AIOps — Planner-Executor-Replanner

```
用户故障描述
    │
    ▼
for turn = 0..4:
    ├── LLM.Stream(messages)
    │   ├── system prompt 指导 LLM 按阶段输出：
    │   │   Plan → Execute → Replan → Report
    │   ├── LLM 自主决定调用哪些工具、以什么顺序
    │   └── 工具返回结果后，LLM 自行评估是否需要重规划
    ├── 检查 ToolCalls
    │   ├── 无 ToolCall → 诊断完成，结束
    │   └── 有 ToolCall → 执行工具，注入 ToolMessage，继续循环
    │
    └── (turn == 4 仍调用工具 → fallback: 注入 UserMessage 强制生成报告)
```

**与 Chat 的区别：**
- 最大轮次从 3 提升到 5（诊断需要更多工具调用）
- System prompt 定义了四阶段工作流（Plan/Execute/Replan/Report），但阶段划分由 LLM 自行在文本中输出，后端不结构化解析
- 工具集专用：`health_check` + `log_analyzer` + `knowledge_search`（Chat 只有 `knowledge_search`）

---

## 7. 配置参考

**文件：** `config.yaml`（程序根目录，不提交到 Git）

```yaml
server:
  host: localhost          # 监听地址
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
- **三种对话模式**：Chat Stream（SSE 流式）/ Chat（非流式）/ AI Ops（SSE 诊断），模式选择器位于输入框上方
- **消息气泡**：用户消息右对齐，AI 消息左对齐，错误消息红色高亮
- **Markdown 渲染**：AI 回复支持标题、列表、代码块、表格、引用等，使用 `marked` 解析 + `DOMPurify` 防 XSS
- **流式光标**：AI 生成期间气泡末尾显示闪烁光标
- **对话历史**：页面加载时自动 `GET /history` 加载历史消息
- **文件工具**：左侧栏集成文档上传和语义搜索
- **侧边栏可拖拽**：180px-400px 自由调整，拖到左边缘自动折叠，有展开按钮
- **主题跟随系统**：CSS 变量 + `prefers-color-scheme` 媒体查询
- **自定义滚动条**：6px 宽度，半透明圆角滑块

**Vite 代理配置：**
```js
// 开发模式下，/chat 等请求代理到 Go 后端
proxy: {
  '^/(chat|chat_stream|ai_ops|upload_file|search|history)': {
    target: 'http://127.0.0.1:8080',
    changeOrigin: true,
  },
}
```

---

## 9. 项目结构

```
GoAgent/
├── cmd/
│   └── main.go                    # 程序入口：加载配置、初始化中间件和路由
├── internal/
│   ├── config/
│   │   └── config.go              # YAML 配置加载（sync.Once 单例）
│   ├── middleware/
│   │   └── session.go             # Session ID 中间件（UUID 生成）
│   ├── handler/
│   │   ├── routes.go              # 路由注册 + GetHistory handler
│   │   ├── chat_handler.go        # POST /chat
│   │   ├── chat_stream_handler.go # POST /chat_stream (SSE)
│   │   ├── aiops_handler.go       # POST /ai_ops (SSE)
│   │   └── file_handler.go        # POST /upload_file, POST /search
│   ├── service/
│   │   ├── chat_service.go        # 非流式 ReAct 编排
│   │   ├── chat_stream_service.go # 流式 ReAct 编排
│   │   ├── aiops_service.go       # AIOps Planner-Executor 编排
│   │   └── file_service.go        # 文档加载、分片、向量化、入库
│   ├── tools/
│   │   ├── knowledge_tool.go      # knowledge_search 工具
│   │   └── diagnostic_tool.go     # health_check + log_analyzer 工具 (mock)
│   └── store/
│       ├── conversation.go        # JSONL 对话历史存储
│       └── chroma.go              # ChromaDB HTTP 客户端
├── frontend/                      # Vue 3 前端（详见第 8 节）
├── milvus-service/                # Python ChromaDB 向量数据库服务
├── test_docs/                     # 测试文档（RAG 测试用）
├── docs/
│   └── plan.md                    # 本文档
├── config.yaml                    # 配置文件（含密钥，不提交 Git）
├── config.yaml.example            # 配置模板
└── data/conversations/            # 对话历史 JSONL 文件（不提交 Git）
```

---

## 10. v0.1 实施状态

### 已完成

| 功能 | 接口 | 说明 |
|------|------|------|
| 文档上传 → 分片 → 向量化 → 入库 | `POST /upload_file` | Markdown Header Splitter + OpenAI Embedding + ChromaDB |
| 文档语义检索 | `POST /search` | Embedding 向量化 + ChromaDB 相似度搜索 |
| 流式智能问答 (SSE + ReAct) | `POST /chat_stream` | Stream Copy(2) 分流：前端渲染 + ToolCall 检测 |
| 非流式问答 (ReAct) | `POST /chat` | 同步 Generate，完整响应一次性返回 |
| AIOps 智能诊断 (SSE) | `POST /ai_ops` | Planner-Executor-Replanner，max 5 轮 |
| 对话历史查询 | `GET /history` | JSONL 文件读取，前端页面加载时自动获取 |
| 多轮对话持久化 | 内部 | JSONL 文件，零外部依赖，纯标准库 |
| 会话管理 | 内部 | Cookie-based Session，UUID 标识 |
| Vue 3 调试前端 | 内部 | 对话式 UI、Markdown 渲染、可拖拽侧边栏、深浅色主题 |

### 待实现

| 功能 | 说明 |
|------|------|
| 单元测试 | handler / service / store 各层测试 |
| README | 项目根目录 README.md |
| MCP 协议集成 | 通过 Model Context Protocol 接入外部工具 |
| Milvus 向量存储 | 替换 ChromaDB，支持分布式向量检索 |
| health_check 真实实现 | 接入实际监控系统（Prometheus / Grafana API） |
| log_analyzer 真实实现 | 接入实际日志系统（ELK / Loki） |
| SSE phase 结构化 | 前端区分 Plan/Execute/Replan/Report 阶段并分别展示 |
| 会话管理 UI | 多会话列表、新建/切换/删除对话 |
