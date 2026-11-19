<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import AgentActivityDisclosure from './AgentActivityDisclosure.vue'

const props = defineProps<{ content: string; active: boolean; previewMode?: boolean }>()
const manualOpen = ref<boolean | null>(null)
const expanded = computed(() => manualOpen.value ?? (props.active && props.content.length <= 112))
const label = computed(() => props.previewMode ? (props.active ? '正在播放' : '示例') : (props.active ? '实时更新' : ''))
const preview = computed(() => (props.content.length > 360 ? '…' : '') + props.content.slice(-360))
const scrollArea = ref<HTMLElement | null>(null)
const followBottom = ref(true)

function trackScroll() {
  const area = scrollArea.value
  if (area) followBottom.value = area.scrollHeight - area.clientHeight - area.scrollTop < 48
}

function rememberToggle(open: boolean) {
  if (open !== expanded.value) manualOpen.value = open
}

watch(() => props.content, async () => {
  if (!props.active || !followBottom.value) return
  await nextTick()
  const area = scrollArea.value
  if (area) area.scrollTop = area.scrollHeight
})
</script>

<template>
  <AgentActivityDisclosure kind="reasoning" :open="expanded" :meta="label" @toggle-open="rememberToggle">
    <p ref="scrollArea" class="reasoning-content" @scroll.passive="trackScroll">{{ content }}</p>
  </AgentActivityDisclosure>
  <p v-if="!expanded && active && manualOpen === null" class="reasoning-tail">{{ preview }}</p>
</template>

<style scoped>
.reasoning-content, .reasoning-tail { margin: 0; color: var(--mf-text-muted); font-size: 13px; line-height: 1.7; white-space: pre-wrap; overflow-wrap: anywhere; }
.reasoning-content { max-height: 180px; overflow: auto; overscroll-behavior: contain; scrollbar-width: none; }
.reasoning-tail { display: -webkit-box; -webkit-box-orient: vertical; -webkit-line-clamp: 2; overflow: hidden; margin: 3px 6px 7px; }
</style>
