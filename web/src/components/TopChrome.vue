<script setup lang="ts">
import { computed, type StyleValue } from 'vue'

type ChromeVariant = 'app' | 'source'
type ClassValue = string | Record<string, boolean> | Array<string | Record<string, boolean>>

const props = withDefaults(
  defineProps<{
    variant?: ChromeVariant
    phase?: string
    progress?: number
    rootClass?: ClassValue
    rootStyle?: StyleValue
  }>(),
  {
    variant: 'app',
    phase: 'visible',
    progress: 1,
    rootClass: '',
    rootStyle: undefined,
  },
)

const baseClass = computed(() =>
  props.variant === 'source'
    ? ['reader-overlay__header', 'reader-overlay__header--source']
    : ['app-header'],
)
</script>

<template>
  <header
    :class="[baseClass, rootClass]"
    :style="rootStyle"
    :data-chrome-phase="phase"
    :data-chrome-progress="progress.toFixed(3)"
  >
    <slot />
  </header>
</template>
