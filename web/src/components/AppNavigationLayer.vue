<script setup lang="ts">
import type { Component, StyleValue } from 'vue'
import {
  IconMenuUnfold,
  IconMoonFill,
  IconSettings,
  IconSunFill,
} from '@arco-design/web-vue/es/icon'

type ManagementItem = {
  key: string
  label: string
  icon: Component
}

withDefaults(
  defineProps<{
    navigationVisible?: boolean
    navigationSettling?: boolean
    feedCornerHidden?: boolean
    detailChromeVisible?: boolean
    navOpenButtonStyle?: StyleValue
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
    navigationSettling: false,
    feedCornerHidden: false,
    detailChromeVisible: false,
    navOpenButtonStyle: undefined,
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
  (event: 'close-navigation'): void
  (event: 'go-home'): void
  (event: 'menu-click', key: string): void
  (event: 'toggle-theme'): void
  (event: 'open-settings'): void
}>()
</script>

<template>
  <button
    v-if="!navigationVisible"
    class="nav-open-button"
    :class="{ 'nav-open-button--hidden': feedCornerHidden, 'nav-open-button--detail': detailChromeVisible }"
    :style="navOpenButtonStyle"
    type="button"
    :aria-label="cornerButtonLabel"
    @pointerdown.stop
    @touchstart.stop
    @click="emit('corner-click')"
  >
    <IconMenuUnfold />
  </button>

  <button
    v-show="navigationVisible"
    class="nav-scrim"
    type="button"
    aria-label="关闭导航"
    :style="navigationScrimStyle"
    @click="emit('close-navigation')"
  />

  <aside
    v-show="navigationVisible"
    class="nav-panel"
    :class="{ 'nav-panel--settling': navigationSettling }"
    :style="navigationPanelStyle"
    aria-label="主导航"
  >
    <div class="nav-panel-glow" />

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
      <button
        v-for="item in managementItems"
        :key="item.key"
        class="nav-item"
        :class="{ 'nav-item--active': selectedKeys.includes(item.key) }"
        type="button"
        @pointerdown.stop
        @touchstart.stop
        @click="emit('menu-click', item.key)"
      >
        <component :is="item.icon" />
        <span>{{ item.label }}</span>
      </button>
    </nav>

    <div class="nav-panel-actions">
      <button
        class="theme-icon-button"
        type="button"
        aria-label="切换主题"
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
