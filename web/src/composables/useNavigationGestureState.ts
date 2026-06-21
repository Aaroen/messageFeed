export function useNavigationGestureState() {
  let edgeSwipeCandidate = false
  let closeSwipeCandidate = false
  let edgeSwipe = false
  let closeSwipe = false
  let dragStarted = false

  function reset() {
    edgeSwipeCandidate = false
    closeSwipeCandidate = false
    edgeSwipe = false
    closeSwipe = false
    dragStarted = false
  }

  function setCandidates(next: { edgeSwipe?: boolean; closeSwipe?: boolean }) {
    edgeSwipeCandidate = next.edgeSwipe ?? false
    closeSwipeCandidate = next.closeSwipe ?? false
  }

  function cancelCandidates() {
    edgeSwipeCandidate = false
    closeSwipeCandidate = false
  }

  function beginEdgeSwipe() {
    edgeSwipe = true
    edgeSwipeCandidate = false
    closeSwipeCandidate = false
    dragStarted = true
  }

  function beginCloseSwipe() {
    closeSwipe = true
    closeSwipeCandidate = false
    edgeSwipeCandidate = false
    dragStarted = true
  }

  function clearActiveSwipes() {
    edgeSwipe = false
    closeSwipe = false
  }

  function hasCandidate() {
    return edgeSwipeCandidate || closeSwipeCandidate
  }

  function hasActiveSwipe() {
    return edgeSwipe || closeSwipe
  }

  return {
    reset,
    setCandidates,
    cancelCandidates,
    beginEdgeSwipe,
    beginCloseSwipe,
    clearActiveSwipes,
    edgeSwipeCandidate: () => edgeSwipeCandidate,
    closeSwipeCandidate: () => closeSwipeCandidate,
    edgeSwipe: () => edgeSwipe,
    closeSwipe: () => closeSwipe,
    dragStarted: () => dragStarted,
    hasCandidate,
    hasActiveSwipe,
  }
}
