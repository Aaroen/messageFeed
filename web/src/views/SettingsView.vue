<script setup lang="ts">
import { onMounted, ref } from 'vue'

const sourceTimelinePreload = ref(true)

function loadSettings() {
  sourceTimelinePreload.value = localStorage.getItem('messagefeed-source-preload') !== 'false'
}

function updateSourceTimelinePreload() {
  localStorage.setItem('messagefeed-source-preload', sourceTimelinePreload.value ? 'true' : 'false')
  window.dispatchEvent(
    new CustomEvent('messagefeed-settings-changed', {
      detail: { sourceTimelinePreload: sourceTimelinePreload.value },
    }),
  )
}

onMounted(loadSettings)
</script>

<template>
  <section class="settings-page">
    <article class="settings-row">
      <div>
        <div class="settings-row__title">源时间线预加载</div>
        <div class="settings-row__meta">详情页左右滑动时提前准备对应来源内容</div>
      </div>
      <label class="settings-switch">
        <input v-model="sourceTimelinePreload" type="checkbox" @change="updateSourceTimelinePreload" />
        <span />
      </label>
    </article>
  </section>
</template>
