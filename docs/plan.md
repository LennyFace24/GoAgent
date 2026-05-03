# 智能 OnCall Agent — 项目概览与实施计划

## 项目背景

传统企业 OnCall 值班依赖人工值守和问题排查，效率低下。智能 OnCall Agent 面向企业级运维场景，通过自动化和智能化手段降低人力成本，提升响应速度和问题解决效率。

## 三大核心 Agent 架构

```
                         用户请求
                            │
              ┌─────────────┼─────────────┐
              ▼             ▼             ▼
        ┌──────────┐ ┌──────────┐ ┌──────────┐
        │ 知识库Agent│ │ 对话Agent │ │ 运维Agent │
        │  (RAG)   │ │  (NLU)   │ │ (AIOps)  │
        └────┬─────┘ └────┬─────┘ └────┬─────┘
             │            │            │
             └────────────┼────────────┘
                          │
              ┌───────────┼───────────┐
              ▼           ▼           ▼
         向量数据库    LLM Client    Tools/MCP
```

- **知识库 Agent (RAG)**：基于检索增强生成，整合向量数据库，支持智能问答、多轮对话、流式输出和自动工具调用。
- **对话 Agent (NLU)**：自然语言理解与多轮对话管理，理解复杂语境，主动追问缺失信息。
- **运维 Agent (AIOps)**：Planner-Executor-Replanner 模式，自动执行告警分析、日志查询、智能诊断和运维报告生成。

---

## v0.1 实施进度

### 已完成

| 功能 | 接口 | 状态 |
|------|------|------|
| 文档上传 → 分片 → 向量化 → 入库 | `POST /upload_file` | ✅ |
| 文档检索 | `POST /search` | ✅ |
| 流式智能问答 (SSE + ReAct) | `POST /chat_stream` | ✅ |
| 普通 HTTP 问答 (ReAct) | `POST /chat` | ✅ |
| 多轮对话持久化 (JSONL) | 内部 | ✅ |
| 命令行式 ReAct 编排 | 内部 | ✅ |

### 进行中

| 功能 | 接口 | 状态 |
|------|------|------|
| 运维 Agent (AIOps) | `POST /ai_ops` | 🔨 |

### 待开始

| 功能 | 说明 |
|------|------|
| 单元测试 + README | v0.1 交付标准 |
| MCP 协议集成 | 外部工具接入 |
| Milvus 向量存储 | 替换内存方案 |

---

## `/ai_ops` 接口规划

### 请求

```
POST /ai_ops
Content-Type: application/json

{
  "problem": "生产环境 API /order/create 响应时间从 200ms 飙升到 3s，错误率 12%",
  "context": {
    "service": "order-service",
    "time_range": "2026-05-03 14:00 - 14:30",
    "logs": "可选的日志片段"
  }
}
```

### 响应

SSE 流式，前端实时看到排查进度：

```
data: {"phase":"plan","content":"1. 查询 order-service 最近 30 分钟错误日志\n2. 检查数据库连接池状态\n3. 分析近期部署变更\n4. 汇总诊断报告"}

data: {"phase":"execute","step":1,"content":"正在查询错误日志..."}

data: {"phase":"execute","step":1,"result":"发现大量 timeout 错误: dial tcp 10.0.1.5:3306: i/o timeout"}

data: {"phase":"execute","step":2,"content":"正在检查数据库连接池..."}

data: {"phase":"execute","step":2,"result":"连接池已耗尽，max=50, active=50, pending=230"}

data: {"phase":"replan","content":"数据库连接池耗尽导致级联超时，调整计划：检查慢查询和连接泄漏"}

data: {"phase":"execute","step":3,"content":"检查慢查询..."}

data: {"phase":"execute","step":3,"result":"发现慢查询: SELECT * FROM orders WHERE ... (avg 4.5s)"}

data: {"phase":"report","content":"## 诊断报告\n\n### 根因\n数据库慢查询导致连接池耗尽...\n\n### 建议\n1. 优化索引\n2. 增加连接池上限"}

data: "done"
```

### Planner-Executor-Replanner 流程

```
用户输入问题
    │
    ▼
[Planner] LLM 分析问题 → 生成排查计划 (步骤列表)
    │
    ▼
[Executor] 按计划逐步执行工具调用 → 收集结果
    │
    ▼
[Replanner] LLM 审阅执行结果
    ├─ 信息不足 → 调整计划 → 回到 Executor
    └─ 信息充分 → 生成诊断报告
```

### 需要新增的工具

| 工具 | 用途 |
|------|------|
| `knowledge_search` | 已有 — 查运维手册/历史故障记录 |
| `log_analyzer` | 新增 — 分析错误日志模式 |
| `diagnostic_check` | 新增 — 系统健康检查（可 mock） |

### 文件规划

| 文件 | 说明 |
|------|------|
| `internal/service/aiops_service.go` | 运维 Agent 服务层 |
| `internal/handler/aiops_handler.go` | HTTP handler |
| `internal/tools/diagnostic_tool.go` | 运维诊断工具 |

---

## 交付与验收标准 (v0.1)

- [x] HTTP 上传文档并索引
- [x] 基于检索上下文的智能问答（流式 + 普通）
- [ ] `/ai_ops` 接受问题描述，输出结构化诊断报告
- [ ] README 含启动说明和示例请求
