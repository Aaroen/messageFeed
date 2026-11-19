<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { AgentLLMProviderConfig } from '@/api/agent'
import { discoverProviderModels, listSavedProviderModels, saveProviderModels } from '@/api/agentModels'
import { formatAPIError } from '@/api/client'

const props = defineProps<{ configs: AgentLLMProviderConfig[]; owner: boolean }>()
const configID = ref(0)
const search = ref('')
const identifiers = ref<string[]>([])
const aliases = ref<Record<string, string>>(Object.create(null))
const manualModel = ref('')
const ready = ref(false)
const enabled = ref<string[]>([])
const guest = ref<string[]>([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const notice = ref('')
let version = 0
const configuration = computed(() => props.configs.find((config) => config.id === configID.value))
const visibleModels = computed(() => {
  const query = search.value.trim().toLowerCase()
  return identifiers.value.filter((id) => id.toLowerCase().includes(query) || aliases.value[id]?.toLowerCase().includes(query))
})

async function load() {
  const requestVersion = ++version
  ready.value = false
  identifiers.value = []
  manualModel.value = ''
  error.value = ''
  notice.value = ''
  if (!configID.value) { loading.value = false; return }
  loading.value = true
  try {
    const result = await listSavedProviderModels(configID.value)
    if (version !== requestVersion) return
    identifiers.value = result.map((model) => model.model)
    enabled.value = result.filter((model) => model.enabled).map((model) => model.model)
    guest.value = result.filter((model) => model.guest_enabled).map((model) => model.model)
    aliases.value = Object.assign(Object.create(null), Object.fromEntries(result.map((model) => [model.model, model.display_name])))
    ready.value = true
  } catch (cause) {
    if (version === requestVersion) error.value = formatAPIError(cause)
  } finally {
    if (version === requestVersion) loading.value = false
  }
}

async function discover() {
  const requestVersion = version
  loading.value = true
  error.value = ''
  try {
    const result = await discoverProviderModels(configID.value)
    if (version === requestVersion) identifiers.value = [...new Set([...identifiers.value, ...result.models.map((model) => model.id)])]
  } catch (cause) {
    if (version === requestVersion) error.value = formatAPIError(cause)
  } finally {
    if (version === requestVersion) loading.value = false
  }
}

function addModel() {
  const id = manualModel.value.trim()
  if (!id) return
  if (!identifiers.value.includes(id)) identifiers.value = [...identifiers.value, id]
  if (!enabled.value.includes(id)) toggleEnabled(id, true)
  search.value = id
  manualModel.value = ''
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
    const names = Object.fromEntries(enabled.value.filter((id) => id in aliases.value).map((id) => [id, aliases.value[id]!.trim()]))
    await saveProviderModels(configID.value, enabled.value, guest.value, names)
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
    <div class="settings-panel__header"><div><div class="settings-panel__title">供应商模型管理</div><div class="settings-panel__meta">显示名称用于对话界面，模型标识用于发送请求。</div></div><button class="settings-action-button" type="button" :disabled="!ready || loading || saving" @click="save">{{ saving ? '保存中' : '保存模型选择' }}</button></div>
    <div class="model-manager-controls">
      <label class="settings-field"><span>模型供应商</span><select v-model="configID" class="settings-input" :disabled="saving" aria-label="模型供应商"><option v-if="!configs.length" :value="0">请先保存供应商配置</option><option v-for="config in configs" :key="config.id" :value="config.id">{{ config.name }}{{ config.enabled ? '' : '（已停用）' }}</option></select></label>
      <label class="settings-field"><span>搜索模型</span><input v-model="search" class="settings-input" type="search" placeholder="输入模型名称" /></label>
      <button class="settings-action-button" type="button" :disabled="!ready || loading || saving" @click="discover">{{ loading ? '读取中' : '同步模型目录' }}</button>
    </div>
    <form v-if="ready" class="model-manager-controls" @submit.prevent="addModel">
      <label class="settings-field"><span>手动添加模型</span><input v-model="manualModel" class="settings-input" maxlength="512" placeholder="输入供应商模型标识" :disabled="saving" /></label>
      <button class="settings-action-button" type="submit" :disabled="saving || !manualModel.trim()">添加模型</button>
    </form>
    <p v-if="error" class="model-manager-error" role="alert">{{ error }}</p>
    <p v-if="notice" role="status">{{ notice }}</p>
    <p v-if="configuration && !configuration.enabled" class="settings-panel__meta">此供应商已停用，当前模型不会出现在对话框中。</p>
    <div v-if="ready" class="model-manager-list" :aria-busy="saving">
      <div class="model-manager-list-heading"><span>{{ enabled.length }} / {{ identifiers.length }} 个模型已启用</span><span v-if="owner">向游客开放</span></div>
      <div v-for="id in visibleModels" :key="id" class="model-manager-row">
        <div class="model-manager-entry">
          <label class="model-manager-model"><input type="checkbox" :checked="enabled.includes(id)" :disabled="saving || id === configuration?.model" :aria-label="`启用 ${id}`" @change="toggleEnabled(id, ($event.target as HTMLInputElement).checked)" /><span>{{ id }}<small v-if="id === configuration?.model">默认模型</small></span></label>
          <input v-if="enabled.includes(id)" v-model="aliases[id]" class="settings-input" type="text" maxlength="80" :disabled="saving" :aria-label="`${id} 的显示名称`" placeholder="自定义显示名称（可选）" />
        </div>
        <label v-if="owner" class="model-manager-guest"><input v-model="guest" type="checkbox" :value="id" :disabled="saving || !enabled.includes(id)" :aria-label="`向游客开放 ${id}`" /><span class="model-manager-mobile-label">游客</span></label>
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
.model-manager-entry { display: grid; gap: 8px; flex: 1; min-width: 0; }
.model-manager-entry > input { width: min(100%, 26rem); }
.model-manager-model { display: flex; align-items: center; gap: 10px; min-width: 0; font-size: 13px; }
.model-manager-model span { overflow-wrap: anywhere; }
.model-manager-model small { display: inline-block; margin-left: 8px; color: var(--mf-text-muted); }
.model-manager-guest { display: flex; align-items: center; gap: 6px; flex-shrink: 0; padding: 6px; }
.model-manager-error { color: #bf4343; font-size: 13px; }
.model-manager-mobile-label { display: none; }
input[type=checkbox] { width: 16px; height: 16px; flex-shrink: 0; accent-color: var(--mf-accent, #252b33); }
@media (max-width: 600px) { .model-manager-list-heading, .model-manager-row { padding: 10px; gap: 10px; } .model-manager-mobile-label { display: inline; font-size: 11px; } }
</style>
