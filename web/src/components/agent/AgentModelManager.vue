<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { AgentLLMProviderConfig } from '@/api/agent'
import { discoverProviderModels, saveProviderModels, type ProviderModelCatalog } from '@/api/agentModels'
import { formatAPIError } from '@/api/client'

const props = defineProps<{ configs: AgentLLMProviderConfig[]; owner: boolean }>()
const configID = ref(0)
const search = ref('')
const catalog = ref<ProviderModelCatalog | null>(null)
const enabled = ref<string[]>([])
const guest = ref<string[]>([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const notice = ref('')
let version = 0
const configuration = computed(() => props.configs.find((config) => config.id === configID.value))
const visibleModels = computed(() => (catalog.value?.models || []).filter((model) => model.id.toLowerCase().includes(search.value.trim().toLowerCase())))
const unavailable = computed(() => (catalog.value?.saved || []).filter((model) => model.enabled && !catalog.value?.models.some((entry) => entry.id === model.model)))

async function load() {
  const requestVersion = ++version
  catalog.value = null
  error.value = ''
  notice.value = ''
  if (!configID.value) return
  loading.value = true
  try {
    const result = await discoverProviderModels(configID.value)
    if (version !== requestVersion) return
    catalog.value = result
    enabled.value = result.saved.filter((model) => model.enabled).map((model) => model.model)
    guest.value = result.saved.filter((model) => model.guest_enabled).map((model) => model.model)
  } catch (cause) {
    if (version === requestVersion) error.value = formatAPIError(cause)
  } finally {
    if (version === requestVersion) loading.value = false
  }
}

function toggleEnabled(model: string, selected: boolean) {
  enabled.value = selected ? [...enabled.value, model] : enabled.value.filter((entry) => entry !== model)
  if (!selected) guest.value = guest.value.filter((entry) => entry !== model)
}

async function save() {
  saving.value = true
  error.value = ''
  notice.value = ''
  try {
    await saveProviderModels(configID.value, enabled.value, guest.value)
    notice.value = '模型选择已保存'
  } catch (cause) { error.value = formatAPIError(cause) }
  finally { saving.value = false }
}

watch(() => props.configs, (configs) => {
  if (!configs.some((config) => config.id === configID.value)) configID.value = configs[0]?.id || 0
}, { immediate: true })
watch(() => [configuration.value?.id, configuration.value?.updated_at], load, { immediate: true })
</script>

<template>
  <section class="model-manager settings-panel settings-panel--wide" aria-label="供应商模型管理">
    <div class="settings-panel__header"><div><div class="settings-panel__title">供应商模型管理</div><div class="settings-panel__meta">启用的模型可在对话框中选择。游客只能使用管理员明确开放的模型。</div></div><button class="settings-action-button" type="button" :disabled="!catalog || loading || saving" @click="save">{{ saving ? '保存中' : '保存模型选择' }}</button></div>
    <div class="model-manager-controls">
      <label class="settings-field"><span>模型供应商</span><select v-model="configID" class="settings-input" :disabled="saving" aria-label="模型供应商"><option v-if="!configs.length" :value="0">请先保存供应商配置</option><option v-for="config in configs" :key="config.id" :value="config.id">{{ config.name }}{{ config.enabled ? '' : '（已停用）' }}</option></select></label>
      <label class="settings-field"><span>搜索模型</span><input v-model="search" class="settings-input" type="search" placeholder="输入模型名称" /></label>
      <button class="settings-action-button" type="button" :disabled="!configID || loading || saving" @click="load">{{ loading ? '同步中' : '同步模型目录' }}</button>
    </div>
    <p v-if="error" class="model-manager-error" role="alert">{{ error }}</p>
    <p v-if="notice" role="status">{{ notice }}</p>
    <p v-if="configuration && !configuration.enabled" class="settings-panel__meta">此供应商已停用，当前模型不会出现在对话框中。</p>
    <div v-if="unavailable.length" class="model-manager-error"><p>以下模型已不在上游目录中，请停用后保存；默认模型需要先在供应商配置中修改。</p><label v-for="model in unavailable" :key="model.model" class="model-manager-model"><input type="checkbox" :checked="enabled.includes(model.model)" :disabled="saving || model.model === configuration?.model" :aria-label="`启用 ${model.model}`" @change="toggleEnabled(model.model, ($event.target as HTMLInputElement).checked)" /><span>{{ model.model }}</span></label></div>
    <div v-if="catalog" class="model-manager-list" :aria-busy="saving">
      <div class="model-manager-list-heading"><span>{{ enabled.length }} / {{ catalog.models.length }} 个模型已启用</span><span v-if="owner">向游客开放</span></div>
      <div v-for="model in visibleModels" :key="model.id" class="model-manager-row">
        <label class="model-manager-model"><input type="checkbox" :checked="enabled.includes(model.id)" :disabled="saving || model.id === configuration?.model" :aria-label="`启用 ${model.id}`" @change="toggleEnabled(model.id, ($event.target as HTMLInputElement).checked)" /><span :title="model.id">{{ model.id }}<small v-if="model.id === configuration?.model">默认模型</small></span></label>
        <label v-if="owner" class="model-manager-guest"><input v-model="guest" type="checkbox" :value="model.id" :disabled="saving || !enabled.includes(model.id)" :aria-label="`向游客开放 ${model.id}`" /><span class="model-manager-mobile-label">游客</span></label>
      </div>
      <p v-if="!visibleModels.length" class="settings-panel__meta">没有符合条件的模型。</p>
    </div>
  </section>
</template>

<style scoped>
.model-manager-controls { display: flex; align-items: end; flex-wrap: wrap; gap: 12px; margin: 16px 0; }
.model-manager > .settings-panel__header { align-items: start; gap: 12px; }
.model-manager > .settings-panel__header > button { flex-shrink: 0; white-space: nowrap; }
.model-manager-controls label { flex: 1 1 180px; min-width: 0; }
.model-manager-list { max-height: 480px; overflow: auto; border: 1px solid var(--mf-border); border-radius: 10px; }
.model-manager-list-heading, .model-manager-row { display: flex; align-items: center; justify-content: space-between; gap: 18px; padding: 12px 16px; }
.model-manager-list-heading { position: sticky; top: 0; background: var(--mf-surface, #fff); font-size: 12px; z-index: 1; }
.model-manager-row { border-top: 1px solid var(--mf-border); }
.model-manager-model { display: flex; align-items: center; gap: 10px; min-width: 0; font-size: 13px; }
.model-manager-model span { overflow-wrap: anywhere; }
.model-manager-model small { display: inline-block; margin-left: 8px; color: var(--mf-text-muted); }
.model-manager-guest { display: flex; align-items: center; gap: 6px; flex-shrink: 0; padding: 6px; }
.model-manager-error { color: #bf4343; font-size: 13px; }
.model-manager-mobile-label { display: none; }
input[type=checkbox] { width: 16px; height: 16px; flex-shrink: 0; accent-color: var(--mf-accent, #252b33); }
@media (max-width: 600px) { .model-manager-list-heading, .model-manager-row { padding: 10px; gap: 10px; } .model-manager-mobile-label { display: inline; font-size: 11px; } }
</style>
