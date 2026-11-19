import { onBeforeUnmount, ref, type Ref } from 'vue'

export function useAgentScroll(element: Ref<HTMLElement | null>) {
  const followBottom = ref(true)
  let frame = 0

  function handleScroll() {
    const area = element.value
    if (area) followBottom.value = area.scrollHeight - area.scrollTop - area.clientHeight < 80
  }

  function scrollBottom(force = false) {
    if (force) followBottom.value = true
    if (!followBottom.value || frame) return
    frame = requestAnimationFrame(() => {
      frame = 0
      const area = element.value
      if (area && followBottom.value) area.scrollTop = area.scrollHeight
    })
  }

  onBeforeUnmount(() => cancelAnimationFrame(frame))
  return { followBottom, handleScroll, scrollBottom }
}
