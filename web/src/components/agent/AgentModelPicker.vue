<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, useId, watch } from 'vue'
import { IconCheck, IconClose, IconDown, IconSearch, IconStar, IconStarFill, IconThunderbolt } from '@arco-design/web-vue/es/icon'
import { Brain } from '@lucide/vue'
import type { AgentModelIcon } from '../../api/agentModels'

type Choice = { key: string; name: string; icon: AgentModelIcon }
const modelIcons = { lightning: IconThunderbolt, brain: Brain }
const props = defineProps<{ choices: Choice[]; disabled?: boolean }>()
const selectedKey = defineModel<string>({ required: true })
const id = useId()
const trigger = ref<HTMLButtonElement | null>(null)
const dialog = ref<HTMLDialogElement | null>(null)
const search = ref<HTMLInputElement | null>(null)
const list = ref<HTMLElement | null>(null)
const open = ref(false)
const query = ref('')
const pinned = ref<string[]>([])
const mobile = window.matchMedia('(max-width: 799px), (pointer: coarse)')
const selected = computed(() => props.choices.find(choice => choice.key === selectedKey.value))
const matches = computed(() => {
  const term = query.value.trim().toLowerCase()
  return props.choices.filter(choice => choice.name.toLowerCase().includes(term))
    .sort((first, second) => Number(pinned.value.includes(second.key)) - Number(pinned.value.includes(first.key)))
})
let pressedOutside = false

function placeDialog() {
  const bounds = trigger.value!.getBoundingClientRect()
  const panel = dialog.value!
  const viewport = window.visualViewport
  const visibleTop = viewport?.scale === 1 ? viewport.offsetTop : 0
  const visibleHeight = viewport?.scale === 1 ? viewport.height : window.innerHeight
  const visibleBottom = visibleTop + visibleHeight
  const bottomEdge = Math.min(visibleBottom - 8, Math.max(bounds.top - 8, visibleTop + 160))
  panel.style.setProperty('--chooser-x', `${bounds.left}px`)
  panel.style.setProperty('--chooser-top', `${visibleTop}px`)
  panel.style.setProperty('--chooser-bottom', `${Math.max(8, window.innerHeight - bottomEdge)}px`)
}

function show() {
  query.value = ''
  pressedOutside = false
  placeDialog()
  dialog.value!.showModal()
  open.value = true
  list.value!.scrollTop = 0
  if (!mobile.matches) search.value!.focus({ preventScroll: true })
  window.addEventListener('resize', placeDialog)
  window.visualViewport?.addEventListener('resize', placeDialog)
  window.visualViewport?.addEventListener('scroll', placeDialog)
}

function close() { dialog.value?.close() }

function onClose() {
  open.value = false
  window.removeEventListener('resize', placeDialog)
  window.visualViewport?.removeEventListener('resize', placeDialog)
  window.visualViewport?.removeEventListener('scroll', placeDialog)
}

function isOutside(event: PointerEvent) {
  const bounds = dialog.value!.getBoundingClientRect()
  return event.clientX < bounds.left || event.clientX > bounds.right || event.clientY < bounds.top || event.clientY > bounds.bottom
}

function onPointerDown(event: PointerEvent) {
  pressedOutside = !('closedBy' in HTMLDialogElement.prototype) && isOutside(event)
}

function onPointerUp(event: PointerEvent) {
  if (pressedOutside && isOutside(event)) close()
  pressedOutside = false
}

function choose(key: string) {
  selectedKey.value = key
  close()
}

async function togglePin(choice: Choice) {
  pinned.value = pinned.value.includes(choice.key) ? pinned.value.filter(key => key !== choice.key) : [...pinned.value, choice.key]
  await nextTick()
  list.value!.querySelector<HTMLButtonElement>(`[data-pin="${CSS.escape(choice.key)}"]`)?.focus()
}

watch(() => props.disabled || !props.choices.length, disabled => { if (disabled) close() })
onBeforeUnmount(onClose)
defineExpose({ dismiss: close })
</script>

<template>
  <button ref="trigger" type="button" class="model-button" :aria-label="selected ? `选择对话模型：${selected.name}` : '选择对话模型'" aria-haspopup="dialog" :aria-controls="`${id}-dialog`" :aria-expanded="open" :disabled="disabled || !choices.length" @click="show">
    <component :is="modelIcons[selected.icon]" v-if="selected?.icon" :data-model-icon="selected.icon" aria-hidden="true" /><span>{{ selected?.name ?? (choices.length ? '选择对话模型' : '暂无可用模型') }}</span><IconDown aria-hidden="true" />
  </button>
  <Teleport to="body">
    <dialog :id="`${id}-dialog`" ref="dialog" class="model-dialog" :aria-labelledby="`${id}-title`" closedby="any" @close="onClose" @pointerdown="onPointerDown" @pointerup="onPointerUp">
      <header><h2 :id="`${id}-title`">选择对话模型</h2><button type="button" class="model-icon" aria-label="关闭模型选择" title="关闭模型选择" autofocus @click="close"><IconClose aria-hidden="true" /></button></header>
      <label class="model-search"><IconSearch aria-hidden="true" /><input ref="search" v-model="query" type="search" placeholder="搜索模型" aria-label="搜索模型" /></label>
      <div ref="list" class="model-list">
        <p v-if="!matches.length" class="model-empty" role="status">没有匹配的模型</p>
        <div v-for="choice in matches" :key="choice.key" class="model-row">
          <button type="button" class="model-option" :aria-pressed="choice.key === selectedKey" @click="choose(choice.key)"><component :is="modelIcons[choice.icon]" v-if="choice.icon" :data-model-icon="choice.icon" aria-hidden="true" /><span>{{ choice.name }}</span><IconCheck v-if="choice.key === selectedKey" aria-hidden="true" /></button>
          <button type="button" class="model-icon pin-button" :data-pin="choice.key" :aria-label="`${pinned.includes(choice.key) ? '取消置顶' : '置顶'} ${choice.name}`" :title="pinned.includes(choice.key) ? '取消置顶' : '置顶'" @click="togglePin(choice)"><IconStarFill v-if="pinned.includes(choice.key)" aria-hidden="true" /><IconStar v-else aria-hidden="true" /></button>
        </div>
      </div>
    </dialog>
  </Teleport>
</template>

<style scoped>
.model-button, .model-icon, .model-option { padding: 0; border: 0; background: transparent; color: inherit; font: inherit; cursor: pointer; }
.model-button { display: inline-flex; align-items: center; gap: 6px; min-width: 0; min-height: 34px; padding-inline: 6px; border-radius: 5px; color: var(--mf-text); font: 13px var(--mf-font-family); }
.model-button span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.model-button svg { flex: none; width: 14px; height: 14px; color: var(--mf-text-muted); }
.model-button svg:last-child { width: 10px; height: 10px; }
.model-button:disabled { opacity: .6; cursor: default; }
.model-button[aria-expanded='true'] { background: var(--mf-surface-muted); }
.model-dialog { --chooser-gutter-left: max(10px, env(safe-area-inset-left, 0px)); --chooser-gutter-right: max(10px, env(safe-area-inset-right, 0px)); --chooser-width: min(220px, calc(100% - var(--chooser-gutter-left) - var(--chooser-gutter-right))); position: fixed; inset: auto auto var(--chooser-bottom) clamp(var(--chooser-gutter-left), var(--chooser-x), calc(100% - var(--chooser-width) - var(--chooser-gutter-right))); box-sizing: border-box; width: var(--chooser-width); max-width: none; max-height: min(260px, calc(100dvh - var(--chooser-bottom) - max(var(--chooser-top), env(safe-area-inset-top, 0px)) - 8px)); margin: 0; padding: 8px; overflow: auto; overscroll-behavior: contain; border: 0; border-radius: 12px; background: var(--mf-surface); color: var(--mf-text); font: 13px/1.45 var(--mf-font-family); box-shadow: var(--mf-panel-shadow); }
.model-dialog[open] { display: flex; flex-direction: column; }
.model-dialog::backdrop { background: rgb(0 0 0 / 10%); overflow: hidden; overscroll-behavior: contain; }
.model-dialog header { display: flex; flex: none; justify-content: space-between; align-items: center; gap: 6px; margin: 0 0 4px 6px; }
.model-dialog h2 { margin: 0; font-size: 13px; font-weight: 650; }
.model-icon { display: grid; place-items: center; flex: none; width: 28px; height: 28px; border-radius: 6px; color: var(--mf-text-muted); }
.model-icon svg { width: 14px; height: 14px; }
.model-search { display: flex; align-items: center; flex: none; gap: 6px; min-height: 30px; padding-inline: 7px; border-radius: 6px; background: var(--mf-surface-muted); color: var(--mf-text-muted); }
.model-search svg { width: 14px; height: 14px; flex: none; }
.model-search input { border: 0; outline: none; background: transparent; padding: 7px 0; min-width: 0; flex: 1; color: var(--mf-text); font: inherit; }
.model-search input::placeholder { color: var(--mf-text-muted); }
.model-search:focus-within, .model-dialog button:focus-visible, .model-button:focus-visible { outline: 2px solid var(--mf-primary-strong); outline-offset: 1px; }
.model-list { min-height: 0; overflow: auto; overscroll-behavior: contain; padding: 3px; margin: 2px -3px -3px; }
.model-empty { margin: 6px; color: var(--mf-text-muted); }
.model-row { display: flex; align-items: center; gap: 2px; min-width: 0; }
.model-option { flex: 1; min-width: 0; min-height: 32px; display: flex; align-items: center; gap: 6px; padding: 5px 6px; border-radius: 6px; text-align: start; }
.model-option svg { width: 14px; height: 14px; flex: none; }
.model-option > span { flex: 1; min-width: 0; overflow-wrap: anywhere; }
.model-option[aria-pressed='true'] { background: color-mix(in oklab, var(--mf-primary) 14%, var(--mf-surface)); color: var(--mf-primary-strong); }
@media (hover: hover) {
  .model-button:hover:not(:disabled), .model-icon:hover, .model-option:hover:not([aria-pressed='true']) { background: var(--mf-surface-muted); }
}
@media (prefers-reduced-motion: no-preference) {
  .model-dialog { transition: transform 190ms cubic-bezier(.2, .75, .25, 1); }
  @starting-style { .model-dialog[open] { transform: translateY(10px); } }
}
@media (max-width: 799px), (pointer: coarse) {
  .model-button, .model-option, .model-search input { min-height: 44px; }
  .model-icon { width: 44px; height: 44px; }
  .model-search input { font-size: 16px; }
}
@media (forced-colors: active) {
  .model-dialog { outline: 1px solid CanvasText; }
  .model-search { outline: 1px solid CanvasText; }
}
</style>
