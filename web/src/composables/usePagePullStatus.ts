import { computed } from 'vue'

type ReadableRef<T> = {
  readonly value: T
}

type PagePullStatusOptions = {
  refreshing: ReadableRef<boolean>
  progress: ReadableRef<number>
  pageTitle: ReadableRef<string>
}

export function usePagePullStatus(options: PagePullStatusOptions) {
  const text = computed(() => {
    if (options.refreshing.value) {
      return '抓取中'
    }
    return options.progress.value >= 1 ? '释放刷新' : '下拉刷新'
  })

  const meta = computed(() => {
    if (options.refreshing.value) {
      return options.pageTitle.value === '订阅管理'
        ? '正在更新订阅源列表与推荐源目录'
        : `正在更新${options.pageTitle.value}`
    }
    return options.pageTitle.value === '订阅管理' ? '下拉更新订阅管理' : `下拉更新${options.pageTitle.value}`
  })

  return {
    text,
    meta,
  }
}
