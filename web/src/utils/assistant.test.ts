import { describe, expect, it } from 'vitest'
import { createApp, h, nextTick, reactive } from 'vue'
import type { AgentActivity, AgentStreamItem } from '@/api/assistant'
import AgentComposer from '@/components/agent/AgentComposer.vue'
import AgentReasoningStream from '@/components/agent/AgentReasoningStream.vue'
import AgentTurnActivity from '@/components/agent/AgentTurnActivity.vue'
import { applyStreamEvent, assistantErrorMessage, flattenRuns, liveTurnText, readableActivityText, runLabel, runRoleLabel, subagentRuns } from './assistant'

describe('助理事件与执行树', () => {
  it('以用户可读提示展示模型繁忙和核验失败', () => {
    expect(assistantErrorMessage('最终回答质量校验未通过。: 模型流式调用失败：{"response":{"error":{"message":"This model is currently experiencing high demand."}}}')).toBe('所选模型当前繁忙，本次回答未能完成，请稍后再次发送。')
    expect(assistantErrorMessage('最终回答质量校验未通过。: evaluation citation is absent from complete source body')).toBe('回答未能通过资料或格式核验，请重新发送或补充要求。')
    expect(assistantErrorMessage('当前 IP 今日已达到 20 次游客请求额度')).toBe('当前 IP 今日已达到 20 次游客请求额度')
  })

  it('侧栏只包含子代理任务', () => {
    const runs = [
      { id: 1, parent_run_id: 0, kind: 'controller' },
      { id: 2, parent_run_id: 1, kind: 'model', stage: 'route' },
      { id: 3, parent_run_id: 1, kind: 'tool', capability: 'web.search' },
      { id: 4, parent_run_id: 1, kind: 'subagent', title: '独立收集资料' },
      { id: 5, parent_run_id: 6, kind: 'model', stage: 'generation' },
      { id: 6, parent_run_id: 4, kind: 'tool', capability: 'web.search' },
    ] as AgentActivity[]
    expect(subagentRuns(runs).map((run) => run.id)).toEqual([4])
    expect(subagentRuns(runs.slice(0, 3))).toEqual([])
    expect(runLabel(runs[2]!)).toBe('搜索网页')
    expect(readableActivityText('{"task_packet":{"stage":"route"}}')).toBe('')
    expect(readableActivityText('```json\n{"route":"direct"}\n```')).toBe('')
    expect(readableActivityText('已经读取相关资讯。')).toBe('已经读取相关资讯。')
    expect(readableActivityText('资讯标题\nEvidence ref：item:123\n发布时间：2026-11-17')).toBe('资讯标题\n发布时间：2026-11-17')
  })

  it('区分主代理、模型阶段、工具调用和子代理', () => {
    const run = { kind: 'controller' } as AgentActivity
    expect(runRoleLabel(run)).toBe('主代理')
    run.kind = 'model'
    expect(runRoleLabel(run)).toBe('模型调用')
    run.kind = 'tool'
    expect(runRoleLabel(run)).toBe('工具调用')
    run.kind = 'subagent'
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
    const runs = [{ id: 2, parent_run_id: 1 }, { id: 1, parent_run_id: 0 }, { id: 3, parent_run_id: 99 }, { id: 4, parent_run_id: 5 }, { id: 5, parent_run_id: 4 }] as AgentActivity[]
    const tree = flattenRuns(runs)
    expect(tree.map((entry) => entry.run.id)).toEqual([1, 2, 3, 4, 5])
    expect(tree[1]?.depth).toBe(1)
  })

  it('长思考收起后持续显示最新片段并保留手动展开状态', async () => {
    const props = reactive({ content: '正在核验资料。', active: true })
    const element = document.createElement('div')
    const app = createApp({ render: () => h(AgentReasoningStream, props) })
    app.mount(element)
    try {
      expect(element.querySelector('button')?.getAttribute('aria-expanded')).toBe('true')
      props.content = '资料核验过程。'.repeat(120) + '最新思考片段。'
      await nextTick()
      expect(element.querySelector('button')?.getAttribute('aria-expanded')).toBe('false')
      expect(element.querySelector('.reasoning-preview')?.textContent).toContain('最新思考片段。')
      props.content += '继续读取正文。'
      await nextTick()
      expect(element.querySelector('.reasoning-preview')?.textContent).toContain('继续读取正文。')
      element.querySelector('button')!.click()
      await nextTick()
      expect(element.querySelector('.reasoning-content')?.textContent).toContain('资料核验过程。'.repeat(120))
      props.active = false
      await nextTick()
      expect(element.querySelector('button')?.getAttribute('aria-expanded')).toBe('true')
    } finally { app.unmount() }
  })

  it('执行详情收起时仍然显示规划阶段的模型思考', async () => {
    const runs: AgentActivity[] = Array.from({ length: 7 }, (_, index) => ({
      id: index + 1, parent_run_id: 0, turn_id: 1, kind: 'model', status: 'succeeded', stage: 'planning',
      started_at: '2026-11-17T00:00:00Z', updated_at: '2026-11-17T00:00:00Z',
    }))
    const items = [{ run_id: 7, turn_id: 1, item_id: 'thinking', kind: 'reasoning', content: '正在制定检索计划。', stage: 'planning', name: '', phase: '', model_key: '' }] as AgentStreamItem[]
    const element = document.createElement('div')
    const app = createApp({ render: () => h(AgentTurnActivity, { runs, items, status: 'running' }) })
    app.mount(element)
    try {
      await nextTick()
      expect(element.querySelector('.turn-activity-toggle')?.getAttribute('aria-expanded')).toBe('false')
      expect(element.querySelector('[aria-label="模型思考"]')?.textContent).toContain('正在制定检索计划。')
      expect(element.querySelector('.turn-activity-body')).toBeNull()
    } finally { app.unmount() }
  })

  it('确认输入法候选词时保留草稿，支持键盘调节输入高度', async () => {
    const element = document.createElement('div')
    let submissions = 0
    const app = createApp({ render: () => h(AgentComposer, { label: '发送消息', modelValue: '草稿', onSubmit: () => { submissions += 1 } }) })
    app.mount(element)
    try {
      const input = element.querySelector('textarea')!
      input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', isComposing: true, bubbles: true }))
      input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', keyCode: 229, bubbles: true }))
      expect(submissions).toBe(0)
      expect(input.value).toBe('草稿')
      const handle = element.querySelector('[role="separator"]')!
      handle.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowUp', bubbles: true }))
      await nextTick()
      expect(handle.getAttribute('aria-valuenow')).toBe('64')
      handle.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }))
      await nextTick()
      expect(handle.getAttribute('aria-valuenow')).toBe('48')
    } finally { app.unmount() }
  })
})
