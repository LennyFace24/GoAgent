# Vue 3 Emit 示例

这个示例演示了 Vue 3 中 `emit` 的工作原理。

## 文件结构

```
emit-demo/
├── App.vue              # 入口组件
├── ParentComponent.vue  # 父组件（监听事件）
├── ChildComponent.vue   # 子组件（触发事件）
└── README.md           # 说明文档
```

## 工作原理

### 1. 子组件定义事件

```typescript
// ChildComponent.vue
const emit = defineEmits<{
  'message': [text: string]  // 定义事件名和参数类型
  'count': [value: number]
}>()
```

### 2. 子组件触发事件

```typescript
// 触发事件，传递数据给父组件
emit('message', 'Hello World')
emit('count', 42)
```

### 3. 父组件监听事件

```html
<!-- ParentComponent.vue -->
<ChildComponent
  @message="handleMessage"   <!-- 监听 message 事件 -->
  @count="handleCount"       <!-- 监听 count 事件 -->
/>
```

### 4. 父组件处理事件

```typescript
function handleMessage(text: string) {
  // 接收子组件传递的数据
  messages.value.push(text)
}
```

## 数据流向

```
┌─────────────────────────────────────────────────────────┐
│  父组件 (ParentComponent.vue)                           │
│                                                         │
│  <ChildComponent @message="handleMessage" />            │
│       ↑                                                 │
│       │ 监听 'message' 事件                              │
│       │                                                 │
│  function handleMessage(text) {                         │
│    messages.value.push(text)  // 处理数据                 │
│  }                                                      │
└─────────────────────────────────────────────────────────┘
                        ↑
                        │ emit('message', 'Hello')
                        │
┌─────────────────────────────────────────────────────────┐
│  子组件 (ChildComponent.vue)                            │
│                                                         │
│  emit('message', inputText.value)                       │
│  // 触发事件，传递输入框的文本                             │
└─────────────────────────────────────────────────────────┘
```

## 与你项目中的对比

在你的 GoAgent 项目中，SubSidebar.vue 使用 emit 的方式：

```typescript
// SubSidebar.vue (子组件)
emit('created', data.conversation.id)  // 通知父组件：新对话已创建

// App.vue (父组件)
<SubSidebar @created="onCreated" />    // 监听 created 事件

function onCreated(id: string) {
  activeConversationId.value = id      // 更新当前对话 ID
}
```

## 运行示例

1. 在 Vue 项目中引入这些组件
2. 访问页面，点击按钮测试
3. 观察父组件如何接收子组件的数据
