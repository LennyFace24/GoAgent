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
│   │   ├── skill_expand_stage.go  # 阶段3：skill 触发词匹配注入
│   │   ├── model_prep_stage.go    # 请求级模型与预算快照
│   │   ├── compact_check_stage.go # 完整上下文预算检查与分级压缩
│   │   └── context_build_stage.go # 组装 slots + history + user
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
│   │   ├── agent_handler.go       # Stream（SSE）+ PermissionResponse（对话级/全局授权）
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
用户输入 → SkillExpand（触发词匹配）→ ModelPrep（模型和预算快照）
        → ContextBuild（slots + 历史 + 当前输入）→ CompactCheck（完整消息预算预检）
        → Stream 启动 runLoop（复用请求快照）
```

- 命令命中（builtin/file-based/skill）由前端调 `/api/commands/execute` 短路，不进管线。
- `pl.Run(ctx, mc)` 顺序调用 Stage；ModelPrep 将本次请求的模型实例和预算写入 `mc`，ContextBuild 每次创建新的请求级 `ContextManager`，避免不同请求、模式和激活 Skill 共享可变 `slots`。
- ContextBuild 先组装完整消息，CompactCheck 再对最终的 system + history + user 消息统一使用 `contexttool.EstimateTokens` 估算，执行微压缩或摘要压缩；不再用 `len/3` 与固定 Slot 开销混算。完整压缩使用无工具绑定的摘要模型并保留系统消息、近期历史和当前输入。
- `cl100k_base` 和字符 fallback 都只是请求前估算；Provider 可能使用不同 tokenizer 或包含额外 Chat Template/Tool Schema 开销，因此通过模型配置的安全余量吸收误差，不宣称估算绝对精确。
- 与原七步规划相比，事件 Hook 层（EventBus/EventTrigger）不实现且相关占位代码已删除；非错误 Skip 短路仍未完成。发送阶段明确留在 Stream，不保留空阶段。原计划保留独立 runLoop，不是把每轮工具执行改写为预处理阶段。

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
- 未使用的 `SkillRegistry.Documents()` 访问器及空 `SubProxy` 类型已移除；Skill 发现与 `/skill:` 命令、已注册的 subproxy 工具及其执行循环保留。

## 5. 核心功能状态表

| 核心组件 | 位置 | 状态 |
| :--- | :--- | :--- |
| 消息预处理管线 | internal/pipeline/ | 4 个 Stage；请求级模型快照、Slot 组装与完整预算预检已接通；事件 Hook 层不实现，非错误 Skip 短路未实现 |
| 斜杠命令系统 | internal/commands/ | **已实现**（builtin/file-based/skill 分层） |
| skill 来源 | internal/skills/ | **已实现**（~/.claude/skills + .agents/skills 扫描） |
| **模型管理** | internal/modelmgr/ | **已实现**（model.json 运行时加载 + 每服务 ModelSet + 切换 API） |
| 流式智能问答 | internal/service/agent_service.go | **已实现**（自定义工具循环 + SSE） |
| 命令消息落库 | internal/handler/command_handler.go | **已实现**（user 原文 + assistant 结果） |
| todo 工具 | internal/tools/extra_tool/todo_tool.go | **已注册启用**（write_todo/read_todo） |
| permission always | internal/permission/ | **已实现**（对话级内存规则 + 显式全局规则原子持久化到 permission.json） |
| 后端路由测试 | internal/handler/routes_test.go | **已实现**（路由表 + 端点行为） |
| 多智能体团队 (AgentTeam) | internal/tools/extra_tool/subproxy_tool.go | 已有真实子代理循环；未实现 Planner/Executor 团队调度 |
| MCP 启动接入 | internal/mcp/ + internal/handler/routes.go | **已实现**（stdio/SSE/Streamable HTTP、发现、Eino 适配、权限与调用回填；非热加载） |

## 6. 已知待办/限制

- `~/.claude/commands` 的 file-based 命令依赖用户本机目录，CI/无此目录环境静默跳过。
- 没有事件 Hook 层：EventBus、EventTriggerStage 及事件名声明已删除，生产管线固定为四个 Stage；不做插件注入。
- MCP 配置与工具集合在启动阶段加载；没有文件监控或运行时重载入口，配置修改后需重启。
- SubProxy 当前生产装配仍使用启动时默认模型，不承诺随模型选择器切换；主 Agent 的请求级快照不改变这个独立工具的配置来源。
- chat 与 aiops 是实际模式；AIOps 提示词的 Plan/Replan 是回答流程指导，不是独立可切换的 plan 模式。连续拒绝只提醒确认权限范围。
- `read_todo` / `write_todo` 是已注册的 Markdown 文件读写工具，与已删除的旧 TodoStore 无关。仅 chat 模式且两个工具均存在时，连续 5 个非待办工具轮次触发一次提醒；不是任务调度器。

## 7. 面试讲解主线

### 7.1 Agent Loop

请求经 SkillExpand、ModelPrep、ContextBuild、CompactCheck 后启动独立 runLoop，执行“模型流式输出 → ToolCall → 权限检查 → 工具执行 → ToolMessage 回填”。ContextManager 是请求级对象；模型、预算和最终消息也固定在本次 `mc` 中。批准与直接允许共用工具执行和错误处理分支。最多执行 20 个工具轮次，随后请求模型在禁用工具的条件下回答；若提供方仍返回 ToolCall，返回错误而不继续执行。首轮和最终回答都走相同的流式事件路径。

上下文预算的所有请求前估算统一使用 `contexttool.EstimateTokens`：文本、ToolCall 名称、参数和每条消息开销走同一 tokenizer/fallback；Provider 的真实 usage 若可用只能作为请求后的观测校正，不替代发送前估算。

权限检查顺序：Bash 严重规则 deny / 可疑规则 ask、全局与对话级 deny、全局与对话级 ask、AST 只读自动放行、对话级与全局 allow，最后默认 ask。`util.AnalyzeBashCommand` 是唯一只读权威，无法判定或 panic 都返回 Unknown（转人工确认）；`permission.Mode` 与未调用的 `bash_readonly.go` 已删除。持续授权按 `sessionID + conversationID` 隔离，显式全局授权才原子写入 `permission.json`，规则只匹配批准的 bash 命令文本或文件路径。

模型流保留最后一个 `finish_reason`。为 `length`（输出上限）时，已生成正文照常推送，末尾追加 `[回答因达到模型输出上限而截断]`，返回明确失败并立即结束；截断轮产生的 ToolCall 不执行，也不自动续写。

### 7.2 MCP 接入

`MCPManager` 管理客户端会话；stdio、旧 SSE 和 Streamable HTTP 的握手与协议编解码由官方 Go SDK 负责。启动时经 initialize 与分页 `tools/list` 获取工具快照，适配器将参数 Schema 转成 Eino `ToolInfo`，工具名规范化并隔离服务器命名空间。Agent 生成 ToolCall 后先经权限判定，适配器保留数字精度转发 `tools/call`，结果经 ToolMessage 进入下一轮并推送 SSE。启动失败回滚会话，应用收到退出信号后关闭连接和子进程。

MCP 配置由每个 `MCPManager` 持有，`LoadMCPConfig` 仅负责磁盘读取。启动初始化、默认配置文件生成和磁盘加载保留；当前配置优先级为可执行文件旁便携配置、用户配置目录、内嵌空配置。已移除闲置的全局配置副本、保存与重载接口；工具适配器直接通过保存的 server/tool 字段路由，不再重复分配无人读取的 `ToolInfo.Extra` 元数据。

### 7.3 安全与风险

MCP 不是自主实现协议栈，也不是对外暴露 GoAgent 的 MCP Server；这是 Host/Client 接入外部 Server。现有精确工具名 `allow/ask/deny` 对 MCP 工具生效，未匹配默认询问；不信任远端 destructive/readOnly annotations 自动放权。服务器启动即执行可信配置中的命令，因此审批不能替代进程隔离。当前没有 MCP 配置管理界面、工具列表热更新、租户隔离或完整 OAuth；结构化结果中的大整数精度受 SDK 解码限制。Memory 当前是文件 Slot 注入，不是自动形成和召回的长期记忆系统。
