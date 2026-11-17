import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { createAgentSession, createAgentTask, listAgentSessions, type AgentSession } from '@/api/agent'
import { getAgentWorkspace, type AgentStreamEvent, type AgentWorkspace } from '@/api/assistant'
import { formatAPIError } from '@/api/client'
import { applyStreamEvent, isActiveStatus } from '@/utils/assistant'
import { listAssistantModels, modelSelectionKey, type AgentModel } from '@/api/agentModels'

export const useAssistantStore = defineStore('assistant', () => {
  const sessions = ref<AgentSession[]>([])
  const sessionID = ref(0)
  const workspaces = ref<Record<number, AgentWorkspace>>({})
  const loading = ref(false)
  const sending = ref(false)
  const error = ref('')
  const connection = ref<'idle' | 'connecting' | 'connected' | 'reconnecting' | 'failed'>('idle')
  const pendingMessage = ref('')
  const models = ref<AgentModel[]>([])
  const selectedModelKey = ref(localStorage.getItem('mf-assistant-selected-model') || '')
  const selectedModel = computed(() => models.value.find((model) => modelSelectionKey(model) === selectedModelKey.value))
  const workspace = computed(() => workspaces.value[sessionID.value] || null)
  const active = computed(() => workspace.value?.turns.some((turn) => isActiveStatus(turn.status)) || false)
  let source: EventSource | null = null
  let selectionVersion = 0

  function disconnect() {
    source?.close()
    source = null
    connection.value = 'idle'
  }

  async function loadSessions() {
    sessions.value = (await listAgentSessions()).sessions
  }

  async function loadModels() {
    models.value = await listAssistantModels()
    const preferred = selectedModelKey.value || localStorage.getItem('mf-assistant-selected-model') || ''
    if (models.value.some((model) => modelSelectionKey(model) === preferred)) selectedModelKey.value = preferred
    else if (preferred) selectedModelKey.value = ''
    else selectedModelKey.value = models.value[0] ? modelSelectionKey(models.value[0]) : ''
  }

  function applySnapshot(snapshot: AgentWorkspace) {
    const previous = workspaces.value[snapshot.session.id]
    const earlierTurns = previous?.turns.filter((turn) => turn.id < (snapshot.turns[0]?.id || 0)) || []
    if (earlierTurns.length) {
      snapshot.turns = [...earlierTurns, ...snapshot.turns]
      const turnIDs = new Set(earlierTurns.map((turn) => turn.id))
      snapshot.runs = [...previous!.runs.filter((run) => turnIDs.has(run.turn_id)), ...snapshot.runs]
      snapshot.plans = [...previous!.plans.filter((plan) => turnIDs.has(plan.turn_id)), ...snapshot.plans]
      snapshot.items = [...previous!.items.filter((item) => turnIDs.has(item.turn_id)), ...snapshot.items]
      snapshot.has_more = previous!.has_more
    }
    workspaces.value[snapshot.session.id] = snapshot
    const index = sessions.value.findIndex((session) => session.id === snapshot.session.id)
    if (index >= 0) sessions.value[index] = snapshot.session
    else sessions.value.unshift(snapshot.session)
  }

  function connect(id: number) {
    disconnect()
    connection.value = 'connecting'
    const cursor = workspaces.value[id]?.cursor || 0
    const stream = new EventSource(`/api/v1/agent/sessions/${id}/events?after_id=${cursor}`, { withCredentials: true })
    source = stream
    stream.onopen = () => { if (source === stream) connection.value = 'connected' }
    stream.onerror = () => {
      if (source !== stream) return
      connection.value = stream.readyState === EventSource.CLOSED ? 'failed' : 'reconnecting'
    }
    stream.addEventListener('snapshot', (event) => {
      if (source !== stream) return
      applySnapshot(JSON.parse((event as MessageEvent).data) as AgentWorkspace)
    })
    stream.addEventListener('state', (event) => {
      if (source !== stream) return
      const update = JSON.parse((event as MessageEvent).data) as AgentWorkspace
      const current = workspaces.value[id]
      if (!current) return
      const visibleTurns = new Set(update.turns.map((turn) => turn.id))
      update.items = current.items.filter((item) => visibleTurns.has(item.turn_id))
      update.cursor = current.cursor
      applySnapshot(update)
    })
    stream.addEventListener('delta', (event) => {
      if (source !== stream) return
      const delta = JSON.parse((event as MessageEvent).data) as AgentStreamEvent
      const current = workspaces.value[id]
      if (!current || delta.session_id !== id || delta.id <= current.cursor) return
      applyStreamEvent(current.items, delta)
      current.cursor = delta.id
    })
    stream.addEventListener('stream_error', (event) => {
      if (source !== stream) return
      error.value = (JSON.parse((event as MessageEvent).data) as { message: string }).message
      stream.close()
      connection.value = 'failed'
    })
  }

  async function selectSession(id: number) {
    const version = ++selectionVersion
    disconnect()
    sessionID.value = id
    error.value = ''
    if (!id) { loading.value = false; return }
    loading.value = true
    try {
      const snapshot = await getAgentWorkspace(id)
      if (version !== selectionVersion) return
      applySnapshot(snapshot)
      connect(id)
    } catch (cause) {
      if (version === selectionVersion) error.value = formatAPIError(cause)
    } finally {
      if (version === selectionVersion) loading.value = false
    }
  }

  async function send(message: string) {
    if (sending.value || !message.trim()) return false
    const model = selectedModel.value
    if (!model) { error.value = '请先在设置中启用并选择模型'; return false }
    localStorage.setItem('mf-assistant-selected-model', selectedModelKey.value)
    sending.value = true
    error.value = ''
    let targetID = sessionID.value
    let accepted = false
    try {
      if (!targetID) {
        const session = await createAgentSession({ external_account_id: 0, title: message.trim().slice(0, 50) })
        targetID = session.id
        await selectSession(targetID)
        if (!workspace.value) throw new Error(error.value || '会话加载失败')
      }
      pendingMessage.value = message
      await createAgentTask({ message, session_id: targetID, channel: 'assistant', model_config_id: model.config_id, model: model.model })
      accepted = true
      applySnapshot(await getAgentWorkspace(targetID))
      return true
    } catch (cause) {
      error.value = formatAPIError(cause)
      return accepted
    } finally {
      pendingMessage.value = ''
      sending.value = false
    }
  }

  async function loadOlder() {
    const current = workspace.value
    if (!current?.has_more || !current.turns.length) return
    const snapshot = await getAgentWorkspace(current.session.id, current.turns[0]!.id)
    const latest = workspaces.value[current.session.id]
    if (!latest) return
    const identifiers = new Set(latest.turns.map((turn) => turn.id))
    const added = snapshot.turns.filter((turn) => !identifiers.has(turn.id))
    const addedIDs = new Set(added.map((turn) => turn.id))
    latest.turns = [...added, ...latest.turns]
    latest.runs = [...snapshot.runs.filter((run) => addedIDs.has(run.turn_id)), ...latest.runs]
    latest.plans = [...snapshot.plans.filter((plan) => addedIDs.has(plan.turn_id)), ...latest.plans]
    latest.items = [...snapshot.items.filter((item) => addedIDs.has(item.turn_id)), ...latest.items]
    latest.has_more = snapshot.has_more
  }

  function reset() {
    ++selectionVersion
    disconnect()
    sessions.value = []
    workspaces.value = {}
    sessionID.value = 0
    models.value = []
  }

  return { sessions, sessionID, workspace, loading, sending, error, connection, active, pendingMessage, models, selectedModelKey, selectedModel, loadModels, loadSessions, selectSession, send, loadOlder, disconnect, reset }
})
