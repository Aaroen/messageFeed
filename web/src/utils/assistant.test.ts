import { describe, expect, it } from 'vitest'
import type { AgentRun } from '@/api/agent'
import type { AgentStreamItem } from '@/api/assistant'
import { applyStreamEvent, flattenRuns, liveTurnText, runRoleLabel } from './assistant'

describe('助理事件与执行树', () => {
  it('区分主代理、模型阶段、工具调用和子代理', () => {
    const run = { role: 'controller', task_packet: {} } as AgentRun
    expect(runRoleLabel(run)).toBe('主代理')
    run.role = 'executor'
    run.task_packet = { stage: 'route' }
    expect(runRoleLabel(run)).toBe('模型调用')
    run.task_packet = { capability_key: 'conversation.query_history' }
    expect(runRoleLabel(run)).toBe('工具调用')
    run.task_packet = { goal: '独立收集资料' }
    expect(runRoleLabel(run)).toBe('子代理')
  })

  it('将完整条目替换增量并隔离不同运行', () => {
    const items: AgentStreamItem[] = []
    applyStreamEvent(items, { id: 1, session_id: 1, turn_id: 2, run_id: 3, kind: 'text', item_id: 'message', name: '', delta: '片段', metadata: { stage: 'generation' } })
    applyStreamEvent(items, { id: 2, session_id: 1, turn_id: 2, run_id: 3, kind: 'text', item_id: 'message', name: '', delta: '完整回答', metadata: { replace: true, phase: 'final_answer' } })
    applyStreamEvent(items, { id: 3, session_id: 1, turn_id: 4, run_id: 5, kind: 'text', item_id: 'message', name: '', delta: '其他回合', metadata: { stage: 'generation' } })
    applyStreamEvent(items, { id: 4, session_id: 1, turn_id: 2, run_id: 6, kind: 'text', item_id: 'message', name: '', delta: '{"should_capture":false}', metadata: { stage: 'repair' } })
    expect(liveTurnText(items, 2)).toBe('完整回答')
    expect(items).toHaveLength(3)
  })

  it('在执行树中保留父子关系并处理缺失父节点和循环', () => {
    const runs = [{ id: 2, parent_run_id: 1 }, { id: 1, parent_run_id: 0 }, { id: 3, parent_run_id: 99 }, { id: 4, parent_run_id: 5 }, { id: 5, parent_run_id: 4 }] as AgentRun[]
    const tree = flattenRuns(runs)
    expect(tree.map((entry) => entry.run.id)).toEqual([1, 2, 3, 4, 5])
    expect(tree[1]?.depth).toBe(1)
  })
})
