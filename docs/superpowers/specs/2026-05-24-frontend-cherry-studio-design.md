# Cherry Studio 风格前端友好重构设计规范 (Design Spec)

## 1. 重构目标

将原有的极简多模式切换界面重构为极具质感、面向实际运维人员极其友好的 **Cherry Studio 风格三栏式运维工作站**。

### 核心用户友好交互主轴
1.  **极简化认知解耦**：隐藏原本生硬的“模式切换”底层概念。通过统一的智能输入框与快捷运维场景卡片，让用户在零门槛下和 AI 进行专业对齐。
2.  **折叠式工具思考链**：将杂乱繁多的 Bash 指令和 RAG 语义搜索日志折叠进美观的“思考/执行中”进度状态条。只有在用户需要时，才可展开查看技术细节。
3.  **双栏 Diff 安全审查**：针对高危的修改文件 (edit_file / write_file) 动作，重构为双栏高亮 Diff 对比安全卡片，给运维人员提供所见即所得的安全确认。
4.  **系统指标数据可视化**：右侧集成可以平滑拉出的抽屉式“实时系统监控面板”，利用极美观的 SVG 仪表盘呈现 CPU、内存和负载等实时运维指标。
5.  **独立前端 Mock 调试机制**：在脱机/不依赖后端的情况下，前端通过自拦截机制模拟完整流畅的 AI 交互流和工具调用动画，极大方便独立 UI/UX 校验。

---

## 2. 视觉系统与样式体系 (CSS & Design Token)

我们将全面采用磨砂毛玻璃和微阴影立体美学，并在 rontend/src/assets/styles/theme.css 中更新以下 Design Token：

`css
:root {
  /* Cherry Studio 浅色主调 */
  --bg-primary: #f5f5f7;
  --bg-sidebar-mini: #eef1f6;
  --bg-sidebar-sub: #ffffff;
  --bg-chat-bubble: #ffffff;
  --bg-chat-bubble-user: #10a37f;
  
  --text-main: #1d1d1f;
  --text-muted: #86868b;
  --border-color: rgba(0, 0, 0, 0.08);
  --glass-blur: blur(20px);
  --border-radius-lg: 12px;
  --border-radius-xl: 16px;
  
  /* 阴影效果 */
  --shadow-sm: 0 1px 2px rgba(0,0,0,0.04);
  --shadow-md: 0 4px 12px rgba(0,0,0,0.05);
  --shadow-lg: 0 12px 24px rgba(0,0,0,0.08);
}

@media (prefers-color-scheme: dark) {
  :root {
    /* Cherry Studio 深色微蓝紫调 */
    --bg-primary: #121216;
    --bg-sidebar-mini: #181822;
    --bg-sidebar-sub: #1c1c28;
    --bg-chat-bubble: #1f1f30;
    --bg-chat-bubble-user: #10a37f;
    
    --text-main: #f5f5f7;
    --text-muted: #86868b;
    --border-color: rgba(255, 255, 255, 0.08);
  }
}
`

---

## 3. 三栏式布局设计 (Layout Structure)

### 3.1 极窄主功能侧边栏 (Slim Nav Sidebar) - 64px
*   **功能**：常驻页面最左侧，包含精美图标按钮：
    *   💬 **聊天 (Chat)**：常规智能运维对话。
    *   🩺 **诊断 (AIOps)**：全盘指标诊断流。
    *   📚 **知识库 (RAG)**：上传管理运维知识手册。
    *   ⚙️ **设置 (Settings)**：模型和全局偏好配置。
*   **底部交互**：暗黑/浅色主题快速切换。

### 3.2 对话与文档列表二级侧边栏 (Sub Sidebar) - 260px (可拖拽折叠)
*   **聊天/诊断模式下**：显示历史对话，支持搜索过滤、会话标题双击重命名、悬浮删除。
*   **知识库模式下**：显示当前向量库中检索到的文档列表。顶部提供拖拽上传区域，集成实时的进度条。

### 3.3 主视窗工作区 (Workspace)
*   **Header**：左侧显示当前会话标题，右侧提供 **[🖥️ 实时系统状态]** 抽屉滑出按钮。
*   **对话流容器**：
    *   **扁平气泡**：精细的圆角微投影，Markdown 排版（代码块提供复刻 Cherry Studio 的语言标签与一键复制）。
    *   **折叠式工具执行条 (Toolchain Accordion)**：
        *   当 AI 调用 ash 或 ead_file 时，页面不倾泻大量无用字符，而是显示：
            [⚙️ AI 正在执行系统诊断...] -> 包含精美 Spinner 动效。
        *   点击可滑动展开查看具体的 Input/Output。
    *   **双栏 Diff 确认组件**：
        *   触发 edit_file 时，将传统的文本输入改为：
            *   左栏（红色高亮）：修改前原文。
            *   右栏（绿色高亮）：修改后新文。
            *   底部一键 **[批准授权并执行]**、**[拒绝操作]**。

### 3.4 实时监控侧滑抽屉 (Monitor Drawer) - 280px
*   平时隐藏，点击 Header 图标时从右侧像抽屉般滑入。
*   实时拉取 Prometheus 指标，以精美的 SVG 环形进度条渲染 CPU、内存、网络 IO 等仪表盘。

---

## 4. 纯前端 Mock 机制 (Mock Stream Engine)

为了让您能够流畅地进行 UI/UX 演示与调试而不需要依赖复杂的 AI 大模型和数据库后端，前端将在本地开发模式（
pm run dev）下：
1.  **自动识别后端连通状态**：如果后端 8080 端口无法连通，前端将自动切入 **“Mock Demo 模式”**（并在顶部显示一行精致不刺眼的 [Demo 体验模式] 轻量小横条）。
2.  **智能 Mock SSE 事件流**：当用户在 Mock 模式下提问时：
    *   自动延时 300ms 模拟网络传输。
    *   流式生成带有 	ool_call 的消息（模拟先执行 health_check 的折叠动画，再显示健康状态）。
    *   模拟完美的 Markdown 输出流（打字机效果逐字渲染）。
3.  **动态监控指标模拟**：右侧系统监控抽屉在 Mock 模式下，利用定时器让 CPU 和内存数值在 30% ~ 75% 之间呈现温润的波浪式起伏，保证视觉的高保真度。