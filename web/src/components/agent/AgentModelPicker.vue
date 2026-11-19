<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, useId, watch } from 'vue'
import { IconCheck, IconClose, IconDown, IconSearch, IconStar, IconStarFill, IconThunderbolt } from '@arco-design/web-vue/es/icon'

type Choice = { key: string; id: string; name: string; provider: string; providerKey: string | number }
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
const groups = computed(() => {
  const providers = new Map<Choice['providerKey'], { key: string | number; name: string; entries: Choice[] }>()
  const favorites: Choice[] = []
  const term = query.value.trim().toLowerCase()
  for (const choice of props.choices) {
    if (![choice.name, choice.id, choice.provider].some(value => value.toLowerCase().includes(term))) continue
    if (pinned.value.includes(choice.key)) {
      favorites.push(choice)
      continue
    }
    const group = providers.get(choice.providerKey) ?? { key: choice.providerKey, name: choice.provider, entries: [] }
    group.entries.push(choice)
    providers.set(choice.providerKey, group)
  }
  return [...(favorites.length ? [{ key: 'pinned', name: '已置顶', entries: favorites }] : []), ...providers.values()]
})
let pressedOutside = false

function placeDialog() {
  const bounds = trigger.value!.getBoundingClientRect()
  const panel = dialog.value!
  const viewport = window.visualViewport
  const keyboardInset = viewport?.scale === 1 ? Math.max(0, window.innerHeight - viewport.height - viewport.offsetTop) : 0
  panel.style.setProperty('--chooser-x', `${Math.max(16, Math.min(bounds.left, window.innerWidth - 426))}px`)
  panel.style.setProperty('--chooser-bottom', `${Math.max(16, Math.min(window.innerHeight - bounds.top + 8, window.innerHeight - 180))}px`)
  panel.style.setProperty('--chooser-keyboard-inset', `${keyboardInset}px`)
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
    <IconThunderbolt aria-hidden="true" /><span>{{ selected?.name ?? (choices.length ? '选择对话模型' : '暂无可用模型') }}</span><IconDown aria-hidden="true" />
  </button>
  <Teleport to="body">
    <dialog :id="`${id}-dialog`" ref="dialog" class="model-dialog" :aria-labelledby="`${id}-title`" closedby="any" @close="onClose" @pointerdown="onPointerDown" @pointerup="onPointerUp">
      <header><h2 :id="`${id}-title`">选择对话模型</h2><button type="button" class="model-icon" aria-label="关闭模型选择" title="关闭模型选择" autofocus @click="close"><IconClose aria-hidden="true" /></button></header>
      <label class="model-search"><IconSearch aria-hidden="true" /><input ref="search" v-model="query" type="search" placeholder="搜索模型或供应商" aria-label="搜索模型或供应商" /></label>
      <div ref="list" class="model-list">
        <p v-if="!groups.length" class="model-empty" role="status">没有匹配的模型</p>
        <section v-for="group in groups" :key="group.key" :aria-label="group.name">
          <h3>{{ group.name }}</h3>
          <div v-for="choice in group.entries" :key="choice.key" class="model-row">
            <button type="button" class="model-option" :aria-pressed="choice.key === selectedKey" @click="choose(choice.key)"><span><strong>{{ choice.name }}</strong><small>{{ choice.id }}</small></span><IconCheck v-if="choice.key === selectedKey" aria-hidden="true" /></button>
            <button type="button" class="model-icon pin-button" :data-pin="choice.key" :aria-label="`${pinned.includes(choice.key) ? '取消置顶' : '置顶'} ${choice.name}`" :title="pinned.includes(choice.key) ? '取消置顶' : '置顶'" @click="togglePin(choice)"><IconStarFill v-if="pinned.includes(choice.key)" aria-hidden="true" /><IconStar v-else aria-hidden="true" /></button>
          </div>
        </section>
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
.model-dialog { position: fixed; inset: auto auto var(--chooser-bottom) var(--chooser-x); box-sizing: border-box; width: min(410px, calc(100% - 32px)); max-width: none; max-height: min(420px, 65dvh, calc(100dvh - var(--chooser-bottom) - 12px)); margin: 0; padding: 12px; overflow: auto; overscroll-behavior: contain; border: 0; border-radius: 14px; background: var(--mf-surface); color: var(--mf-text); font: 14px/1.5 var(--mf-font-family); box-shadow: var(--mf-panel-shadow); }
.model-dialog[open] { display: flex; flex-direction: column; }
.model-dialog::backdrop { background: rgb(0 0 0 / 10%); overflow: hidden; overscroll-behavior: contain; }
.model-dialog header { display: flex; flex: none; justify-content: space-between; align-items: center; gap: 8px; margin: 0 0 8px 3px; }
.model-dialog h2 { margin: 0; font-size: 16px; font-weight: 650; }
.model-icon { display: grid; place-items: center; flex: none; width: 34px; height: 34px; border-radius: 6px; color: var(--mf-text-muted); }
.model-icon svg { width: 17px; height: 17px; }
.pin-button svg { width: 15px; height: 15px; }
.model-search { display: flex; align-items: center; flex: none; gap: 7px; min-height: 40px; padding-inline: 9px; border-radius: 9px; background: color-mix(in oklab, var(--mf-surface-muted) 54%, var(--mf-surface)); color: var(--mf-text-muted); }
.model-search svg { width: 14px; height: 14px; flex: none; }
.model-search input { border: 0; outline: none; background: transparent; padding: 7px 0; min-width: 0; flex: 1; color: var(--mf-text); font: inherit; }
.model-search input::placeholder { color: var(--mf-text-muted); }
.model-search:focus-within, .model-dialog button:focus-visible, .model-button:focus-visible { outline: 2px solid var(--mf-primary-strong); outline-offset: 1px; }
.model-list { min-height: 0; overflow: auto; overscroll-behavior: contain; padding-inline: 3px; margin-inline: -3px; }
.model-list h3, .model-empty { margin: 12px 7px 4px; color: var(--mf-text-muted); font-size: 12px; }
.model-list h3 { font-weight: 600; }
.model-row { display: flex; align-items: center; gap: 3px; min-width: 0; }
.model-option { flex: 1; min-width: 0; display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 6px 7px; border-radius: 6px; text-align: start; }
.model-option svg { width: 14px; height: 14px; flex: none; }
.model-option > span { display: grid; min-width: 0; overflow-wrap: anywhere; }
.model-option small { color: var(--mf-text-muted); font-size: 11px; }
.model-option[aria-pressed='true'] { background: color-mix(in oklab, var(--mf-primary) 14%, var(--mf-surface)); color: var(--mf-primary-strong); }
@media (hover: hover) {
  .model-button:hover:not(:disabled), .model-icon:hover, .model-option:hover:not([aria-pressed='true']) { background: var(--mf-surface-muted); }
}
@media (prefers-reduced-motion: no-preference) {
  .model-dialog { transition: transform 190ms cubic-bezier(.2, .75, .25, 1); }
  @starting-style { .model-dialog[open] { transform: translateY(10px); } }
}
@media (max-width: 799px), (pointer: coarse) {
  .model-button, .model-option { min-height: 44px; }
  .model-icon { width: 44px; height: 44px; }
}
@media (max-width: 799px) {
  .model-dialog { inset: auto 0 var(--chooser-keyboard-inset, 0px); width: 100%; max-height: min(460px, 75dvh, calc(100dvh - var(--chooser-keyboard-inset, 0px))); padding: 12px 14px max(14px, env(safe-area-inset-bottom, 0px)); border-radius: 16px 16px 0 0; }
  .model-search input { font-size: 16px; }
}
@media (forced-colors: active) {
  .model-dialog { outline: 1px solid CanvasText; }
  .model-search { outline: 1px solid CanvasText; }
}
</style>
