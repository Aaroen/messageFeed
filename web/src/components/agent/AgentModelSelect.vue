<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { getModelReasoningLevels, modelSelectionKey, type AgentModel } from '@/api/agentModels'
import AgentEffortSlider from './AgentEffortSlider.vue'
import AgentModelPicker from './AgentModelPicker.vue'

const props = defineProps<{ models: AgentModel[]; disabled?: boolean; guest?: boolean }>()
const modelValue = defineModel<string>({ required: true })
const effort = defineModel<string>('effort', { required: true })
const levels = ref<string[]>([])
const levelsLoading = ref(false)
const levelsUnavailable = ref(false)
const selected = computed(() => props.models.find(model => modelSelectionKey(model) === modelValue.value))
const choices = computed(() => props.models.map(model => ({
  key: modelSelectionKey(model), id: model.model, name: model.display_name || model.model, provider: model.provider_name, providerKey: model.config_id,
})))

watch(selected, async (model, _previous, onCleanup) => {
  levels.value = []
  effort.value = ''
  levelsUnavailable.value = false
  levelsLoading.value = Boolean(model)
  if (!model) return
  const request = new AbortController()
  onCleanup(() => request.abort())
  try {
    const result = await getModelReasoningLevels(model, Boolean(props.guest), request.signal)
    if (!request.signal.aborted) levels.value = result
  } catch {
    if (!request.signal.aborted) levelsUnavailable.value = true
  } finally {
    if (!request.signal.aborted) levelsLoading.value = false
  }
}, { immediate: true })

</script>

<template>
  <div class="assistant-model-controls" @pointerdown.stop @touchstart.stop @touchmove.stop @touchend.stop>
    <AgentModelPicker v-model="modelValue" :choices="choices" :disabled="disabled" />
    <AgentEffortSlider v-if="levels.length && selected" v-model="effort" :levels="levels" :model="selected.display_name || selected.model" :disabled="disabled" />
    <span v-else-if="levelsLoading || levelsUnavailable" class="assistant-effort-status" role="status">{{ levelsLoading ? '正在读取思考档位…' : '暂时无法读取思考档位' }}</span>
  </div>
</template>

<style scoped>
.assistant-model-controls { display: flex; align-items: center; gap: 6px; min-width: 0; }
.assistant-effort-status { font-size: 12px; color: var(--mf-text-muted); }
</style>
