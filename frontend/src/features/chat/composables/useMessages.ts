import { ref, type Ref } from 'vue'
import type { Message } from '../../../shared/types'

export function useMessages(conversationId: Ref<string>) {
  const messages = ref<Message[]>([])

  async function loadHistory(): Promise<void> {
    messages.value = []
    try {
      const res = await fetch(`/conversation/${conversationId.value}`)
      if (!res.ok) return
      const data = await res.json()
      const lines: Array<{ role: string; content: string }> = data.messages || []
      for (const line of lines) {
        if (line.role === 'user' || line.role === 'assistant') {
          messages.value.push({
            id: Date.now() + Math.random(),
            role: line.role as 'user' | 'assistant',
            content: line.content,
            streaming: false,
          })
        }
      }
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
