import { ref, computed, onMounted } from 'vue'

// ---------- 斜杠命令 composable ----------

export interface Command {
  name: string
  description: string
  usage: string
  category: string
  type: 'client' | 'server'
}

export function useCommands() {
  const commands = ref<Command[]>([])
  const visible = ref(false)
  const selectedIndex = ref(0)
  const filterText = ref('')

  // 获取命令列表
  async function fetchCommands() {
    try {
      const res = await fetch('/api/commands')
      if (res.ok) {
        const data = await res.json()
        commands.value = data.commands || []
      }
    } catch {
      // 静默失败
    }
  }

  // 过滤后的命令列表
  const filteredCommands = computed(() => {
    const filter = filterText.value.toLowerCase()
    if (!filter) return commands.value
    return commands.value.filter(
      (cmd) =>
        cmd.name.toLowerCase().includes(filter) ||
        cmd.description.toLowerCase().includes(filter)
    )
  })

  // 根据输入文本更新可见性和过滤
  function updateFromInput(text: string) {
    if (text.startsWith('/')) {
      const query = text.slice(1)
      // 如果包含空格，说明用户已经在输入参数，隐藏面板
      if (query.includes(' ')) {
        visible.value = false
        return
      }
      filterText.value = query
      visible.value = filteredCommands.value.length > 0
      // 重置选中索引
      if (selectedIndex.value >= filteredCommands.value.length) {
        selectedIndex.value = 0
      }
    } else {
      visible.value = false
      filterText.value = ''
    }
  }

  // 键盘导航
  function onKeydown(e: KeyboardEvent): boolean {
    if (!visible.value) return false

    const list = filteredCommands.value
    if (list.length === 0) return false

    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault()
        selectedIndex.value = (selectedIndex.value + 1) % list.length
        return true
      case 'ArrowUp':
        e.preventDefault()
        selectedIndex.value = (selectedIndex.value - 1 + list.length) % list.length
        return true
      case 'Enter':
        // 只在面板可见且没有 shift 时拦截
        if (!e.shiftKey) {
          e.preventDefault()
          selectCommand(list[selectedIndex.value])
          return true
        }
        return false
      case 'Escape':
        visible.value = false
        return true
      default:
        return false
    }
  }

  // 选择命令
  function selectCommand(cmd: Command): string {
    visible.value = false
    filterText.value = ''
    return cmd.name
  }

  // 初始化
  onMounted(() => {
    fetchCommands()
  })

  return {
    commands,
    visible,
    selectedIndex,
    filterText,
    filteredCommands,
    updateFromInput,
    onKeydown,
    selectCommand,
    fetchCommands,
  }
}
