export function useMotionTimings() {
  const quickDuration = 180
  const normalDuration = 260
  const stretchAnchorClearDuration = 280
  const headerSwapDuration = 320
  const readerDuration = 360
  const chromeDuration = 1000
  const detailFrameMetricsInitialDelay = quickDuration
  const detailFrameMetricsSettledDelay = 520
  const readerScrollRestoreRetryDelay = 120
  const readerScrollRestoreSettledDelay = detailFrameMetricsSettledDelay
  const readerMorphDuration = readerDuration
  const readerMorphCleanupBuffer = 96
  const readerMorphCleanupDelay = readerMorphDuration + readerMorphCleanupBuffer
  const readerRectRetryDelay = 64

  function delay(duration = readerMorphDuration) {
    return duration === readerMorphDuration ? readerMorphCleanupDelay : duration + readerMorphCleanupBuffer
  }

  return {
    quickDuration,
    normalDuration,
    stretchAnchorClearDuration,
    headerSwapDuration,
    readerDuration,
    chromeDuration,
    detailFrameMetricsInitialDelay,
    detailFrameMetricsSettledDelay,
    readerScrollRestoreRetryDelay,
    readerScrollRestoreSettledDelay,
    readerMorphDuration,
    readerRectRetryDelay,
    delay,
  }
}
