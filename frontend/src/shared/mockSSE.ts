export interface MockEvent {
  type: 'tool_call' | 'tool_result' | 'content' | 'done'
  name?: string
  args?: string
  result?: string
  content?: string
}

export function simulateSSE(
  userText: string,
  mode: string,
  onEvent: (event: MockEvent) => void,
  onDone: () => void
) {
  let timer: any = null
  const events: MockEvent[] = []

  // 根据用户的不同模式或输入，编排不同的运维智能回答
  if (mode === 'aiops' || userText.includes('诊断') || userText.includes('故障')) {
    events.push(
      { type: 'tool_call', name: 'health_check', args: '{}' },
      { type: 'tool_result', name: 'health_check', result: '{"cpu_utilization": 87, "memory_utilization": 92, "disk_status": "OK", "prometheus_up": true}' },
      { type: 'tool_call', name: 'bash', args: '{"command": "df -h"}' },
      { type: 'tool_result', name: 'bash', result: 'Filesystem      Size  Used Avail Use% Mounted on
/dev/sda1        40G   14G   26G  33% /' },
      { type: 'content', content: '### 🩺 诊断分析结果

经过对系统底层指标的综合调用与审计排查，我定位到了以下异常：

1. **内存水位偏高 (92%)**：检测到主要由日志检索缓存进程占用引起。
2. **存储空间正常**：磁盘根目录挂载健康，余量为 26GB。

**建议行动**：一键清理系统的 Redis 脏缓存或重启 log-collector 容器。' }
    )
  } else if (userText.includes('手册') || userText.includes('知识库') || userText.includes('RAG')) {
    events.push(
      { type: 'tool_call', name: 'knowledge_search', args: JSON.stringify({ query: userText }) },
      { type: 'tool_result', name: 'knowledge_search', result: '根据《运维故障排查黄金手册》第12条：在内存超过90%时，优先检索 /var/log。' },
      { type: 'content', content: '根据本地 RAG 知识库的最佳实践建议：

- 您应当通过 	ools/file_tools.go 定位日志异常，并在 MonitorDrawer 中保持观测。' }
    )
  } else {
    // 常规智能聊天
    events.push(
      { type: 'content', content: 您好！我是您的 **GoAgent (MiniAgent) 智能运维助理**。

您刚才提问了：*""*

作为面向用户最友好的 Cherry Studio 风格工作站，我不仅支持常规的**流式多轮问答**，还能自动分析并折叠工具日志。您可以点击右侧的 **🖥️ 实时系统状态** 面板，时刻掌控服务指标！ }
    )
  }

  // 模拟流式打字机效果
  let index = 0
  function next() {
    if (index >= events.length) {
      onDone()
      return
    }

    const ev = events[index]
    if (ev.type === 'content') {
      // 逐字推送以模拟真实打字机 SSE 效果
      const text = ev.content || ''
      let charIndex = 0
      const chunkTimer = setInterval(() => {
        if (charIndex >= text.length) {
          clearInterval(chunkTimer)
          index++
          timer = setTimeout(next, 500)
        } else {
          // 推送一个字
          onEvent({ type: 'content', content: text[charIndex] })
          charIndex++
        }
      }, 15)
    } else {
      // 快速推送 tool_call 或 tool_result
      onEvent(ev)
      index++
      timer = setTimeout(next, 1000)
    }
  }

  timer = setTimeout(next, 500)

  return {
    abort: () => {
      if (timer) clearTimeout(timer)
    }
  }
}