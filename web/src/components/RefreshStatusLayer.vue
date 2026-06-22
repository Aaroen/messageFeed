<script setup lang="ts">
import { computed } from 'vue'
import type { StyleValue } from 'vue'
import { IconSync } from '@arco-design/web-vue/es/icon'

import { chromeStyleIsVisible } from '@/composables/chromeStyleInteractivity'

type ClassValue = string | Record<string, boolean> | Array<string | Record<string, boolean>>

const props = withDefaults(
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

const semanticHidden = computed(() => props.hidden || !chromeStyleIsVisible(props.rootStyle))
</script>

<template>
  <div
    :class="rootClass"
    :style="rootStyle"
    :aria-hidden="semanticHidden ? 'true' : undefined"
    :aria-live="semanticHidden ? 'off' : 'polite'"
    :role="semanticHidden ? undefined : 'status'"
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
