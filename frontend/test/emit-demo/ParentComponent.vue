<script setup lang="ts">
import { ref } from 'vue'
import ChildComponent from './ChildComponent.vue'

const messages = ref<string[]>([])
const currentCount = ref(0)
const deletedIds = ref<number[]>([])

// 处理子组件发送的消息
function handleMessage(text: string) {
  messages.value.push(text)
  console.log('收到消息:', text)
}

// 处理子组件的计数更新
function handleCount(value: number) {
  currentCount.value = value
  console.log('计数更新:', value)
}

// 处理子组件的删除请求
function handleDelete(id: number) {
  deletedIds.value.push(id)
  console.log('删除 ID:', id)
}
</script>

<template>
  <div class="parent">
    <h2>父组件</h2>

    <!-- 显示接收到的数据 -->
    <div class="status">
      <div>📨 收到的消息: {{ messages.join(', ') || '暂无' }}</div>
      <div>🔢 当前计数: {{ currentCount }}</div>
      <div>🗑️ 已删除 ID: {{ deletedIds.join(', ') || '暂无' }}</div>
    </div>

    <!-- 子组件 - 监听 emit 事件 -->
    <ChildComponent
      @message="handleMessage"
      @count="handleCount"
      @delete="handleDelete"
    />
  </div>
</template>

<style scoped>
.parent {
  border: 2px solid #3498db;
  padding: 20px;
  border-radius: 8px;
  background: #f0f8ff;
  max-width: 500px;
}

.status {
  background: white;
  padding: 12px;
  border-radius: 6px;
  margin: 12px 0;
  font-size: 14px;
}

.status div {
  margin: 6px 0;
}
</style>
