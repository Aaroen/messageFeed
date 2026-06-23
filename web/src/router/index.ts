import { createRouter, createWebHistory } from 'vue-router'

import { getCurrentAuth } from '@/api/auth'

const LoginView = () => import('@/views/LoginView.vue')
const SettingsView = () => import('@/views/SettingsView.vue')
const SubscriptionFeedView = () => import('@/views/SubscriptionFeedView.vue')
const SubscriptionSourcesView = () => import('@/views/SubscriptionSourcesView.vue')

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
      path: '/auth/bindings',
      redirect: '/settings',
    },
    {
      path: '/timeline',
      redirect: '/subscriptions',
    },
    {
      path: '/subscriptions',
      name: 'subscriptions',
      component: SubscriptionFeedView,
      meta: { title: '订阅', section: 'subscriptions' },
    },
    {
      path: '/recommendations',
      name: 'recommendations',
      component: SubscriptionFeedView,
      meta: { title: '推荐', section: 'recommendations' },
    },
    {
      path: '/sources',
      name: 'sources',
      component: SubscriptionSourcesView,
      meta: { title: '订阅管理', section: 'sources' },
    },
    {
      path: '/settings',
      name: 'settings',
      component: SettingsView,
      meta: { title: '设置', section: 'settings' },
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
