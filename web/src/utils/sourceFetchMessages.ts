import type { FetchSourceResult, FetchSourcesResult, FetchSourcesStatus } from '@/api/feed'

export type SourceFetchNotice = {
  type: 'success' | 'warning'
  message: string
}

type SourceFetchError = {
  source_name?: string
  message: string
}

export function formatSourceFetchErrors(errors: SourceFetchError[] = []) {
  const details = errors
    .map((item) => {
      const name = item.source_name?.trim() || '未知来源'
      const message = item.message.trim()
      return message ? `${name}：${message}` : name
    })
    .filter(Boolean)
    .slice(0, 3)
  if (!details.length) {
    return '服务未返回具体错误原因'
  }
  const overflow = errors.length > details.length ? `；另有 ${errors.length - details.length} 个失败来源` : ''
  return `${details.join('；')}${overflow}`
}

function sourceFetchFailurePrefix(result: FetchSourcesResult) {
  return result.success_count > 0 ? '刷新异常' : '刷新失败'
}

export function subscriptionFeedFetchNotice(
  result: FetchSourcesResult,
  successMessage: string,
): SourceFetchNotice {
  if (result.requested_count === 0) {
    return {
      type: 'success',
      message: '当前暂无已开启订阅源，请在订阅管理中开启或导入来源',
    }
  }

  if (result.failure_count > 0) {
    const prefix = sourceFetchFailurePrefix(result)
    return {
      type: 'warning',
      message: `${prefix}：已刷新 ${result.success_count} 个订阅源，${result.failure_count} 个失败。失败原因：${formatSourceFetchErrors(result.errors)}`,
    }
  }

  return { type: 'success', message: successMessage }
}

export function singleSourceFetchNotice(result: FetchSourceResult, sourceName = '当前来源'): SourceFetchNotice {
  if (result.created_count <= 0) {
    return { type: 'success', message: '暂无更新内容' }
  }
  return {
    type: 'success',
    message: `已更新 ${sourceName} 的 ${result.created_count} 条内容`,
  }
}

export function sourceFetchStatusNotice(
  status: FetchSourcesStatus,
): SourceFetchNotice {
  if (status.requested_count === 0) {
    return {
      type: 'success',
      message: '当前暂无已开启订阅源，请在订阅管理中开启或导入来源',
    }
  }

  if (status.failure_count > 0) {
    const prefix = status.success_count > 0 ? '刷新异常' : '刷新失败'
    const updateText =
      status.created_count > 0
        ? `已更新 ${status.updated_source_count || status.sources.length || status.success_count} 个订阅源的 ${status.created_count} 条内容，`
        : '暂无更新内容，'
    return {
      type: 'warning',
      message: `${prefix}：${updateText}${status.failure_count} 个失败。失败原因：${formatSourceFetchErrors(status.errors)}`,
    }
  }

  if (status.created_count <= 0) {
    return { type: 'success', message: '暂无更新内容' }
  }

  const sourceCount = status.updated_source_count || status.sources.length || status.success_count
  return {
    type: 'success',
    message: `已更新 ${sourceCount} 个订阅源的 ${status.created_count} 条内容`,
  }
}

export function sourceFetchContentStatusNotice(status: FetchSourcesStatus): SourceFetchNotice {
  if (status.requested_count === 0 || status.created_count <= 0) {
    if (status.failure_count > 0) {
      return {
        type: 'warning',
        message: `刷新失败：暂无更新内容，${status.failure_count} 个失败。失败原因：${formatSourceFetchErrors(status.errors)}`,
      }
    }
    return { type: 'success', message: '暂无更新内容' }
  }

  if (status.failure_count > 0) {
    const prefix = status.success_count > 0 ? '刷新异常' : '刷新失败'
    return {
      type: 'warning',
      message: `${prefix}：已更新 ${status.created_count} 条内容，${status.failure_count} 个失败。失败原因：${formatSourceFetchErrors(status.errors)}`,
    }
  }

  return {
    type: 'success',
    message: `已更新 ${status.created_count} 条内容`,
  }
}

export function subscriptionManagementFetchNotice(result: FetchSourcesResult): SourceFetchNotice {
  if (result.async) {
    if ((result.failure_count ?? 0) > 0) {
      return {
        type: 'warning',
        message: `后台刷新排队异常：${result.failure_count} 个订阅源未能排队。失败原因：${formatSourceFetchErrors(result.errors)}`,
      }
    }
    return {
      type: 'success',
      message:
        result.requested_count === 0
          ? '当前暂无已开启订阅源，请在订阅管理中开启或导入来源'
          : '后台刷新已开始',
    }
  }

  if (result.failure_count > 0) {
    const prefix = sourceFetchFailurePrefix(result)
    return {
      type: 'warning',
      message: `${prefix}：推荐源目录已更新；已抓取 ${result.success_count} 个订阅源，${result.failure_count} 个失败。失败原因：${formatSourceFetchErrors(result.errors)}`,
    }
  }

  return {
    type: 'success',
    message:
      result.requested_count === 0
        ? '暂无更新内容'
        : '',
  }
}
