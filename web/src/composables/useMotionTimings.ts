type NoticeType = 'success' | 'warning'

export function useMotionTimings() {
  const quickDuration = 180
  const shortDuration = 220
  const normalDuration = 260
  const stretchAnchorClearDuration = 280
  const headerSwapDuration = 320
  const readerDuration = 360
  const chromeDuration = 1000
  const noticeSuccessDuration = 1000
  const noticeFailureDuration = 3000
  const noticeRevealDelay = quickDuration
  const topRefreshReleaseDelay = 120
  const topRefreshSettleDuration = chromeDuration
  const navigationDrawerSettleDuration = shortDuration
  const hiddenSourceCleanupDelay = quickDuration
  const sourceReaderCloseCleanupDelay = 340
  const detailHeaderSwapDelay = headerSwapDuration
  const morphingItemContentUnlockDelay = quickDuration
  const readerMotionSettleDelay = normalDuration
  const motionCleanupBuffer = 96
  const detailFrameMetricsInitialDelay = quickDuration
  const detailFrameMetricsSettledDelay = 520
  const viewSwipeChromeRevealDelay = detailFrameMetricsSettledDelay
  const readerScrollRestoreRetryDelay = 120
  const readerScrollRestoreSettledDelay = detailFrameMetricsSettledDelay
  const readerMorphDuration = readerDuration
  const readerMorphCleanupBuffer = motionCleanupBuffer
  const readerMorphCleanupDelay = readerMorphDuration + readerMorphCleanupBuffer
  const readerRectRetryDelay = 64

  function delay(duration = readerMorphDuration) {
    return duration === readerMorphDuration ? readerMorphCleanupDelay : duration + readerMorphCleanupBuffer
  }

  function noticeDuration(type: NoticeType) {
    return type === 'success' ? noticeSuccessDuration : noticeFailureDuration
  }

  return {
    quickDuration,
    shortDuration,
    normalDuration,
    stretchAnchorClearDuration,
    headerSwapDuration,
    readerDuration,
    chromeDuration,
    noticeSuccessDuration,
    noticeFailureDuration,
    noticeRevealDelay,
    topRefreshReleaseDelay,
    topRefreshSettleDuration,
    navigationDrawerSettleDuration,
    hiddenSourceCleanupDelay,
    sourceReaderCloseCleanupDelay,
    detailHeaderSwapDelay,
    morphingItemContentUnlockDelay,
    readerMotionSettleDelay,
    motionCleanupBuffer,
    detailFrameMetricsInitialDelay,
    detailFrameMetricsSettledDelay,
    viewSwipeChromeRevealDelay,
    readerScrollRestoreRetryDelay,
    readerScrollRestoreSettledDelay,
    readerMorphDuration,
    readerRectRetryDelay,
    noticeDuration,
    delay,
  }
}
