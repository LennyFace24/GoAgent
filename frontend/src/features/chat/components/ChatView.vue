<script setup lang="ts">
import { ref, watch, onMounted, nextTick, toRef } from 'vue'
import type { Message, PermissionMessage } from '../../../shared/types'
import { useMessages } from '../composables/useMessages'
import { useSSE } from '../composables/useSSE'
import MessageBubble from './MessageBubble.vue'
import ToolCallBlock from './ToolCallBlock.vue'
import ToolResultBlock from './ToolResultBlock.vue'
import PermissionCard from './PermissionCard.vue'
import InputBar from './InputBar.vue'

const props = defineProps<{
  mode: string
  conversationId: string
}>()

const emit = defineEmits<{
  'update:mode': [value: string]
}>()

const msgBox = ref<HTMLElement | null>(null)
const conversationIdRef = toRef(props, 'conversationId')
const { messages, loadHistory } = useMessages(conversationIdRef)
const { sending, send, respondPermission } = useSSE(messages)

function scrollBottom(): void {
  nextTick(() => {
    if (msgBox.value) msgBox.value.scrollTop = msgBox.value.scrollHeight
  })
}

async function handleSend(text: string): Promise<void> {
  messages.value.push({ id: Date.now() + Math.random(), role: 'user', content: text })
  scrollBottom()
  await send(text, props.mode, props.conversationId, msgBox.value)
  scrollBottom()
}

function handlePermission(msg: PermissionMessage, approved: boolean, always: boolean): void {
  respondPermission(msg, approved, always)
}

onMounted(() => { loadHistory() })
watch(() => props.conversationId, () => { loadHistory() })
</script>

<template>
  <div class="chat-container">
    <div class="msg-area" ref="msgBox">
      <div v-if="messages.length === 0" class="empty-state">
        <div class="empty-text">发送一条消息开始对话</div>
      </div>

      <template v-for="msg in messages" :key="msg.id">
        <ToolCallBlock
          v-if="msg.role === 'tool_call'"
          :name="msg.name"
          :args="msg.args"
        />
        <ToolResultBlock
          v-else-if="msg.role === 'tool_result'"
          :name="msg.name"
          :result="msg.result"
        />
        <PermissionCard
          v-else-if="msg.role === 'permission_req'"
          :message="msg as PermissionMessage"
          @respond="(approved, always) => handlePermission(msg as PermissionMessage, approved, always)"
        />
        <MessageBubble v-else :message="msg" />
      </template>
    </div>

    <InputBar
      :mode="mode"
      :sending="sending"
      @send="handleSend"
      @update:mode="emit('update:mode', $event)"
    />
  </div>
</template>

<style scoped>
.chat-container {
  flex: 1; display: flex; flex-direction: column;
  min-width: 0; height: 100vh;
}

.msg-area {
  flex: 1; overflow-y: auto; padding: 32px 48px;
  display: flex; flex-direction: column; gap: 20px;
  scrollbar-width: thin;
  scrollbar-color: rgba(128,128,160,0.15) transparent;
}
.msg-area::-webkit-scrollbar { width: 5px; }
.msg-area::-webkit-scrollbar-track { background: transparent; }
.msg-area::-webkit-scrollbar-thumb {
  background: rgba(128,128,160,0.15); border-radius: 3px;
}
.msg-area::-webkit-scrollbar-thumb:hover {
  background: rgba(128,128,160,0.3);
}

.empty-state {
  margin: auto; text-align: center;
}
.empty-text {
  font-size: 0.82rem; color: var(--text-muted); letter-spacing: 0.02em;
}
</style>
