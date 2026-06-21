export const readerSettingsChangedEvent = 'messagefeed-settings-changed'

const sourceTimelinePreloadStorageKey = 'messagefeed-source-preload'

export type ReaderSettingsChangedDetail = {
  sourceTimelinePreload?: boolean
}

export function readSourceTimelinePreloadSetting() {
  return localStorage.getItem(sourceTimelinePreloadStorageKey) !== 'false'
}

export function updateSourceTimelinePreloadSetting(enabled: boolean) {
  localStorage.setItem(sourceTimelinePreloadStorageKey, enabled ? 'true' : 'false')
  window.dispatchEvent(
    new CustomEvent<ReaderSettingsChangedDetail>(readerSettingsChangedEvent, {
      detail: { sourceTimelinePreload: enabled },
    }),
  )
}

type ReaderSettingsSyncOptions = {
  setSourceTimelinePreloadEnabled: (enabled: boolean) => void
}

export function useReaderSettingsSync(options: ReaderSettingsSyncOptions) {
  function loadReaderSettings() {
    options.setSourceTimelinePreloadEnabled(readSourceTimelinePreloadSetting())
  }

  function handleReaderSettingsChanged(event: Event) {
    const detail = (event as CustomEvent<ReaderSettingsChangedDetail>).detail
    if (typeof detail?.sourceTimelinePreload === 'boolean') {
      options.setSourceTimelinePreloadEnabled(detail.sourceTimelinePreload)
      return
    }

    loadReaderSettings()
  }

  return {
    loadReaderSettings,
    handleReaderSettingsChanged,
  }
}
