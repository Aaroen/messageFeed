<script setup lang="ts">
import { computed } from 'vue'

import type { FeedItem } from '@/api/feed'
import { formatItemDate } from '@/utils/readerContent'

const props = defineProps<{ item: FeedItem }>()
const time = computed(() => formatItemDate(props.item))
</script>

<template>
  <component :is="time.datetime ? 'time' : 'span'" :datetime="time.datetime">{{ time.text }}</component>
</template>
