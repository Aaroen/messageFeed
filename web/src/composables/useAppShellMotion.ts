import { computed, type Ref } from 'vue'

type AppShellMotionOptions = {
  feedHeaderHeight: Ref<number>
  feedContentSpace: Ref<number>
  detailSurfaceProgress: Ref<number>
}

function cssNumber(value: number, precision = 2) {
  return (Number.isFinite(value) ? value : 0).toFixed(precision)
}

function cssPx(value: number) {
  return `${cssNumber(value)}px`
}

export function useAppShellMotion(options: AppShellMotionOptions) {
  const style = computed(() => ({
    '--feed-header-height': `${options.feedHeaderHeight.value}px`,
    '--feed-header-space': cssPx(options.feedContentSpace.value),
    '--detail-underlay-blur': `${(options.detailSurfaceProgress.value * 7).toFixed(2)}px`,
    '--detail-underlay-opacity': (1 - options.detailSurfaceProgress.value * 0.08).toFixed(3),
  }))

  return {
    style,
  }
}
