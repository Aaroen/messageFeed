<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, useId } from 'vue'
import { IconApps, IconBranch, IconClockCircle, IconClose, IconFullscreen, IconFullscreenExit, IconMenuFold, IconMessage, IconPlus } from '@arco-design/web-vue/es/icon'

defineProps<{ title: string; sessions: { id: number; title: string }[]; activeID: number; sessionBusy?: boolean }>()
const emit = defineEmits<{ selectSession: [id: number]; toggleDetails: [] }>()
const fullscreen = defineModel<boolean>('fullscreen', { required: true })
const sessionsID = useId()
const detailsID = useId()
const leftMedia = window.matchMedia('(min-width: 800px)')
const rightMedia = window.matchMedia('(min-width: 1190px)')
const leftDocked = ref(leftMedia.matches)
const rightDocked = ref(rightMedia.matches)
const sessionsOpen = ref(true)
const detailsOpen = ref(true)
const drawer = ref<'sessions' | 'details' | null>(null)
const sessionsDialog = ref<HTMLDialogElement | null>(null)
const detailsDialog = ref<HTMLDialogElement | null>(null)
const detailsToggle = ref<HTMLButtonElement | null>(null)

function syncViewport() {
  if (leftMedia.matches) sessionsDialog.value?.close()
  if (rightMedia.matches) detailsDialog.value?.close()
  leftDocked.value = leftMedia.matches
  rightDocked.value = rightMedia.matches
}

function openDrawer(side: 'sessions' | 'details', dialog: HTMLDialogElement | null) {
  if (!dialog) return
  dialog.showModal()
  drawer.value = side
}

function toggleSessions() {
  if (leftDocked.value) sessionsOpen.value = !sessionsOpen.value
  else openDrawer('sessions', sessionsDialog.value)
}

function openDetails() {
  if (rightDocked.value) detailsOpen.value = true
  else openDrawer('details', detailsDialog.value)
}

function toggleDetails() {
  if (rightDocked.value && detailsOpen.value) detailsOpen.value = false
  else {
    emit('toggleDetails')
    openDetails()
  }
}

function closeDetails() {
  if (!rightDocked.value) detailsDialog.value?.close()
  else {
    detailsOpen.value = false
    nextTick(() => detailsToggle.value?.focus())
  }
}

function closePanels() {
  sessionsDialog.value?.close()
  detailsDialog.value?.close()
  drawer.value = null
}

function dismissOutside(event: MouseEvent) {
  if (event.target === event.currentTarget) (event.currentTarget as HTMLDialogElement).close()
}

function selectSession(id: number) {
  sessionsDialog.value?.close()
  emit('selectSession', id)
}

onMounted(() => {
  leftMedia.addEventListener('change', syncViewport)
  rightMedia.addEventListener('change', syncViewport)
  syncViewport()
})
onBeforeUnmount(() => {
  closePanels()
  leftMedia.removeEventListener('change', syncViewport)
  rightMedia.removeEventListener('change', syncViewport)
})
defineExpose({ openDetails, closePanels })
</script>

<template>
  <section class="assistant-shell" :class="{ 'has-sessions': leftDocked && sessionsOpen, 'has-details': rightDocked && detailsOpen, 'is-fullscreen': fullscreen }" aria-label="助理工作区" @touchstart.stop @touchmove.stop @touchend.stop @pointerdown.stop>
    <section class="assistant-shell-conversation" aria-label="当前对话">
      <header class="assistant-shell-toolbar">
        <slot name="navigation" />
        <button type="button" class="assistant-shell-button" :aria-label="leftDocked && sessionsOpen ? '收起会话侧栏' : '打开会话列表'" :title="leftDocked && sessionsOpen ? '收起会话侧栏' : '打开会话列表'" :aria-expanded="leftDocked ? sessionsOpen : drawer === 'sessions'" :aria-controls="sessionsID" @click="toggleSessions"><IconMenuFold aria-hidden="true" /></button>
        <div class="assistant-shell-title"><h1>{{ title }}</h1><slot name="title-meta" /></div>
        <button type="button" class="assistant-shell-button" :aria-label="fullscreen ? '退出全屏' : '全屏打开助理'" :title="fullscreen ? '退出全屏' : '全屏打开助理'" @click="fullscreen = !fullscreen"><IconFullscreenExit v-if="fullscreen" aria-hidden="true" /><IconFullscreen v-else aria-hidden="true" /></button>
        <button ref="detailsToggle" type="button" class="assistant-shell-button assistant-shell-details-toggle" :aria-label="rightDocked && detailsOpen ? '收起子代理侧栏' : '打开子代理面板'" :title="rightDocked && detailsOpen ? '收起子代理侧栏' : '打开子代理面板'" :aria-expanded="rightDocked ? detailsOpen : drawer === 'details'" :aria-controls="detailsID" @click="toggleDetails"><IconBranch aria-hidden="true" /><span>子代理</span></button>
      </header>
      <slot />
    </section>

    <dialog ref="sessionsDialog" class="assistant-shell-dialog from-left" aria-label="会话列表" @click="dismissOutside" @close="drawer === 'sessions' && (drawer = null)"></dialog>
    <dialog ref="detailsDialog" class="assistant-shell-dialog from-right" aria-label="子代理" @click="dismissOutside" @close="drawer === 'details' && (drawer = null)"></dialog>

    <Teleport :to="sessionsDialog ?? 'body'" :disabled="leftDocked || !sessionsDialog">
      <aside :id="sessionsID" class="assistant-shell-sidebar assistant-shell-sessions" :inert="leftDocked && !sessionsOpen" aria-label="会话列表">
        <header class="assistant-shell-sidebar-header"><span class="assistant-shell-mark" aria-hidden="true"><IconApps /></span><div class="assistant-shell-brand"><strong>MessageFeed</strong><span>助理工作区</span></div><button v-if="!leftDocked" type="button" class="assistant-shell-button" aria-label="关闭会话列表" title="关闭会话列表" autofocus @click="sessionsDialog?.close()"><IconClose aria-hidden="true" /></button></header>
        <button class="assistant-shell-new-session" type="button" :disabled="sessionBusy" @click="selectSession(0)"><IconPlus aria-hidden="true" />新建对话</button>
        <h2 class="assistant-shell-section-label"><IconClockCircle aria-hidden="true" />最近对话</h2>
        <nav class="assistant-shell-history" aria-label="最近对话">
          <button v-for="session in sessions" :key="session.id" type="button" class="assistant-shell-session" :aria-current="session.id === activeID ? 'true' : undefined" :title="session.title" :disabled="sessionBusy" @click="selectSession(session.id)"><IconMessage aria-hidden="true" /><span>{{ session.title }}</span></button>
          <p v-if="!sessions.length" class="assistant-shell-empty">暂无会话记录</p>
        </nav>
        <div class="assistant-shell-foot"><slot name="sessions-footer" /></div>
      </aside>
    </Teleport>

    <Teleport :to="detailsDialog ?? 'body'" :disabled="rightDocked || !detailsDialog">
      <aside :id="detailsID" class="assistant-shell-sidebar assistant-shell-details" :inert="rightDocked && !detailsOpen" aria-label="子代理详情">
        <header class="assistant-shell-sidebar-header assistant-shell-details-header"><h2><IconBranch aria-hidden="true" />子代理</h2><button type="button" class="assistant-shell-button" aria-label="关闭子代理面板" title="关闭子代理面板" :autofocus="!rightDocked" @click="closeDetails"><IconClose aria-hidden="true" /></button></header>
        <slot name="details" />
      </aside>
    </Teleport>

    <slot v-if="rightDocked && detailsOpen" name="resize-handle" />
  </section>
</template>

<style scoped>
.assistant-shell { --assistant-left: 0px; --assistant-right: 0px; --assistant-panel-motion: 190ms cubic-bezier(.2, .75, .25, 1); position: relative; isolation: isolate; display: grid; grid-template-columns: var(--assistant-left) minmax(0, 1fr) var(--assistant-right); flex: 1; min-height: 0; height: 100%; overflow: clip; background: var(--mf-surface); color: var(--mf-text); font: 14px/1.5 var(--mf-font-family); }
.has-sessions { --assistant-left: 236px; }
.has-details { --assistant-right: var(--assistant-panel-width, 288px); }
.assistant-shell-conversation { display: flex; flex-direction: column; grid-column: 2; min-width: 0; min-height: 0; container-type: inline-size; }
.assistant-shell-toolbar { display: flex; align-items: center; gap: 12px; flex: none; min-width: 0; min-height: 63px; padding-inline: 18px; }
.assistant-shell-title { flex: 1; min-width: 0; display: flex; align-items: center; gap: 9px; }
.assistant-shell-title h1 { min-width: 0; margin: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 14px; font-weight: 650; }
.assistant-shell-button, .assistant-shell-session, .assistant-shell-new-session { padding: 0; border: 0; background: transparent; color: var(--mf-text-muted); font: inherit; cursor: pointer; }
.assistant-shell-button { display: inline-flex; align-items: center; justify-content: center; gap: 7px; flex: none; min-width: 34px; min-height: 34px; border-radius: 6px; }
.assistant-shell-button svg { width: 17px; height: 17px; }
.assistant-shell-button:hover, .assistant-shell-session:hover:not(:disabled), .assistant-shell-foot :slotted(a:hover) { background: color-mix(in oklab, var(--mf-text-muted) 12%, var(--mf-surface)); color: var(--mf-text); }
.assistant-shell-details-toggle { padding-inline: 8px; white-space: nowrap; font-size: 13px; }
.assistant-shell-sidebar { position: absolute; z-index: 2; inset-block: 0; display: flex; flex-direction: column; min-height: 0; overflow: clip; background: var(--mf-surface-muted); }
.assistant-shell-sessions { --assistant-shadow-offset: 4px; inset-inline-start: 0; width: 236px; transform: translateX(-100%); }
.assistant-shell-details { --assistant-shadow-offset: -4px; inset-inline-end: 0; width: var(--assistant-panel-width, 288px); transform: translateX(100%); }
.has-sessions > .assistant-shell-sessions, .has-details > .assistant-shell-details { transform: translateX(0); box-shadow: var(--assistant-shadow-offset) 0 12px -6px rgb(0 0 0 / 22%); }
.assistant-shell-sidebar-header { flex: none; display: flex; align-items: center; gap: 10px; min-height: 70px; padding: 12px 17px; }
.assistant-shell-mark { display: grid; place-items: center; flex: none; width: 32px; height: 32px; border-radius: 7px; color: var(--mf-primary-strong); background: color-mix(in oklab, var(--mf-primary) 14%, var(--mf-surface)); font-size: 18px; }
.assistant-shell-brand { flex: 1; display: grid; min-width: 0; }
.assistant-shell-brand strong { font-size: 15px; font-weight: 700; }
.assistant-shell-brand > span { margin-top: 1px; font-size: 11px; color: var(--mf-text-muted); }
.assistant-shell-new-session { flex: none; display: flex; align-items: center; gap: 8px; min-height: 37px; margin: 5px 12px 0; padding-inline: 10px; border-radius: 8px; background: var(--mf-surface); color: var(--mf-text); text-align: start; font-weight: 600; }
.assistant-shell-new-session:hover:not(:disabled), .assistant-shell-session[aria-current] { background: color-mix(in oklab, var(--mf-primary) 14%, var(--mf-surface)); color: var(--mf-primary-strong); }
.assistant-shell-section-label, .assistant-shell-foot, .assistant-shell-empty { display: flex; align-items: center; gap: 8px; padding-inline: 17px; color: var(--mf-text-muted); font-size: 12px; font-weight: 550; }
.assistant-shell-section-label { margin: 25px 0 8px; }
.assistant-shell-history { flex: 1; min-height: 0; overflow: auto; overscroll-behavior: contain; padding: 2px 12px; }
.assistant-shell-session { display: flex; align-items: center; gap: 9px; width: 100%; min-height: 37px; margin-bottom: 3px; padding: 5px 9px; border-radius: 6px; text-align: start; }
.assistant-shell-session svg { flex: none; }
.assistant-shell-session span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.assistant-shell-session[aria-current] { font-weight: 650; }
.assistant-shell-session:disabled, .assistant-shell-new-session:disabled { opacity: .65; cursor: default; }
.assistant-shell-empty { margin: 0; padding: 10px 9px; }
.assistant-shell-foot { min-height: 52px; margin-top: auto; padding-bottom: env(safe-area-inset-bottom, 0px); }
.assistant-shell-foot :slotted(a) { display: inline-flex; align-items: center; gap: 8px; min-height: 44px; border-radius: 6px; padding-inline: 6px; margin-inline-start: -6px; color: inherit; text-decoration: none; }
.assistant-shell-details-header { justify-content: space-between; min-height: 63px; padding: 10px 13px 10px 17px; }
.assistant-shell-details-header h2 { display: flex; align-items: center; gap: 8px; margin: 0; font-size: 14px; font-weight: 650; }
.assistant-shell-details-header h2 svg { color: var(--mf-primary-strong); }
.assistant-shell-dialog { inset: 0; width: 100%; height: 100%; max-width: none; max-height: none; margin: 0; padding: 0; border: 0; background: transparent; color: inherit; overflow: clip; }
.assistant-shell-dialog::backdrop { background: rgb(0 0 0 / 28%); }
.assistant-shell-dialog > .assistant-shell-sidebar { width: min(328px, calc(100% - 36px)); transform: translateX(0); box-shadow: var(--mf-panel-shadow); }
.assistant-shell-dialog > .assistant-shell-details { width: min(350px, calc(100% - 32px)); }
.assistant-shell-dialog .assistant-shell-sidebar-header { padding-top: max(12px, env(safe-area-inset-top)); }
.is-fullscreen:not(:has(.assistant-shell-toolbar > [aria-label='打开站点导航'])) .assistant-shell-toolbar { padding-inline-start: 74px; }
@container (max-width: 630px) { .assistant-shell-toolbar { gap: 8px; padding-inline: 12px; } }
@media (max-width: 799px) {
  .assistant-shell-toolbar { min-height: 54px; padding-inline: 9px; }
  .assistant-shell-details-toggle > span { display: none; }
  .is-fullscreen:not(:has(.assistant-shell-toolbar > [aria-label='打开站点导航'])) .assistant-shell-toolbar { padding-inline-start: 61px; }
}
@media (max-width: 799px), (pointer: coarse) {
  .assistant-shell-button { min-width: 44px; min-height: 44px; }
  .assistant-shell-session, .assistant-shell-new-session { min-height: 44px; }
  .assistant-shell-sidebar-header .assistant-shell-button { margin-inline-end: -5px; }
}
@media (prefers-reduced-motion: no-preference) {
  .assistant-shell-sidebar { transition: transform var(--assistant-panel-motion); }
  @supports (transition-behavior: allow-discrete) {
    .assistant-shell-dialog, .assistant-shell-dialog::backdrop { transition: opacity var(--assistant-panel-motion), display var(--assistant-panel-motion) allow-discrete, overlay var(--assistant-panel-motion) allow-discrete; }
    .assistant-shell-dialog:not([open])::backdrop { opacity: 0; }
    .assistant-shell-dialog.from-left:not([open]) > .assistant-shell-sidebar { transform: translateX(-100%); }
    .assistant-shell-dialog.from-right:not([open]) > .assistant-shell-sidebar { transform: translateX(100%); }
    @starting-style {
      .assistant-shell-dialog[open]::backdrop { opacity: 0; }
      .assistant-shell-dialog.from-left[open] > .assistant-shell-sidebar { transform: translateX(-100%); }
      .assistant-shell-dialog.from-right[open] > .assistant-shell-sidebar { transform: translateX(100%); }
    }
  }
}
</style>
