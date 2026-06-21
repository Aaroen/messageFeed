import { defineStore } from 'pinia'

export const useFeedInteractionStore = defineStore('feedInteraction', {
  state: () => ({
    pullViewKey: '',
    pullActive: false,
    pullOffset: 0,
    pullRefreshing: false,
    lastUpdatedAt: '',
    statusText: '下拉刷新',
    statusMeta: '尚未更新',
  }),
  actions: {
    setPullState(payload: {
      viewKey?: string
      offset: number
      active: boolean
      refreshing: boolean
      lastUpdatedAt: string
      statusText: string
      statusMeta: string
    }) {
      const { viewKey, offset, active, refreshing, lastUpdatedAt, statusText, statusMeta } = payload
      this.pullViewKey = viewKey ?? ''
      this.pullOffset = offset
      this.pullActive = active
      this.pullRefreshing = refreshing
      this.lastUpdatedAt = lastUpdatedAt
      this.statusText = statusText
      this.statusMeta = statusMeta
    },
    resetPullState() {
      this.pullViewKey = ''
      this.pullOffset = 0
      this.pullActive = false
      this.pullRefreshing = false
    },
  },
})
