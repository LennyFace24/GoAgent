<script setup lang="ts">
import { ref, watch } from 'vue'
import CommandPalette from './CommandPalette.vue'
import { useCommands, type Command } from '../composables/useCommands'

// ---------- 输入栏组件 ----------

defineProps<{
  mode: string
  sending: boolean
}>()

const emit = defineEmits<{
  send: [text: string]
  'update:mode': [value: string]
}>()

const input = ref<string>('')
const modes = [
  { key: 'chat_stream', label: 'Chat Stream' },
  { key: 'ai_ops', label: 'AI Ops' },
]

const placeholders: Record<string, string> = {
  chat_stream: '输入消息，Enter 发送，Shift+Enter 换行。输入 / 查看可用命令',
  ai_ops: '描述故障现象，如：API 响应时间从 200ms 飙升到 3s，错误率 12%',
}

// 斜杠命令
const {
  visible: paletteVisible,
  selectedIndex,
  filteredCommands,
  updateFromInput,
  onKeydown: onCommandKeydown,
  selectCommand,
} = useCommands()

// 监听输入变化，更新命令面板
watch(input, (val) => {
  updateFromInput(val)
})

// 键盘事件处理
function onKeydown(e: KeyboardEvent): void {
  // 先交给命令面板处理
  if (paletteVisible.value) {
    const handled = onCommandKeydown(e)
    if (handled) return
  }

  // Enter 发送
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    doSend()
  }
}

// 选择命令
function onCommandSelect(cmd: Command): void {
  const name = selectCommand(cmd)
  executeCommand(name)
}

// 执行命令
function executeCommand(name: string): void {
  switch (name) {
    case 'clear':
      // 清空输入，发送清空命令
      input.value = ''
      emit('send', '/clear')
      break
    case 'new':
      input.value = ''
      emit('send', '/new')
      break
    case 'compact':
      input.value = ''
      emit('send', '/compact')
      break
    case 'context':
      input.value = ''
      emit('send', '/context')
      break
    case 'help':
      input.value = ''
      emit('send', '/help')
      break
    case 'model':
      input.value = ''
      emit('send', '/model')
      break
    default:
      // 未知命令，填入输入框让用户继续编辑
      input.value = `/${name} `
      break
  }
}

function doSend(): void {
  const text = input.value.trim()
  if (!text) return
  input.value = ''
  emit('send', text)
}
</script>

<template>
  <div class="input-bar">
    <div class="mode-selector">
      <button
        v-for="m in modes" :key="m.key"
        :class="['mode-pill', { active: mode === m.key }]"
        @click="emit('update:mode', m.key)"
      >{{ m.label }}</button>
    </div>
    <div class="input-wrapper">
      <CommandPalette
        :commands="filteredCommands"
        :selectedIndex="selectedIndex"
        :visible="paletteVisible"
        @select="onCommandSelect"
      />
      <div class="input-row">
        <textarea
          v-model="input"
          :placeholder="placeholders[mode] || '输入消息...'"
          rows="1"
          :disabled="sending"
          @keydown="onKeydown"
        ></textarea>
        <button class="send-btn" :disabled="!input.trim() || sending" @click="doSend">
          <svg v-if="!sending" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/></svg>
          <svg v-else width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10" stroke-dasharray="32" stroke-dashoffset="12"><animateTransform attributeName="transform" type="rotate" values="0 12 12;360 12 12" dur="0.8s" repeatCount="indefinite"/></circle></svg>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.input-bar {
  padding: 10px 32px 16px;
  border-top: 1px solid var(--border-color);
}

.mode-selector {
  display: flex; gap: 0; margin-bottom: 10px;
  border-bottom: 1px solid var(--border-color);
}
.mode-pill {
  background: none; border: none; border-bottom: 2px solid transparent;
  color: var(--text-muted); padding: 6px 14px 8px;
  font-size: 0.75rem; font-weight: 500; cursor: pointer;
  transition: all var(--transition-fast); white-space: nowrap;
}
.mode-pill:hover { color: var(--text-secondary); }
.mode-pill.active {
  color: var(--text-primary); border-bottom-color: var(--accent);
}

.input-wrapper {
  position: relative;
  max-width: 768px;
  margin: 0 auto;
}

.input-row {
  display: flex; gap: 8px; align-items: flex-end;
}
.input-row textarea {
  flex: 1; background: var(--bg-input); border: 1px solid var(--border-input);
  border-radius: 16px; color: var(--text-primary); padding: 10px 16px;
  font-size: 0.875rem; outline: none; resize: none;
  font-family: inherit; line-height: 1.5;
  min-height: 42px; max-height: 120px;
  transition: border-color var(--transition-fast);
}
.input-row textarea:focus { border-color: var(--border-focus); }
.input-row textarea::placeholder { color: var(--text-muted); }

.send-btn {
  width: 42px; height: 42px; border-radius: 12px;
  background: var(--accent); border: none; color: #fff;
  cursor: pointer; display: flex; align-items: center; justify-content: center;
  flex-shrink: 0; transition: opacity var(--transition-fast);
}
.send-btn:hover { opacity: 0.85; }
.send-btn:disabled { opacity: 0.25; cursor: not-allowed; }
</style>
