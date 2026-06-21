import { readonly, ref } from 'vue'

function normalizeScrollTop(value: number | null | undefined) {
  return typeof value === 'number' && Number.isFinite(value) ? Math.max(0, value) : 0
}

export function useFeedContentState() {
  const contentElement = ref<HTMLElement | null>(null)

  function setContentElement(element: HTMLElement | null) {
    contentElement.value = element
  }

  function currentScrollTop() {
    return contentElement.value?.scrollTop ?? 0
  }

  function scrollTo(scrollTop: number | null | undefined) {
    if (contentElement.value) {
      contentElement.value.scrollTop = normalizeScrollTop(scrollTop)
    }
  }

  function findItemElement(itemID: number | null | undefined, activePaneIndex: number) {
    if (!itemID || !contentElement.value) {
      return null
    }

    const activePane = contentElement.value.querySelectorAll('.feed-pane').item(activePaneIndex)
    return (
      activePane?.querySelector(`[data-feed-item-id="${itemID}"]`) ??
      contentElement.value.querySelector(`[data-feed-item-id="${itemID}"]`)
    )
  }

  return {
    contentElement: readonly(contentElement),
    setContentElement,
    currentScrollTop,
    scrollTo,
    findItemElement,
  }
}
