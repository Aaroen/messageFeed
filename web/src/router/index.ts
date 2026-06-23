import { createRouter, createWebHistory } from 'vue-router'

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

export default router
