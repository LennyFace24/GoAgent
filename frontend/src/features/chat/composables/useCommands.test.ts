import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { useCommands, type Command } from '../composables/useCommands'

const SAMPLE_COMMANDS: Command[] = [
  { name: 'clear', description: '清空对话', usage: '/clear', category: 'conversation', type: 'client' },
  { name: 'compact', description: '压缩上下文', usage: '/compact', category: 'context', type: 'client' },
  { name: 'context', description: '查看上下文', usage: '/context', category: 'context', type: 'client' },
]

function mockCommandsEndpoint() {
  globalThis.fetch = vi.fn(async () =>
    new Response(JSON.stringify({ commands: SAMPLE_COMMANDS }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }),
  ) as unknown as typeof fetch
}

function mountCommands() {
  let ctx: ReturnType<typeof useCommands> | undefined
  mount(
    defineComponent({
      setup() {
        ctx = useCommands()
        return () => h('div')
      },
    }),
  )
  return ctx!
}

const flush = () => new Promise<void>((r) => setTimeout(r, 0))

describe('useCommands', () => {
  beforeEach(() => {
    mockCommandsEndpoint()
  })

  it('挂载后从 /api/commands 加载命令列表', async () => {
    const ctx = mountCommands()
    await flush()
    expect(ctx.commands.value.length).toBe(3)
    expect(ctx.commands.value.map((c) => c.name)).toEqual(['clear', 'compact', 'context'])
  })

  it('updateFromInput 按 / 前缀过滤命令名与描述', async () => {
    const ctx = mountCommands()
    await flush()

    ctx.updateFromInput('/c')
    expect(ctx.visible.value).toBe(true)
    expect(ctx.filteredCommands.value.map((c) => c.name).sort()).toEqual(['clear', 'compact', 'context'])

    ctx.updateFromInput('/cl')
    expect(ctx.filteredCommands.value.map((c) => c.name)).toEqual(['clear'])

    // 描述匹配：搜“压缩”应命中 compact
    ctx.updateFromInput('/压缩')
    expect(ctx.filteredCommands.value.map((c) => c.name)).toEqual(['compact'])

    // 非斜杠输入隐藏面板
    ctx.updateFromInput('hello')
    expect(ctx.visible.value).toBe(false)
  })

  it('updateFromInput 带空格（输入参数）时隐藏面板', async () => {
    const ctx = mountCommands()
    await flush()

    ctx.updateFromInput('/clear ')
    expect(ctx.visible.value).toBe(false)
  })

  it('selectCommand 返回命令名并隐藏面板', async () => {
    const ctx = mountCommands()
    await flush()
    ctx.updateFromInput('/cl')

    const name = ctx.selectCommand(SAMPLE_COMMANDS[0])
    expect(name).toBe('clear')
    expect(ctx.visible.value).toBe(false)
  })

  it('键盘导航：ArrowDown/Up 切换选中，Enter 选中并关闭，Escape 关闭', async () => {
    const ctx = mountCommands()
    await flush()
    ctx.updateFromInput('/c') // 3 条命令，selectedIndex=0

    ctx.onKeydown(new KeyboardEvent('keydown', { key: 'ArrowDown' }) as KeyboardEvent)
    expect(ctx.selectedIndex.value).toBe(1)

    ctx.onKeydown(new KeyboardEvent('keydown', { key: 'ArrowDown' }) as KeyboardEvent)
    expect(ctx.selectedIndex.value).toBe(2)

    ctx.onKeydown(new KeyboardEvent('keydown', { key: 'ArrowDown' }) as KeyboardEvent)
    expect(ctx.selectedIndex.value).toBe(0) // 环绕

    ctx.onKeydown(new KeyboardEvent('keydown', { key: 'ArrowUp' }) as KeyboardEvent)
    expect(ctx.selectedIndex.value).toBe(2) // 反向环绕

    // Enter 选中当前并关闭面板
    const handled = ctx.onKeydown(new KeyboardEvent('keydown', { key: 'Enter' }) as KeyboardEvent)
    expect(handled).toBe(true)
    expect(ctx.visible.value).toBe(false)

    // Escape
    ctx.updateFromInput('/c')
    ctx.onKeydown(new KeyboardEvent('keydown', { key: 'Escape' }) as KeyboardEvent)
    expect(ctx.visible.value).toBe(false)
  })

  it('面板不可见时 onKeydown 不拦截', async () => {
    const ctx = mountCommands()
    await flush()
    // 没有输入 /，面板不可见
    const handled = ctx.onKeydown(new KeyboardEvent('keydown', { key: 'Enter' }) as KeyboardEvent)
    expect(handled).toBe(false)
  })
})
