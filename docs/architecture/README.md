# GoAgent (MiniAgent) 项目真实架构总览

---

## 1. 当前项目物理文件图 (Code Map)

```
GoAgent/
├── cmd/
│   ├── main.go                    # 程序入口（gin 装配：CORS/sessions/SessionID + 路由）
│   └── sandbox-server/            # 远程沙箱服务（docker 容器创建/执行/销毁）
├── internal/
│   ├── modelmgr/                  # ★ 模型管理（新增）
│   │   ├── model.go               # Model 结构 + ContextBudgetTokens + eino 构建
│   │   ├── registry.go            # model.json 运行时加载 + 分级校验错误
│   │   ├── modelset.go            # ★ 每服务 Model 对象（currentID + 惰性构建 + Switch）
│   │   └── registry_test.go       # 加载/校验/切换测试
│   ├── pipeline/                  # ★ 消息预处理管线（新增）
│   │   ├── pipeline.go            # MessageContext + Stage 接口 + Pipeline.Run
│   │   ├── event_bus.go           # 预留的进程内同步 Hook 分发器，未接入请求链路
│   │   ├── skill_expand_stage.go  # 阶段3：skill 触发词匹配注入
│   │   ├── model_prep_stage.go    # 阶段4：模型实例与认证准备
│   │   ├── compact_check_stage.go # 阶段5：历史预算预检压缩
│   │   ├── context_build_stage.go # 阶段6：ContextManager 组装 slots + BuildMessages
│   │   └── send_stage.go          # 阶段7：管线终点
│   ├── commands/                  # ★ 斜杠命令系统（重构）
│   │   ├── types.go               # Command + ArgsPolicy（none/optional/required）+ Handler
│   │   ├── registry.go            # 注册中心（内置 + file-based + skill 命令）
│   │   ├── builtin.go             # 内置命令：compact/context/model（server handler）
│   │   ├── file_based.go          # ~/.claude/commands markdown 模板命令（递归+foo:bar 别名）
│   │   └── skill_commands.go      # /skill:<name> 命令注册
│   ├── skills/                    # ★ skill 系统（重构）
│   │   ├── registry.go            # 多来源扫描：.agents/skills + ~/.claude/skills
│   │   └── parser.go              # frontmatter 解析（支持折叠/字面块标量 >- |）
│   ├── handler/                   # HTTP 控制层
│   │   ├── routes.go              # ★ 依赖注入重构：Handlers + SetupRoutesWith
│   │   ├── agent_handler.go       # Stream（SSE）+ PermissionResponse（always 生效）
│   │   ├── command_handler.go     # ★ /api/commands/execute（落库 user 原文 + assistant 结果）
│   │   ├── conversation_handler.go# 会话 CRUD
│   │   ├── file_handler.go        # 上传/检索
│   │   ├── metrics_handler.go     # 系统指标
│   │   └── routes_test.go         # ★ 路由表 + 端点行为测试（新增）
│   ├── service/
│   │   ├── agent_service.go       # Stream 预处理管线 + 手写工具循环 + SSE
│   │   └── file_service.go        # 知识库处理（Chroma RAG）
│   ├── context/
│   │   └── manager.go             # ContextManager：slot 组装（system_prompt/soul/skills/...）
│   ├── tools/
│   │   ├── tool_handler.go        # 16 个内置工具 + 启动发现的 MCP 工具
│   │   ├── context_tool/          # compact（全局触发器）/ context_status / token 估算
│   │   ├── basic_tool/            # bash（安全校验）/ read/write/edit 文件
│   │   ├── extra_tool/            # health_check / knowledge_search / subproxy / todo
│   │   └── sandbox_tool/          # 远程容器沙箱
│   ├── permission/                # 规则匹配 + bash AST 只读分析 + pending 决策
│   ├── task/                      # 任务工具（.tasks/ JSON 持久化）
│   ├── config/                    # config.yaml 加载
│   ├── middleware/                # CORS / SessionID
│   ├── store/                     # ConversationStore（JSONL）/ ChromaStore
│   ├── mcp/                       # MCP 配置、连接管理与 Eino 工具适配
│   └── metrics/                   # gopsutil 收集器
├── frontend/                      # Vue3 + Vite + TS
│   └── src/features/chat/
│       ├── composables/useChat.ts # SSE 流处理 + 跨会话缓冲
│       └── components/ChatWorkspace.vue # ★ 命令分流（client/builtin/file/skill/未命中）
├── data/                          # conversations/ transcripts/ soul.md memory.md
└── permission.json                # allow/ask/deny 规则（always 持久化目标）
```

## 2. 消息预处理管线（internal/pipeline）

```
用户输入 → SkillExpand（触发词匹配）→ ModelPrep（模型准备）
        → CompactCheck（历史预算预检）→ ContextBuild（slots 组装）
        → Send → runLoop
```

- 命令命中（builtin/file-based/skill）由前端调 `/api/commands/execute` 短路，不进管线。
- runLoop 内的每轮 compact 检查保留，与阶段 5 预检并存（预检保证首轮不超载）。

## 3. 斜杠命令系统（对齐 omp 分层）

| 层 | 来源 | 语义 |
|---|---|---|
| builtin | Go handler（compact/context/model） | 立即执行，结果落库为 assistant，不进 LLM |
| client | 前端本地（clear/help） | 不进上下文 |
| file-based | `~/.claude/commands/**/*.md` + `<cwd>/.claude/commands/` | 模板渲染（`$1 $ARGUMENTS`）后作为消息发 LLM |
| skill | `/skill:<name>`（`~/.claude/skills` + `.agents/skills` 扫描） | 注入 skill 全文作为消息发 LLM |
| 未命中 | — | `/xxx` 原样进管线（与 omp 一致不拒绝） |

**命令消息进上下文**：`/xxx` 原文落库为 user，builtin 结果落库为 assistant。

## 4. Skill 来源（多 provider 直接读取，零迁移）

- `~/.claude/skills/<name>/SKILL.md`（user 级，复用现有 skill）
- `.agents/skills/<name>/SKILL.md`（project 级，可进 git，优先级最高）
- 同名冲突：project > user，高优先级胜出并告警。

## 5. 核心功能状态表

| 核心组件 | 位置 | 状态 |
| :--- | :--- | :--- |
| 消息预处理管线 | internal/pipeline/ | **已实现**（Stream 实际组装 5 个 Stage，命令由单独端点处理） |
| 斜杠命令系统 | internal/commands/ | **已实现**（builtin/file-based/skill 分层） |
| skill 来源 | internal/skills/ | **已实现**（~/.claude/skills + .agents/skills 扫描） |
| **模型管理** | internal/modelmgr/ | **已实现**（model.json 运行时加载 + 每服务 ModelSet + 切换 API） |
| 流式智能问答 | internal/service/agent_service.go | **已实现**（自定义工具循环 + SSE） |
| 命令消息落库 | internal/handler/command_handler.go | **已实现**（user 原文 + assistant 结果） |
| todo 工具 | internal/tools/extra_tool/todo_tool.go | **已注册启用**（write_todo/read_todo） |
| permission always | internal/permission/ | **已实现**（AddRule + 持久化 permission.json） |
| 后端路由测试 | internal/handler/routes_test.go | **已实现**（路由表 + 端点行为） |
| 多智能体团队 (AgentTeam) | internal/tools/extra_tool/subproxy_tool.go | 已有真实子代理循环；未实现 Planner/Executor 团队调度 |
| MCP 启动接入 | internal/mcp/ + internal/handler/routes.go | **已实现**（stdio/SSE/Streamable HTTP、发现、Eino 适配、权限与调用回填；非热加载） |

## 6. 已知待办/限制

- `~/.claude/commands` 的 file-based 命令依赖用户本机目录，CI/无此目录环境静默跳过。
- EventBus 与 EventTriggerStage 保留为未接入的扩展代码：生产链路没有创建总线、注册 Hook，也未装配事件阶段；`pre_send`、`post_send` 目前仅有事件名声明。它不是 SSE 工具事件通道，未接入的原因不是 Windows 不支持 Go plugin。
- MCP 工具在启动阶段完成发现并绑定到模型；配置 Reload 只替换配置，不会自动重建现有模型工具集合。

## 7. 面试讲解主线

### 7.1 Agent Loop

请求进入 `AgentService.Stream` 后经过 skill、模型准备、上下文预算检查和上下文组装；`runLoop` 反复执行“模型流式输出 → 累积 ToolCall → 权限判定 → 工具执行 → ToolMessage 回填”，模型不再调用工具时结束。实际初始化的是五个处理 Stage，并非文档中七个阶段全部执行；`maxTurns` 的边界实现也不等价于严格 20 轮。普通文本和工具事件通过 SSE 推送，JSONL Store 负责持久化。

### 7.2 MCP 接入

`MCPManager` 管理客户端会话；stdio、旧 SSE 和 Streamable HTTP 的握手与协议编解码由官方 Go SDK 负责。启动时经 initialize 与分页 `tools/list` 获取工具快照，适配器将参数 Schema 转成 Eino `ToolInfo`，工具名规范化并隔离服务器命名空间。Agent 生成 ToolCall 后先经权限判定，适配器保留数字精度转发 `tools/call`，结果经 ToolMessage 进入下一轮并推送 SSE。启动失败回滚会话，应用收到退出信号后关闭连接和子进程。

### 7.3 安全与风险

MCP 不是自主实现协议栈，也不是对外暴露 GoAgent 的 MCP Server；这是 Host/Client 接入外部 Server。现有精确工具名 `allow/ask/deny` 对 MCP 工具生效，未匹配默认询问；不信任远端 destructive/readOnly annotations 自动放权。服务器启动即执行可信配置中的命令，因此审批不能替代进程隔离。当前没有 MCP 配置管理界面、工具列表热更新、租户隔离或完整 OAuth；结构化结果中的大整数精度受 SDK 解码限制。Memory 当前是文件 Slot 注入，不是自动形成和召回的长期记忆系统。
