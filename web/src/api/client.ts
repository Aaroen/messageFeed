import axios, { AxiosError } from 'axios'

export const apiClient = axios.create({
  baseURL: '',
  timeout: 8000,
  withCredentials: true,
})

function readErrorData(data: unknown) {
  if (!data || typeof data !== 'object') {
    return { message: '' }
  }

  const record = data as Record<string, unknown>
  return {
    message: typeof record.message === 'string' ? record.message : '',
  }
}

function friendlyErrorMessage(message: string) {
  const cleaned = message.replace(/\s*\(request_id:[^)]+\)\s*$/i, '').trim()
  const normalized = cleaned.toLowerCase()
  if (normalized === 'username and password are required') {
    return '请输入账号和密码'
  }
  if (normalized === 'username is required') {
    return '请输入账号'
  }
  if (normalized === 'password is required') {
    return '请输入密码'
  }
  if (normalized === 'invite code is required') {
    return '请输入邀请码'
  }
  if (normalized === 'invalid request body') {
    return '提交内容不完整，请检查后重试'
  }
  return cleaned
}

export function formatAPIError(error: unknown): string {
  if (error instanceof AxiosError) {
    const { message } = readErrorData(error.response?.data)
    if (message) {
      return friendlyErrorMessage(message)
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
    return friendlyErrorMessage(error.message)
  }
  if (error instanceof Error) {
    return friendlyErrorMessage(error.message)
  }
  return '未知错误'
}
