<script setup lang="ts">
import { useFileUpload } from '../composables/useFileUpload'

const { file, uploading, msg, onFileChange, upload } = useFileUpload()
</script>

<template>
  <div class="file-upload">
    <div class="section-title">文件</div>
    <label class="file-drop-zone">
      <input type="file" accept=".txt,.md,.json,.csv" @change="onFileChange" />
      <span v-if="!file">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg>
        选择文件
      </span>
      <span v-else class="file-selected">{{ file.name }}</span>
    </label>
    <button class="panel-btn" :disabled="!file || uploading" @click="upload">
      {{ uploading ? '上传中...' : '上传' }}
    </button>
    <div class="upload-msg" v-if="msg">{{ msg }}</div>
  </div>
</template>

<style scoped>
.section-title {
  font-size: 0.68rem; font-weight: 500; text-transform: uppercase;
  letter-spacing: 0.06em; color: var(--text-muted); margin-bottom: 10px;
}
.file-drop-zone {
  display: flex; align-items: center; justify-content: center; gap: 6px;
  padding: 12px; border: 1px dashed var(--border-input); border-radius: 8px;
  cursor: pointer; font-size: 0.78rem; color: var(--text-muted);
  transition: all var(--transition-fast); text-align: center;
}
.file-drop-zone:hover { border-color: var(--border-focus); color: var(--text-secondary); }
.file-drop-zone input { display: none; }
.file-selected { color: var(--accent-light); font-size: 0.75rem; word-break: break-all; }

.panel-btn {
  width: 100%; background: none; border: 1px solid var(--border-input);
  color: var(--text-secondary); padding: 7px 14px; border-radius: 8px;
  font-size: 0.78rem; cursor: pointer; margin-top: 8px;
  transition: all var(--transition-fast);
}
.panel-btn:hover { border-color: var(--border-focus); color: var(--text-primary); }
.panel-btn:disabled { opacity: 0.3; cursor: not-allowed; }

.upload-msg { font-size: 0.72rem; color: var(--text-muted); margin-top: 8px; }
</style>
