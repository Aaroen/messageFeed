<script setup lang="ts">
import { computed, onBeforeUnmount, ref, useId, watch } from 'vue'
import { IconBulb, IconClose, IconDown, IconThunderbolt } from '@arco-design/web-vue/es/icon'
import AgentEffortFlame from './AgentEffortFlame.vue'

const props = defineProps<{ levels: string[]; model: string; disabled?: boolean }>()
const effort = defineModel<string>({ required: true })
const id = useId()
const trigger = ref<HTMLButtonElement | null>(null)
const dialog = ref<HTMLDialogElement | null>(null)
const open = ref(false)
const labels: Record<string, string> = { off: '关闭', none: '关闭', minimal: '极低', low: '低', medium: '中等', high: '高', xhigh: '极高', max: '最高' }
const options = computed(() => ['', ...props.levels])
const committed = computed(() => Math.max(0, options.value.indexOf(effort.value)))
const position = ref(committed.value)
const selected = computed(() => Math.round(position.value))
const atLimit = computed(() => props.levels.length > 0 && selected.value === props.levels.length)
const ratio = computed(() => position.value / Math.max(1, props.levels.length))
const dragging = ref(false)
const particles = ref<{ kind: string; style: Record<string, string> }[]>([])
const flow = ref<Record<string, string>[][]>([])
const tailParticles = computed(() => particles.value.slice(0, 24))
const spillParticles = computed(() => particles.value.slice(24))
let dragBounds: DOMRect
let backdropPressed = false

watch(committed, cancelDrag)
watch(() => props.disabled, disabled => { if (disabled) dialog.value?.close() })

function label(level: string) { return level ? labels[level] || level : '默认' }

function placePanel() {
  const bounds = trigger.value!.getBoundingClientRect()
  const viewportTop = window.visualViewport?.offsetTop ?? 0
  dialog.value!.style.setProperty('--effort-anchor', `${bounds.right}px`)
  dialog.value!.style.setProperty('--effort-bottom', `${window.innerHeight - Math.max(viewportTop + 150, bounds.top - 8)}px`)
}

function show() {
  backdropPressed = false
  placePanel()
  flow.value = [5, 9, 14].map((count, zone) => Array.from({ length: count }, (_, index) => {
    const start = (index + .1 + Math.random() * .8) / count
    const progress = (zone + start) / 3
    const distance = 8 + progress * 12 + Math.random() * 7
    const speed = 7 + 62 * progress ** 1.5
    const duration = 1650 * distance / speed
    return {
      '--flow-start': `${start * 100}%`,
      '--flow-height': `${18 + Math.random() * 64}%`,
      '--flow-drift': `${-6 + Math.random() * 12}px`,
      '--flow-distance': `${distance}px`,
      '--flow-duration': `${duration}ms`,
      '--flow-delay': `${-Math.random() * duration}ms`,
      '--flow-opacity': `${.42 + progress * .33 + Math.random() * .14}`,
      '--flow-size': `${1.25 + progress * 1.2 + Math.random() * 1.2}px`,
    }
  }))
  particles.value = Array.from({ length: 32 }, (_, index) => {
    const kind = index < 10 ? 'spark' : index < 18 ? 'shard' : index < 24 ? 'ember' : 'spill'
    const direction = index % 2 ? -1 : 1
    const angle = kind === 'spill' ? 40 + Math.random() * 30 : kind === 'spark' ? 3 + Math.random() * 8 : 9 + Math.random() * 25
    const distance = kind === 'spill' ? 15 + Math.random() * 14 : kind === 'ember' ? -(32 + Math.random() * 55) : -(56 + Math.random() * 112)
    const duration = kind === 'spark' ? 440 + Math.random() * 470 : 920 + Math.random() * 1100
    return {
      kind,
      style: {
        '--particle-angle': `${direction * angle}deg`,
        '--particle-bend': `${direction * (4 + Math.random() * 14)}px`,
        '--particle-distance': `${distance}px`,
        '--particle-delay': `${-Math.random() * duration}ms`,
        '--particle-duration': `${duration}ms`,
        '--particle-size': `${1.25 + Math.random() * 2.5}px`,
        '--particle-brightness': `${.36 + Math.random() * .58}`,
        '--particle-turn': `${direction * (40 + Math.random() * 125)}deg`,
      },
    }
  })
  dialog.value!.showModal()
  open.value = true
  window.addEventListener('resize', placePanel)
  window.visualViewport?.addEventListener('resize', placePanel)
}

function cancelDrag() {
  dragging.value = false
  position.value = committed.value
}

function close() {
  cancelDrag()
  open.value = false
  window.removeEventListener('resize', placePanel)
  window.visualViewport?.removeEventListener('resize', placePanel)
}

onBeforeUnmount(close)

function recordBackdropPress(event: PointerEvent) {
  backdropPressed = event.target === dialog.value
}

function closeBackdrop(event: MouseEvent) {
  if (backdropPressed && event.target === dialog.value) dialog.value?.close()
  backdropPressed = false
}

function startDrag(event: PointerEvent) {
  if (!event.isPrimary || event.button !== 0) return
  event.preventDefault()
  const input = event.currentTarget as HTMLInputElement
  dragBounds = input.getBoundingClientRect()
  input.focus({ preventScroll: true })
  input.setPointerCapture(event.pointerId)
  dragging.value = true
  moveDrag(event)
}

function moveDrag(event: PointerEvent) {
  const input = event.currentTarget as HTMLInputElement
  if (!dragging.value || !input.hasPointerCapture(event.pointerId)) return
  const fraction = (event.clientX - dragBounds.left - 14) / (dragBounds.width - 28)
  position.value = Math.min(1, Math.max(0, fraction)) * props.levels.length
}

function endDrag(event: PointerEvent) {
  if (!dragging.value || !(event.currentTarget as HTMLInputElement).hasPointerCapture(event.pointerId)) return
  moveDrag(event)
  commit(position.value)
}

function commit(index: number) {
  dragging.value = false
  position.value = Math.min(props.levels.length, Math.max(0, Math.round(index)))
  effort.value = options.value[position.value]!
}

function handleKey(event: KeyboardEvent) {
  const offsets = new Map([['ArrowLeft', -1], ['ArrowDown', -1], ['PageDown', -1], ['ArrowRight', 1], ['ArrowUp', 1], ['PageUp', 1]])
  const offset = offsets.get(event.key)
  if (event.key !== 'Home' && event.key !== 'End' && offset === undefined) return
  event.preventDefault()
  commit(event.key === 'Home' ? 0 : event.key === 'End' ? props.levels.length : selected.value + offset!)
}
</script>

<template>
  <button ref="trigger" class="effort-trigger" type="button" :aria-label="`思考强度：${label(effort)}`" :disabled="disabled" :aria-expanded="open" :aria-controls="`${id}-dialog`" aria-haspopup="dialog" @click="show">
    <IconBulb aria-hidden="true" /><strong>{{ label(effort) }}</strong><IconDown aria-hidden="true" />
  </button>
  <Teleport to="body">
    <dialog :id="`${id}-dialog`" ref="dialog" class="effort-panel" :aria-labelledby="`${id}-title`" @close="close" @pointerdown="recordBackdropPress" @click="closeBackdrop">
      <div class="effort-sheet" :class="{ 'at-limit': atLimit }">
        <header class="effort-heading">
          <div><label :id="`${id}-title`" :for="id">思考强度</label><small>{{ model }}</small></div>
          <span class="effort-badge" aria-hidden="true"><IconThunderbolt v-if="atLimit" />{{ label(options[selected]!) }}</span>
          <button type="button" aria-label="关闭思考强度" @click="dialog?.close()"><IconClose aria-hidden="true" /></button>
        </header>
        <div class="effort-range-wrap" :class="{ dragging, lit: ratio > 0 }" :style="{ '--effort-ratio': ratio }">
          <div class="effort-track" aria-hidden="true">
            <span class="effort-fill"></span>
            <span class="effort-flow">
              <span v-for="(zone, zoneIndex) in flow" :key="zoneIndex" class="effort-flow-zone">
                <span v-for="(speck, index) in zone" :key="index" class="effort-flow-speck" :style="speck"></span>
              </span>
            </span>
          </div>
          <AgentEffortFlame v-if="flow.length" :active="open" :ratio="ratio" :at-limit="atLimit" />
          <div class="effort-markers" aria-hidden="true">
            <span v-for="(option, index) in options" :key="option" class="effort-tick" :class="{ reached: index <= selected }" :style="{ left: `${index / levels.length * 100}%` }"></span>
            <span class="effort-travel">
              <span class="effort-tail">
                <span v-for="(particle, index) in tailParticles" :key="index" class="effort-particle" :class="particle.kind" :style="particle.style"></span>
              </span>
              <span v-for="(particle, index) in spillParticles" v-show="atLimit" :key="index" class="effort-particle spill" :style="particle.style"></span>
              <span class="effort-thumb"></span>
            </span>
          </div>
          <input :id="id" class="effort-input" type="range" min="0" :max="levels.length" step="any" :value="position" :aria-valuetext="`${label(options[selected]!)}${atLimit ? '，模型最高档' : ''}`" autofocus @change="commit(($event.target as HTMLInputElement).valueAsNumber)" @pointerdown="startDrag" @pointermove="moveDrag" @pointerup="endDrag" @lostpointercapture="cancelDrag" @keydown="handleKey" />
        </div>
        <div class="effort-labels" aria-hidden="true"><span v-for="(option, index) in options" :key="option" :class="{ selected: index === selected }" :style="{ left: `${index / levels.length * 100}%` }">{{ label(option) }}</span></div>
        <p class="effort-hint">{{ atLimit ? '已达到此模型的最高档' : '' }}</p>
      </div>
    </dialog>
  </Teleport>
</template>

<style scoped>
.effort-trigger, .effort-heading button { display: inline-flex; align-items: center; justify-content: center; gap: 6px; min-height: 34px; padding: 0 8px; border: 0; border-radius: 8px; background: transparent; color: var(--mf-text); cursor: pointer; white-space: nowrap; }
.effort-trigger:hover:not(:disabled), .effort-trigger[aria-expanded='true'], .effort-heading button:hover { background: var(--mf-surface-muted); }
.effort-trigger:disabled { opacity: .6; cursor: default; }
.effort-panel { --effort-width: min(352px, calc(100% - 24px)); position: fixed; inset: auto auto var(--effort-bottom) clamp(12px, calc(var(--effort-anchor) - var(--effort-width)), calc(100% - var(--effort-width) - 12px)); width: var(--effort-width); max-width: none; margin: 0; padding: 0; max-height: max(92px, calc(100dvh - var(--effort-bottom) - 12px)); overflow: auto; overscroll-behavior: contain; border: 0; border-radius: 16px; background: var(--mf-surface); color: var(--mf-text); box-shadow: var(--mf-panel-shadow); }
.effort-panel::backdrop { background: rgb(0 0 0 / 12%); }
.effort-sheet { padding: 12px 17px 14px; }
.effort-heading { display: flex; align-items: center; gap: 8px; }
.effort-heading > div { min-width: 0; flex: 1; display: grid; }
.effort-heading label { font-weight: 700; }
.effort-heading small { overflow: hidden; color: var(--mf-text-muted); text-overflow: ellipsis; white-space: nowrap; }
.effort-badge { display: flex; align-items: center; gap: 5px; padding: 3px 9px; border-radius: 7px; background: var(--mf-tab-active-bg); font-size: 12px; }
.effort-range-wrap { position: relative; container-type: inline-size; height: 46px; margin-block-start: 8px; }
.effort-input { position: absolute; inset: 0; width: 100%; height: 100%; margin: 0; opacity: 0; appearance: none; background: transparent; cursor: pointer; touch-action: none; }
.effort-input::-webkit-slider-thumb { appearance: none; width: 28px; height: 28px; }
.effort-input::-moz-range-thumb { width: 28px; height: 28px; box-sizing: border-box; border: 0; }
.effort-track { position: absolute; inset: 11px 0; overflow: hidden; border-radius: 999px; background: var(--mf-surface-muted); }
.effort-range-wrap:has(:focus-visible) .effort-track { outline: 2px solid var(--mf-primary); outline-offset: 4px; }
.effort-fill { position: absolute; inset: 0; background: linear-gradient(90deg, var(--mf-primary), var(--mf-aqua)); transform: scaleX(var(--effort-ratio)); transform-origin: left; }
.effort-flow { position: absolute; inset: 0 4px; display: grid; grid-template-columns: repeat(3, 1fr); }
.effort-flow-zone { position: relative; }
.effort-flow-speck { position: absolute; left: var(--flow-start); top: var(--flow-height); width: var(--flow-size); height: var(--flow-size); border-radius: 50%; background: #fff; box-shadow: 0 0 2px rgb(0 0 0 / 74%); opacity: 0; }
.effort-flow-zone:last-child .effort-flow-speck:nth-child(even) { width: calc(var(--flow-size) * 2.5); }
.effort-markers { position: absolute; inset: 0 14px; pointer-events: none; }
.effort-tick, .effort-travel { position: absolute; top: 50%; }
.effort-tick { width: 4px; height: 4px; border-radius: 50%; background: var(--mf-text-muted); transform: translate(-50%, -50%); }
.effort-tick.reached { background: var(--mf-surface); }
.effort-travel { left: 0; transform: translateX(calc((100cqw - 28px) * var(--effort-ratio))); }
.effort-thumb { display: block; width: 28px; height: 28px; border-radius: 50%; background: var(--mf-surface); box-shadow: 0 2px 5px rgb(0 0 0 / 32%); transform: translate(-50%, -50%); }
.effort-thumb::after { content: ''; position: absolute; inset: 9px; border-radius: 50%; background: var(--mf-primary); }
.effort-tail { display: none; position: absolute; inset: 0 calc(100% + 2px) auto auto; width: min(var(--tail-length, 108px), calc((100cqw - 28px) * var(--effort-ratio))); height: var(--tail-height, 22px); overflow: hidden; transform: translateY(-50%); }
.lit .effort-tail { display: block; }
.at-limit .effort-tail { --tail-length: 188px; --tail-height: 46px; }
.effort-particle { position: absolute; z-index: 1; top: 50%; right: 6px; width: var(--particle-size); height: var(--particle-size); border-radius: 50%; background: white; opacity: 0; }
.effort-particle.spark { width: calc(var(--particle-size) * 3); height: calc(var(--particle-size) * .65); background: linear-gradient(90deg, transparent, white); }
.effort-particle.shard { display: none; width: calc(var(--particle-size) * 1.6); height: calc(var(--particle-size) * .55); background: color-mix(in srgb, var(--mf-aqua) 45%, white); }
.at-limit .effort-particle.shard { display: block; }
.effort-particle.ember { background: color-mix(in srgb, var(--mf-primary) 38%, white); }
.effort-particle.spill { right: -2px; background: var(--mf-text-muted); box-shadow: 0 0 2px var(--mf-surface); }
.effort-labels { position: relative; height: 18px; margin-inline: 14px; color: var(--mf-text-muted); font-size: 11px; }
.effort-labels span { position: absolute; transform: translateX(-50%); white-space: nowrap; }
.effort-labels .selected { color: var(--mf-text); font-weight: 700; }
.effort-hint { min-height: 17px; margin: 5px 0 0; color: var(--mf-text-muted); font-size: 12px; }
@media (max-width: 767px), (pointer: coarse) { .effort-trigger, .effort-heading button { min-width: 44px; min-height: 44px; } }
@media (forced-colors: active) { .effort-track, .effort-markers { display: none; } .effort-input { appearance: auto; opacity: 1; } }
@media (prefers-reduced-motion: no-preference) {
  .effort-range-wrap:not(.dragging) :is(.effort-fill, .effort-travel) { transition: transform var(--motion-quick, 180ms) var(--ease-emphasized, ease-out); }
  .effort-panel[open] .lit .effort-particle { animation: effort-particle var(--particle-duration) cubic-bezier(.16, .55, .58, 1) var(--particle-delay) infinite; }
  .effort-panel[open] .effort-flow-speck { animation: effort-flow var(--flow-duration) linear var(--flow-delay) infinite; }
}
@keyframes effort-flow {
  0% { transform: translateX(calc(var(--flow-distance) * -.65)) scale(.65); }
  0%, 100% { opacity: 0; }
  20%, 72% { opacity: var(--flow-opacity); }
  100% { transform: translate(var(--flow-distance), var(--flow-drift)) scaleX(1.5); }
}
@keyframes effort-particle {
  0% { transform: rotate(var(--particle-angle)) translateX(0) scale(.45); opacity: 0; }
  16% { opacity: var(--particle-brightness); }
  52% { transform: rotate(var(--particle-angle)) translate(calc(var(--particle-distance) * .54), calc(var(--particle-bend) * .28)) rotate(calc(var(--particle-turn) * .36)); }
  66% { opacity: calc(var(--particle-brightness) * .74); }
  100% { transform: rotate(var(--particle-angle)) translate(var(--particle-distance), var(--particle-bend)) rotate(var(--particle-turn)) scale(.15); opacity: 0; }
}
</style>
