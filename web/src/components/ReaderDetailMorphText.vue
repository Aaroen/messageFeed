<script setup lang="ts">
import type { StyleValue } from 'vue'

import type { FeedItem } from '@/api/feed'

withDefaults(
  defineProps<{
    item?: FeedItem | null
    visible?: boolean
    rootStyle?: StyleValue
    sourceLabelStyle?: StyleValue
    displayDate?: string
    summaryVisible?: boolean
    summary?: string
  }>(),
  {
    item: null,
    visible: false,
    rootStyle: undefined,
    sourceLabelStyle: undefined,
    displayDate: '',
    summaryVisible: false,
    summary: '',
  },
)
</script>

<template>
  <article v-if="item && visible" class="reader-morph-text" :style="rootStyle">
    <div class="reader-morph-text__meta">
      <span class="reader-morph-text__source-label" :style="sourceLabelStyle">
        {{ item.source_name || '未知来源' }}
      </span>
      <span>{{ displayDate }}</span>
      <span v-if="item.author">{{ item.author }}</span>
    </div>
    <h2>{{ item.title }}</h2>
    <p v-if="summaryVisible">{{ summary }}</p>
  </article>
</template>
