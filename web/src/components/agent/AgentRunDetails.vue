<script setup lang="ts">
import { computed } from 'vue'
import type { AgentRun } from '@/api/agent'
import type { AgentStreamItem } from '@/api/assistant'
import AgentMarkdown from './AgentMarkdown.vue'
import { assistantErrorMessage, capabilityLabel, flattenRuns, formatDuration, isActiveStatus, readableActivityText, runLabel, runRoleLabel, statusLabel } from '@/utils/assistant'

const props = defineProps<{ runs: AgentRun[]; items: AgentStreamItem[]; selectedID: number; detail: AgentRun | null; loading: boolean; error: string }>()
const emit = defineEmits<{ select: [id: number] }>()
const tree = computed(() => flattenRuns(props.runs))
const selected = computed(() => props.runs.some((run) => run.id === props.selectedID) ? props.detail?.id === props.selectedID ? props.detail : props.runs.find((run) => run.id === props.selectedID) : undefined)
const selectedItems = computed(() => props.items.filter((item) => item.run_id === props.selectedID))
const request = computed(() => selected.value?.context_traces?.find((trace) => trace.trace_kind === 'llm_request')?.content)
const response = computed(() => selected.value?.context_traces?.find((trace) => trace.trace_kind === 'llm_response')?.content)
const requestedModel = computed(() => {
  const configuration = request.value?.model_configuration as Record<string, unknown> | undefined
  return String(configuration?.model || selected.value?.model_key || '未记录')
})
const actualModel = computed(() => String(response.value?.model || selected.value?.model_key || '等待返回'))
const displaysAnswer = computed(() => !selected.value?.task_packet.stage || ['generation', 'followup_answer', 'feedback'].includes(String(selected.value.task_packet.stage)))
const responseText = computed(() => displaysAnswer.value && typeof response.value?.content === 'string' ? readableActivityText(response.value.content) : '')
const responseReasoning = computed(() => displaysAnswer.value && selected.value?.role !== 'controller' && typeof response.value?.reasoning_content === 'string' ? response.value.reasoning_content : '')
const usage = computed(() => response.value?.usage as Record<string, number> | undefined)
const description = computed(() => ['goal', 'description', 'task'].map((key) => selected.value?.task_packet[key]).find((value) => typeof value === 'string' && readableActivityText(value)))
</script>

<template>
  <div class="agent-details-content">
    <div class="agent-tree-heading"><span>任务进度</span><span>{{ runs.length }} 项活动</span></div>
    <div v-if="!runs.length" class="agent-details-empty">当前对话没有子代理任务。</div>
    <div v-else class="agent-run-tree" role="list" aria-label="代理执行节点">
      <button v-for="entry in tree" :key="entry.run.id" type="button" class="agent-run-node" :class="{ selected: selectedID === entry.run.id }" :style="{ paddingLeft: `${12 + Math.min(entry.depth, 6) * 14}px` }" :aria-pressed="selectedID === entry.run.id" @click="emit('select', entry.run.id)">
        <span class="agent-node-status" :class="{ active: isActiveStatus(entry.run.status), failed: entry.run.status === 'failed' }" />
        <span class="agent-node-copy"><strong>{{ runLabel(entry.run) }}</strong><small>{{ entry.run.task_packet.capability_key ? '工具活动' : entry.run.role === 'controller' ? '主代理' : entry.run.model_key || statusLabel(entry.run.status) }}</small></span>
        <span class="agent-node-state">{{ statusLabel(entry.run.status) }}</span>
      </button>
    </div>
    <section v-if="selected" class="agent-selected-detail" aria-label="选中节点详情">
      <h3>{{ runLabel(selected) }}</h3>
      <dl class="agent-detail-facts">
        <div><dt>状态</dt><dd>{{ statusLabel(selected.status) }}</dd></div>
        <div><dt>执行类型</dt><dd>{{ runRoleLabel(selected) }}</dd></div>
        <div v-if="!selected.task_packet.capability_key && selected.role !== 'controller'"><dt>请求模型</dt><dd>{{ requestedModel }}</dd></div>
        <div v-if="!selected.task_packet.capability_key && selected.role !== 'controller'"><dt>实际模型</dt><dd>{{ actualModel }}</dd></div>
        <div><dt>耗时</dt><dd>{{ formatDuration(selected.started_at, selected.completed_at) }}</dd></div>
        <div v-if="usage"><dt>Token 用量</dt><dd>{{ usage.TotalTokens ?? usage.total_tokens ?? '—' }}</dd></div>
      </dl>
      <p v-if="loading" class="agent-detail-muted" role="status">正在读取详情…</p>
      <p v-if="error" class="agent-detail-error" role="alert">{{ assistantErrorMessage(error) }}</p>
      <p v-if="selected.error_message" class="agent-detail-error">本项活动未完成，请查看对话中的提示。</p>
      <p v-if="description" class="agent-detail-block">{{ description }}</p>
      <section v-for="item in selectedItems" :key="`${item.kind}:${item.item_id}`" class="agent-detail-block">
        <details v-if="item.kind === 'reasoning' && displaysAnswer && selected.role !== 'controller'" open><summary>公开推理摘要</summary><AgentMarkdown :content="item.content" /></details>
        <p v-else-if="item.kind === 'tool'">{{ capabilityLabel(item.name) }}</p>
        <template v-else-if="item.kind === 'text' && displaysAnswer && readableActivityText(item.content)"><h4>回答内容</h4><AgentMarkdown :content="item.content" /></template>
      </section>
      <section v-if="responseText && !selectedItems.some((item) => item.kind === 'text')" class="agent-detail-block"><h4>模型输出</h4><AgentMarkdown :content="responseText" /></section>
      <details v-if="responseReasoning && !selectedItems.some((item) => item.kind === 'reasoning')" class="agent-detail-block" open><summary>公开推理摘要</summary><AgentMarkdown :content="responseReasoning" /></details>
      <section v-for="observation in selected.observations" :key="observation.id" class="agent-detail-block"><h4>{{ capabilityLabel(observation.capability_key) }} · {{ statusLabel(observation.status) }}</h4><p v-if="observation.error" class="agent-detail-error">资料处理未完成。</p></section>
      <section v-for="artifact in (selected.artifacts || []).filter((entry) => readableActivityText(entry.summary))" :key="artifact.id" class="agent-detail-block"><h4>资料结果</h4><AgentMarkdown :content="readableActivityText(artifact.summary)" /></section>
    </section>
  </div>
</template>

<style scoped>
.agent-details-content { font-size: 12px; color: var(--mf-text); }
.agent-tree-heading { display: flex; justify-content: space-between; padding: 18px 18px 10px; color: var(--mf-text-muted); font-size: 11px; }
.agent-run-tree { padding: 0 8px 14px; }
.agent-run-node { display: flex; align-items: center; gap: 9px; width: 100%; border: 0; border-radius: 7px; background: transparent; padding: 10px 9px; cursor: pointer; color: inherit; text-align: left; }
.agent-run-node:hover, .agent-run-node.selected { background: var(--mf-surface-muted, #f1f2f4); }
.agent-node-status { width: 7px; height: 7px; flex-shrink: 0; border-radius: 50%; background: #399b72; }
.agent-node-status.active { background: #b98336; box-shadow: 0 0 0 3px #b9833619; }
.agent-node-status.failed { background: #c15454; }
.agent-node-copy { display: grid; gap: 4px; flex: 1; min-width: 0; }
.agent-node-copy strong { font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.agent-node-copy small { color: var(--mf-text-muted); font-size: 10px; overflow-wrap: anywhere; }
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
</style>
