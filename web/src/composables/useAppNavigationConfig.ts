import type { Component } from 'vue'
import { IconBook } from '@arco-design/web-vue/es/icon'

export type AppManagementItem = {
  key: string
  label: string
  path: string
  icon: Component
}

export type AppFeedTab = {
  key: string
  label: string
  path: string
}

export function useAppNavigationConfig() {
  const managementItems: AppManagementItem[] = [
    { key: 'sources', label: '订阅管理', path: '/sources', icon: IconBook },
  ]
  const feedTabs: AppFeedTab[] = [
    { key: 'subscriptions', label: '订阅', path: '/subscriptions' },
    { key: 'recommendations', label: '推荐', path: '/recommendations' },
  ]

  return {
    managementItems,
    feedTabs,
  }
}
