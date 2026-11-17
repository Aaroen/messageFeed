<script setup lang="ts">
import { computed } from 'vue'
import type { AgentRun } from '@/api/agent'
import type { AgentStreamItem } from '@/api/assistant'
import AgentMarkdown from './AgentMarkdown.vue'
import { flattenRuns, formatDuration, isActiveStatus, runLabel, runRoleLabel, statusLabel } from '@/utils/assistant'

const props = defineProps<{ runs: AgentRun[]; items: AgentStreamItem[]; selectedID: number; detail: AgentRun | null; loading: boolean; error: string }>()
const emit = defineEmits<{ select: [id: number] }>()
const tree = computed(() => flattenRuns(props.runs))
const selected = computed(() => props.detail?.id === props.selectedID ? props.detail : props.runs.find((run) => run.id === props.selectedID))
const selectedItems = computed(() => props.items.filter((item) => item.run_id === props.selectedID))
const request = computed(() => selected.value?.context_traces?.find((trace) => trace.trace_kind === 'llm_request')?.content)
const response = computed(() => selected.value?.context_traces?.find((trace) => trace.trace_kind === 'llm_response')?.content)
const requestedModel = computed(() => {
  const configuration = request.value?.model_configuration as Record<string, unknown> | undefined
  return String(configuration?.model || selected.value?.model_key || '未记录')
})
const actualModel = computed(() => String(response.value?.model || selected.value?.model_key || '等待返回'))
const responseText = computed(() => typeof response.value?.content === 'string' ? response.value.content : '')
const responseReasoning = computed(() => typeof response.value?.reasoning_content === 'string' ? response.value.reasoning_content : '')
const usage = computed(() => response.value?.usage as Record<string, number> | undefined)
</script>

<template>
  <div class="agent-details-content">
    <div class="agent-tree-heading"><span>执行树</span><span>{{ runs.length }} 个节点</span></div>
    <div v-if="!runs.length" class="agent-details-empty">提交消息后，在这里查看执行过程。</div>
    <div v-else class="agent-run-tree" role="list" aria-label="代理执行节点">
      <button v-for="entry in tree" :key="entry.run.id" type="button" class="agent-run-node" :class="{ selected: selectedID === entry.run.id }" :style="{ paddingLeft: `${12 + Math.min(entry.depth, 6) * 14}px` }" :aria-pressed="selectedID === entry.run.id" @click="emit('select', entry.run.id)">
        <span class="agent-node-status" :class="{ active: isActiveStatus(entry.run.status), failed: entry.run.status === 'failed' }" />
        <span class="agent-node-copy"><strong>{{ runLabel(entry.run) }}</strong><small>{{ entry.run.model_key || statusLabel(entry.run.status) }}</small></span>
        <span class="agent-node-state">{{ statusLabel(entry.run.status) }}</span>
      </button>
    </div>
    <section v-if="selected" class="agent-selected-detail" aria-label="选中节点详情">
      <h3>{{ runLabel(selected) }}</h3>
      <dl class="agent-detail-facts">
        <div><dt>状态</dt><dd>{{ statusLabel(selected.status) }}</dd></div>
        <div><dt>执行类型</dt><dd>{{ runRoleLabel(selected) }}</dd></div>
        <div><dt>请求模型</dt><dd>{{ requestedModel }}</dd></div>
        <div><dt>实际模型</dt><dd>{{ actualModel }}</dd></div>
        <div><dt>耗时</dt><dd>{{ formatDuration(selected.started_at, selected.completed_at) }}</dd></div>
        <div v-if="selected.parent_run_id"><dt>父节点</dt><dd>#{{ selected.parent_run_id }}</dd></div>
        <div v-if="usage"><dt>Token 用量</dt><dd>{{ usage.TotalTokens ?? usage.total_tokens ?? '—' }}</dd></div>
      </dl>
      <p v-if="loading" class="agent-detail-muted" role="status">正在读取详情…</p>
      <p v-if="error" class="agent-detail-error" role="alert">{{ error }}</p>
      <p v-if="selected.error_message" class="agent-detail-error">{{ selected.error_message }}</p>
      <details v-if="Object.keys(selected.task_packet).length" class="agent-detail-block"><summary>任务说明</summary><pre>{{ JSON.stringify(selected.task_packet, null, 2) }}</pre></details>
      <section v-for="item in selectedItems" :key="`${item.kind}:${item.item_id}`" class="agent-detail-block">
        <details v-if="item.kind === 'reasoning'" open><summary>公开推理摘要</summary><AgentMarkdown :content="item.content" /></details>
        <details v-else-if="item.kind === 'tool'" open><summary>工具调用 · {{ item.name || '生成参数' }}</summary><pre>{{ item.content }}</pre></details>
        <template v-else-if="item.kind === 'text'"><h4>{{ item.phase === 'final_answer' ? '回答内容' : '模型输出' }}</h4><AgentMarkdown :content="item.content" /></template>
      </section>
      <section v-if="responseText && !selectedItems.some((item) => item.kind === 'text')" class="agent-detail-block"><h4>模型输出</h4><AgentMarkdown :content="responseText" /></section>
      <details v-if="responseReasoning && !selectedItems.some((item) => item.kind === 'reasoning')" class="agent-detail-block" open><summary>公开推理摘要</summary><AgentMarkdown :content="responseReasoning" /></details>
      <section v-for="observation in selected.observations" :key="observation.id" class="agent-detail-block"><h4>{{ observation.capability_key }} · {{ statusLabel(observation.status) }}</h4><p>{{ observation.input_summary }}</p><AgentMarkdown :content="observation.output_summary" /><p v-if="observation.error" class="agent-detail-error">{{ observation.error }}</p></section>
      <section v-for="artifact in selected.artifacts" :key="artifact.id" class="agent-detail-block"><h4>执行产物 · {{ artifact.artifact_type }}</h4><AgentMarkdown :content="artifact.summary" /><code>{{ artifact.content_ref }}</code></section>
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
