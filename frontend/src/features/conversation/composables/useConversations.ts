import { ref } from 'vue'
import type { Conversation } from '../types'

const conversations = ref<Conversation[]>([])

export function useConversations() {

  async function load(): Promise<void> {
    try {
      const res = await fetch('/api/conversations', { credentials: 'include' })
      if (!res.ok) return
      const data = await res.json()
      conversations.value = data.conversations || []
    } catch { /* ignore */ }
  }

  async function create(title = '新对话'): Promise<string | null> {
    try {
      const res = await fetch('/api/conversation', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ title }),
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

  async function rename(id: string, title: string): Promise<boolean> {
    try {
      const res = await fetch(`/api/conversation/${id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ title }),
      })
      if (!res.ok) return false
      const data = await res.json()
      const updated = data.conversation as Conversation | undefined
      const current = conversations.value.find((c) => c.id === id)
      if (current && updated) {
        current.title = updated.title
        current.updated_at = updated.updated_at
      }
      return true
    } catch {
      return false
    }
  }

  async function remove(id: string): Promise<boolean> {
    try {
      const res = await fetch(`/api/conversation/${id}`, {
        method: 'DELETE',
        credentials: 'include',
      })
      if (!res.ok) return false
      conversations.value = conversations.value.filter((c: Conversation) => c.id !== id)
      return true
    } catch {
      return false
    }
  }

  return { conversations, load, create, rename, remove }
}
