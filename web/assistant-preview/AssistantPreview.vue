<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  IconApps, IconArrowDown, IconArrowLeft, IconArrowUp, IconBook, IconBranch, IconBulb, IconCheck,
  IconClose, IconCompass, IconCopy, IconFile, IconList, IconMenuUnfold,
  IconMoonFill, IconRight, IconStop, IconSunFill,
} from '@arco-design/web-vue/es/icon'
import AgentComposer from '../src/components/agent/AgentComposer.vue'
import AgentMarkdown from '../src/components/agent/AgentMarkdown.vue'
import AgentWorkspaceShell from '../src/components/agent/AgentWorkspaceShell.vue'
import AgentSubagentList from '../src/components/agent/AgentSubagentList.vue'
import AgentEffortSlider from '../src/components/agent/AgentEffortSlider.vue'
import AgentModelPicker from '../src/components/agent/AgentModelPicker.vue'
import { useThemeState } from '../src/composables/useThemeState'
import PreviewFeed from './PreviewFeed.vue'
import { agents, examples, feeds, models, stages, type FeedArticle, type FeedPage } from './examples'

type Turn = {
  prompt: string
  model: string
  reasoning: string
  reply: string
  status: 'reasoning' | 'replying' | 'complete' | 'stopped'
  summaryOpen: boolean | null
}
type Session = { id: number; title: string; draft: string; turns: Turn[] }
type PreviewPage = FeedPage | 'agent' | 'agent/fullscreen'

const navigation = [
  { page: 'subscriptions', title: '订阅', icon: IconBook },
  { page: 'recommendations', title: '推荐', icon: IconCompass },
  { page: 'agent', title: '助理', icon: IconApps },
] as const

function readRoute(): { page: PreviewPage; article?: FeedArticle } {
  const [section, view, index] = window.location.hash.slice(2).split('/')
  if (section === 'subscriptions' || section === 'recommendations') {
    return { page: section, article: view === 'article' ? feeds[section][Number(index) - 1] : undefined }
  }
  return { page: section === 'agent' && view === 'fullscreen' ? 'agent/fullscreen' : 'agent' }
}

const route = ref(readRoute())
const page = computed(() => route.value.page)
const article = computed(() => route.value.article)
const fullscreen = computed(() => page.value === 'agent/fullscreen')
const assistantVisible = computed(() => page.value === 'agent' || fullscreen.value)
const topNavigationVisible = ref(true)
const { dark: darkTheme, load: loadTheme, toggle: toggleTheme } = useThemeState()
const themeAction = computed(() => darkTheme.value ? '切换到日间模式' : '切换到深色模式')
loadTheme()

function syncPage() {
  const next = readRoute()
  if (next.page !== page.value || next.article !== article.value) {
    topNavigationVisible.value = true
    if (siteDialog.value?.open) siteDialog.value.close()
  }
  if (next.page === 'subscriptions' || next.page === 'recommendations') {
    shell.value?.closePanels()
    modelPicker.value?.dismiss()
    effortSlider.value?.dismiss()
  }
  route.value = next
  document.title = `${next.article?.title ?? navigation.find(item => item.page === next.page)?.title ?? '助理工作区'} · MessageFeed 预览`
}

function toggleFullscreen() {
  if (!fullscreen.value) window.history.pushState({ fromPreviewAssistant: true }, '', '#/agent/fullscreen')
  else if (window.history.state?.fromPreviewAssistant) {
    window.history.back()
    return
  } else window.history.replaceState(null, '', '#/agent')
  syncPage()
}

const defaultModel = models[0]!
const sessions = ref<Session[]>(examples.map((example, index) => ({
  id: index + 1,
  title: example.title,
  draft: '',
  turns: [{ ...example, model: defaultModel.alias, status: 'complete', summaryOpen: null }],
})))
const activeID = ref(1)
const session = computed(() => sessions.value.find(entry => entry.id === activeID.value)!)
const selectedModel = ref(defaultModel.id)
const model = computed(() => models.find(entry => entry.id === selectedModel.value)!)
const effort = ref('')
const modelChoices = models.map(entry => ({ key: entry.id, name: entry.alias, icon: entry.icon }))
watch(selectedModel, () => { effort.value = '' })

const leftMedia = window.matchMedia('(min-width: 800px)')
const shell = ref<InstanceType<typeof AgentWorkspaceShell> | null>(null)
const siteDialog = ref<HTMLDialogElement | null>(null)
const siteNavigationOpen = ref(false)
const modelPicker = ref<InstanceType<typeof AgentModelPicker> | null>(null)
const effortSlider = ref<InstanceType<typeof AgentEffortSlider> | null>(null)
const timeline = ref<HTMLElement | null>(null)
const composer = ref<InstanceType<typeof AgentComposer> | null>(null)
const followLatest = ref(true)
const selectedAgent = ref<number | null>(null)
const inspectedTurn = ref<number | null>(null)
const playing = ref(false)
const activeTurn = computed(() => session.value.turns.find(turn => turn.status === 'reasoning' || turn.status === 'replying'))
const agentTurn = computed(() => session.value.turns[inspectedTurn.value ?? session.value.turns.length - 1])
const agentEntries = computed(() => agentTurn.value ? agents.map((agent, index) => ({ id: index, title: agent.name, description: agent.task, status: agentStatus(index), complete: agentStatus(index) === '已完成' })) : [])
const copyAvailable = Boolean(navigator.clipboard)
const copyState = ref('')
const surface = ref<HTMLElement | null>(null)
let replayTimer: ReturnType<typeof setInterval> | undefined
let nextID = examples.length + 1

function adaptKeyboard() {
  const viewport = window.visualViewport
  const keyboardInset = viewport && viewport.scale === 1 && !leftMedia.matches
    ? window.innerHeight - viewport.height - viewport.offsetTop : 0
  if (keyboardInset > 100 && viewport) {
    surface.value?.style.setProperty('--keyboard-height', `${Math.round(viewport.height)}px`)
  } else {
    surface.value?.style.removeProperty('--keyboard-height')
  }
}

onMounted(() => {
  window.addEventListener('hashchange', syncPage)
  window.addEventListener('storage', loadTheme)
  syncPage()
  window.visualViewport?.addEventListener('resize', adaptKeyboard)
  window.visualViewport?.addEventListener('scroll', adaptKeyboard)
})
onBeforeUnmount(() => {
  window.removeEventListener('hashchange', syncPage)
  window.removeEventListener('storage', loadTheme)
  window.visualViewport?.removeEventListener('resize', adaptKeyboard)
  window.visualViewport?.removeEventListener('scroll', adaptKeyboard)
  clearInterval(replayTimer)
})

function selectAgentTurn(index: number | null = null) {
  if (inspectedTurn.value !== index) selectedAgent.value = null
  inspectedTurn.value = index
}

function showAgents(index: number) {
  selectAgentTurn(index)
  shell.value?.openDetails()
}

function dismissOutside(event: MouseEvent) {
  if (event.target === event.currentTarget) (event.currentTarget as HTMLDialogElement).close()
}

function openSiteNavigation() {
  siteDialog.value?.showModal()
  siteNavigationOpen.value = true
}

function chooseSession(id: number) {
  if (!id) {
    newSession()
    return
  }
  activeID.value = id
  inspectedTurn.value = null
  selectedAgent.value = null
  copyState.value = ''
  followLatest.value = true
  nextTick(() => { if (timeline.value) timeline.value.scrollTop = 0 })
}

function newSession() {
  const id = nextID++
  sessions.value.unshift({ id, title: '新建对话', draft: '', turns: [] })
  chooseSession(id)
  if (leftMedia.matches) nextTick(() => composer.value?.focus())
}

function jumpToLatest() {
  const area = timeline.value
  if (area) area.scrollTop = area.scrollHeight
  followLatest.value = true
}

function trackScroll() {
  const area = timeline.value
  if (area) followLatest.value = area.scrollHeight - area.clientHeight - area.scrollTop < 48
}

watch(activeTurn, async () => {
  if (!followLatest.value) return
  await nextTick()
  if (followLatest.value) jumpToLatest()
}, { deep: true, flush: 'post' })

function summaryIsOpen(turn: Turn) {
  return turn.summaryOpen ?? (turn.status === 'reasoning' && turn.reasoning.length <= 112)
}

function submit() {
  const prompt = session.value.draft.trim()
  if (!prompt || playing.value) return
  const example = examples.find(entry => entry.prompt === prompt) ?? examples[0]!
  if (!session.value.turns.length) session.value.title = prompt.replace(/\s+/g, ' ')
  session.value.draft = ''
  const turnIndex = session.value.turns.push({ prompt, model: model.value.alias, reasoning: '', reply: '', status: 'reasoning', summaryOpen: null }) - 1
  const turn = session.value.turns[turnIndex]!
  inspectedTurn.value = null
  selectedAgent.value = null
  copyState.value = ''
  followLatest.value = true
  playing.value = true
  let cursor = 0
  replayTimer = setInterval(() => {
    if (turn.status === 'reasoning') {
      cursor += 4
      turn.reasoning = example.reasoning.slice(0, cursor)
      if (cursor >= example.reasoning.length) {
        turn.status = 'replying'
        cursor = 0
      }
    } else {
      cursor += 6
      turn.reply = example.reply.slice(0, cursor)
      if (cursor >= example.reply.length) {
        turn.status = 'complete'
        clearInterval(replayTimer)
        replayTimer = undefined
        playing.value = false
      }
    }
  }, 65)
}

function stop() {
  if (!activeTurn.value) return
  clearInterval(replayTimer)
  replayTimer = undefined
  activeTurn.value.status = 'stopped'
  playing.value = false
}

function agentStatus(index: number) {
  if (agentTurn.value?.status === 'reasoning') return index === 0 ? '进行中' : '等待中'
  if (agentTurn.value?.status === 'stopped' && !agentTurn.value.reply) return '演示已停止'
  return '已完成'
}

async function copyReply(reply: string, index: number) {
  try {
    await navigator.clipboard.writeText(reply)
    copyState.value = `copied-${index}`
  } catch {
    copyState.value = `unavailable-${index}`
  }
}
</script>

<template>
  <div ref="surface" class="preview-surface" :class="{ 'preview-surface-feed': !assistantVisible }">
    <header v-if="!fullscreen" class="preview-nav" :class="{ 'preview-nav-hidden': !topNavigationVisible }">
      <div class="preview-brand-group">
        <button type="button" class="icon-button" aria-label="打开站点导航" title="打开站点导航" aria-controls="preview-site-navigation" :aria-expanded="siteNavigationOpen" @click="openSiteNavigation"><IconMenuUnfold aria-hidden="true" /></button>
        <a class="preview-brand" href="#/recommendations" aria-label="MessageFeed 推荐"><span class="brand-mark" aria-hidden="true"><IconApps /></span><strong>MessageFeed</strong></a>
      </div>
      <nav aria-label="主导航"><a v-for="item in navigation" :key="item.page" :href="`#/${item.page}`" :aria-current="page === item.page ? 'page' : undefined"><component :is="item.icon" aria-hidden="true" /><span>{{ item.title }}</span></a></nav>
    </header>
    <dialog id="preview-site-navigation" ref="siteDialog" class="side-dialog from-left site-dialog" aria-label="站点导航" @click="dismissOutside" @close="siteNavigationOpen = false">
      <aside class="sidebar site-drawer">
        <header class="sidebar-header"><span class="brand-mark" aria-hidden="true"><IconApps /></span><strong>MessageFeed</strong><button type="button" class="icon-button" aria-label="关闭站点导航" title="关闭站点导航" autofocus @click="siteDialog?.close()"><IconClose aria-hidden="true" /></button></header>
        <nav class="site-links" aria-label="站点页面"><a v-for="item in navigation" :key="item.page" class="session-item" :href="`#/${item.page}`" :aria-current="(fullscreen ? 'agent' : page) === item.page ? 'page' : undefined" @click="siteDialog?.close()"><component :is="item.icon" aria-hidden="true" /><span>{{ item.title }}</span></a></nav>
        <footer class="site-drawer-actions"><button type="button" class="icon-button preview-theme" :aria-label="themeAction" :title="themeAction" @click="toggleTheme"><component :is="darkTheme ? IconSunFill : IconMoonFill" aria-hidden="true" /></button></footer>
      </aside>
    </dialog>
    <PreviewFeed v-show="page === 'subscriptions' && !article" page="subscriptions" :active="page === 'subscriptions' && !article" @chrome="topNavigationVisible = $event" />
    <PreviewFeed v-show="page === 'recommendations' && !article" page="recommendations" :active="page === 'recommendations' && !article" @chrome="topNavigationVisible = $event" />
    <main v-if="article" :key="article.title" class="preview-feed preview-original">
      <article class="feed-reading">
        <a class="original-back" :href="`#/${page}`"><IconArrowLeft aria-hidden="true" />返回{{ page === 'subscriptions' ? '订阅' : '推荐' }}</a>
        <header class="feed-heading">
          <h1>{{ article.title }}</h1>
          <p>本地示例 · {{ article.source }} · {{ article.category }}</p>
        </header>
        <p>{{ article.description }}</p>
        <p v-for="paragraph in article.body" :key="paragraph">{{ paragraph }}</p>
      </article>
    </main>
    <AgentWorkspaceShell v-show="assistantVisible" ref="shell" :fullscreen="fullscreen" :title="session.title" :sessions="sessions" :active-i-d="activeID" @update:fullscreen="toggleFullscreen" @select-session="chooseSession" @toggle-details="selectAgentTurn()">
      <template #navigation><button v-if="fullscreen" type="button" class="icon-button" aria-label="打开站点导航" title="打开站点导航" aria-controls="preview-site-navigation" :aria-expanded="siteNavigationOpen" @click="openSiteNavigation"><IconMenuUnfold aria-hidden="true" /></button></template>
      <template #title-meta><span v-if="fullscreen" class="local-indicator">本地预览</span></template>
      <template #sessions-footer><span class="status-dot" />本地演示空间</template>
      <template #details>
        <AgentSubagentList :entries="agentEntries" :selected-i-d="selectedAgent" :heading="inspectedTurn === null ? '最新一轮' : `第 ${inspectedTurn + 1} 轮`" @select="selectedAgent = selectedAgent === $event ? null : $event">
          <template #result="{ id }"><p>{{ agentStatus(id) === '已完成' ? agents[id]?.result : '本次演示暂无任务结果。' }}</p></template>
        </AgentSubagentList>
      </template>
        <div ref="timeline" class="timeline" aria-label="对话消息" @scroll.passive="trackScroll">
          <div v-if="!session.turns.length" class="reading-column welcome">
            <span class="welcome-mark" aria-hidden="true"><IconApps /></span>
            <h2>有什么需要处理？</h2>
            <div class="suggestions">
              <button v-for="example in examples.slice(0, 2)" :key="example.title" type="button" @click="session.draft = example.prompt; composer?.focus()"><IconFile />{{ example.title }}<IconArrowUp /></button>
            </div>
          </div>
          <div v-else class="reading-column transcript">
            <article v-for="(turn, index) in session.turns" :key="`${session.id}-${index}`" class="turn">
              <div class="prompt"><p>{{ turn.prompt }}</p></div>
              <div class="answer">
                <div class="answer-byline"><span class="assistant-mark" aria-hidden="true"><IconApps /></span><strong>助理</strong><span class="turn-model">{{ turn.model }}</span></div>

                <div class="activity" :aria-label="`第 ${index + 1} 次回复的活动`">
                  <button v-if="turn.reasoning" type="button" class="activity-toggle" :aria-expanded="summaryIsOpen(turn)" @click="turn.summaryOpen = !summaryIsOpen(turn)"><IconBulb /><span>思考摘要</span><span class="activity-meta">{{ turn.status === 'reasoning' ? '正在播放' : '示例' }}</span><IconRight class="disclosure" /></button>
                  <p v-if="summaryIsOpen(turn) && turn.reasoning" class="summary-text">{{ turn.reasoning }}</p>
                  <details v-if="turn.reply || turn.status === 'complete'" class="execution">
                    <summary class="activity-toggle"><IconList /><span>执行详情</span><span class="activity-meta">{{ stages.length }} 项示例活动</span><IconRight class="disclosure" /></summary>
                    <ol><li v-for="stage in stages" :key="stage"><IconCheck aria-hidden="true" />{{ stage }}</li></ol>
                  </details>
                </div>

                <AgentMarkdown v-if="turn.reply" :content="turn.reply" />
                <p v-if="turn.status === 'reasoning'" class="turn-status" role="status"><span class="status-dot" />正在播放示例摘要</p>
                <div class="turn-footer">
                  <span class="turn-status" role="status">{{ turn.status === 'replying' ? '正在播放示例回答' : turn.status === 'stopped' ? '演示已停止' : turn.status === 'complete' ? '示例回答' : '' }}</span>
                  <button v-if="copyAvailable && turn.reply && turn.status !== 'replying'" type="button" class="icon-button copy-button" :aria-label="copyState === `copied-${index}` ? '已复制回答' : '复制回答'" title="复制回答" @click="copyReply(turn.reply, index)"><IconCheck v-if="copyState === `copied-${index}`" /><IconCopy v-else /></button>
                  <span v-if="copyState === `unavailable-${index}`" role="status">无法访问剪贴板，可直接选取正文。</span>
                  <button v-if="turn.status !== 'reasoning'" class="turn-agents" type="button" @click="showAgents(index)"><IconBranch />查看本次子代理</button>
                </div>
              </div>
            </article>
          </div>
        </div>

        <footer class="composer-area">
          <div class="reading-column composer-column">
            <button v-if="!followLatest" type="button" class="jump-button" @click="jumpToLatest"><IconArrowDown />查看最新消息</button>
            <AgentComposer ref="composer" v-model="session.draft" label="对话输入" @submit="submit">
              <div class="composer-controls">
                <AgentModelPicker ref="modelPicker" v-model="selectedModel" :choices="modelChoices" :disabled="Boolean(activeTurn)" />
                <AgentEffortSlider ref="effortSlider" v-model="effort" :levels="model.levels" :model="model.alias" :disabled="Boolean(activeTurn)" />
              </div>
              <button v-if="activeTurn" type="button" class="send-button stop-button" aria-label="停止演示" title="停止演示" @click="stop"><IconStop /></button>
              <button v-else type="submit" class="send-button" :disabled="!session.draft.trim() || playing" aria-label="发送并播放示例" title="发送并播放示例"><IconArrowUp /></button>
            </AgentComposer>
            <div class="preview-note">{{ playing && !activeTurn ? '另一会话正在播放示例' : '本地示例 · 不会调用模型' }}</div>
          </div>
        </footer>
    </AgentWorkspaceShell>

  </div>
</template>
