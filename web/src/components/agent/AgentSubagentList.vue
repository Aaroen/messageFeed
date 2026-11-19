<script setup lang="ts">
import { IconBranch, IconList, IconRight } from '@arco-design/web-vue/es/icon'

defineProps<{ entries: { id: number; title: string; description?: string; status: string; complete?: boolean; depth?: number }[]; selectedID?: number | null; heading?: string }>()
defineEmits<{ select: [id: number] }>()
</script>

<template>
  <div v-if="entries.length" class="subagent-section">
    <div class="subagent-heading"><h3>{{ heading || '最新一轮' }}</h3><span>{{ entries.length }} 项任务</span></div>
    <div v-for="entry in entries" :key="entry.id" class="subagent-entry" :style="{ marginInlineStart: `${Math.min(entry.depth || 0, 6) * 12}px` }">
      <button type="button" class="subagent-item" :aria-expanded="selectedID === entry.id" @click="$emit('select', entry.id)">
        <span class="subagent-icon" aria-hidden="true"><IconList /></span>
        <span class="subagent-copy"><strong>{{ entry.title }}</strong><span v-if="entry.description">{{ entry.description }}</span></span>
        <IconRight class="subagent-chevron" aria-hidden="true" />
      </button>
      <div class="subagent-state" :class="{ complete: entry.complete }"><span class="subagent-dot" aria-hidden="true" />{{ entry.status }}</div>
      <div v-if="selectedID === entry.id" class="subagent-result"><slot name="result" :id="entry.id" /></div>
    </div>
  </div>
  <div v-else class="subagent-empty"><IconBranch aria-hidden="true" /><p>当前对话暂无子代理</p></div>
</template>

<style scoped>
.subagent-section { min-height: 0; overflow: auto; overscroll-behavior: contain; padding: 21px 16px max(16px, env(safe-area-inset-bottom, 0px)); }
.subagent-heading { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 10px; }
.subagent-heading h3 { margin: 0; font-size: 13px; font-weight: 650; }
.subagent-heading > span, .subagent-copy > span, .subagent-state { color: var(--mf-text-muted); font-size: 12px; }
.subagent-entry { padding-block: 5px 13px; }
.subagent-item { display: flex; align-items: center; gap: 8px; width: 100%; min-height: 45px; padding: 0; border: 0; border-radius: 6px; background: transparent; color: inherit; font: inherit; text-align: start; cursor: pointer; }
.subagent-item:hover { background: color-mix(in oklab, var(--mf-text-muted) 12%, var(--mf-surface)); }
.subagent-icon { display: grid; place-items: center; flex: none; width: 28px; height: 28px; border-radius: 6px; color: var(--mf-text-muted); background: var(--mf-surface); }
.subagent-copy { display: grid; flex: 1; min-width: 0; gap: 2px; padding-block: 3px; overflow-wrap: anywhere; }
.subagent-copy strong { font-weight: 650; }
.subagent-chevron { flex: none; }
.subagent-item[aria-expanded='true'] .subagent-chevron { transform: rotate(90deg); }
.subagent-state { display: flex; align-items: center; gap: 6px; margin-top: 5px; padding-inline-start: 36px; }
.subagent-state.complete { color: var(--mf-aqua); }
.subagent-dot { width: 6px; height: 6px; border-radius: 50%; background: currentColor; flex: none; }
.subagent-result { margin-top: 10px; padding: 9px 10px; border-radius: 6px; background: var(--mf-surface); color: var(--mf-text-muted); font-size: 13px; line-height: 1.65; overflow-wrap: anywhere; }
.subagent-result :slotted(p) { margin: 0; }
.subagent-empty { display: flex; align-items: center; gap: 9px; padding: 27px 17px; color: var(--mf-text-muted); }
.subagent-empty p { margin: 0; }
@media (prefers-reduced-motion: no-preference) { .subagent-chevron { transition: transform 190ms cubic-bezier(.2, .75, .25, 1); } }
</style>
