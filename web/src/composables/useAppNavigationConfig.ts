import type { Component } from 'vue'
import {
  IconBook,
  IconCode,
  IconDashboard,
  IconExperiment,
  IconHistory,
  IconIdcard,
  IconLink,
  IconLock,
  IconMessage,
  IconSettings,
  IconStorage,
  IconUser,
  IconUserGroup,
} from '@arco-design/web-vue/es/icon'

export type AppManagementItem = {
  key: string
  label: string
  path: string
  icon: Component
  ownerOnly?: boolean
}

export type AppFeedTab = {
  key: string
  label: string
  path: string
}

export function useAppNavigationConfig() {
  const managementItems: AppManagementItem[] = [
    { key: 'sources', label: '订阅管理', path: '/sources', icon: IconBook },
    { key: 'history', label: '阅读历史', path: '/history', icon: IconHistory },
    { key: 'settings-account', label: '账户', path: '/settings/account', icon: IconUser },
    { key: 'settings-profile', label: '资料', path: '/settings/profile', icon: IconIdcard },
    { key: 'settings-security', label: '安全', path: '/settings/security', icon: IconLock },
    { key: 'settings-wechat', label: '企业微信', path: '/settings/wechat', icon: IconMessage },
    { key: 'settings-preferences', label: '偏好', path: '/settings/preferences', icon: IconSettings },
    { key: 'settings-overview', label: '系统概览', path: '/settings/overview', icon: IconDashboard, ownerOnly: true },
    { key: 'settings-invites', label: '邀请码', path: '/settings/invites', icon: IconLink, ownerOnly: true },
    { key: 'settings-users', label: '用户管理', path: '/settings/users', icon: IconUserGroup, ownerOnly: true },
    { key: 'settings-runtime', label: '运行配置', path: '/settings/runtime', icon: IconStorage, ownerOnly: true },
    { key: 'settings-tests', label: '连通性测试', path: '/settings/tests', icon: IconExperiment, ownerOnly: true },
    { key: 'settings-context', label: '上下文', path: '/settings/context', icon: IconCode, ownerOnly: true },
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
