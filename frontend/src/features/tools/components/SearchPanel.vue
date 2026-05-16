<script setup lang="ts">
import { useSearch } from '../composables/useSearch'

const { query, results, searching, search } = useSearch()
</script>

<template>
  <div class="search-panel">
    <div class="section-title">搜索</div>
    <div class="search-input-wrap">
      <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
      <input
        class="search-input"
        v-model="query"
        placeholder="搜索文档..."
        @keydown.enter="search"
      />
    </div>
    <button
      class="panel-btn"
      :disabled="!query.trim() || searching"
      @click="search"
    >
      {{ searching ? '搜索中...' : '搜索' }}
    </button>
    <div class="search-results" v-if="results.length">
      <div v-for="(r, i) in results" :key="i" class="search-item">
        <div class="search-text">{{ typeof r === 'string' ? r : (r.content || r.text || JSON.stringify(r)) }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.section-title {
  font-size: 0.68rem; font-weight: 500; text-transform: uppercase;
  letter-spacing: 0.06em; color: var(--text-muted); margin-bottom: 10px;
}
.search-input-wrap {
  position: relative; display: flex; align-items: center;
}
.search-input-wrap svg {
  position: absolute; left: 10px; color: var(--text-muted); pointer-events: none;
}
.search-input {
  width: 100%; background: var(--bg-input); border: 1px solid var(--border-input);
  border-radius: 8px; color: var(--text-primary); padding: 8px 10px 8px 32px;
  font-size: 0.8rem; outline: none;
  transition: border-color var(--transition-fast);
}
.search-input:focus { border-color: var(--border-focus); }
.search-input::placeholder { color: var(--text-muted); }

.panel-btn {
  width: 100%; background: none; border: 1px solid var(--border-input);
  color: var(--text-secondary); padding: 7px 14px; border-radius: 8px;
  font-size: 0.78rem; cursor: pointer; margin-top: 8px;
  transition: all var(--transition-fast);
}
.panel-btn:hover { border-color: var(--border-focus); color: var(--text-primary); }
.panel-btn:disabled { opacity: 0.3; cursor: not-allowed; }

.search-results { margin-top: 10px; max-height: 240px; overflow-y: auto; }
.search-item {
  padding: 8px 10px; margin-bottom: 4px; border-radius: 6px;
  background: var(--bg-input); border: 1px solid var(--border-color);
}
.search-text {
  font-size: 0.75rem; color: var(--text-secondary);
  max-height: 60px; overflow: hidden; text-overflow: ellipsis;
  display: -webkit-box; -webkit-line-clamp: 3; -webkit-box-orient: vertical;
}
</style>
