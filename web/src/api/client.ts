import axios, { AxiosError } from 'axios'

export const apiClient = axios.create({
  baseURL: '',
  timeout: 8000,
  withCredentials: true,
})

function readErrorData(data: unknown) {
  if (!data || typeof data !== 'object') {
    return { message: '', requestID: '' }
  }

  const record = data as Record<string, unknown>
  return {
    message: typeof record.message === 'string' ? record.message : '',
    requestID: typeof record.request_id === 'string' ? record.request_id : '',
  }
}

export function formatAPIError(error: unknown): string {
  if (error instanceof AxiosError) {
    const { message, requestID } = readErrorData(error.response?.data)
    if (message && requestID) {
      return `${message} (request_id: ${requestID})`
    }
    if (message) {
      return message
    }
    if (error.code === 'ECONNABORTED') {
      return '请求超时'
    }
    if (error.code === 'ERR_NETWORK' || error.message === 'Network Error') {
      return '网络请求失败，请检查网络连接或服务可用性'
    }
    if (error.code === 'ERR_CANCELED') {
      return '请求已取消'
    }
    if (error.response?.status === 404) {
      return '请求路径不存在或服务未更新，请刷新页面并确认后端已重启'
    }
    if (error.response?.status) {
      return `请求失败（HTTP ${error.response.status}）`
    }
    return error.message
  }
  if (error instanceof Error) {
    return error.message
  }
  return '未知错误'
}
