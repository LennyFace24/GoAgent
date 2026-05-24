import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 3000,
    fs: {
      strict: true,
      allow: ['.'] // 严格限制 Vite FS 作用域在当前前端工作区，避免向上越界触发沙箱 Denied 错误
    },
    proxy: {
      '^/(chat|chat_stream|ai_ops|upload_file|search|history|conversations?|conversation|api/metrics|permission/response)': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
})