import { ref } from 'vue'

export function useFileUpload() {
  const file = ref<File | null>(null)
  const uploading = ref<boolean>(false)
  const msg = ref<string>('')

  function onFileChange(e: Event): void {
    const target = e.target as HTMLInputElement
    file.value = target.files?.[0] || null
    msg.value = ''
  }

  async function upload(): Promise<void> {
    if (!file.value || uploading.value) return
    uploading.value = true
    msg.value = ''
    try {
      const form = new FormData()
      form.append('file', file.value)
      const res = await fetch('/api/upload_file', { method: 'POST', body: form })
      const data = await res.json()
      msg.value = data.error ? `错误: ${data.error}` : `已上传: ${data.file}`
    } catch (e) {
      msg.value = `失败: ${e instanceof Error ? e.message : String(e)}`
    } finally {
      uploading.value = false
    }
  }

  return { file, uploading, msg, onFileChange, upload }
}
