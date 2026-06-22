import { readonly, ref } from 'vue'

export type PageRefreshOptions = {
  noticeDelayMS?: number
  suppressStartNotice?: boolean
  onRefreshSettled?: (callback: () => void) => void
}

export type PageViewExpose = {
  refreshPage?: (options?: PageRefreshOptions) => Promise<void> | void
  clearNotice?: () => void
}

export function usePageOutletState() {
  const contentElement = ref<HTMLElement | null>(null)
  const viewInstance = ref<PageViewExpose | null>(null)

  function setContentElement(element: HTMLElement | null) {
    contentElement.value = element
  }

  function setViewInstance(view: PageViewExpose | null) {
    viewInstance.value = view
  }

  function currentScrollTop() {
    return contentElement.value?.scrollTop ?? 0
  }

  function currentRefreshPage() {
    return viewInstance.value?.refreshPage ?? null
  }

  function currentClearNotice() {
    return viewInstance.value?.clearNotice ?? null
  }

  function clearCurrentNotice() {
    currentClearNotice()?.()
  }

  function hasRefreshPage() {
    return currentRefreshPage() !== null
  }

  return {
    contentElement: readonly(contentElement),
    viewInstance: readonly(viewInstance),
    setContentElement,
    setViewInstance,
    currentScrollTop,
    currentRefreshPage,
    currentClearNotice,
    clearCurrentNotice,
    hasRefreshPage,
  }
}
