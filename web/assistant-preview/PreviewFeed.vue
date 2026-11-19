<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { IconFile, IconLaunch, IconRefresh } from '@arco-design/web-vue/es/icon'
import { usePullRefresh } from '../src/composables/usePullRefresh'
import { feeds, type FeedPage } from './examples'

const props = defineProps<{ page: FeedPage; active: boolean }>()
const emit = defineEmits<{ chrome: [visible: boolean] }>()
const pull = usePullRefresh()
const articles = ref([...feeds[props.page]])
const scrollArea = ref<HTMLElement | null>(null)
const status = ref('')
const pulling = computed(() => pull.offset.value > 0)
const armed = computed(() => pull.offset.value >= pull.threshold)
let scrollAnchor = 0
let lastScroll = 0
let scrollDirection = 0
let refreshTimer = 0
let noticeTimer = 0

watch(() => props.active, active => {
  if (active) {
    scrollAnchor = lastScroll = scrollArea.value?.scrollTop ?? 0
    scrollDirection = 0
    emit('chrome', true)
  } else if (!pull.refreshing.value) pull.cancelGesture()
})

onBeforeUnmount(() => {
  window.clearTimeout(refreshTimer)
  window.clearTimeout(noticeTimer)
  pull.reset()
})

function trackScroll() {
  const current = Math.max(0, scrollArea.value?.scrollTop ?? 0)
  const direction = Math.sign(current - lastScroll)
  if (direction && direction !== scrollDirection) {
    scrollAnchor = lastScroll
    scrollDirection = direction
  }
  lastScroll = current
  if (!props.active) return
  if (current <= 1 || current < scrollAnchor - 18) emit('chrome', true)
  else if (current > scrollAnchor + 28) emit('chrome', false)
}

function startPull(event: TouchEvent) {
  if (!props.active || pull.refreshing.value || event.touches.length !== 1 || (scrollArea.value?.scrollTop ?? 0) > 1) return
  const touch = event.touches[0]!
  pull.beginGestureCandidate(touch.clientX, touch.clientY)
}

function movePull(event: TouchEvent) {
  if (!pull.gestureCandidate.value && !pull.gestureTracking.value) return
  if (event.touches.length !== 1 || (scrollArea.value?.scrollTop ?? 0) > 1) {
    pull.cancelGesture()
    return
  }
  const touch = event.touches[0]!
  const { deltaX, deltaY } = pull.gestureDelta(touch.clientX, touch.clientY)
  if (!pull.gestureTracking.value) {
    if (deltaY < -6 || Math.abs(deltaX) > 10 && Math.abs(deltaX) > Math.abs(deltaY)) pull.cancelGesture()
    if (!pull.gestureCandidate.value || deltaY < 8 || deltaY < Math.abs(deltaX)) return
    pull.beginGestureTracking()
    emit('chrome', true)
  }
  if (event.cancelable) event.preventDefault()
  pull.setGestureOffset(pull.rubberBandDistance(deltaY, true))
}

function refresh() {
  if (pull.refreshing.value) return
  pull.finishGestureTracking()
  pull.commitRefreshOffset()
  pull.beginRefreshing()
  emit('chrome', true)
  window.clearTimeout(noticeTimer)
  status.value = '正在刷新'
  refreshTimer = window.setTimeout(() => {
    articles.value = [...feeds[props.page]]
    status.value = '本地列表已刷新'
    pull.finishBackgroundRefresh()
    noticeTimer = window.setTimeout(() => { status.value = '' }, 1900)
  }, 400)
}

function endPull() {
  if (pull.gestureTracking.value && armed.value) refresh()
  else if (!pull.refreshing.value) pull.cancelGesture()
}
</script>

<template>
  <main ref="scrollArea" class="preview-feed" :aria-label="page === 'subscriptions' ? '订阅' : '推荐'" @scroll.passive="trackScroll" @touchstart.passive="startPull" @touchmove="movePull" @touchend="endPull" @touchcancel="pull.cancelGesture">
    <div class="feed-reading" :style="{ '--pull-offset': `${pull.offset.value}px` }" :class="{ 'feed-pulling': pull.gestureTracking.value }">
      <div class="feed-pull-indicator" :class="{ armed }" :aria-hidden="!pulling"><IconRefresh /><span>{{ pull.refreshing.value ? '正在刷新' : armed ? '松开刷新' : '下拉刷新' }}</span></div>
      <header class="feed-heading">
        <div class="feed-title-row"><h1>{{ page === 'subscriptions' ? '订阅' : '推荐' }}</h1><span class="feed-refresh-notice" role="status">{{ status }}</span><button class="feed-refresh-button" type="button" :disabled="pull.refreshing.value" :aria-label="page === 'subscriptions' ? '刷新订阅' : '刷新推荐'" :title="page === 'subscriptions' ? '刷新订阅' : '刷新推荐'" @click="refresh"><IconRefresh aria-hidden="true" /></button></div>
        <p>{{ page === 'subscriptions' ? '来自已关注来源的文章' : '值得继续阅读的内容' }}</p>
      </header>
      <ul class="feed-articles">
        <li v-for="(article, index) in articles" :key="article.title">
          <article class="feed-article">
            <div class="feed-article-meta"><IconFile aria-hidden="true" /><span>{{ article.source }}</span><time>{{ article.time }}</time></div>
            <h2>{{ article.title }}</h2>
            <p>{{ article.description }}</p>
            <div class="feed-article-footer"><span>{{ article.category }}</span><a :href="`#/${page}/article/${index + 1}`" target="_blank" rel="noreferrer" :aria-label="`阅读原文：${article.title}（新标签页）`">阅读原文<IconLaunch aria-hidden="true" /></a></div>
          </article>
        </li>
      </ul>
    </div>
  </main>
</template>
