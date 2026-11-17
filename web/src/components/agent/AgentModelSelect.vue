<script setup lang="ts">
import { computed } from 'vue'
import { modelSelectionKey, type AgentModel } from '@/api/agentModels'

const props = defineProps<{ models: AgentModel[]; modelValue: string; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
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
  <select class="assistant-model-select" :value="modelValue" :disabled="disabled || !models.length" aria-label="选择对话模型" @change="emit('update:modelValue', ($event.target as HTMLSelectElement).value)">
    <option v-if="!models.length" value="">暂无可用模型</option>
    <option v-else-if="!modelValue" value="" disabled>请选择对话模型</option>
    <optgroup v-for="provider in groups" :key="provider.id" :label="provider.name"><option v-for="model in provider.models" :key="modelSelectionKey(model)" :value="modelSelectionKey(model)">{{ model.model }} · {{ provider.name }}</option></optgroup>
  </select>
</template>

<style scoped>
.assistant-model-select { max-width: min(100%, 350px); min-width: 0; padding: 7px 24px 7px 8px; border: 1px solid var(--mf-border); border-radius: 7px; background: var(--mf-surface, #fff); color: var(--mf-text); font: inherit; font-size: 12px; text-overflow: ellipsis; cursor: pointer; }
.assistant-model-select:focus-visible { outline: 2px solid var(--mf-accent, #555); outline-offset: 2px; }
.assistant-model-select:disabled { opacity: .6; cursor: default; }
@media (max-width: 767px) { .assistant-model-select { max-width: calc(100vw - 114px); } }
</style>
