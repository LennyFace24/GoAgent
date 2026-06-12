<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, nextTick, toRef } from 'vue'
import type { Message, PermissionMessage, MessageGroup, ThinkingMessage, ToolCallMessage, ToolResultMessage, ChatMessage } from '../../../shared/types'
import { useMessages } from '../composables/useMessages'
import { useSSE } from '../composables/useSSE'
import MessageBubble from './MessageBubble.vue'
import ToolCallBlock from './ToolCallBlock.vue'
import ToolResultBlock from './ToolResultBlock.vue'
import PermissionCard from './PermissionCard.vue'
import ThinkingBlock from './ThinkingBlock.vue'
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
const { sending, send, abort, respondPermission } = useSSE(messages)

// ---- 消息分组逻辑 ----
const messageGroups = ref<MessageGroup[]>([])
let currentGroup: MessageGroup = { thinking: null, toolCalls: [], toolResults: [], reply: null }

// 监听消息数量变化，处理新增消息
watch(() => messages.value.length, (newLen, oldLen) => {
  const addedMessages = messages.value.slice(oldLen)
  for (const msg of addedMessages) {
    switch (msg.role) {
      case 'thinking':
        if (currentGroup.reply) {
          messageGroups.value.push(currentGroup)
          currentGroup = { thinking: msg as ThinkingMessage, toolCalls: [], toolResults: [], reply: null }
        } else {
          currentGroup.thinking = msg as ThinkingMessage
        }
        break
      case 'tool_call':
        currentGroup.toolCalls.push(msg as ToolCallMessage)
        break
      case 'tool_result':
        currentGroup.toolResults.push(msg as ToolResultMessage)
        break
      case 'assistant':
        currentGroup.reply = msg as ChatMessage
        messageGroups.value.push(currentGroup)
        currentGroup = { thinking: null, toolCalls: [], toolResults: [], reply: null }
        break
      // user 和 permission_req 不需要分组，直接渲染
    }
  }
})

// 对话切换时重置分组
watch(() => props.conversationId, () => {
  messageGroups.value = []
  currentGroup = { thinking: null, toolCalls: [], toolResults: [], reply: null }
})

onUnmounted(() => {
  messageGroups.value = []
})

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
watch(() => props.conversationId, () => {
  abort()
  loadHistory()
})
</script>


<template>
  <div class="chat-container">
    <div class="msg-area" ref="msgBox">
      <div v-if="messages.length === 0" class="empty-state">
        <div class="empty-text">发送一条消息开始对话</div>
      </div>

      <template v-for="msg in messages" :key="msg.id">
        <!-- 用户消息：直接渲染 -->
        <MessageBubble v-if="msg.role === 'user'" :message="msg" />

        <!-- 权限请求：直接渲染 -->
        <PermissionCard
          v-else-if="msg.role === 'permission_req'"
          :message="msg as PermissionMessage"
          @respond="(approved, always) => handlePermission(msg as PermissionMessage, approved, always)"
        />
      </template>

      <!-- AI 分组消息：按组渲染 -->
      <template v-for="(group, idx) in messageGroups" :key="idx">
        <ThinkingBlock v-if="group.thinking" :content="group.thinking.content" />

        <template v-for="(tc, i) in group.toolCalls" :key="tc.id">
          <ToolCallBlock :name="tc.name" :args="tc.args" />
          <ToolResultBlock
            v-if="group.toolResults[i]"
            :name="group.toolResults[i].name"
            :result="group.toolResults[i].result"
          />
        </template>

        <MessageBubble v-if="group.reply" :message="group.reply" />
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
