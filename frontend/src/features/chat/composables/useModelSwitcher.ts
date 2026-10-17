import { ref } from 'vue'

// ---------- 模型切换 composable ----------
// 负责：加载模型列表 / 当前模型、切换模型、错误处理。
// 与 ChatWorkspace 解耦，便于单测。

export interface ModelOption {
  id: string
  name: string
}

export function useModelSwitcher() {
  const models = ref<ModelOption[]>([])
  const currentModel = ref('')
  const switchError = ref('')
  // 加载模型列表与当前模型；model.json 加载失败时 error 字段透出
  async function loadModels(): Promise<void> {
    try {
      const res = await fetch('/api/model', { credentials: 'include' })
      if (res.ok) {
        const data = await res.json()
        if (data.error) {
          switchError.value = data.error
        }
        models.value = (data.models || []).map((m: { id: string; name?: string }) => ({
          id: m.id,
          name: m.name || m.id,
        }))
        currentModel.value = data.current || ''
      } else {
        switchError.value = `模型服务异常 HTTP ${res.status}`
      }
    } catch {
      switchError.value = '模型服务不可用'
    }
  }
  // 切换模型；成功更新 currentModel，失败写入 switchError
  async function switchModel(id: string): Promise<boolean> {
    if (id === currentModel.value) return true
    switchError.value = ''
    try {
      const res = await fetch('/api/model', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ id }),
      })
      if (!res.ok) {
        const err = await res.json().catch(() => ({}))
        switchError.value = err.error || '切换失败'
        return false
      }
      const data = await res.json()
      currentModel.value = data.current || id
      return true
    } catch {
      switchError.value = '切换失败，请重试'
      return false
    }
  }

  return { models, currentModel, switchError, loadModels, switchModel }
}
