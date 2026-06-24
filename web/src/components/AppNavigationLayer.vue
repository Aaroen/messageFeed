<script setup lang="ts">
import { storeToRefs } from 'pinia'
import type { Component, StyleValue } from 'vue'
import { watch } from 'vue'
import {
  IconMenuUnfold,
  IconMoonFill,
  IconSettings,
  IconSunFill,
} from '@arco-design/web-vue/es/icon'

import { useFeedFiltersStore } from '@/stores/feedFilters'

type ManagementItem = {
  key: string
  label: string
  icon: Component
}

const props = withDefaults(
  defineProps<{
    navigationVisible?: boolean
    detailChromeVisible?: boolean
    navOpenButtonStyle?: StyleValue
    navOpenButtonInteractive?: boolean
    cornerButtonLabel?: string
    navigationScrimStyle?: StyleValue
    navigationPanelStyle?: StyleValue
    managementItems?: ManagementItem[]
    selectedKeys?: string[]
    darkTheme?: boolean
    settingsActive?: boolean
    subscriptionFiltersVisible?: boolean
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
    subscriptionFiltersVisible: false,
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

const feedFilters = useFeedFiltersStore()
const {
  selectedSourceID,
  readFilter,
  favoriteFilter,
  hiddenFilter,
  sources,
  loading: filtersLoading,
  error: filtersError,
} = storeToRefs(feedFilters)

watch(
  () => [props.navigationVisible, props.subscriptionFiltersVisible],
  ([navigationVisible, subscriptionFiltersVisible]) => {
    if (navigationVisible && subscriptionFiltersVisible) {
      void feedFilters.loadSources()
    }
  },
  { immediate: true },
)
</script>

<template>
  <button
    v-if="!navigationVisible"
    class="nav-open-button"
    :class="{ 'nav-open-button--detail': detailChromeVisible }"
    :style="navOpenButtonStyle"
    type="button"
    :aria-label="cornerButtonLabel"
    :aria-hidden="navOpenButtonInteractive ? undefined : 'true'"
    :tabindex="navOpenButtonInteractive ? undefined : -1"
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

    <section
      v-if="subscriptionFiltersVisible"
      class="nav-filter-section"
      aria-label="订阅筛选"
    >
      <div class="nav-filter-section__title">订阅筛选</div>
      <label class="nav-filter-field">
        <span>来源</span>
        <select v-model.number="selectedSourceID" :disabled="filtersLoading">
          <option :value="0">全部来源</option>
          <option v-for="source in sources" :key="source.id" :value="source.id">
            {{ source.name }}
          </option>
        </select>
      </label>
      <label class="nav-filter-field">
        <span>阅读</span>
        <select v-model="readFilter">
          <option value="all">全部</option>
          <option value="false">未读</option>
          <option value="true">已读</option>
        </select>
      </label>
      <label class="nav-filter-field">
        <span>收藏</span>
        <select v-model="favoriteFilter">
          <option value="all">全部</option>
          <option value="true">已收藏</option>
          <option value="false">未收藏</option>
        </select>
      </label>
      <label class="nav-filter-field">
        <span>隐藏</span>
        <select v-model="hiddenFilter">
          <option value="visible">可见</option>
          <option value="all">全部</option>
          <option value="hidden">已隐藏</option>
        </select>
      </label>
      <p v-if="filtersError" class="nav-filter-section__error">{{ filtersError }}</p>
    </section>

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

<style scoped>
.nav-panel {
  grid-template-rows: auto minmax(0, 1fr) auto auto;
}

.nav-filter-section {
  position: relative;
  z-index: 1;
  display: grid;
  gap: 10px;
  margin: 0 14px 12px;
  padding: 12px;
  border: 1px solid var(--mf-border);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.42);
}

body[arco-theme='dark'] .nav-filter-section {
  background: rgba(255, 255, 255, 0.05);
}

.nav-filter-section__title {
  color: var(--mf-text);
  font-size: 13px;
  font-weight: 780;
}

.nav-filter-field {
  display: grid;
  gap: 5px;
  min-width: 0;
  color: var(--mf-text-muted);
  font-size: 12px;
  font-weight: 700;
}

.nav-filter-field select {
  width: 100%;
  min-width: 0;
  height: 34px;
  border: 1px solid var(--mf-border);
  border-radius: 8px;
  background: var(--mf-surface);
  color: var(--mf-text);
  font: inherit;
}

.nav-filter-section__error {
  margin: 0;
  color: #b91c1c;
  font-size: 12px;
  line-height: 1.45;
}
</style>
