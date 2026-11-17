import type { AgentRun } from '@/api/agent'
import type { AgentStreamEvent, AgentStreamItem } from '@/api/assistant'

const stageLabels: Record<string, string> = {
  route: '理解请求', planning: '制定计划', generation: '生成回答', followup_answer: '回答追问',
  followup_intent: '理解追问', feedback: '处理反馈', repair: '修正输出', evidence: '核验资料',
  evaluation: '评估结果', rerank: '整理资料', memory_classification: '整理记忆',
}

export function statusLabel(status: string) {
  return ({ queued: '等待执行', pending: '等待处理', running: '正在执行', succeeded: '执行完成', completed: '执行完成', failed: '执行失败', cancelled: '已停止', canceled: '已停止', awaiting_approval: '等待确认', awaiting_confirmation: '等待确认', stopped: '已停止', not_required: '无需发送', empty: '没有匹配资料', skipped: '未执行', approved: '已确认', rejected: '未通过', timed_out: '执行超时' } as Record<string, string>)[status] || '状态更新中'
}

export function isActiveStatus(status: string) {
  return ['queued', 'pending', 'running', 'awaiting_approval', 'awaiting_confirmation'].includes(status)
}

export function runLabel(run: AgentRun) {
  const stage = String(run.task_packet.stage || '')
  if (stage) return stageLabels[stage] || '处理任务'
  if (run.task_packet.capability_key) return capabilityLabel(String(run.task_packet.capability_key))
  for (const key of ['title', 'goal', 'task', 'description']) {
    const value = run.task_packet[key]
    if (typeof value === 'string' && value.trim()) return value
  }
  return run.role === 'controller' ? '主代理' : '执行任务'
}

export function capabilityLabel(key: string) {
  return ({ 'feed.query_recent_items': '读取最近资讯', 'source.query_latest_items': '读取订阅资讯', 'conversation.query_history': '查询对话记录', 'memory.fact_recall': '查找相关记忆', 'agent.schedule_message': '安排消息提醒', 'agent.schedule_task': '安排任务', 'web.search': '搜索网页', 'web.fetch_page': '读取网页', 'web.extract_page': '提取网页正文', 'repo.search': '搜索项目内容', 'repo.inspect_remote': '查看项目资料', 'content.summarize_text': '整理资料摘要', 'agent.write_transcript': '保存对话', 'agent.write_audit': '保存执行记录' } as Record<string, string>)[key] || '处理资料'
}

export function subagentRuns(runs: AgentRun[]) {
  const identifiers = new Set(runs.filter((run) => run.role === 'executor' && !run.task_packet.stage && !run.task_packet.capability_key).map((run) => run.id))
  let previousSize = -1
  while (identifiers.size !== previousSize) {
    previousSize = identifiers.size
    for (const run of runs) if (identifiers.has(run.parent_run_id)) identifiers.add(run.id)
  }
  return runs.filter((run) => identifiers.has(run.id))
}

export function readableActivityText(content: string) {
  const trimmed = content.trim()
  if (!trimmed || /^(?:[{[]|```\s*json\b)/i.test(trimmed)) return ''
  return content.split('\n').filter((line) => !/(?:Evidence ref|task_packet|canonical_ref|content_ref|parent_run_id|artifact_refs|capability_key|tool_arguments)\s*[:：]/i.test(line)).join('\n').trim()
}

export function assistantErrorMessage(message: string) {
  if (/high demand|server_overloaded|模型.*繁忙/i.test(message)) return '所选模型当前繁忙，本次回答未能完成，请稍后再次发送。'
  if (message.includes('最终回答质量校验未通过')) return '回答未能通过资料或格式核验，请重新发送或补充要求。'
  if (/模型流式调用失败|\{"(?:error|response)"|context deadline exceeded/i.test(message)) return '模型服务暂时无法完成回答，请稍后再次发送。'
  return message
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
