<script setup lang="ts">
import { ref, type StyleValue } from 'vue'

const props = withDefaults(
  defineProps<{
    visible?: boolean
    progress?: number
    rootStyle?: StyleValue
    fillStyle?: StyleValue
    thumbStyle?: StyleValue
  }>(),
  {
    visible: false,
    progress: 0,
    rootStyle: undefined,
    fillStyle: undefined,
    thumbStyle: undefined,
  },
)

const emit = defineEmits<{
  (event: 'drag-start'): void
  (event: 'drag-end'): void
  (event: 'progress-change', progress: number): void
}>()

const trackRef = ref<HTMLElement | null>(null)
let activePointerID: number | null = null

function progressFromPointer(clientY: number) {
  const track = trackRef.value
  if (!track) {
    return null
  }

  const rect = track.getBoundingClientRect()
  const nextProgress = (clientY - rect.top) / Math.max(1, rect.height)
  if (!Number.isFinite(nextProgress)) {
    return null
  }
  return Math.min(Math.max(nextProgress, 0), 1)
}

function updateProgress(clientY: number) {
  const nextProgress = progressFromPointer(clientY)
  if (nextProgress === null) {
    return
  }
  emit('progress-change', nextProgress)
}

function handlePointerDown(event: PointerEvent) {
  if (!props.visible || (event.pointerType === 'mouse' && event.button !== 0)) {
    return
  }

  event.preventDefault()
  event.stopPropagation()
  activePointerID = event.pointerId
  ;(event.currentTarget as HTMLElement | null)?.setPointerCapture?.(event.pointerId)
  emit('drag-start')
  updateProgress(event.clientY)
}

function handlePointerMove(event: PointerEvent) {
  if (activePointerID !== event.pointerId) {
    return
  }

  event.preventDefault()
  event.stopPropagation()
  updateProgress(event.clientY)
}

function finishDrag(event?: PointerEvent) {
  if (event && activePointerID !== event.pointerId) {
    return
  }

  if (event) {
    ;(event.currentTarget as HTMLElement | null)?.releasePointerCapture?.(event.pointerId)
  }
  activePointerID = null
  emit('drag-end')
}
</script>

<template>
  <div
    class="reader-detail-progress"
    role="scrollbar"
    aria-label="正文阅读进度"
    aria-orientation="vertical"
    :aria-valuenow="Math.round(props.progress * 100)"
    aria-valuemin="0"
    aria-valuemax="100"
    :style="props.rootStyle"
    @pointerdown="handlePointerDown"
    @pointermove="handlePointerMove"
    @pointerup="finishDrag"
    @pointercancel="finishDrag"
    @touchstart.stop.prevent
  >
    <div ref="trackRef" class="reader-detail-progress__track">
      <div class="reader-detail-progress__fill" :style="props.fillStyle" />
      <div class="reader-detail-progress__thumb" :style="props.thumbStyle" />
    </div>
  </div>
</template>
