import { ref } from 'vue'
import type { Conversation } from '../types'

const conversations = ref<Conversation[]>([])

export function useConversations() {

  async function load(): Promise<void> {
    try {
      const res = await fetch('/api/conversations')
      if (!res.ok) return
      const data = await res.json()
      conversations.value = data.conversations || []
    } catch { /* ignore */ }
  }

  async function create(): Promise<string | null> {
    try {
      const res = await fetch('/api/conversation', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title: '新对话' }),
      })
      if (!res.ok) return null
      const data = await res.json()
      if (data.conversation) {
        conversations.value.unshift(data.conversation)
        return data.conversation.id as string
      }
    } catch { /* ignore */ }
    return null
  }

  async function remove(id: string): Promise<void> {
    try {
      await fetch(`/api/conversation/${id}`, { method: 'DELETE' })
      conversations.value = conversations.value.filter((c: Conversation) => c.id !== id)
    } catch { /* ignore */ }
  }

  return { conversations, load, create, remove }
}
