<script setup lang="ts">
import type { StyleValue } from 'vue'

withDefaults(
  defineProps<{
    rootStyle?: StyleValue
  }>(),
  {
    rootStyle: undefined,
  },
)
</script>

<template>
  <div class="reader-transition-surface" :style="rootStyle">
    <slot />
  </div>
</template>
