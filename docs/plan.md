# 项目概览与实施计划

![项目概览](images/summary.svg)

## 三张图片总结
- 图片1（架构图）: 展示了系统分层——上层为 API 接口（如 `/chat`、`/chat_stream`、`/upload_file`、`/ai_ops`），中间为多种 Agent（对话Agent、运维Agent、知识库Agent），下层为核心组件（Loader、Indexer、Retriever、Transformer、Chat Model、Prompt、Tool、MCP），以及知识库与向量数据库的集成。
- 图片2（功能与能力）: 强调产品能做的事情：智能问答、故障诊断、知识库管理；并列出技术栈亮点：RAG、ReAct、Plan-Executor、Multi-Agent、MCP、Prompt 工程等，适合作为实现路线的技术选择参考。
- 图片3（项目亮点）: 从面试角度列出可展示的亮点：RAG 思路、Prompt 设计与迭代、多轮对话记忆、流式 SSE 输出、Agent 设计模式（ReAct / Plan-Execute-Replan）以及如何用这些点在面试中讲清楚设计权衡。

综合以上，系统核心是“以向量化知识库 + LLM 为基础，通过多种 Agent 编排（ReAct / Plan-Execute）实现对话、诊断与运维自动化”的工程化实现。

## 实施 Plan（短期可落地版本 v0.1）
目标：在 2 周内搭建可运行的最小可用系统（MVP），支持：上传文档、向量化、简单检索、以及一个可交互的 `/chat` 接口。

1. 初始化与基础（已开始）
   - 初始化 `go.mod`（若尚未）并确认目录结构。
   - 创建基础 HTTP 服务骨架与路由（`/chat`, `/chat_stream`, `/upload_file`, `/ai_ops`）。

2. 核心接口与组件抽象
   - 定义接口：`Loader`, `Indexer`, `Retriever`, `Transformer`, `LLMClient`, `Tool`。
   - 在 `internal/` 下各自建子包（例如 `internal/components/loader` 等）。

3. 向量数据库抽象与示例实现
   - 提供本地内存向量存储实现（便于开发测试），并保留 `VectorStore` 接口以便后续替换为 Milvus/Weaviate/Pinecone。

4. 文档上传与导入流程
   - 实现 `/upload_file`：接收文件、文本抽取、分片、向量化、入库。

5. LLM 客户端与流式响应
   - 封装 `LLMClient`，支持同步与 SSE/Stream（`/chat_stream`）。

6. 简单 Agent 实现（v0.1）
   - 实现一个基础对话 Agent：检索 + prompt 拼接 + 调用 LLM。

7. CI/测试与 README
   - 添加基本单元测试（组件接口）与 README，同步运行步骤与示例请求。

## 文件与目录建议
- `cmd/`：主入口 `main.go`（HTTP server 启动）
- `internal/handler/`：路由与 HTTP handler（`routes.go`）
- `internal/agent/`：Agent 抽象与实现
- `internal/components/`：Loader/Indexer/Retriever/Transformer
- `internal/vectorstore/`：向量存储接口与内存实现
- `internal/llm/`：LLM 客户端封装

## 交付与验收标准（v0.1）
- 可以通过 HTTP 上传一份文档并被索引进本地向量库。
- 可以对上传内容进行检索并通过 `/chat` 得到基于检索上下文的回答。
- README 提供启动与测试说明，且包含示例请求。

---
如需我现在把 `routes.go`、`internal/llm` 的接口骨架和一个内存向量存储实现补上，我可以继续创建对应文件并提交 patch。你希望我接下来先实现哪一步？
