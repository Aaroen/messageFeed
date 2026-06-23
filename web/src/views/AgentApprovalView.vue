<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'

import {
  approveAgentApproval,
  getAgentApproval,
  rejectAgentApproval,
  type AgentApprovalDetail,
} from '@/api/approvals'
import { formatAPIError } from '@/api/client'

const route = useRoute()
const approval = ref<AgentApprovalDetail | null>(null)
const loading = ref(false)
const deciding = ref('')
const errorMessage = ref('')

const token = computed(() => {
  const value = route.params.token
  return typeof value === 'string' ? value : ''
})

const metadataRows = computed(() => {
  const metadata = approval.value?.metadata || {}
  return Object.entries(metadata).map(([key, value]) => ({
    key,
    value: formatMetadataValue(value),
  }))
})

const canDecide = computed(() => approval.value?.status === 'pending')

async function loadApproval() {
  if (!token.value) {
    errorMessage.value = '缺少确认 token'
    return
  }
  loading.value = true
  errorMessage.value = ''
  try {
    approval.value = await getAgentApproval(token.value)
  } catch (error) {
    errorMessage.value = formatAPIError(error)
  } finally {
    loading.value = false
  }
}

async function decide(action: 'approve' | 'reject') {
  if (!token.value) {
    return
  }
  deciding.value = action
  errorMessage.value = ''
  try {
    approval.value =
      action === 'approve' ? await approveAgentApproval(token.value) : await rejectAgentApproval(token.value)
  } catch (error) {
    errorMessage.value = formatAPIError(error)
  } finally {
    deciding.value = ''
  }
}

function formatMetadataValue(value: unknown) {
  if (typeof value === 'string') {
    return value
  }
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value)
  }
  if (value == null) {
    return ''
  }
  return JSON.stringify(value)
}

function formatTime(value: string | undefined) {
  if (!value) {
    return '无'
  }
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

onMounted(() => {
  void loadApproval()
})
</script>

<template>
  <section class="settings-page approval-page">
    <section class="settings-panel settings-panel--wide">
      <div class="settings-panel__header">
        <div>
          <div class="settings-panel__title">操作确认</div>
          <div class="settings-panel__meta">当前确认只记录决策，后续由 Agent 执行器恢复处理</div>
        </div>
        <button class="settings-action-button" type="button" :disabled="loading" @click="loadApproval">
          {{ loading ? '刷新中' : '刷新' }}
        </button>
      </div>

      <div v-if="errorMessage" class="settings-inline-alert settings-inline-alert--warning">
        {{ errorMessage }}
      </div>

      <dl v-if="approval" class="settings-description-list">
        <div>
          <dt>状态</dt>
          <dd>{{ approval.status }}</dd>
        </div>
        <div>
          <dt>渠道</dt>
          <dd>{{ approval.channel || 'web' }}</dd>
        </div>
        <div>
          <dt>过期时间</dt>
          <dd>{{ formatTime(approval.expires_at) }}</dd>
        </div>
        <div v-if="approval.decided_at">
          <dt>决策时间</dt>
          <dd>{{ formatTime(approval.decided_at) }}</dd>
        </div>
      </dl>

      <div v-if="metadataRows.length" class="approval-metadata">
        <div v-for="row in metadataRows" :key="row.key" class="approval-metadata__row">
          <span>{{ row.key }}</span>
          <strong>{{ row.value }}</strong>
        </div>
      </div>

      <div class="approval-actions">
        <button
          class="settings-action-button"
          type="button"
          :disabled="!canDecide || Boolean(deciding)"
          @click="decide('reject')"
        >
          {{ deciding === 'reject' ? '拒绝中' : '拒绝' }}
        </button>
        <button
          class="settings-action-button"
          type="button"
          :disabled="!canDecide || Boolean(deciding)"
          @click="decide('approve')"
        >
          {{ deciding === 'approve' ? '批准中' : '批准' }}
        </button>
      </div>
    </section>
  </section>
</template>
