<script setup lang="ts">
import { onBeforeUnmount, ref, useId } from 'vue'

defineProps<{ disabled?: boolean; label: string }>()
const draft = defineModel<string>({ required: true })
const emit = defineEmits<{ submit: [] }>()
const input = ref<HTMLTextAreaElement | null>(null)
const id = useId()
const minHeight = 48
const maxHeight = 224
const height = ref(minHeight)
let drag: { id: number; start: number; height: number; nextHeight: number } | null = null
let frame = 0

function resize(value: number) {
  height.value = Math.max(minHeight, Math.min(maxHeight, value))
}

function handleKey(event: KeyboardEvent) {
  if (event.key !== 'Enter' || event.shiftKey || event.altKey || event.ctrlKey || event.metaKey) return
  if (event.isComposing || event.keyCode === 229 || !window.matchMedia('(min-width: 768px) and (pointer: fine)').matches) return
  event.preventDefault()
  emit('submit')
}

function startResize(event: PointerEvent) {
  if (event.button !== 0 || drag) return
  const handle = event.currentTarget as HTMLElement
  handle.setPointerCapture(event.pointerId)
  drag = { id: event.pointerId, start: event.clientY, height: height.value, nextHeight: height.value }
}

function moveResize(event: PointerEvent) {
  if (!drag || drag.id !== event.pointerId) return
  drag.nextHeight = drag.height + drag.start - event.clientY
  if (frame) return
  frame = requestAnimationFrame(() => {
    frame = 0
    if (drag) resize(drag.nextHeight)
  })
}

function stopResize(event: PointerEvent) {
  if (drag?.id !== event.pointerId) return
  cancelAnimationFrame(frame)
  frame = 0
  resize(event.type === 'pointerup' ? drag.height + drag.start - event.clientY : drag.nextHeight)
  drag = null
}

onBeforeUnmount(() => cancelAnimationFrame(frame))
defineExpose({ focus: () => input.value?.focus() })
</script>

<template>
  <form class="agent-composer" @submit.prevent="emit('submit')">
    <div class="agent-composer-handle" role="separator" tabindex="0" aria-orientation="horizontal"
      aria-label="调整输入框高度" :aria-controls="id" :aria-valuenow="height" :aria-valuemin="minHeight" :aria-valuemax="maxHeight"
      @pointerdown="startResize" @pointermove="moveResize" @pointerup="stopResize" @pointercancel="stopResize" @lostpointercapture="stopResize"
      @keydown.up.prevent="resize(height + 16)" @keydown.down.prevent="resize(height - 16)"
      @keydown.home.prevent="resize(minHeight)" @keydown.end.prevent="resize(maxHeight)" />
    <label class="agent-composer-label" :for="id">{{ label }}</label>
    <div class="agent-composer-field" :data-draft="draft" :style="{ '--composer-height': `${height}px` }">
      <textarea :id="id" ref="input" v-model="draft" rows="2" :disabled="disabled" placeholder="向助理发送消息…" @keydown="handleKey" />
    </div>
    <div class="agent-composer-toolbar"><slot /></div>
  </form>
</template>

<style scoped>
.agent-composer { position: relative; display: grid; gap: 5px; padding: 13px 14px 9px; border: 1px solid transparent; border-radius: 24px; background: var(--mf-surface); box-shadow: 0 2px 5px rgb(0 0 0 / 12%), 0 6px 20px rgb(0 0 0 / 9%); }
.agent-composer::after { content: ''; position: absolute; inset: -1px; border-radius: inherit; pointer-events: none; opacity: 0; box-shadow: 0 4px 12px color-mix(in srgb, var(--mf-primary) 22%, transparent), 0 10px 24px rgb(0 0 0 / 17%); }
.agent-composer:has(textarea:focus)::after { opacity: 1; }
.agent-composer:has(textarea:focus-visible) { border-color: var(--mf-primary); }
@media (prefers-reduced-motion: no-preference) { .agent-composer::after { transition: opacity 180ms ease-out; } }
.agent-composer-label { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); }
.agent-composer-handle { position: absolute; z-index: 1; inset: -8px 0 auto; height: 16px; cursor: ns-resize; touch-action: none; }
.agent-composer-handle::after { content: ''; display: block; width: 32px; height: 3px; margin: 6px auto; border-radius: 3px; background: var(--mf-text-muted); opacity: .55; }
.agent-composer-handle:focus-visible { outline: 2px solid var(--mf-primary, #2563eb); border-radius: 4px; }
.agent-composer-field { min-width: 0; }
.agent-composer textarea { display: block; box-sizing: border-box; width: 100%; height: var(--composer-height); padding: 4px; overflow: auto; resize: none; border: 0; background: transparent; color: var(--mf-text); font: inherit; font-size: 15px; line-height: 1.55; }
.agent-composer textarea:focus { outline: none; }
.agent-composer textarea::placeholder { color: var(--mf-text-muted); }
.agent-composer-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 8px; min-width: 0; }
@media (max-width: 799px), (pointer: coarse) {
  .agent-composer { border-radius: 22px; padding: 10px 11px 7px; gap: 2px; }
  .agent-composer-handle { display: none; }
  .agent-composer textarea { field-sizing: content; height: auto; min-height: 48px; max-height: min(180px, 30dvh); font-size: 16px; }
  @supports not (field-sizing: content) {
    .agent-composer-field { display: grid; }
    .agent-composer-field::after, .agent-composer textarea { grid-area: 1 / 1; padding: 4px; min-height: 48px; max-height: min(180px, 30dvh); font: inherit; font-size: 16px; line-height: 1.55; white-space: pre-wrap; overflow-wrap: anywhere; }
    .agent-composer-field::after { content: attr(data-draft) ' '; visibility: hidden; overflow: hidden; }
  }
}
</style>
