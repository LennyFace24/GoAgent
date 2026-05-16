import { ref, nextTick, type Ref } from 'vue'
import type { Message, ToolEventData } from '../../../shared/types'

export function useSSE(messages: Ref<Message[]>) {
  const sending = ref<boolean>(false)

  function scrollBottom(el: HTMLElement | null): void {
    nextTick(() => {
      if (el) el.scrollTop = el.scrollHeight
    })
  }

  function appendLast(content: string): void {
    const last = messages.value[messages.value.length - 1]
    if (last && 'content' in last && last.content !== undefined) {
      (last as { content: string }).content += content
    }
  }

  function finishLast(): void {
    const last = messages.value[messages.value.length - 1]
    if (last) last.streaming = false
  }

  async function send(
    text: string,
    mode: string,
    conversationId: string,
    scrollEl: HTMLElement | null,
  ): Promise<void> {
    sending.value = true

    const assistantMsg: Message = {
      id: Date.now() + Math.random(),
      role: 'assistant',
      content: '',
      streaming: true,
    }
    messages.value.push(assistantMsg)
    scrollBottom(scrollEl)

    try {
      const endpoint = mode === 'ai_ops' ? '/ai_ops' : '/chat_stream'
      const res = await fetch(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message: text, conversation_id: conversationId }),
      })

      if (!res.ok) {
        finishLast()
        appendLast(`\n\n[HTTP ${res.status}]`)
        return
      }

      const reader = res.body!.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })

        const lines = buffer.split('\n')
        buffer = lines.pop() || ''

        // 解析 data: 事件
        for (const line of lines) {
          if (!line.startsWith('data:')) continue
          const payload = line.slice(5).trim()
          if (!payload || payload === '"done"' || payload === 'done') continue
          try {
            const data = JSON.parse(payload)
            if (data.content) {
              appendLast(data.content)
              scrollBottom(scrollEl)
            } else if (data.error) {
              appendLast(`\n\n[错误: ${data.error}]`)
            }
          } catch { /* ignore */ }
        }

        // 解析 event:tool 事件
        for (let i = 0; i < lines.length; i++) {
          if (!lines[i].startsWith('event:tool')) continue
          const dataLine = lines[i + 1]
          if (!dataLine || !dataLine.startsWith('data:')) continue
          try {
            const ev: ToolEventData = JSON.parse(dataLine.slice(5).trim())
            if (ev.type === 'tool_call') {
              const insertIdx = messages.value.indexOf(assistantMsg)
              messages.value.splice(insertIdx, 0, {
                id: Date.now() + Math.random(),
                role: 'tool_call',
                name: ev.name,
                args: ev.args,
                callId: ev.call_id,
              })
            } else if (ev.type === 'tool_result') {
              messages.value.push({
                id: Date.now() + Math.random(),
                role: 'tool_result',
                name: ev.name,
                result: ev.result,
                callId: ev.call_id,
                collapsed: true,
              })
            } else if (ev.type === 'permission_request') {
              messages.value.push({
                id: Date.now() + Math.random(),
                role: 'permission_req',
                requestId: ev.request_id!,
                name: ev.name,
                args: ev.args,
                reason: ev.reason,
                responded: false,
              })
            }
            scrollBottom(scrollEl)
          } catch { /* ignore */ }
        }
      }
    } catch (e) {
      appendLast(`\n\n[连接断开: ${e instanceof Error ? e.message : String(e)}]`)
    } finally {
      finishLast()
      sending.value = false
    }
  }

  async function respondPermission(msg: Message, approved: boolean, always = false): Promise<void> {
    if (msg.role !== 'permission_req') return
    msg.responded = true
    msg.approved = approved
    try {
      await fetch('/permission/response', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: msg.requestId, approved, always }),
      })
    } catch { /* ignore */ }
  }

  return { sending, send, respondPermission }
}
