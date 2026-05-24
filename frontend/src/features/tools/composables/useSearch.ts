import { ref } from 'vue'
import type { SearchResult } from '../types'

export function useSearch() {
  const query = ref<string>('')
  const results = ref<SearchResult[]>([])
  const searching = ref<boolean>(false)

  async function search(): Promise<void> {
    if (!query.value.trim() || searching.value) return
    searching.value = true
    results.value = []
    try {
      const res = await fetch('/api/search', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ query: query.value }),
      })
      const data = await res.json()
      results.value = data.results || []
    } catch (e) {
      results.value = [{ error: e instanceof Error ? e.message : String(e) } as SearchResult]
    } finally {
      searching.value = false
    }
  }

  return { query, results, searching, search }
}
