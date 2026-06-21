import { computed, ref, type Ref } from 'vue'

type RefreshLayoutFreezeOptions = {
  targetRef: Ref<HTMLElement | null>
}

function cssPx(value: number) {
  return `${(Number.isFinite(value) ? value : 0).toFixed(2)}px`
}

export function useRefreshLayoutFreeze(options: RefreshLayoutFreezeOptions) {
  const frozenHeight = ref<number | null>(null)
  let freezeToken = 0
  const active = computed(() => frozenHeight.value !== null)
  const style = computed(() => (frozenHeight.value === null ? {} : { minHeight: cssPx(frozenHeight.value) }))

  function capture() {
    freezeToken += 1
    const token = freezeToken
    const height = options.targetRef.value?.getBoundingClientRect().height ?? 0
    frozenHeight.value = height > 0 ? height : null
    return () => release(token)
  }

  function release(token?: number) {
    if (token !== undefined && token !== freezeToken) {
      return
    }
    freezeToken += 1
    frozenHeight.value = null
  }

  return {
    active,
    style,
    capture,
    release,
  }
}
