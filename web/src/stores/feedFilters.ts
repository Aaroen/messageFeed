import { defineStore } from 'pinia'

import { formatAPIError } from '@/api/client'
import { listSources, type Source } from '@/api/feed'

export type TriStateFilter = 'all' | 'true' | 'false'
export type HiddenFilter = 'visible' | 'all' | 'hidden'

export const useFeedFiltersStore = defineStore('feedFilters', {
  state: () => ({
    selectedSourceID: 0,
    readFilter: 'all' as TriStateFilter,
    favoriteFilter: 'all' as TriStateFilter,
    hiddenFilter: 'visible' as HiddenFilter,
    sources: [] as Source[],
    loading: false,
    error: '',
  }),
  actions: {
    async loadSources() {
      if (this.sources.length || this.loading) {
        return
      }
      this.loading = true
      this.error = ''
      try {
        this.sources = (await listSources()).filter((source) => source.status === 'active')
      } catch (err) {
        this.error = `筛选项加载失败：${formatAPIError(err)}`
      } finally {
        this.loading = false
      }
    },
  },
})
