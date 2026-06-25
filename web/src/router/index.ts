import { createRouter, createWebHistory } from 'vue-router'

import { getCurrentAuth } from '@/api/auth'
import { getFeedViewMode, saveFeedViewMode, type FeedViewMode } from '@/api/feed'

const LoginView = () => import('@/views/LoginView.vue')
const RegisterView = () => import('@/views/RegisterView.vue')
const AgentApprovalView = () => import('@/views/AgentApprovalView.vue')
const AgentPlanView = () => import('@/views/AgentPlanView.vue')
const ItemDetailView = () => import('@/views/ItemDetailView.vue')
const SettingsView = () => import('@/views/SettingsView.vue')
const SubscriptionFeedView = () => import('@/views/SubscriptionFeedView.vue')
const SubscriptionSourcesView = () => import('@/views/SubscriptionSourcesView.vue')

const feedViewModeStorageKey = 'messagefeed-feed-view-mode'

function preferredFeedPath() {
  if (typeof window === 'undefined') {
    return '/recommendations'
  }
  return window.localStorage.getItem(feedViewModeStorageKey) === 'timeline' ? '/subscriptions' : '/recommendations'
}

function rememberFeedViewMode(mode: FeedViewMode) {
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(feedViewModeStorageKey, mode)
  }
  void saveFeedViewMode(mode).catch(() => undefined)
}

async function restoreFeedViewModeForRoute(routeName: string | symbol | null | undefined) {
  if (typeof window === 'undefined') {
    return null
  }
  if (routeName !== 'subscriptions' && routeName !== 'recommendations') {
    return null
  }
  if (window.localStorage.getItem(feedViewModeStorageKey)) {
    return null
  }
  try {
    const preference = await getFeedViewMode()
    window.localStorage.setItem(feedViewModeStorageKey, preference.view_mode)
    if (preference.view_mode === 'timeline' && routeName !== 'subscriptions') {
      return { name: 'subscriptions' }
    }
    if (preference.view_mode === 'recommendations' && routeName !== 'recommendations') {
      return { name: 'recommendations' }
    }
  } catch {
    return null
  }
  return null
}

const settingsRoutes = [
  { path: 'account', name: 'settings-account', title: '账户', section: 'account' },
  { path: 'profile', name: 'settings-profile', title: '资料', section: 'profile' },
  { path: 'security', name: 'settings-security', title: '安全', section: 'security' },
  { path: 'wechat', name: 'settings-wechat', title: '企业微信', section: 'wechat' },
  { path: 'preferences', name: 'settings-preferences', title: '偏好', section: 'preferences' },
  { path: 'overview', name: 'settings-overview', title: '系统概览', section: 'overview', ownerOnly: true },
  { path: 'invites', name: 'settings-invites', title: '邀请码', section: 'invites', ownerOnly: true },
  { path: 'users', name: 'settings-users', title: '用户管理', section: 'users', ownerOnly: true },
  { path: 'runtime', name: 'settings-runtime', title: '运行配置', section: 'runtime', ownerOnly: true },
  { path: 'tests', name: 'settings-tests', title: '连通性测试', section: 'tests', ownerOnly: true },
  { path: 'context', name: 'settings-context', title: '上下文', section: 'context', ownerOnly: true },
]

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      redirect: preferredFeedPath,
    },
    {
      path: '/auth/login',
      name: 'login',
      component: LoginView,
      meta: { title: '登录', section: 'auth', public: true },
    },
    {
      path: '/auth/register',
      name: 'register',
      component: RegisterView,
      meta: { title: '注册', section: 'auth', public: true },
    },
    {
      path: '/auth/bindings',
      redirect: '/settings/wechat',
    },
    {
      path: '/agent/approvals/:token',
      name: 'agent-approval',
      component: AgentApprovalView,
      meta: { title: '操作确认', section: 'agent' },
    },
    {
      path: '/agent',
      name: 'agent',
      component: AgentPlanView,
      meta: { title: 'Agent 任务', section: 'agent' },
    },
    {
      path: '/agent/plans/:id',
      name: 'agent-plan',
      component: AgentPlanView,
      meta: { title: '执行进度', section: 'agent' },
    },
    {
      path: '/agent/plans/:id/evidence/:recordKey',
      name: 'agent-evidence-detail',
      component: AgentPlanView,
      meta: { title: '执行证据', section: 'agent' },
    },
    {
      path: '/timeline',
      redirect: '/subscriptions',
    },
    {
      path: '/subscriptions',
      name: 'subscriptions',
      component: SubscriptionFeedView,
      meta: { title: '订阅', section: 'subscriptions', public: true },
    },
    {
      path: '/recommendations',
      name: 'recommendations',
      component: SubscriptionFeedView,
      meta: { title: '推荐', section: 'recommendations', public: true },
    },
    {
      path: '/items/:id',
      name: 'item-detail',
      component: ItemDetailView,
      meta: { title: '条目详情', section: 'items', public: true },
    },
    {
      path: '/sources',
      name: 'sources',
      component: SubscriptionSourcesView,
      meta: { title: '订阅管理', section: 'sources' },
    },
    {
      path: '/favorites',
      name: 'favorites',
      component: SubscriptionFeedView,
      props: { mode: 'favorites' },
      meta: { title: '收藏', section: 'favorites' },
    },
    {
      path: '/history',
      name: 'history',
      component: SubscriptionFeedView,
      props: { mode: 'history' },
      meta: { title: '阅读历史', section: 'history' },
    },
    {
      path: '/settings',
      redirect: '/settings/account',
    },
    ...settingsRoutes.map((route) => ({
      path: `/settings/${route.path}`,
      name: route.name,
      component: SettingsView,
      meta: {
        title: route.title,
        section: 'settings',
        settingsSection: route.section,
        ownerOnly: route.ownerOnly ?? false,
      },
    })),
    {
      path: '/settings/:pathMatch(.*)*',
      redirect: '/settings/account',
    },
  ],
})

router.beforeEach(async (to) => {
  const restoredFeedRoute = await restoreFeedViewModeForRoute(to.name)
  if (restoredFeedRoute) {
    return restoredFeedRoute
  }
  if (to.name === 'subscriptions') {
    rememberFeedViewMode('timeline')
  } else if (to.name === 'recommendations') {
    rememberFeedViewMode('recommendations')
  }
  if (to.meta.public) {
    return true
  }
  try {
    const auth = await getCurrentAuth()
    if (auth.authenticated) {
      if (to.meta.ownerOnly && auth.user?.role !== 'owner') {
        return { name: 'settings-account' }
      }
      return true
    }
  } catch {
    return {
      name: 'login',
      query: { redirect: to.fullPath },
    }
  }
  return {
    name: 'login',
    query: { redirect: to.fullPath },
  }
})

export default router
