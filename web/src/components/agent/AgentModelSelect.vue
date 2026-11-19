<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { getModelReasoningLevels, modelSelectionKey, type AgentModel } from '@/api/agentModels'
import AgentEffortSlider from './AgentEffortSlider.vue'

const props = defineProps<{ models: AgentModel[]; modelValue: string; disabled?: boolean; guest?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const effort = defineModel<string>('effort', { required: true })
const levels = ref<string[]>([])
const levelsLoading = ref(false)
const levelsUnavailable = ref(false)
const selected = computed(() => props.models.find((model) => modelSelectionKey(model) === props.modelValue))

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

const groups = computed(() => {
  const providers = new Map<number, { id: number; name: string; models: AgentModel[] }>()
  for (const model of props.models) {
    const provider = providers.get(model.config_id) || { id: model.config_id, name: model.provider_name, models: [] }
    provider.models.push(model)
    providers.set(model.config_id, provider)
  }
  return [...providers.values()]
})
</script>

<template>
  <div class="assistant-model-controls" @pointerdown.stop @touchstart.stop @touchmove.stop @touchend.stop>
    <select class="assistant-model-select" :value="modelValue" :disabled="disabled || !models.length" aria-label="选择对话模型" @change="emit('update:modelValue', ($event.target as HTMLSelectElement).value)">
      <option v-if="!models.length" value="">暂无可用模型</option>
      <option v-else-if="!modelValue" value="" disabled>请选择对话模型</option>
      <optgroup v-for="provider in groups" :key="provider.id" :label="provider.name"><option v-for="model in provider.models" :key="modelSelectionKey(model)" :value="modelSelectionKey(model)">{{ model.display_name || model.model }}</option></optgroup>
    </select>
    <AgentEffortSlider v-if="levels.length && selected" v-model="effort" :levels="levels" :model="selected.display_name || selected.model" :disabled="disabled" />
    <span v-else-if="levelsLoading || levelsUnavailable" class="assistant-effort-status" role="status">{{ levelsLoading ? '正在读取思考档位…' : '暂时无法读取思考档位' }}</span>
  </div>
</template>

<style scoped>
.assistant-model-controls { display: flex; align-items: center; gap: 6px; min-width: 0; }
.assistant-model-select { max-width: 350px; min-width: 0; padding: 7px 8px; border: 0; border-radius: 8px; background: var(--mf-surface-muted); color: var(--mf-text); font: inherit; font-size: 12px; text-overflow: ellipsis; cursor: pointer; }
.assistant-model-controls select:focus-visible { outline: 2px solid var(--mf-accent, #555); outline-offset: 2px; }
.assistant-model-controls select:disabled { opacity: .6; cursor: default; }
.assistant-effort-status { font-size: 12px; color: var(--mf-text-muted); }
@media (max-width: 767px), (pointer: coarse) { .assistant-model-controls select { min-height: 44px; } }
</style>
