<script setup lang="ts">
type SourceNotice = {
  type: 'running' | 'success' | 'warning'
  message: string
}

defineProps<{
  notice?: SourceNotice | null
}>()
</script>

<template>
  <div
    v-if="notice"
    class="sources-toast reader-toast"
    :class="`sources-toast--${notice.type}`"
    role="status"
    aria-live="polite"
  >
    {{ notice.message }}
  </div>
</template>
