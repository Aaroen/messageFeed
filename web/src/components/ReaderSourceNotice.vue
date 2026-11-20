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
  <Teleport to="body">
    <div class="refresh-notice-region" role="status" aria-live="polite" aria-atomic="true">
      <Transition name="sources-toast-motion">
        <div v-if="notice" class="sources-toast" :class="`sources-toast--${notice.type}`">
          {{ notice.message }}
        </div>
      </Transition>
    </div>
  </Teleport>
</template>
