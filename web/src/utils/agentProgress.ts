export function resolveAgentProgressPollingInterval(status: string, hasRefreshNotice = false) {
  if (status === 'running' || status === 'executing' || status === 'approved') {
    return 3000
  }
  if (status === 'awaiting_approval' || status === 'input_required' || status === 'queued') {
    return 8000
  }
  if (hasRefreshNotice) {
    return 10000
  }
  return 5000
}

export function isTerminalAgentProgressStatus(status: string) {
  return status === 'completed' || status === 'failed' || status === 'rejected' || status === 'expired'
    || status === 'succeeded' || status === 'canceled'
}
