<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { getAgentRun, type AgentRun } from '@/api/agent'
import type { AgentStreamItem } from '@/api/assistant'
import { formatAPIError } from '@/api/client'
import { isActiveStatus, runLabel } from '@/utils/assistant'
import AgentMarkdown from './AgentMarkdown.vue'
import AgentRunDetails from './AgentRunDetails.vue'

const props = defineProps<{ runs: AgentRun[]; items: AgentStreamItem[]; status: string }>()
const manualOpen = ref<boolean | null>(null)
const selectedID = ref(0)
const detail = ref<AgentRun | null>(null)
const loading = ref(false)
const error = ref('')
let version = 0
const reasoning = computed(() => props.items.filter((item) => item.kind === 'reasoning' && item.content && ['generation', 'followup_answer', 'feedback'].includes(item.stage)))
const lengthy = computed(() => props.runs.length > 6 || reasoning.value.reduce((length, item) => length + item.content.length, 0) > 800)
const expanded = computed(() => manualOpen.value ?? (isActiveStatus(props.status) && !lengthy.value))
const current = computed(() => [...props.runs].reverse().find((run) => isActiveStatus(run.status)))

async function select(id: number) {
  const currentVersion = ++version
  selectedID.value = id
  detail.value = null
  error.value = ''
  loading.value = true
  try {
    const result = await getAgentRun(id)
    if (version === currentVersion) detail.value = result
  } catch (cause) { if (version === currentVersion) error.value = formatAPIError(cause) }
  finally { if (version === currentVersion) loading.value = false }
}

watch(() => props.runs.find((run) => run.id === selectedID.value)?.updated_at, (updated) => {
  if (expanded.value && detail.value && updated !== detail.value.updated_at) void select(selectedID.value)
})
</script>

<template>
  <section class="turn-activity" aria-label="执行详情">
    <button type="button" class="turn-activity-toggle" :aria-expanded="expanded" @click="manualOpen = !expanded"><i :class="{ active: isActiveStatus(status) }" /><span>{{ current ? runLabel(current) : '执行详情' }}</span><small>{{ runs.length }} 项活动 · {{ expanded ? '收起' : '展开' }}</small></button>
    <div v-if="expanded" class="turn-activity-body">
      <section v-for="item in reasoning" :key="`${item.run_id}:${item.item_id}`" class="turn-reasoning"><small>公开推理摘要</small><AgentMarkdown :content="item.content" /></section>
      <AgentRunDetails :runs="runs" :items="items.filter((item) => item.kind !== 'reasoning')" :selected-i-d="selectedID" :detail="detail" :loading="loading" :error="error" @select="select" />
    </div>
  </section>
</template>

<style scoped>
.turn-activity { margin: 0 0 18px; border: 1px solid var(--mf-border); border-radius: 10px; overflow: hidden; }
.turn-activity-toggle { display: flex; align-items: center; gap: 9px; width: 100%; padding: 12px; background: transparent; border: 0; color: var(--mf-text-muted); font: inherit; font-size: 12px; cursor: pointer; text-align: left; }
.turn-activity-toggle i { width: 6px; height: 6px; border-radius: 50%; background: #399b72; flex-shrink: 0; }
.turn-activity-toggle i.active { background: #b98336; }
.turn-activity-toggle span { flex: 1; min-width: 0; overflow-wrap: anywhere; }
.turn-activity-toggle small { white-space: nowrap; font-size: 10px; }
.turn-activity-body { border-top: 1px solid var(--mf-border); max-height: 560px; overflow: auto; overscroll-behavior: contain; }
.turn-reasoning { padding: 14px; font-size: 12px; }
.turn-reasoning > small { display: block; margin-bottom: 8px; color: var(--mf-text-muted); }
</style>
