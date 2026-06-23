type NoticeType = 'running' | 'success' | 'warning'

export function useMotionTimings() {
  const quickDuration = 180
  const shortDuration = 220
  const normalDuration = 260
  const stretchAnchorClearDuration = 280
  const headerSwapDuration = 320
  const readerDuration = 360
  const slowDuration = 420
  const chromeDuration = 1000
  const refreshCompletionDuration = headerSwapDuration
  const noticeSuccessDuration = 1000
  const noticeFailureDuration = 3000
  const homeExitDoubleBackTimeout = 1600
  const topRefreshReleaseDelay = 120
  const topRefreshSettleDuration = refreshCompletionDuration
  const topChromeSettleDuration = chromeDuration
  const clickSuppressionDuration = slowDuration
  const motionCleanupBuffer = 96
  const topRefreshNoticeDelay = topRefreshReleaseDelay + topRefreshSettleDuration + motionCleanupBuffer
  const noticeRevealDelay = topRefreshNoticeDelay
  const navigationDrawerSettleDuration = shortDuration
  const hiddenSourceCleanupDelay = quickDuration
  const sourceReaderCloseCleanupDelay = 340
  const detailHeaderSwapDelay = headerSwapDuration
  const morphingItemContentUnlockDelay = quickDuration
  const readerMotionSettleDelay = normalDuration
  const detailFrameMetricsInitialDelay = quickDuration
  const detailFrameMetricsSettledDelay = 520
  const viewSwipeChromeRevealDelay = detailFrameMetricsSettledDelay
  const readerScrollRestoreRetryDelay = 120
  const readerScrollRestoreSettledDelay = detailFrameMetricsSettledDelay
  const readerMorphDuration = readerDuration
  const readerMorphCleanupBuffer = motionCleanupBuffer
  const readerMorphCleanupDelay = readerMorphDuration + readerMorphCleanupBuffer
  const readerRectRetryDelay = 64
  const readerSessionSaveDelay = 80

  function delay(duration = readerMorphDuration) {
    return duration === readerMorphDuration ? readerMorphCleanupDelay : duration + readerMorphCleanupBuffer
  }

  function noticeDuration(type: NoticeType) {
    if (type === 'running') {
      return 0
    }
    return type === 'success' ? noticeSuccessDuration : noticeFailureDuration
  }

  return {
    quickDuration,
    shortDuration,
    normalDuration,
    stretchAnchorClearDuration,
    headerSwapDuration,
    readerDuration,
    slowDuration,
    chromeDuration,
    refreshCompletionDuration,
    noticeSuccessDuration,
    noticeFailureDuration,
    homeExitDoubleBackTimeout,
    noticeRevealDelay,
    topRefreshReleaseDelay,
    topRefreshNoticeDelay,
    topRefreshSettleDuration,
    topChromeSettleDuration,
    clickSuppressionDuration,
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
    readerSessionSaveDelay,
    noticeDuration,
    delay,
  }
}
