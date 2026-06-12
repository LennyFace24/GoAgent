import { ref, nextTick, type Ref } from 'vue'
import type { Message, ToolEventData, ThinkingMessage, ChatMessage } from '../../../shared/types'

export function useSSE(messages: Ref<Message[]>) {
  const sending = ref<boolean>(false)
  let currentAbort: AbortController | null = null

  function scrollBottom(el: HTMLElement | null): void {
    nextTick(() => {
      if (el) el.scrollTop = el.scrollHeight
    })
  }

  /** 获取或创建当前正在流式输出的 assistant 消息 */
  function getOrCreateAssistant(): ChatMessage {
    const last = messages.value[messages.value.length - 1]
    if (last && last.role === 'assistant' && last.streaming) {
      return last as ChatMessage
    }
    // 没有正在流式的 assistant，创建一个新的
    const msg: ChatMessage = {
      id: Date.now() + Math.random(),
      role: 'assistant',
      content: '',
      streaming: true,
    }
    messages.value.push(msg)
    return msg
  }

  /** 结束当前 assistant 消息的流式状态 */
  function finishStreaming(): void {
    const last = messages.value[messages.value.length - 1]
    if (last && last.role === 'assistant') {
      last.streaming = false
    }
  }

  /** 取消当前正在进行的 SSE 请求 */
  function abort(): void {
    if (currentAbort) {
      currentAbort.abort()
      currentAbort = null
    }
    sending.value = false
  }

  async function send(
    text: string,
    mode: string,
    conversationId: string,
    scrollEl: HTMLElement | null,
  ): Promise<void> {
    // 取消之前的请求
    abort()

    const abortCtrl = new AbortController()
    currentAbort = abortCtrl
    sending.value = true

    try {
      const base = import.meta.env.VITE_API_URL || window.location.origin
      const endpoint = mode === 'ai_ops' ? '/api/ai_ops' : '/api/chat_stream'
      const url = base + endpoint
      console.log('[SSE] 请求 URL:', url)
      const res = await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message: text, conversation_id: conversationId }),
        signal: abortCtrl.signal,
      })
      console.log('[SSE] 响应状态:', res.status, res.statusText)
      console.log('[SSE] 响应头:', Object.fromEntries(res.headers.entries()))

      if (!res.ok) {
        const errAssistant = getOrCreateAssistant()
        errAssistant.content += `\n\n[HTTP ${res.status}]`
        errAssistant.streaming = false
        return
      }

      if (!res.body) {
        console.error('[SSE] 响应 body 为空')
        return
      }

      const reader = res.body.getReader()

      const decoder = new TextDecoder()
      let buffer = ''
      let pendingEvent = '' // 缓存当前事件类型

      while (true) {
        // 检查是否被取消
        if (abortCtrl.signal.aborted) {
          reader.cancel()
          break
        }

        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })

        // 按双换行分割完整事件
        const events = buffer.split('\n\n')
        buffer = events.pop() || '' // 最后一个可能不完整，留到下次处理
        console.log('Received events:', events)
        console.log('Current pending event type:', buffer)

        for (const event of events) {
          if (!event.trim()) continue

          let eventType = pendingEvent
          let eventData = ''

          // 解析事件的每一行
          for (const line of event.split('\n')) {
            if (line.startsWith('event:')) {
              eventType = line.slice(6).trim()
            } else if (line.startsWith('data:')) {
              eventData = line.slice(5).trim()
            }
          }

          // 如果只有 event 没有 data，缓存事件类型等待下一个事件
          if (eventType && !eventData) {
            pendingEvent = eventType
            continue
          }

          // 如果只有 data 没有 event，使用缓存的事件类型
          if (!eventType && eventData) {
            eventType = pendingEvent
          }

          pendingEvent = '' // 重置

          if (!eventData) continue
          if (eventData === '"done"' || eventData === 'done') continue

          try {
            const data = JSON.parse(eventData)

            // ---- thinking_delta ----
            if (eventType === 'thinking_delta' && data.content) {
              const lastMsg = messages.value[messages.value.length - 1]
              if (lastMsg && lastMsg.role === 'thinking' && lastMsg.streaming) {
                lastMsg.content += data.content
              } else {
                messages.value.push({
                  id: Date.now() + Math.random(),
                  role: 'thinking',
                  content: data.content,
                  streaming: true,
                })
              }
              scrollBottom(scrollEl)
              continue
            }

            // ---- thinking_done ----
            if (eventType === 'thinking_done') {
              const lastMsg = messages.value[messages.value.length - 1]
              if (lastMsg && lastMsg.role === 'thinking') {
                lastMsg.streaming = false
              }
              continue
            }

            // ---- tool 事件 ----
            if (eventType === 'tool') {
              if (data.type === 'tool_call') {
                messages.value.push({
                  id: Date.now() + Math.random(),
                  role: 'tool_call',
                  name: data.name,
                  args: data.args,
                  callId: data.call_id,
                })
              } else if (data.type === 'tool_result') {
                messages.value.push({
                  id: Date.now() + Math.random(),
                  role: 'tool_result',
                  name: data.name,
                  result: data.result,
                  callId: data.call_id,
                  collapsed: true,
                })
              } else if (data.type === 'permission_request') {
                messages.value.push({
                  id: Date.now() + Math.random(),
                  role: 'permission_req',
                  requestId: data.request_id!,
                  name: data.name,
                  args: data.args,
                  reason: data.reason,
                  responded: false,
                })
              }
              scrollBottom(scrollEl)
              continue
            }

            // ---- delta（旧格式兼容）----
            if (eventType === 'delta' && data.content) {
              const assistant = getOrCreateAssistant()
              assistant.content += data.content
              scrollBottom(scrollEl)
              continue
            }

            // ---- message（旧格式兼容）----
            if (eventType === 'message' && data.content) {
              const assistant = getOrCreateAssistant()
              assistant.content += data.content
              scrollBottom(scrollEl)
              continue
            }

            // ---- error ----
            if (eventType === 'error' || data.error) {
              const assistant = getOrCreateAssistant()
              assistant.content += `\n\n[错误: ${data.error || '未知错误'}]`
              continue
            }

            // ---- 未知事件，尝试作为 delta 处理 ----
            if (data.content) {
              const assistant = getOrCreateAssistant()
              assistant.content += data.content
              scrollBottom(scrollEl)
            }
          } catch { /* ignore parse errors */ }
        }
      }

    } catch (e) {
      if (abortCtrl.signal.aborted) return // 被取消，不显示错误
      const errAssistant = getOrCreateAssistant()
      errAssistant.content += `\n\n[连接断开: ${e instanceof Error ? e.message : String(e)}]`
    } finally {
      if (!abortCtrl.signal.aborted) {
        finishStreaming()
      }
      if (currentAbort === abortCtrl) {
        currentAbort = null
      }
      sending.value = false
    }
  }

  async function respondPermission(msg: Message, approved: boolean, always = false): Promise<void> {
    if (msg.role !== 'permission_req') return
    msg.responded = true
    msg.approved = approved
    try {
      const base = import.meta.env.VITE_API_URL || window.location.origin
      await fetch(base + '/api/permission/response', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: msg.requestId, approved, always }),
      })
    } catch { /* ignore */ }
  }


  return { sending, send, abort, respondPermission }
}
