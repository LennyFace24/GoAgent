# GoAgent (MiniAgent) 项目真实架构总览与占位符清单

本文件是对当前项目物理代码库的实事求是架构总结。为了方便开发者快速掌握当前哪些模块属于真实已可用状态、哪些模块在代码中仅作为骨架或完全未集成，特此提供此真实架构树。

---

## 1. 当前项目物理文件图 (Code Map)

`
GoAgent/
├── cmd/
│   └── main.go                    # 程序入口 (已实现)
├── internal/
│   ├── config/
│   │   └── config.go              # 配置加载 (已实现)
│   ├── handler/
│   │   ├── routes.go              # 路由注册与初始化 (已实现，未注册/chat与/task)
│   │   ├── chat_stream_handler.go # SSE 聊天控制层 (已实现)
│   │   ├── aiops_handler.go       # AIOps 诊断控制层 (已实现，复用单Agent逻辑)
│   │   └── file_handler.go        # 知识库管理接口 (已实现)
│   ├── service/
│   │   ├── chat_stream_service.go # 单 Agent 流式 ReAct 执行引擎 (已实现)
│   │   └── aiops_service.go       # AIOps 单 Agent 模拟分析服务 (已实现)
│   ├── task/
│   │   └── task.go                # [⚠️ 占位符] 任务系统空骨架，未实现
│   ├── tools/
│   │   ├── tool_handler.go        # 工具注册中心 (已实现，未注册/task)
│   │   ├── basic_tool/
│   │   │   ├── bash_tool.go       # Bash 执行 (已实现)
│   │   │   └── file_tools.go      # 文件读写 (已实现)
│   │   └── extra_tool/
│   │       ├── diagnostic_tool.go # Prometheus 查询 (已实现)
│   │       ├── knowledge_tool.go  # ChromaDB RAG 检索 (已实现)
│   │       ├── todo_tool.go       # [⚠️ 未集成] 待办事项，未加入 tool_handler
│   │       └── task_tool.go       # 临时子 Agent 代理 (SubProxy) 工具 (已实现)
└── docs/
    └── architecture/              # 架构分解专栏 (已校准)
`

---

## 2. 核心功能及占位符状态表 (Status Board)

| 核心组件 | 物理文件路径 | 当前代码真实状态 |
| :--- | :--- | :--- |
| **流式智能问答 (ChatStream)** | internal/service/chat_stream_service.go | **已完成**。提供 Eino + ReAct 单 Agent 20回合循环与 SSE 流式事件推送。 |
| **AIOps 智能诊断 (aiops)** | internal/service/aiops_service.go | **已完成**。实质上复用了单 Agent 控制层并注入了 AIOps 系统提示。 |
| **安全过滤与审计 (bash/file)**| internal/tools/basic_tool/ | **已完成**。支持敏感 Shell 拦截，以及路径沙箱隔离。 |
| **待办提示机制 (todo_tool)**  | internal/tools/extra_tool/todo_tool.go| **代码已实现，未集成**。由于 	ool_handler.go 未装配，导致流式服务 5 回合强提示报错。 |
| **任务依赖调度图 (task)**     | internal/task/task.go | **纯空代码占位符 (Skeleton)**。仅定义了空结构体与未填充空方法，未实际实现，且未暴露 Eino 工具和注入 	ool_handler.go。 |
| **多智能体团队 (AgentTeam)**  | internal/tools/extra_tool/task_tool.go| **仅有 SubProxy 占位模拟**。无独立的 Planner/Executor 双 Agent 协同层，仅提供了能派生一个 SubAgent 进行递归调用的 subproxy 工具。 |
| **持久化长期记忆 (Memory)**   | *无* | **完全空缺设计占位符**。代码中无对应包与目录，属于纯概念规划，当前暂不存在。 |
| **MCP Server 自由导入 (mcp)** | *无* | **完全空缺设计占位符**。代码中无对应包与目录，属于纯概念规划，当前暂不存在。 |