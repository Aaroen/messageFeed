<script setup lang="ts">
import { computed } from 'vue'
import { IconCheck, IconClose, IconClockCircle } from '@arco-design/web-vue/es/icon'
import type { AgentActivity } from '@/api/assistant'
import AgentMarkdown from './AgentMarkdown.vue'
import { assistantErrorMessage, flattenRuns, formatDuration, isActiveStatus, readableActivityText, runLabel, runRoleLabel, statusLabel } from '@/utils/assistant'

const props = defineProps<{ runs: AgentActivity[]; selectedID: number; detail: AgentActivity | null; loading: boolean; error: string; subagents?: boolean }>()
const emit = defineEmits<{ select: [id: number] }>()
const tree = computed(() => flattenRuns(props.runs))
const selected = computed(() => props.runs.some((run) => run.id === props.selectedID) ? props.detail?.id === props.selectedID ? props.detail : props.runs.find((run) => run.id === props.selectedID) : undefined)
const results = computed(() => (selected.value?.results || []).map(readableActivityText).filter(Boolean))
</script>

<template>
  <div class="agent-details-content">
    <div v-if="!runs.length" class="agent-details-empty">{{ subagents ? '当前对话没有子代理任务。' : '暂无执行活动。' }}</div>
    <div v-else class="agent-run-tree" role="group" aria-label="执行活动">
      <button v-for="entry in tree" :key="entry.run.id" type="button" class="agent-run-node" :class="{ selected: selectedID === entry.run.id }" :style="{ paddingLeft: `${12 + Math.min(entry.depth, 6) * 14}px` }" :aria-pressed="selectedID === entry.run.id" @click="emit('select', entry.run.id)">
        <component :is="entry.run.status === 'succeeded' ? IconCheck : isActiveStatus(entry.run.status) ? IconClockCircle : IconClose" class="agent-node-icon" aria-hidden="true" />
        <span class="agent-node-copy"><strong>{{ runLabel(entry.run) }}</strong></span>
        <span class="agent-node-state">{{ statusLabel(entry.run.status) }}</span>
      </button>
    </div>
    <section v-if="selected" class="agent-selected-detail" aria-label="选中节点详情">
      <h3>{{ runLabel(selected) }}</h3>
      <dl class="agent-detail-facts">
        <div><dt>状态</dt><dd>{{ statusLabel(selected.status) }}</dd></div>
        <div v-if="!subagents"><dt>执行类型</dt><dd>{{ runRoleLabel(selected) }}</dd></div>
        <div><dt>耗时</dt><dd>{{ formatDuration(selected.started_at, selected.completed_at) }}</dd></div>
      </dl>
      <p v-if="loading" class="agent-detail-muted" role="status">正在读取详情…</p>
      <p v-if="error" class="agent-detail-error" role="alert">{{ assistantErrorMessage(error) }}</p>
      <section v-if="results.length" class="agent-detail-block"><h4>任务结果</h4><AgentMarkdown v-for="(result, index) in results" :key="index" :content="result" /></section>
    </section>
  </div>
</template>

<style scoped>
.agent-details-content { font-size: 13px; color: var(--mf-text-muted); max-height: 360px; overflow: auto; overscroll-behavior: contain; scrollbar-width: none; }
.agent-run-node { display: flex; align-items: center; gap: 8px; width: 100%; border: 0; border-radius: 6px; background: transparent; padding: 3px 6px; min-height: 28px; cursor: pointer; color: inherit; text-align: left; }
.agent-run-node:hover, .agent-run-node.selected { background: var(--mf-surface-muted, #f1f2f4); }
.agent-node-icon { color: var(--mf-primary-strong); flex: none; }
.agent-node-copy { display: grid; gap: 4px; flex: 1; min-width: 0; }
.agent-node-copy strong { font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.agent-node-state { color: var(--mf-text-muted); font-size: 10px; white-space: nowrap; }
.agent-selected-detail { border-top: 1px solid var(--mf-border); padding: 18px; }
.agent-selected-detail h3 { margin: 0 0 18px; font-size: 14px; font-weight: 600; }
.agent-detail-facts { display: grid; gap: 10px; margin: 0 0 20px; }
.agent-detail-facts > div { display: grid; grid-template-columns: 72px 1fr; gap: 10px; }
.agent-detail-facts dt, .agent-detail-muted { color: var(--mf-text-muted); }
.agent-detail-facts dd { margin: 0; overflow-wrap: anywhere; }
.agent-detail-block { border-top: 1px solid var(--mf-border); padding: 16px 0; }
.agent-detail-block summary { cursor: pointer; font-weight: 550; margin-bottom: 12px; }
.agent-detail-block h4 { font-size: 12px; margin: 0 0 12px; }
.agent-detail-block pre { overflow: auto; white-space: pre-wrap; overflow-wrap: anywhere; font-family: ui-monospace, Consolas, monospace; font-size: 11px; line-height: 1.7; }
.agent-detail-error { color: #b84747; overflow-wrap: anywhere; line-height: 1.7; }
.agent-details-empty { padding: 24px 18px; color: var(--mf-text-muted); line-height: 1.8; }
@media (max-width: 760px) { .agent-run-node { min-height: 44px; } }
</style>
