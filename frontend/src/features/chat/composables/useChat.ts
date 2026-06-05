import { ref, type Ref } from 'vue'
import type { Message } from '../../../shared/types'

// ---------- 聊天核心 composable ----------

export interface UseChatOptions {
  messages: Ref<Message[]>
  conversationId: Ref<string>
  chatMode: Ref<'chat' | 'aiops'>
  scrollToBottom: () => void
}

export function useChat(options: UseChatOptions) {
  const { messages, conversationId, chatMode, scrollToBottom } = options

  const isSending = ref(false)
  let activeAbort: AbortController | null = null

  // 统一的发送函数
  async function send(text: string) {
    if (!text.trim() || isSending.value) return

    // 添加用户消息
    messages.value.push({
      id: Date.now(),
      role: 'user',
      content: text,
    })
    scrollToBottom()

    // 添加 AI 思考消息
    isSending.value = true
    const aiMessageId = Date.now() + 1
    messages.value.push({
      id: aiMessageId,
      role: 'assistant',
      content: '',
      isThinking: true,
      isGenerating: false,
    })
    scrollToBottom()

    // 创建 abort controller
    const abortCtrl = new AbortController()
    activeAbort = abortCtrl

    try {
      const endpoint = chatMode.value === 'aiops' ? '/api/ai_ops' : '/api/chat_stream'
      const res = await fetch(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message: text, conversation_id: conversationId.value }),
        signal: abortCtrl.signal,
      })

      if (!res.ok) {
        updateAiMessage(aiMessageId, {
          isThinking: false,
          content: `[HTTP ${res.status}] 请求失败`,
        })
        return
      }

      await readSSEStream(res, aiMessageId, abortCtrl)
    } catch (err) {
      if (err instanceof Error && err.name === 'AbortError') return
      updateAiMessage(aiMessageId, {
        isThinking: false,
        isGenerating: false,
        content: '请求失败，请重试',
      })
    } finally {
      if (!abortCtrl.signal.aborted) {
        updateAiMessage(aiMessageId, { isThinking: false, isGenerating: false })
      }
      if (activeAbort === abortCtrl) {
        activeAbort = null
      }
      isSending.value = false
    }
  }

  // 读取 SSE 流
  async function readSSEStream(res: Response, aiMessageId: number, abortCtrl: AbortController) {
    const reader = res.body!.getReader()
    const decoder = new TextDecoder()
    let buffer = ''

    while (true) {
      if (abortCtrl.signal.aborted) {
        reader.cancel()
        break
      }

      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })

      const lines = buffer.split('\n')
      buffer = lines.pop() || ''

      let currentEvent = ''
      for (const line of lines) {
        if (line.startsWith('event:')) {
          currentEvent = line.slice(6).trim()
          continue
        }
        if (!line.startsWith('data:')) continue
        const payload = line.slice(5).trim()
        if (!payload) continue

        handleSSEEvent(currentEvent, payload, aiMessageId)
      }
    }
  }

  // 处理 SSE 事件
  function handleSSEEvent(event: string, payload: string, aiMessageId: number) {
    switch (event) {
      case 'thinking':
        // AI 开始思考/生成
        updateAiMessage(aiMessageId, { isThinking: false, isGenerating: true })
        break
      case 'delta':
        handleDeltaEvent(payload, aiMessageId)
        break
      case 'message':
        handleMessageEvent(payload, aiMessageId)
        break
      case 'tool':
        handleToolEvent(payload, aiMessageId)
        break
      case 'done':
        updateAiMessage(aiMessageId, { isThinking: false, isGenerating: false })
        break
      case 'error':
        handleErrorEvent(payload, aiMessageId)
        break
    }
    scrollToBottom()
  }

  // 处理 delta 事件
  function handleDeltaEvent(payload: string, aiMessageId: number) {
    try {
      const data = JSON.parse(payload)
      const aiMsg = findAiMessage(aiMessageId)
      if (aiMsg && data.content) {
        aiMsg.isThinking = false
        aiMsg.isGenerating = true
        aiMsg.content = (aiMsg.content || '') + data.content
      }
    } catch { /* ignore */ }
  }

  // 处理 message 事件
  function handleMessageEvent(payload: string, aiMessageId: number) {
    try {
      const data = JSON.parse(payload)
      const aiMsg = findAiMessage(aiMessageId)
      if (aiMsg && data.content) {
        aiMsg.isThinking = false
        aiMsg.isGenerating = false
        aiMsg.content = data.content
      }
    } catch { /* ignore */ }
  }

  // 处理 tool 事件
  function handleToolEvent(payload: string, aiMessageId: number) {
    try {
      const ev = JSON.parse(payload)
      const aiMsg = findAiMessage(aiMessageId)
      if (aiMsg) {
        aiMsg.isThinking = false
        aiMsg.isGenerating = false
      }

      switch (ev.type) {
        case 'tool_call':
          messages.value.push({
            id: Date.now() + Math.random(),
            role: 'tool_call',
            name: ev.name,
            args: ev.args,
            callId: ev.call_id,
          })
          break
        case 'tool_result':
          messages.value.push({
            id: Date.now() + Math.random(),
            role: 'tool_result',
            name: ev.name,
            result: ev.result,
            callId: ev.call_id,
          })
          break
        case 'permission_request':
          messages.value.push({
            id: Date.now() + Math.random(),
            role: 'permission_req',
            name: ev.name,
            args: ev.args,
            requestId: ev.request_id,
            reason: ev.reason,
            callId: ev.call_id,
            responded: false,
          })
          break
      }
    } catch { /* ignore */ }
  }

  // 处理 error 事件
  function handleErrorEvent(payload: string, aiMessageId: number) {
    try {
      const data = JSON.parse(payload)
      const aiMsg = findAiMessage(aiMessageId)
      if (aiMsg) {
        aiMsg.isThinking = false
        aiMsg.isGenerating = false
        aiMsg.content = (aiMsg.content || '') + `\n\n[错误: ${data.error}]`
      }
    } catch { /* ignore */ }
  }

  // 查找 AI 消息
  function findAiMessage(id: number): Message | undefined {
    return messages.value.find((m) => m.id === id)
  }

  // 更新 AI 消息
  function updateAiMessage(id: number, updates: Partial<Message>) {
    const aiMsg = findAiMessage(id)
    if (aiMsg) {
      Object.assign(aiMsg, updates)
    }
  }

  // 中止当前请求
  function abort() {
    if (activeAbort) {
      activeAbort.abort()
      activeAbort = null
    }
  }

  // 加载历史记录
  async function loadHistory(defaultMessage: string) {
    messages.value = []
    try {
      const res = await fetch(`/api/conversation/${conversationId.value}`)
      if (!res.ok) {
        messages.value.push({
          id: Date.now(),
          role: 'assistant',
          content: defaultMessage,
        })
        return
      }
      const data = await res.json()
      const lines = data.messages || []
      for (const line of lines) {
        if (line.role === 'user') {
          messages.value.push({
            id: Date.now() + Math.random(),
            role: 'user',
            content: line.content,
          })
        } else if (line.role === 'assistant') {
          if (line.tool_calls) {
            for (const tc of line.tool_calls) {
              messages.value.push({
                id: Date.now() + Math.random(),
                role: 'tool_call',
                name: tc.function.name,
                args: tc.function.arguments,
                callId: tc.id,
              })
            }
          }
          if (line.content) {
            messages.value.push({
              id: Date.now() + Math.random(),
              role: 'assistant',
              content: line.content,
            })
          }
        } else if (line.role === 'tool') {
          messages.value.push({
            id: Date.now() + Math.random(),
            role: 'tool_result',
            name: line.tool_name || '',
            result: line.content,
            callId: line.tool_call_id || '',
          })
        }
      }
      if (messages.value.length === 0) {
        messages.value.push({
          id: Date.now(),
          role: 'assistant',
          content: defaultMessage,
        })
      }
    } catch {
      messages.value.push({
        id: Date.now(),
        role: 'assistant',
        content: defaultMessage,
      })
    }
  }

  return {
    isSending,
    send,
    abort,
    loadHistory,
  }
}
