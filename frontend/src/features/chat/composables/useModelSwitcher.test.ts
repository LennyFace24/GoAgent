// @vitest-environment node
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { useModelSwitcher } from '../composables/useModelSwitcher'

const SAMPLE_RESPONSE = {
  models: [
    { id: 'mimo', name: '小米 MiMo', base_url: 'https://a.com/v1', api_key: 'k1' },
    { id: 'gpt', base_url: 'https://b.com/v1', api_key: 'k2' }, // 无 name → 回退 id
  ],
  current: 'mimo',
}

function mockFetch(impl: (url: string, init?: RequestInit) => Promise<Response>) {
  const fn = vi.fn(impl)
  vi.stubGlobal('fetch', fn)
  return fn
}

describe('useModelSwitcher', () => {
  beforeEach(() => {
    mockFetch(async (url, init) => {
      if (url === '/api/model' && (!init || !init.method || init.method === 'GET')) {
        return new Response(JSON.stringify(SAMPLE_RESPONSE), { status: 200 })
      }
      if (url === '/api/model' && init?.method === 'POST') {
        const body = JSON.parse(String(init.body))
        if (body.id === 'gpt') {
          return new Response(JSON.stringify({ current: 'gpt' }), { status: 200 })
        }
        return new Response(JSON.stringify({ error: '模型 "nope" 不存在' }), { status: 400 })
      }
      return new Response('not found', { status: 404 })
    })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('loadModels 填充列表（name 缺省回退 id）与当前模型', async () => {
    const sw = useModelSwitcher()
    await sw.loadModels()
    expect(sw.models.value.length).toBe(2)
    expect(sw.models.value[1].name).toBe('gpt') // 无 name 回退 id
    expect(sw.currentModel.value).toBe('mimo')
  })

  it('loadModels 失败时保持空状态不抛错', async () => {
    mockFetch(async () => new Response('err', { status: 500 }))
    const sw = useModelSwitcher()
    await sw.loadModels()
    expect(sw.models.value.length).toBe(0)
    expect(sw.currentModel.value).toBe('')
  })

  it('switchModel 成功更新当前模型', async () => {
    const sw = useModelSwitcher()
    await sw.loadModels()
    const ok = await sw.switchModel('gpt')
    expect(ok).toBe(true)
    expect(sw.currentModel.value).toBe('gpt')
    expect(sw.switchError.value).toBe('')
  })

  it('switchModel 失败写入错误且不改变当前模型', async () => {
    const sw = useModelSwitcher()
    await sw.loadModels()
    const ok = await sw.switchModel('nope')
    expect(ok).toBe(false)
    expect(sw.currentModel.value).toBe('mimo') // 保持原模型
    expect(sw.switchError.value).toContain('不存在')
  })

  it('switchModel 切换相同模型直接成功不请求', async () => {
    const fn = mockFetch(async (url, init) => {
      if (url === '/api/model' && init?.method === 'POST') {
        return new Response(JSON.stringify({ current: 'mimo' }), { status: 200 })
      }
      return new Response(JSON.stringify({ models: [], current: 'mimo' }), { status: 200 })
    })
    const sw = useModelSwitcher()
    await sw.loadModels()
    const ok = await sw.switchModel('mimo') // 与当前相同
    expect(ok).toBe(true)
    // 相同 id 不应发 POST
    const postCalls = fn.mock.calls.filter(([, init]) => init?.method === 'POST')
    expect(postCalls.length).toBe(0)
  })

  it('网络异常时 switchModel 返回失败并提示', async () => {
    mockFetch(async () => {
      throw new Error('network down')
    })
    const sw = useModelSwitcher()
    const ok = await sw.switchModel('gpt')
    expect(ok).toBe(false)
    expect(sw.switchError.value).toContain('切换失败')
  })
})
