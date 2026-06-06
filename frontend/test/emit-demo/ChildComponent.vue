<script setup lang="ts">
import { ref } from 'vue'

// 定义组件可以触发的事件
const emit = defineEmits<{
  'message': [text: string]      // 发送消息事件
  'count': [value: number]       // 计数事件
  'delete': [id: number]         // 删除事件
}>()

const inputText = ref('')
const count = ref(0)

function sendMessage() {
  if (inputText.value.trim()) {
    // 触发 'message' 事件，传递文本给父组件
    emit('message', inputText.value)
    inputText.value = ''
  }
}

function increment() {
  count.value++
  // 触发 'count' 事件，传递当前计数给父组件
  emit('count', count.value)
}

function deleteItem(id: number) {
  // 触发 'delete' 事件，传递要删除的 ID
  emit('delete', id)
}
</script>

<template>
  <div class="child">
    <h3>子组件</h3>

    <!-- 发送消息 -->
    <div class="section">
      <input v-model="inputText" placeholder="输入消息..." @keyup.enter="sendMessage" />
      <button @click="sendMessage">发送消息</button>
    </div>

    <!-- 计数器 -->
    <div class="section">
      <span>计数: {{ count }}</span>
      <button @click="increment">+1</button>
      <button @click="deleteItem(123)">删除 ID:123</button>
    </div>
  </div>
</template>

<style scoped>
.child {
  border: 2px solid #42b883;
  padding: 16px;
  border-radius: 8px;
  background: #f0fff0;
}

.section {
  margin: 10px 0;
  display: flex;
  gap: 8px;
  align-items: center;
}

input {
  padding: 6px 10px;
  border: 1px solid #ccc;
  border-radius: 4px;
}

button {
  padding: 6px 12px;
  background: #42b883;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}

button:hover {
  background: #36a06e;
}
</style>
