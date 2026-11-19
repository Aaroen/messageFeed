<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { getAgentActivity, type AgentActivity, type AgentStreamItem } from '@/api/assistant'
import { formatAPIError } from '@/api/client'
import { isActiveStatus, readableActivityText } from '@/utils/assistant'
import AgentActivityDisclosure from './AgentActivityDisclosure.vue'
import AgentReasoningStream from './AgentReasoningStream.vue'
import AgentRunDetails from './AgentRunDetails.vue'

const props = defineProps<{ runs: AgentActivity[]; items: AgentStreamItem[]; status: string; replying: boolean }>()
const executionOpen = ref(false)
const selectedID = ref(0)
const detail = ref<AgentActivity | null>(null)
const loading = ref(false)
const error = ref('')
let version = 0
const privateStages = ['evaluation', 'memory_classification', 'repair', 'rerank', 'feedback']
const activities = computed(() => props.runs.filter((run) => ['model', 'tool'].includes(run.kind) && !privateStages.includes(run.stage ?? '')))
const reasoning = computed(() => props.items
  .filter((item) => item.kind === 'reasoning' && item.content && !privateStages.includes(item.stage))
  .map((item) => readableActivityText(item.content)).filter(Boolean).join('\n\n'))

async function select(id: number) {
  const currentVersion = ++version
  selectedID.value = id
  detail.value = null
  error.value = ''
  loading.value = true
  try {
    const result = await getAgentActivity(id)
    if (version === currentVersion) detail.value = result
  } catch (cause) { if (version === currentVersion) error.value = formatAPIError(cause) }
  finally { if (version === currentVersion) loading.value = false }
}

watch(() => props.runs.find((run) => run.id === selectedID.value)?.updated_at, (updated) => {
  if (executionOpen.value && detail.value && updated !== detail.value.updated_at) void select(selectedID.value)
})
</script>

<template>
  <div v-if="reasoning || activities.length" class="turn-activity" aria-label="回复活动">
    <AgentReasoningStream v-if="reasoning" :content="reasoning" :active="isActiveStatus(status) && !replying" />
    <AgentActivityDisclosure v-if="activities.length" kind="execution" :meta="`${activities.length} 项活动`" @toggle-open="executionOpen = $event">
      <AgentRunDetails v-if="executionOpen" :runs="activities" :selected-i-d="selectedID" :detail="detail" :loading="loading" :error="error" @select="select" />
    </AgentActivityDisclosure>
  </div>
</template>

<style scoped>
.turn-activity { margin: 0 0 17px; }
</style>
