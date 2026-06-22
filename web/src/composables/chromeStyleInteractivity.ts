function styleRecord(style: unknown) {
  if (!style || typeof style !== 'object' || Array.isArray(style)) {
    return null
  }
  return style as Record<string, unknown>
}

export function chromeStyleIsInteractive(style: unknown) {
  const record = styleRecord(style)
  return record?.pointerEvents === 'auto' && record.visibility !== 'hidden'
}

export function chromeStyleIsVisible(style: unknown) {
  const record = styleRecord(style)
  if (!record) {
    return true
  }

  const opacity = Number(record.opacity ?? 1)
  return record.visibility !== 'hidden' && (!Number.isFinite(opacity) || opacity > 0.01)
}
