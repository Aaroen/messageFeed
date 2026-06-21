<script setup lang="ts">
import { onMounted } from 'vue'
import { storeToRefs } from 'pinia'

import type { PageRefreshOptions } from '@/composables/usePageOutletState'
import { useAppStatusStore } from '@/stores/appStatus'

const statusStore = useAppStatusStore()
const { health, readiness, node, lastCheckedAt } = storeToRefs(statusStore)
const frontendOrigin = window.location.origin

async function refreshPage(_options: PageRefreshOptions = {}) {
  await statusStore.refresh()
}

onMounted(() => {
  void refreshPage().catch(() => undefined)
})

defineExpose({ refreshPage })
</script>

<template>
  <div class="status-view">
    <a-alert
      v-if="statusStore.hasError"
      type="warning"
      show-icon
      class="status-alert"
      title="链路尚未完全可用"
      content="当前入口暂未完成后端健康检查。"
    />

    <div class="status-grid">
      <a-card class="status-card" :bordered="false">
        <template #title>前端入口</template>
        <a-descriptions :column="1" size="medium">
          <a-descriptions-item label="当前地址">
            {{ frontendOrigin }}
          </a-descriptions-item>
          <a-descriptions-item label="检查时间">
            {{ lastCheckedAt || '尚未检查' }}
          </a-descriptions-item>
        </a-descriptions>
      </a-card>

      <a-card class="status-card" :bordered="false">
        <template #title>健康检查</template>
        <a-result
          v-if="health.error"
          status="warning"
          title="失败"
          :subtitle="health.error"
        />
        <a-result v-else status="success" title="可用" :subtitle="health.data?.status || 'ok'" />
      </a-card>

      <a-card class="status-card" :bordered="false">
        <template #title>就绪检查</template>
        <a-list v-if="readiness.data?.checks?.length" size="small" :bordered="false">
          <a-list-item v-for="check in readiness.data.checks" :key="check.name">
            <a-list-item-meta :title="check.name" :description="check.message" />
            <a-tag :color="check.status === 'ready' ? 'green' : 'orangered'">
              {{ check.status }}
            </a-tag>
          </a-list-item>
        </a-list>
        <a-result
          v-else-if="readiness.error"
          status="warning"
          title="失败"
          :subtitle="readiness.error"
        />
        <a-empty v-else description="暂无就绪数据" />
      </a-card>

      <a-card class="status-card" :bordered="false">
        <template #title>运行节点</template>
        <a-result
          v-if="node.error"
          status="warning"
          title="失败"
          :subtitle="node.error"
        />
        <a-descriptions v-else :column="1" size="medium">
          <a-descriptions-item label="节点 ID">
            {{ node.data?.node_id || 'unknown' }}
          </a-descriptions-item>
          <a-descriptions-item label="部署模式">
            {{ node.data?.deployment_mode || 'unknown' }}
          </a-descriptions-item>
          <a-descriptions-item label="公开地址">
            {{ node.data?.public_base_url || '未配置' }}
          </a-descriptions-item>
        </a-descriptions>
      </a-card>
    </div>
  </div>
</template>
