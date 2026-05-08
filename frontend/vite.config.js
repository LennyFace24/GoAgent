import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 3000,
    proxy: {
      '^/(chat|chat_stream|ai_ops|upload_file|search|history|conversations?|conversation)': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
})
