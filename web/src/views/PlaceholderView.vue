<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()

const title = computed(() => route.meta.title?.toString() ?? '页面')
const section = computed(() => route.meta.section?.toString() ?? '')
const description = computed(() => {
  if (section.value === 'subscriptions') {
    return '暂无订阅内容。'
  }
  if (section.value === 'recommendations') {
    return '暂无推荐内容。'
  }
  if (section.value === 'sources') {
    return '暂无订阅源。'
  }
  if (section.value === 'settings') {
    return '暂无设置项。'
  }
  return '暂无内容。'
})
</script>

<template>
  <section class="empty-surface">
    <div class="empty-surface__mark">{{ title.slice(0, 1) }}</div>
    <h2>{{ title }}</h2>
    <p>{{ description }}</p>
  </section>
</template>
