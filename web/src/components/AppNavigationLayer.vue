<script setup lang="ts">
import { computed, nextTick, ref, watch, type Component, type StyleValue } from 'vue'
import {
  IconArrowLeft,
  IconMenuUnfold,
  IconMoonFill,
  IconSettings,
  IconSunFill,
} from '@arco-design/web-vue/es/icon'

type ManagementItem = {
  key: string
  label: string
  icon: Component
  ownerOnly?: boolean
}

const props = withDefaults(
  defineProps<{
    navigationVisible?: boolean
    detailChromeVisible?: boolean
    navOpenButtonStyle?: StyleValue
    navOpenButtonInteractive?: boolean
    readerBackLabel?: string
    cornerButtonLabel?: string
    navigationScrimStyle?: StyleValue
    navigationPanelStyle?: StyleValue
    managementItems?: ManagementItem[]
    selectedKeys?: string[]
    darkTheme?: boolean
    settingsActive?: boolean
  }>(),
  {
    navigationVisible: false,
    detailChromeVisible: false,
    navOpenButtonStyle: undefined,
    navOpenButtonInteractive: true,
    cornerButtonLabel: '打开导航',
    navigationScrimStyle: undefined,
    navigationPanelStyle: undefined,
    managementItems: () => [],
    selectedKeys: () => [],
    darkTheme: false,
    settingsActive: false,
  },
)

const emit = defineEmits<{
  (event: 'corner-click'): void
  (event: 'reader-back'): void
  (event: 'close-navigation'): void
  (event: 'go-home'): void
  (event: 'menu-click', key: string): void
  (event: 'toggle-theme'): void
  (event: 'open-settings'): void
}>()

const panel = ref<HTMLElement | null>(null)
const opener = ref<HTMLButtonElement | null>(null)
const readingKeys = new Set(['sources', 'favorites', 'history'])
const sections = computed(() => [
  { label: '阅读', items: props.managementItems.filter(item => readingKeys.has(item.key)) },
  { label: '账户与偏好', items: props.managementItems.filter(item => !readingKeys.has(item.key) && !item.ownerOnly) },
  { label: '管理员', items: props.managementItems.filter(item => item.ownerOnly) },
].filter(section => section.items.length))

watch(() => props.navigationVisible, async (visible) => {
  await nextTick()
  if (visible !== props.navigationVisible) return
  if (visible) panel.value?.focus({ preventScroll: true })
  else opener.value?.focus({ preventScroll: true })
})

function keepFocusInPanel(event: KeyboardEvent) {
  const controls = panel.value?.querySelectorAll<HTMLButtonElement>('button:not(:disabled)')
  if (!controls?.length) return
  const first = controls[0]
  const last = controls[controls.length - 1]
  const active = document.activeElement
  if (event.shiftKey && (active === first || active === panel.value)) {
    event.preventDefault()
    last?.focus()
  } else if (!event.shiftKey && active === last) {
    event.preventDefault()
    first?.focus()
  }
}
</script>

<template>
  <button
    v-if="!navigationVisible"
    ref="opener"
    class="nav-open-button"
    :class="{ 'nav-open-button--detail': detailChromeVisible, 'nav-open-button--has-reader-back': readerBackLabel }"
    :style="navOpenButtonStyle"
    type="button"
    :aria-label="cornerButtonLabel"
    :aria-expanded="navigationVisible"
    aria-controls="main-navigation-panel"
    :aria-hidden="navOpenButtonInteractive ? undefined : 'true'"
    :tabindex="navOpenButtonInteractive ? undefined : -1"
    @pointerdown.stop
    @touchstart.stop
    @click="emit('corner-click')"
  >
    <IconMenuUnfold />
  </button>

  <button
    v-if="readerBackLabel && !navigationVisible"
    class="nav-open-button nav-open-button--reader-back"
    type="button"
    :aria-label="readerBackLabel"
    :title="readerBackLabel"
    @pointerdown.stop
    @touchstart.stop
    @click="emit('reader-back')"
  >
    <IconArrowLeft aria-hidden="true" />
  </button>

  <button
    v-show="navigationVisible"
    class="nav-scrim"
    type="button"
    aria-label="关闭导航"
    tabindex="-1"
    :style="navigationScrimStyle"
    @click="emit('close-navigation')"
  />

  <aside
    v-show="navigationVisible"
    id="main-navigation-panel"
    ref="panel"
    class="nav-panel"
    :style="navigationPanelStyle"
    aria-label="主导航"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    @keydown.esc.stop="emit('close-navigation')"
    @keydown.tab="keepFocusInPanel"
  >
    <div class="brand">
      <div class="brand-mark">MF</div>
      <button
        class="brand-home-button"
        type="button"
        aria-label="返回主页"
        @pointerdown.stop
        @touchstart.stop
        @click="emit('go-home')"
      >
        <div class="brand-title">messageFeed</div>
        <div class="brand-subtitle">信息阅读</div>
      </button>
    </div>

    <nav class="app-menu" aria-label="管理导航">
      <section v-for="section in sections" :key="section.label" class="nav-section" :aria-label="section.label">
        <h2>{{ section.label }}</h2>
        <button
          v-for="item in section.items"
          :key="item.key"
          class="nav-item"
          :class="{ 'nav-item--active': selectedKeys.includes(item.key) }"
          :aria-current="selectedKeys.includes(item.key) ? 'page' : undefined"
          type="button"
          @pointerdown.stop
          @touchstart.stop
          @click="emit('menu-click', item.key)"
        >
          <component :is="item.icon" aria-hidden="true" />
          <span>{{ item.label }}</span>
        </button>
      </section>
    </nav>

    <div class="nav-panel-actions">
      <button
        class="theme-icon-button"
        type="button"
        :aria-label="darkTheme ? '切换到日间模式' : '切换到深色模式'"
        :title="darkTheme ? '切换到日间模式' : '切换到深色模式'"
        @pointerdown.stop
        @touchstart.stop
        @click="emit('toggle-theme')"
      >
        <component :is="darkTheme ? IconSunFill : IconMoonFill" />
      </button>

      <button
        class="settings-icon-button"
        :class="{ 'settings-icon-button--active': settingsActive }"
        type="button"
        aria-label="设置"
        @pointerdown.stop
        @touchstart.stop
        @click="emit('open-settings')"
      >
        <IconSettings />
      </button>
    </div>
  </aside>
</template>

<style scoped>
.nav-open-button--reader-back { top: max(8px, env(safe-area-inset-top)); z-index: 150; background: var(--mf-surface); }
@media (min-width: 800px) and (pointer: fine) {
  .nav-open-button--has-reader-back { display: none; }
}
@media (max-width: 799px), (pointer: coarse), (pointer: none) {
  .nav-open-button--reader-back { display: none; }
}
</style>
