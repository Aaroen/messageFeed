<script setup lang="ts">
import { computed } from 'vue'

import type { AgentWeChatWebProgressLink } from '@/api/agent'

const props = defineProps<{
  progressLink: AgentWeChatWebProgressLink | null
  statusLabel: (status: string) => string
}>()

const summary = computed(() => {
  if (!props.progressLink) {
    return ''
  }
  return `${props.statusLabel(props.progressLink.status)} / 地址 ${props.progressLink.url_source} / 通道 ${props.progressLink.delivery_channel} / 模板 ${props.progressLink.template_status} / fallback ${props.progressLink.fallback_status} / ${props.progressLink.next_action}`
})

const visibleChecks = computed(() => props.progressLink?.checks?.slice(0, 8) || [])
</script>

<template>
  <div v-if="summary && progressLink" class="agent-plan-summary">
    <div class="agent-plan-summary__meta">
      <span>企微 Web 进度地址 {{ summary }}</span>
      <a v-if="progressLink.progress_url" :href="progressLink.progress_url" target="_blank" rel="noreferrer">
        {{ progressLink.progress_url }}
      </a>
      <span v-for="check in visibleChecks" :key="`wechat-web-progress-link-${check.key}`">
        {{ check.key }} {{ statusLabel(check.status) }}
      </span>
    </div>
  </div>
</template>
