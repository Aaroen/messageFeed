import type { AgentRun } from '@/api/agent'
import type { AgentStreamEvent, AgentStreamItem } from '@/api/assistant'

const stageLabels: Record<string, string> = {
  route: '理解请求', planning: '制定计划', generation: '生成回答', followup_answer: '回答追问',
  followup_intent: '理解追问', feedback: '处理反馈', repair: '修正输出', evidence: '核验资料',
  evaluation: '评估结果', rerank: '整理资料', memory_classification: '整理记忆',
}

export function statusLabel(status: string) {
  return ({ queued: '等待执行', pending: '等待处理', running: '正在执行', succeeded: '执行完成', completed: '执行完成', failed: '执行失败', cancelled: '已停止', canceled: '已停止', awaiting_approval: '等待确认', stopped: '已停止', not_required: '无需发送' } as Record<string, string>)[status] || status || '等待执行'
}

export function isActiveStatus(status: string) {
  return ['queued', 'pending', 'running', 'awaiting_approval', 'awaiting_confirmation'].includes(status)
}

export function runLabel(run: AgentRun) {
  const stage = String(run.task_packet.stage || '')
  if (stage) return stageLabels[stage] || stage
  for (const key of ['title', 'goal', 'task', 'description', 'capability_key']) {
    const value = run.task_packet[key]
    if (typeof value === 'string' && value.trim()) return value
  }
  return run.role === 'controller' ? '主代理' : '执行任务'
}

export function runRoleLabel(run: AgentRun) {
  if (run.role === 'controller') return '主代理'
  if (run.task_packet.stage) return '模型调用'
  if (run.task_packet.capability_key) return '工具调用'
  return '子代理'
}

export function flattenRuns(runs: AgentRun[]) {
  const byParent = new Map<number, AgentRun[]>()
  const identifiers = new Set(runs.map((run) => run.id))
  for (const run of runs) {
    const parent = identifiers.has(run.parent_run_id) ? run.parent_run_id : 0
    byParent.set(parent, [...(byParent.get(parent) || []), run])
  }
  const result: { run: AgentRun; depth: number }[] = []
  const visited = new Set<number>()
  function visit(run: AgentRun, depth: number) {
    if (visited.has(run.id)) return
    visited.add(run.id)
    result.push({ run, depth })
    for (const child of byParent.get(run.id) || []) visit(child, depth + 1)
  }
  for (const run of byParent.get(0) || []) visit(run, 0)
  for (const run of runs) if (!visited.has(run.id)) visit(run, 0)
  return result
}

export function applyStreamEvent(items: AgentStreamItem[], event: AgentStreamEvent) {
  let item = items.find((entry) => entry.run_id === event.run_id && entry.kind === event.kind && entry.item_id === event.item_id)
  if (!item) {
    item = { run_id: event.run_id, turn_id: event.turn_id, item_id: event.item_id, kind: event.kind, name: '', content: '', stage: '', phase: '', model_key: '' }
    items.push(item)
  }
  item.content = event.metadata.replace ? event.delta : item.content + event.delta
  if (event.name) item.name = event.name
  if (event.metadata.stage) item.stage = event.metadata.stage
  if (event.metadata.phase) item.phase = event.metadata.phase
  if (event.metadata.model_key) item.model_key = event.metadata.model_key
}

export function liveTurnText(items: AgentStreamItem[], turnID: number) {
  const candidates = items.filter((item) => item.turn_id === turnID && item.kind === 'text' && ['generation', 'followup_answer', 'feedback'].includes(item.stage))
  const latestRun = Math.max(0, ...candidates.map((item) => item.run_id))
  const latest = candidates.filter((item) => item.run_id === latestRun)
  const hasFinal = latest.some((item) => item.phase === 'final_answer')
  return latest.filter((item) => !hasFinal || item.phase === 'final_answer').map((item) => item.content).join('\n\n')
}

export function formatDuration(start: string, end?: string) {
  const duration = Math.max(0, Math.floor((new Date(end || Date.now()).getTime() - new Date(start).getTime()) / 1000))
  if (!Number.isFinite(duration)) return '—'
  return duration < 60 ? `${duration} 秒` : `${Math.floor(duration / 60)} 分 ${duration % 60} 秒`
}
