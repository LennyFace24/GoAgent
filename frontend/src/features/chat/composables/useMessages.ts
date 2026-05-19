import { ref, type Ref } from 'vue'
import type { Message, ApiMessage } from '../../../shared/types'

export function useMessages(conversationId: Ref<string>) {
  const messages = ref<Message[]>([])

  async function loadHistory(): Promise<void> {
    messages.value = []
    try {
      const res = await fetch(`/conversation/${conversationId.value}`)
      if (!res.ok) return
      const data = await res.json()
      const lines: ApiMessage[] = data.messages || []
      let order = 0
      for (const line of lines) {
        if (line.role === 'user') {
          messages.value.push({
            id: Date.now() + Math.random(),
            role: 'user',
            content: line.content,
            streaming: false,
            order: ++order,
          })
        } else if (line.role === 'assistant') {
          // 从 assistant 消息的 tool_calls 数组重建 tool_call 消息
          if (line.tool_calls) {
            for (const tc of line.tool_calls) {
              messages.value.push({
                id: Date.now() + Math.random(),
                role: 'tool_call',
                name: tc.function.name,
                args: tc.function.arguments,
                callId: tc.id,
                order: ++order,
              })
            }
          }
          messages.value.push({
            id: Date.now() + Math.random(),
            role: 'assistant',
            content: line.content,
            streaming: false,
            order: ++order,
          })
        } else if (line.role === 'tool') {
          messages.value.push({
            id: Date.now() + Math.random(),
            role: 'tool_result',
            name: line.tool_name || '',
            result: line.content,
            callId: line.tool_call_id || '',
            collapsed: true,
            order: ++order,
          })
        }
      }
      // 按 order 排序，保持原始顺序
      messages.value.sort((a, b) => (a.order || 0) - (b.order || 0))
    } catch { /* ignore */ }
  }

  function addMsg(role: Message['role'], content: string, streaming = false): void {
    messages.value.push({ id: Date.now() + Math.random(), role, content, streaming } as Message)
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

  return { messages, loadHistory, addMsg, appendLast, finishLast }
}
