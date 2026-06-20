import axios, { AxiosError } from 'axios'

export const apiClient = axios.create({
  baseURL: '',
  timeout: 8000,
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
    return error.message
  }
  if (error instanceof Error) {
    return error.message
  }
  return '未知错误'
}
