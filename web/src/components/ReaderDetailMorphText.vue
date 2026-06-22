<script setup lang="ts">
import type { StyleValue } from 'vue'

import type { FeedItem } from '@/api/feed'

withDefaults(
  defineProps<{
    item?: FeedItem | null
    visible?: boolean
    rootStyle?: StyleValue
    metaStyle?: StyleValue
    titleStyle?: StyleValue
    summaryStyle?: StyleValue
    sourceLabelStyle?: StyleValue
    displayDate?: string
    summaryVisible?: boolean
    summary?: string
  }>(),
  {
    item: null,
    visible: false,
    rootStyle: undefined,
    metaStyle: undefined,
    titleStyle: undefined,
    summaryStyle: undefined,
    sourceLabelStyle: undefined,
    displayDate: '',
    summaryVisible: false,
    summary: '',
  },
)
</script>

<template>
  <article v-if="item && visible" class="reader-morph-text" :style="rootStyle">
    <div class="reader-morph-text__meta" :style="metaStyle">
      <span class="reader-morph-text__source-label" :style="sourceLabelStyle">
        {{ item.source_name || '未知来源' }}
      </span>
      <span>{{ displayDate }}</span>
      <span v-if="item.author">{{ item.author }}</span>
    </div>
    <h2 :style="titleStyle">{{ item.title }}</h2>
    <p v-if="summaryVisible" :style="summaryStyle">{{ summary }}</p>
  </article>
</template>
