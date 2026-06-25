<script setup lang="ts">
import { ref, watch } from 'vue'
import CommandPalette from './CommandPalette.vue'
import { useCommands, type Command } from '../composables/useCommands'

// ---------- 输入栏组件（纯输入：只负责文本输入、命令面板、发送） ----------

const props = defineProps<{
  mode: string
  sending: boolean
}>()

const emit = defineEmits<{
  send: [text: string]
  stop: []
}>()

const input = ref<string>('')
const isComposing = ref(false) // 标记是否正在进行 IME 组合输入

const placeholders: Record<string, string> = {
  chat: '向 AI 助理提问或下达运维诊断指令... 输入 / 查看可用命令',
  aiops: '描述故障现象，如：API 响应时间从 200ms 飙升到 3s，错误率 12%',
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

// IME 组合输入事件
function onCompositionStart(): void {
  isComposing.value = true
}

function onCompositionEnd(e: CompositionEvent): void {
  isComposing.value = false
  // 组合结束后，手动同步 input 的值（某些 IME 不会自动触发 input 事件）
  input.value = (e.target as HTMLTextAreaElement).value
}

// 选中命令：填入输入框供用户确认后再发送，不立即执行
function pickCommand(cmd: Command): void {
  input.value = `/${cmd.name} `
  selectCommand(cmd)
}

// 键盘事件处理
function onKeydown(e: KeyboardEvent): void {
  // 命令面板可见时
  if (paletteVisible.value) {
    // Enter：把选中的命令填入输入框（供确认），不直接发送
    if (e.key === 'Enter' && !e.shiftKey) {
      const cmd = filteredCommands.value[selectedIndex.value]
      if (cmd) {
        e.preventDefault()
        pickCommand(cmd)
        return
      }
    }
    // 其余导航键（↑↓ Esc）交给命令面板
    const handled = onCommandKeydown(e)
    if (handled) return
  }

  // Enter 发送（IME 组合输入期间跳过，避免误发送）
  if (e.key === 'Enter' && !e.shiftKey && !isComposing.value) {
    e.preventDefault()
    doSend()
  }
}

function doSend(): void {
  const text = input.value.trim()
  if (!text) return
  input.value = ''
  emit('send', text)
}

// 按钮点击：发送中 → 停止；否则 → 发送
function onButtonClick(): void {
  if (props.sending) {
    emit('stop')
  } else {
    doSend()
  }
}
</script>

<template>
  <div class="input-bar">
    <div class="input-wrapper">
      <CommandPalette
        :commands="filteredCommands"
        v-model:selectedIndex="selectedIndex"
        :visible="paletteVisible"
        @select="pickCommand"
      />
      <div class="input-row">
        <textarea
          v-model="input"
          :placeholder="placeholders[mode] || '输入消息...'"
          rows="1"
          :disabled="sending"
          @keydown="onKeydown"
          @compositionstart="onCompositionStart"
          @compositionend="onCompositionEnd"
        ></textarea>
        <button
          class="send-btn"
          :class="{ 'is-stop': sending }"
          :disabled="!sending && !input.trim()"
          @click="onButtonClick"
        >
          <svg v-if="!sending" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/></svg>
          <svg v-else width="16" height="16" viewBox="0 0 24 24" fill="currentColor"><rect x="6" y="6" width="12" height="12" rx="2"/></svg>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.input-bar {
  display: flex;
}

.input-wrapper {
  flex: 1;
  position: relative;
}

.input-row {
  display: flex; gap: 8px; align-items: flex-end;
}
.input-row textarea {
  flex: 1; background: var(--bg-input); border: 1px solid var(--border-input);
  border-radius: 8px; color: var(--text-primary); padding: 10px 14px;
  font-size: 14px; outline: none; resize: none;
  font-family: inherit; line-height: 1.5;
  min-height: 42px; max-height: 120px;
  transition: border-color var(--transition-fast);
}
.input-row textarea:focus { border-color: var(--accent); }
.input-row textarea::placeholder { color: var(--text-secondary); }

.send-btn {
  width: 42px; height: 42px; border-radius: 8px;
  background: var(--accent); border: none; color: #fff;
  cursor: pointer; display: flex; align-items: center; justify-content: center;
  flex-shrink: 0; transition: opacity var(--transition-fast);
}
.send-btn:hover { opacity: 0.9; }
.send-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.send-btn.is-stop { background: #ef4444; }
.send-btn.is-stop:hover { opacity: 0.9; }
</style>
