import { createRouter, createWebHistory } from 'vue-router'

import { getCurrentAuth } from '@/api/auth'

const LoginView = () => import('@/views/LoginView.vue')
const RegisterView = () => import('@/views/RegisterView.vue')
const AgentApprovalView = () => import('@/views/AgentApprovalView.vue')
const SettingsView = () => import('@/views/SettingsView.vue')
const SubscriptionFeedView = () => import('@/views/SubscriptionFeedView.vue')
const SubscriptionSourcesView = () => import('@/views/SubscriptionSourcesView.vue')

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
      redirect: '/recommendations',
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
      path: '/sources',
      name: 'sources',
      component: SubscriptionSourcesView,
      meta: { title: '订阅管理', section: 'sources' },
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
