import { describe, expect, it } from 'vitest'
import type { AgentRun } from '@/api/agent'
import type { AgentStreamItem } from '@/api/assistant'
import { applyStreamEvent, assistantErrorMessage, flattenRuns, liveTurnText, readableActivityText, runLabel, runRoleLabel, subagentRuns } from './assistant'

describe('助理事件与执行树', () => {
  it('以用户可读提示展示模型繁忙和核验失败', () => {
    expect(assistantErrorMessage('最终回答质量校验未通过。: 模型流式调用失败：{"response":{"error":{"message":"This model is currently experiencing high demand."}}}')).toBe('所选模型当前繁忙，本次回答未能完成，请稍后再次发送。')
    expect(assistantErrorMessage('最终回答质量校验未通过。: evaluation citation is absent from complete source body')).toBe('回答未能通过资料或格式核验，请重新发送或补充要求。')
    expect(assistantErrorMessage('当前 IP 今日已达到 20 次游客请求额度')).toBe('当前 IP 今日已达到 20 次游客请求额度')
  })

  it('侧栏只包含独立子代理及其后续活动', () => {
    const runs = [
      { id: 1, parent_run_id: 0, role: 'controller', task_packet: {} },
      { id: 2, parent_run_id: 1, role: 'executor', task_packet: { stage: 'route' } },
      { id: 3, parent_run_id: 1, role: 'executor', task_packet: { capability_key: 'web.search' } },
      { id: 4, parent_run_id: 1, role: 'executor', task_packet: { goal: '独立收集资料' } },
      { id: 5, parent_run_id: 6, role: 'executor', task_packet: { stage: 'generation' } },
      { id: 6, parent_run_id: 4, role: 'executor', task_packet: { capability_key: 'web.search' } },
    ] as AgentRun[]
    expect(subagentRuns(runs).map((run) => run.id)).toEqual([4, 5, 6])
    expect(subagentRuns(runs.slice(0, 3))).toEqual([])
    expect(runLabel(runs[2]!)).toBe('搜索网页')
    expect(readableActivityText('{"task_packet":{"stage":"route"}}')).toBe('')
    expect(readableActivityText('```json\n{"route":"direct"}\n```')).toBe('')
    expect(readableActivityText('已经读取相关资讯。')).toBe('已经读取相关资讯。')
    expect(readableActivityText('资讯标题\nEvidence ref：item:123\n发布时间：2026-11-17')).toBe('资讯标题\n发布时间：2026-11-17')
  })

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
