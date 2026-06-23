<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { getCurrentAuth, login } from '@/api/auth'
import { formatAPIError } from '@/api/client'

const route = useRoute()
const router = useRouter()
const username = ref('owner')
const password = ref('')
const loading = ref(false)
const checking = ref(true)
const errorMessage = ref('')
const loginEnabled = ref(true)

const redirectPath = computed(() => {
  const value = route.query.redirect
  if (typeof value === 'string' && value.startsWith('/') && !value.startsWith('//')) {
    return value
  }
  return '/recommendations'
})

async function submitLogin() {
  loading.value = true
  errorMessage.value = ''
  try {
    await login({ username: username.value, password: password.value })
    await router.replace(redirectPath.value)
  } catch (error) {
    errorMessage.value = formatAPIError(error)
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  checking.value = true
  try {
    const auth = await getCurrentAuth()
    loginEnabled.value = auth.login_enabled
    if (auth.authenticated) {
      await router.replace(redirectPath.value)
    }
  } catch (error) {
    errorMessage.value = formatAPIError(error)
  } finally {
    checking.value = false
  }
})
</script>

<template>
  <section class="auth-page">
    <form class="settings-panel auth-panel" @submit.prevent="submitLogin">
      <div>
        <div class="settings-panel__title">登录</div>
        <div class="settings-panel__meta">messageFeed 管理入口</div>
      </div>

      <div v-if="errorMessage" class="settings-inline-alert settings-inline-alert--warning">
        {{ errorMessage }}
      </div>

      <div v-if="!loginEnabled" class="settings-inline-alert settings-inline-alert--warning">
        本地密码登录尚未配置
      </div>

      <label class="settings-field">
        <span>账号</span>
        <input v-model="username" class="settings-input" type="text" autocomplete="username" :disabled="checking" />
      </label>

      <label class="settings-field">
        <span>密码</span>
        <input
          v-model="password"
          class="settings-input"
          type="password"
          autocomplete="current-password"
          :disabled="checking"
        />
      </label>

      <button class="settings-action-button auth-submit-button" type="submit" :disabled="loading || checking || !loginEnabled">
        {{ loading ? '登录中' : '登录' }}
      </button>

      <button class="settings-link-button" type="button" @click="router.replace({ name: 'register', query: { redirect: redirectPath } })">
        使用邀请码注册
      </button>
    </form>
  </section>
</template>
