<script setup lang="ts">
import FileUpload from './FileUpload.vue'
import SearchPanel from './SearchPanel.vue'
</script>

<template>
  <aside class="tools-panel">
    <div class="panel-inner">
      <FileUpload />
      <div class="panel-divider"></div>
      <SearchPanel />
    </div>
  </aside>
</template>

<style scoped>
.tools-panel {
  width: var(--sidebar-right-w);
  flex-shrink: 0;
  background: var(--bg-sidebar);
  border-left: 1px solid var(--border-color);
  display: flex; flex-direction: column;
  overflow: hidden;
}
.panel-inner {
  flex: 1; display: flex; flex-direction: column;
  padding: 20px 16px; overflow-y: auto;
}
.panel-divider {
  height: 1px; background: var(--border-color);
  margin: 16px 0;
}
</style>
