// @vitest-environment node
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import { useChat } from '../composables/useChat'

// 把 SSE 文本块编码为 Uint8Array
function enc(s: string): Uint8Array {
  return new TextEncoder().encode(s)
}

// 受控流：emit 按需推入一块，close 关闭。用于精确控制流式时序。
function makeStepStream() {
  let controller!: ReadableStreamDefaultController<Uint8Array>
  const stream = new ReadableStream<Uint8Array>({
    start(c) { controller = c },
  })
  return {
    stream,
    emit: (chunk: string) => controller.enqueue(enc(chunk)),
    close: () => controller.close(),
  }
}

// 一次性流：构造时即推入所有块并关闭
function makeOneShotStream(chunks: string[]): ReadableStream<Uint8Array> {
  return new ReadableStream<Uint8Array>({
    start(c) {
      for (const ch of chunks) c.enqueue(enc(ch))
      c.close()
    },
  })
}

const flush = () => new Promise<void>((r) => setTimeout(r, 0))

function mockFetchWith(stream: ReadableStream<Uint8Array>) {
  return vi.fn(async (url: string) => {
    if (typeof url === 'string' && url.startsWith('/api/conversation/')) {
      return new Response(JSON.stringify({ messages: [] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    }
    // /api/chat_stream 或 /api/ai_ops
    return new Response(stream, { status: 200, headers: { 'Content-Type': 'text/event-stream' } })
  }) as unknown as typeof fetch
}

function setupChat(initialConv = 'A') {
  const conversationId = ref(initialConv)
  const chatMode = ref<'chat' | 'aiops'>('chat')
  const scrollToBottom = vi.fn()
  const chat = useChat({ conversationId, chatMode, scrollToBottom, defaultWelcome: 'welcome' })
  return { conversationId, chat }
}

describe('useChat 跨会话事件路由', () => {
  beforeEach(() => {
    // 每个用例独立设置 fetch
  })

  it('后台流写入自己的会话缓冲，不泄漏到当前视图', async () => {
    const s = makeStepStream()
    globalThis.fetch = mockFetchWith(s.stream)

    const { conversationId, chat } = setupChat('A')
    await flush() // loadHistory(A) 完成

    // A 开始流式（流被阻塞，尚未推数据）
    const sendPromise = chat.send('hi')
    await flush()

    // 切换到 B，A 的流仍在后台
    conversationId.value = 'B'
    await flush()

    // B 不是发送中；B 视图里没有 A 的内容
    expect(chat.isSending.value).toBe(false)
    expect(chat.messages.value.some((m: any) => m.content?.includes('hello'))).toBe(false)

    // 释放 A 的流：delta + done
    s.emit('event: delta\ndata: {"content":"hello"}\n\n')
    s.emit('data: "done"\n\n')
    s.close()
    await sendPromise
    await flush()

    // 仍在看 B，A 的内容不该出现在 B 视图
    expect(chat.messages.value.some((m: any) => m.content?.includes('hello'))).toBe(false)

    // 切回 A：累积的内容应可见
    conversationId.value = 'A'
    await flush()
    expect(chat.messages.value.some((m: any) => m.role === 'assistant' && m.content?.includes('hello'))).toBe(true)
  })

  it('isSending 按会话独立：A 流式中切到 B，B 的 isSending 为 false', async () => {
    const s = makeStepStream()
    globalThis.fetch = mockFetchWith(s.stream)
    const { conversationId, chat } = setupChat('A')
    await flush()

    const sendPromise = chat.send('hi')
    await flush()
    expect(chat.isSending.value).toBe(true) // A 在发送

    conversationId.value = 'B'
    await flush()
    expect(chat.isSending.value).toBe(false) // B 不在发送

    // 收尾 A 的流，避免悬挂
    s.emit('data: "done"\n\n')
    s.close()
    await sendPromise
    await flush()
  })
})

describe('useChat 停止标记', () => {
  it('abort 在最后一条流式 assistant 消息末尾追加终止标记', async () => {
    const s = makeStepStream()
    globalThis.fetch = mockFetchWith(s.stream)
    const { chat } = setupChat('A')
    await flush()

    const sendPromise = chat.send('hi')
    await flush()

    // 推一部分内容，形成一条 streaming 的 assistant 消息
    s.emit('event: delta\ndata: {"content":"partial"}\n\n')
    await flush()

    // 用户主动停止
    chat.abort()
    // 关闭流以解除 reader.read() 阻塞，让 send 收尾
    s.close()
    await sendPromise
    await flush()

    const last = chat.messages.value[chat.messages.value.length - 1] as any
    expect(last.role).toBe('assistant')
    expect(last.content).toContain('partial')
    expect(last.content).toContain('该回答此处被用户终止')
    expect(last.streaming).toBe(false)
  })
})

describe('useChat 基本发送', () => {
  it('一次性流：消息正确写入当前会话缓冲', async () => {
    const stream = makeOneShotStream([
      'event: delta\ndata: {"content":"hello"}\n\n',
      'data: "done"\n\n',
    ])
    globalThis.fetch = mockFetchWith(stream)
    const { chat } = setupChat('A')
    await flush()

    await chat.send('hi')
    await flush()

    const msgs = chat.messages.value as any[]
    // [welcome, user 'hi', assistant 'hello']
    expect(msgs.some((m) => m.role === 'user' && m.content === 'hi')).toBe(true)
    expect(msgs.some((m) => m.role === 'assistant' && m.content === 'hello')).toBe(true)
    expect(chat.isSending.value).toBe(false)
  })
})
