import { describe, expect, it } from 'vitest'

import { agentProgressStreamURL } from '@/api/agent'
import { isTerminalAgentProgressStatus, resolveAgentProgressPollingInterval } from './agentProgress'

describe('agentProgress', () => {
  it('resolves polling interval by status', () => {
    expect(resolveAgentProgressPollingInterval('running')).toBe(3000)
    expect(resolveAgentProgressPollingInterval('executing')).toBe(3000)
    expect(resolveAgentProgressPollingInterval('awaiting_approval')).toBe(8000)
    expect(resolveAgentProgressPollingInterval('queued')).toBe(8000)
    expect(resolveAgentProgressPollingInterval('pending', true)).toBe(10000)
    expect(resolveAgentProgressPollingInterval('pending')).toBe(5000)
  })

  it('detects terminal statuses', () => {
    expect(isTerminalAgentProgressStatus('completed')).toBe(true)
    expect(isTerminalAgentProgressStatus('failed')).toBe(true)
    expect(isTerminalAgentProgressStatus('running')).toBe(false)
  })

  it('builds progress stream urls from positive identifiers', () => {
    expect(agentProgressStreamURL({ plan_id: 7 })).toBe('/api/v1/agent/progress/stream?plan_id=7')
    expect(agentProgressStreamURL({ scheduled_task_id: 9, run_id: 0 })).toBe(
      '/api/v1/agent/progress/stream?scheduled_task_id=9',
    )
    expect(agentProgressStreamURL({})).toBe('')
  })
})
