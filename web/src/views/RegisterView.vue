<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { getCurrentAuth, registerWithInvite } from '@/api/auth'
import { formatAPIError } from '@/api/client'

const route = useRoute()
const router = useRouter()
const inviteCode = ref('')
const username = ref('')
const displayName = ref('')
const email = ref('')
const password = ref('')
const loading = ref(false)
const registrationEnabled = ref(true)
const errorMessage = ref('')

const redirectPath = computed(() => {
  const value = route.query.redirect
  if (typeof value === 'string' && value.startsWith('/') && !value.startsWith('//')) {
    return value
  }
  return '/recommendations'
})

async function submitRegister() {
  const invite = inviteCode.value.trim()
  if (!invite) {
    errorMessage.value = '邀请码为必填项'
    return
  }
  loading.value = true
  errorMessage.value = ''
  try {
    await registerWithInvite({
      invite_code: invite,
      username: username.value.trim(),
      password: password.value,
      display_name: displayName.value.trim(),
      email: email.value.trim(),
    })
    await router.replace(redirectPath.value)
  } catch (error) {
    errorMessage.value = formatAPIError(error)
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  try {
    const auth = await getCurrentAuth()
    registrationEnabled.value = auth.registration_enabled
  } catch (error) {
    errorMessage.value = formatAPIError(error)
  }
})
</script>

<template>
  <section class="auth-page">
    <form class="settings-panel auth-panel" @submit.prevent="submitRegister">
      <div>
        <div class="settings-panel__title">注册</div>
        <div class="settings-panel__meta">使用管理员生成的邀请码创建账号</div>
      </div>

      <div v-if="errorMessage" class="settings-inline-alert settings-inline-alert--warning">
        {{ errorMessage }}
      </div>

      <div v-if="!registrationEnabled" class="settings-inline-alert settings-inline-alert--warning">
        注册入口尚未就绪
      </div>

      <label class="settings-field">
        <span>邀请码（必填）</span>
        <input
          v-model="inviteCode"
          class="settings-input"
          type="text"
          autocomplete="one-time-code"
          required
          aria-required="true"
        />
      </label>

      <label class="settings-field">
        <span>账号</span>
        <input v-model="username" class="settings-input" type="text" autocomplete="username" required />
      </label>

      <label class="settings-field">
        <span>显示名</span>
        <input v-model="displayName" class="settings-input" type="text" autocomplete="name" />
      </label>

      <label class="settings-field">
        <span>邮箱</span>
        <input v-model="email" class="settings-input" type="email" autocomplete="email" />
      </label>

      <label class="settings-field">
        <span>密码</span>
        <input
          v-model="password"
          class="settings-input"
          type="password"
          minlength="6"
          autocomplete="new-password"
          required
        />
      </label>

      <button class="settings-action-button auth-submit-button" type="submit" :disabled="loading || !registrationEnabled">
        {{ loading ? '注册中' : '注册' }}
      </button>

      <button class="settings-link-button" type="button" @click="router.replace({ name: 'login', query: { redirect: redirectPath } })">
        已有账号，返回登录
      </button>
    </form>
  </section>
</template>
