import { readonly, ref } from 'vue'

function normalizeScrollTop(value: number | null | undefined) {
  return typeof value === 'number' && Number.isFinite(value) ? Math.max(0, value) : 0
}

export function useFeedScrollState() {
  const scrollTop = ref(0)

  function update(nextScrollTop: number | null | undefined) {
    scrollTop.value = normalizeScrollTop(nextScrollTop)
  }

  function restore(savedScrollTop: number | null | undefined) {
    update(savedScrollTop)
  }

  return {
    scrollTop: readonly(scrollTop),
    update,
    restore,
  }
}
