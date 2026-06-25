<script setup lang="ts">
import { computed } from 'vue'

import type { AgentWeChatFinalReport } from '@/api/agent'

const props = defineProps<{
  report: AgentWeChatFinalReport | null
  statusLabel: (status: string) => string
}>()

const summary = computed(() => {
  if (!props.report) {
    return ''
  }
  return `${props.statusLabel(props.report.status)} / 通知 ${props.report.completion_notice_status} / 投递 ${props.report.delivery_status} / 模板 ${props.report.template_status} / 文本 ${props.report.text_status} / 入口 ${props.report.final_report_entry} / ${props.report.next_action}`
})

const visibleChecks = computed(() => props.report?.checks?.slice(0, 8) || [])
</script>

<template>
  <div v-if="summary && report" class="agent-plan-summary">
    <div class="agent-plan-summary__meta">
      <span>企微最终汇报 {{ summary }}</span>
      <a v-if="report.progress_url" :href="report.progress_url" target="_blank" rel="noreferrer">
        {{ report.progress_url }}
      </a>
      <span v-for="check in visibleChecks" :key="`wechat-final-report-${check.key}`">
        {{ check.key }} {{ statusLabel(check.status) }}
      </span>
    </div>
  </div>
</template>
