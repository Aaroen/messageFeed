<script setup lang="ts">
import { computed, defineAsyncComponent, defineComponent, h, onErrorCaptured, ref } from 'vue'

const bootError = ref('')
const AppBootLoading = defineComponent({
  render() {
    return h('div', { class: 'app-shell app-shell--boot', role: 'status' }, [
      h('div', { class: 'app-boot-card' }, [
        h('div', { class: 'app-boot-mark' }, 'M'),
        h('div', { class: 'app-boot-title' }, '正在加载'),
      ]),
    ])
  },
})
const AppBootError = defineComponent({
  props: {
    error: {
      type: Error,
      required: false,
      default: null,
    },
  },
  render() {
    const error = this.error as Error | null
    return h('div', { class: 'app-shell app-shell--boot app-shell--boot-error' }, [
      h('div', { class: 'app-boot-card', role: 'alert' }, [
        h('div', { class: 'app-boot-mark' }, '!'),
        h('div', { class: 'app-boot-title' }, '页面加载失败'),
        h('div', { class: 'app-boot-message' }, error?.message || '请刷新页面后重试。'),
        h(
          'button',
          {
            class: 'app-boot-button',
            type: 'button',
            onClick: () => window.location.reload(),
          },
          '刷新',
        ),
      ]),
    ])
  },
})
const AppShell = defineAsyncComponent({
  loader: () => import('./App.vue'),
  loadingComponent: AppBootLoading,
  errorComponent: AppBootError,
  delay: 120,
  timeout: 15000,
})
const bootErrorMessage = computed(() => bootError.value.trim())

function reloadPage() {
  window.location.reload()
}

onErrorCaptured((error) => {
  bootError.value = error instanceof Error ? error.message : String(error)
  return false
})
</script>

<template>
  <Suspense>
    <div v-if="bootErrorMessage" class="app-shell app-shell--boot app-shell--boot-error">
      <div class="app-boot-card" role="alert">
        <div class="app-boot-mark">!</div>
        <div class="app-boot-title">页面运行异常</div>
        <div class="app-boot-message">{{ bootErrorMessage }}</div>
        <button class="app-boot-button" type="button" @click="reloadPage">刷新</button>
      </div>
    </div>
    <AppShell v-else />
    <template #fallback>
      <div class="app-shell app-shell--boot" role="status">
        <div class="app-boot-card">
          <div class="app-boot-mark">M</div>
          <div class="app-boot-title">正在加载</div>
        </div>
      </div>
    </template>
  </Suspense>
</template>

<style>
.app-shell--boot {
  display: grid;
  place-items: center;
  min-height: var(--mf-viewport-height);
  padding: 24px;
  background:
    linear-gradient(130deg, rgba(37, 99, 235, 0.08), transparent 34%),
    linear-gradient(215deg, rgba(14, 116, 144, 0.09), transparent 42%),
    var(--mf-page);
}

.app-boot-card {
  display: grid;
  justify-items: center;
  gap: 12px;
  width: min(360px, 100%);
  padding: 28px;
  border: 1px solid var(--mf-border);
  border-radius: 8px;
  background: var(--mf-surface-raised);
  box-shadow: 0 18px 52px rgba(31, 45, 61, 0.12);
  color: var(--mf-text);
  text-align: center;
  backdrop-filter: blur(18px) saturate(1.16);
}

.app-boot-mark {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border-radius: 8px;
  background: rgba(37, 99, 235, 0.12);
  color: var(--mf-primary);
  font-weight: 700;
}

.app-shell--boot-error .app-boot-mark {
  background: rgba(183, 121, 31, 0.14);
  color: var(--mf-amber);
}

.app-boot-title {
  font-size: 16px;
  font-weight: 700;
}

.app-boot-message {
  max-width: 100%;
  color: var(--mf-text-muted);
  font-size: 13px;
  line-height: 1.7;
  overflow-wrap: anywhere;
}

.app-boot-button {
  height: 36px;
  padding: 0 16px;
  border: 1px solid rgba(37, 99, 235, 0.28);
  border-radius: 8px;
  background: var(--mf-primary);
  color: #ffffff;
  cursor: pointer;
}
</style>
