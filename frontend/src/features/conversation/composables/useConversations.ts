import { ref } from 'vue'
import type { Conversation } from '../types'

export function useConversations() {
  const conversations = ref<Conversation[]>([])
  const activeId = ref<string>('default')

  async function load(): Promise<void> {
    try {
      const res = await fetch('/conversations')
      if (!res.ok) return
      const data = await res.json()
      conversations.value = data.conversations || []
      if (conversations.value.length === 0) {
        await create()
      } else if (!conversations.value.find((c: Conversation) => c.id === activeId.value)) {
        activeId.value = conversations.value[0].id
      }
    } catch { /* ignore */ }
  }

  async function create(): Promise<void> {
    try {
      const res = await fetch('/conversation', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title: '新对话' }),
      })
      if (!res.ok) return
      const data = await res.json()
      if (data.conversation) {
        conversations.value.unshift(data.conversation)
        activeId.value = data.conversation.id
      }
    } catch { /* ignore */ }
  }

  async function remove(id: string): Promise<void> {
    try {
      await fetch(`/conversation/${id}`, { method: 'DELETE' })
      conversations.value = conversations.value.filter((c: Conversation) => c.id !== id)
      if (activeId.value === id) {
        activeId.value = conversations.value.length > 0 ? conversations.value[0].id : 'default'
      }
    } catch { /* ignore */ }
  }

  function select(id: string): void {
    activeId.value = id
  }

  return { conversations, activeId, load, create, remove, select }
}
