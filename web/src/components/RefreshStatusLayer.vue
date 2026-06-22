<script setup lang="ts">
import type { StyleValue } from 'vue'
import { IconSync } from '@arco-design/web-vue/es/icon'

type ClassValue = string | Record<string, boolean> | Array<string | Record<string, boolean>>

withDefaults(
  defineProps<{
    rootClass?: ClassValue
    hidden?: boolean
    rootStyle?: StyleValue
    refreshing?: boolean
    iconStyle?: StyleValue
    title?: string
    meta?: string
  }>(),
  {
    rootClass: '',
    hidden: false,
    rootStyle: undefined,
    refreshing: false,
    iconStyle: undefined,
    title: '',
    meta: '',
  },
)
</script>

<template>
  <div
    :class="rootClass"
    :style="rootStyle"
    :aria-hidden="hidden ? 'true' : undefined"
    :aria-live="hidden ? 'off' : 'polite'"
    :role="hidden ? undefined : 'status'"
  >
    <span
      class="feed-refresh-header__icon"
      :class="{ 'feed-refresh-header__icon--refreshing': refreshing }"
      :style="iconStyle"
    >
      <IconSync />
    </span>
    <div class="feed-refresh-header__copy">
      <div class="feed-refresh-header__title">{{ title }}</div>
      <div class="feed-refresh-header__meta">{{ meta }}</div>
    </div>
  </div>
</template>
