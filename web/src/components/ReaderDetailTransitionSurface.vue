<script setup lang="ts">
import type { StyleValue } from 'vue'

withDefaults(
  defineProps<{
    entrySettling?: boolean
    chromeSettling?: boolean
    rootStyle?: StyleValue
  }>(),
  {
    entrySettling: false,
    chromeSettling: false,
    rootStyle: undefined,
  },
)
</script>

<template>
  <div
    class="reader-transition-surface"
    :class="{
      'reader-transition-surface--entry-settling': entrySettling,
      'reader-transition-surface--chrome-settling': chromeSettling,
    }"
    :style="rootStyle"
  >
    <slot />
  </div>
</template>
