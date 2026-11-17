<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import AgentMarkdown from './AgentMarkdown.vue'

const props = defineProps<{ content: string; active: boolean }>()
const manualOpen = ref<boolean | null>(null)
const expanded = computed(() => manualOpen.value ?? (props.active && props.content.length <= 800))
const preview = computed(() => (props.content.length > 360 ? '…' : '') + props.content.slice(-360))
const scrollArea = ref<HTMLElement | null>(null)
const followBottom = ref(true)

function trackScroll() {
  const area = scrollArea.value
  if (area) followBottom.value = area.scrollHeight - area.clientHeight - area.scrollTop < 48
}

watch(() => props.content, async () => {
  if (!props.active || !followBottom.value) return
  await nextTick()
  const area = scrollArea.value
  if (area) area.scrollTop = area.scrollHeight
})
</script>

<template>
  <section class="reasoning-stream" aria-label="模型思考" :aria-busy="active">
    <button type="button" :aria-expanded="expanded" @click="manualOpen = !expanded"><i :class="{ active }" /><span>{{ active ? '模型思考 · 实时输出' : '模型思考' }}</span><small>{{ expanded ? '收起' : '展开完整内容' }}</small></button>
    <div v-if="expanded" ref="scrollArea" class="reasoning-content" @scroll="trackScroll"><AgentMarkdown :content="content" /></div>
    <p v-else-if="active" ref="scrollArea" class="reasoning-preview" aria-live="polite" @scroll="trackScroll">{{ preview }}</p>
  </section>
</template>

<style scoped>
.reasoning-stream { border: 1px solid var(--mf-border); border-radius: 10px; margin: 0 0 16px; overflow: hidden; }
.reasoning-stream > button { display: flex; align-items: center; gap: 8px; width: 100%; padding: 11px 12px; background: transparent; border: 0; color: var(--mf-text-muted); font: inherit; font-size: 12px; text-align: left; cursor: pointer; }
.reasoning-stream > button span { flex: 1; }
.reasoning-stream > button small { font-size: 11px; }
.reasoning-stream i { width: 6px; height: 6px; flex-shrink: 0; border-radius: 50%; background: #399b72; }
.reasoning-stream i.active { background: #b98336; animation: reasoning-pulse 1.4s ease-in-out infinite; }
.reasoning-content { max-height: 360px; padding: 12px; overflow: auto; border-top: 1px solid var(--mf-border); overscroll-behavior: contain; }
.reasoning-preview { max-height: 7.2em; margin: 0; padding: 0 12px 12px; overflow: auto; color: var(--mf-text-muted); font-size: 12px; line-height: 1.8; white-space: pre-wrap; overflow-wrap: anywhere; overscroll-behavior: contain; }
@keyframes reasoning-pulse { 50% { opacity: .35; } }
@media (prefers-reduced-motion: reduce) { .reasoning-stream i.active { animation: none; } }
</style>
