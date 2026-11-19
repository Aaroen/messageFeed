<script setup lang="ts">
import { computed, ref, watch, type StyleValue } from 'vue'
import { useRoute } from 'vue-router'

import type { FeedItem } from '@/api/feed'
import AgentPlanView from '@/views/AgentPlanView.vue'
import AgentWorkspaceView from '@/views/AgentWorkspaceView.vue'
import SubscriptionFeedView from '@/views/SubscriptionFeedView.vue'

type FeedSourceKind = 'subscriptions' | 'recommendations'

const props = withDefaults(
  defineProps<{
    activeKey?: string | symbol | null
    detailReaderOpen?: boolean
    sourceReaderOpen?: boolean
    feedTrackStyle?: StyleValue
    feedScrollTop?: number
    topChromeProgress?: number
    topChromeContentCollapsed?: boolean
    feedHeaderHeight?: number
    freezeBodyDuringTopRefresh?: boolean
    morphingItemId?: number | null
    morphingHeightLockItemId?: number | null
    morphingItemHeight?: number | null
    feedItemPreviewProgress?: number
  }>(),
  {
    activeKey: null,
    detailReaderOpen: false,
    sourceReaderOpen: false,
    feedTrackStyle: undefined,
    feedScrollTop: 0,
    topChromeProgress: 1,
    topChromeContentCollapsed: false,
    feedHeaderHeight: 86,
    freezeBodyDuringTopRefresh: false,
    morphingItemId: null,
    morphingHeightLockItemId: null,
    morphingItemHeight: null,
    feedItemPreviewProgress: 0,
  },
)

const emit = defineEmits<{
  (event: 'top-pull-start', startedWithVisibleChrome: boolean): void
  (event: 'top-pull-move', distance: number): void
  (event: 'top-pull-end', shouldRefresh: boolean): void
  (event: 'open-item', item: FeedItem, sourceKind: FeedSourceKind, originRect?: DOMRect): void
}>()

const readerClosed = computed(() => !props.detailReaderOpen && !props.sourceReaderOpen)
const route = useRoute()
const conversationMode = computed(() => route.name === 'agent' && !['turn_id', 'turnID', 'plan_id', 'planID', 'run_id', 'runID', 'scheduled_task_id', 'task_id'].some((key) => route.query[key]))
const subscriptionsActive = computed(() => props.activeKey === 'subscriptions' && readerClosed.value)
const recommendationsActive = computed(() => props.activeKey === 'recommendations' && readerClosed.value)
const agentActive = computed(() => props.activeKey === 'agent' && readerClosed.value)
const fullscreen = ref(false)
watch(() => agentActive.value && conversationMode.value, active => { if (!active) fullscreen.value = false })

function handleOpenItem(item: FeedItem, sourceKind: FeedSourceKind, originRect?: DOMRect) {
  emit('open-item', item, sourceKind, originRect)
}
</script>

<template>
  <div class="feed-stage" :class="{ 'feed-stage--assistant': conversationMode, 'feed-stage--fullscreen': fullscreen && conversationMode && agentActive }">
    <div class="feed-track" :style="feedTrackStyle">
      <section class="feed-pane" :aria-hidden="!subscriptionsActive" :inert="!subscriptionsActive">
        <SubscriptionFeedView
          mode="subscriptions"
          :active="subscriptionsActive"
          preload
          :scroll-top="feedScrollTop"
          :top-chrome-progress="topChromeProgress"
          :top-chrome-content-collapsed="topChromeContentCollapsed"
          :header-height="feedHeaderHeight"
          :freeze-body-during-top-refresh="freezeBodyDuringTopRefresh"
          :morphing-item-id="morphingItemId"
          :morphing-height-lock-item-id="morphingHeightLockItemId"
          :morphing-item-height="morphingItemHeight"
          :morphing-preview-progress="feedItemPreviewProgress"
          @top-pull-start="emit('top-pull-start', $event)"
          @top-pull-move="emit('top-pull-move', $event)"
          @top-pull-end="emit('top-pull-end', $event)"
          @open-item="handleOpenItem"
        />
      </section>
      <section class="feed-pane" :aria-hidden="!recommendationsActive" :inert="!recommendationsActive">
        <SubscriptionFeedView
          mode="recommendations"
          :active="recommendationsActive"
          preload
          :scroll-top="feedScrollTop"
          :top-chrome-progress="topChromeProgress"
          :top-chrome-content-collapsed="topChromeContentCollapsed"
          :header-height="feedHeaderHeight"
          :freeze-body-during-top-refresh="freezeBodyDuringTopRefresh"
          :morphing-item-id="morphingItemId"
          :morphing-height-lock-item-id="morphingHeightLockItemId"
          :morphing-item-height="morphingItemHeight"
          :morphing-preview-progress="feedItemPreviewProgress"
          @top-pull-start="emit('top-pull-start', $event)"
          @top-pull-move="emit('top-pull-move', $event)"
          @top-pull-end="emit('top-pull-end', $event)"
          @open-item="handleOpenItem"
        />
      </section>
      <section class="feed-pane feed-pane--agent" :aria-hidden="!agentActive" :inert="!agentActive">
        <AgentWorkspaceView v-if="agentActive && conversationMode" v-model:fullscreen="fullscreen" />
        <AgentPlanView v-else-if="agentActive" />
      </section>
    </div>
  </div>
</template>

<style>
.app-content--feed:has(.feed-stage--assistant) { --app-content-top-offset: 0px; padding-bottom: 0; }
.feed-stage--assistant { height: 100%; }
.feed-stage--assistant .feed-track { height: 100%; grid-template-rows: minmax(0, 1fr); }
.feed-stage--assistant .feed-pane { height: 100%; min-height: 0; overflow: hidden; }
.feed-stage--assistant .feed-pane--agent { padding-inline: 0; }
.app-main--feed:has(.feed-stage--assistant) > .app-header { box-shadow: none; }
.app-main--feed:has(.feed-stage--fullscreen) > .app-header { display: none; }
.app-content--feed:has(.feed-stage--fullscreen) { padding-top: env(safe-area-inset-top, 0px) !important; }
.app-shell:has(.feed-stage--fullscreen) .nav-open-button { top: calc(env(safe-area-inset-top, 0px) + 9px); left: 18px; }
.app-shell:has(.feed-stage--fullscreen .assistant-shell.has-sessions) .nav-open-button { left: 254px; }
@media (max-width: 799px) {
  .app-shell:has(.feed-stage--fullscreen) .nav-open-button { top: calc(env(safe-area-inset-top, 0px) + 5px); left: 9px; }
}
</style>
