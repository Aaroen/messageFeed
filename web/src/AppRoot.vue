<script setup lang="ts">
import { defineAsyncComponent } from 'vue'

const AppShell = defineAsyncComponent(() => import('./App.vue'))
</script>

<template>
  <Suspense>
    <AppShell />
    <template #fallback>
      <div class="app-shell app-shell--boot"></div>
    </template>
  </Suspense>
</template>
