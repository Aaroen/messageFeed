<script setup lang="ts">
import { IconBulb, IconList, IconRight } from '@arco-design/web-vue/es/icon'

defineProps<{ kind: 'reasoning' | 'execution'; meta?: string; open?: boolean }>()
const emit = defineEmits<{ toggleOpen: [open: boolean] }>()
</script>

<template>
  <details class="activity-disclosure" :open="open" @toggle="emit('toggleOpen', ($event.currentTarget as HTMLDetailsElement).open)">
    <summary class="activity-toggle">
      <component :is="kind === 'reasoning' ? IconBulb : IconList" class="activity-icon" aria-hidden="true" />
      <span>{{ kind === 'reasoning' ? '思考摘要' : '执行详情' }}</span>
      <small v-if="meta" class="activity-meta">{{ meta }}</small>
      <IconRight class="activity-chevron" aria-hidden="true" />
    </summary>
    <div class="activity-panel"><slot /></div>
  </details>
</template>

<style scoped>
.activity-toggle { display: flex; align-items: center; gap: 8px; min-height: 33px; padding-inline: 6px; border-radius: 6px; color: var(--mf-text-muted); list-style: none; font-size: 13px; cursor: pointer; }
.activity-toggle:hover { background: var(--agent-activity-tint, color-mix(in oklab, var(--mf-surface-muted) 54%, var(--mf-surface))); }
.activity-toggle::-webkit-details-marker { display: none; }
.activity-icon { color: var(--mf-primary-strong); flex: none; }
.activity-meta { padding-inline-start: 2px; font-size: 11px; overflow-wrap: anywhere; }
.activity-chevron { margin-inline-start: auto; width: 12px; height: 12px; flex: none; }
.activity-disclosure[open] > .activity-toggle .activity-chevron { rotate: 90deg; }
.activity-panel { margin: 3px 0 7px; padding: 8px 14px; border-radius: 6px; background: var(--agent-activity-tint, color-mix(in oklab, var(--mf-surface-muted) 54%, var(--mf-surface))); color: var(--mf-text-muted); font-size: 13px; line-height: 1.7; }
@media (prefers-reduced-motion: no-preference) { .activity-chevron { transition: rotate 140ms ease-out; } }
@media (max-width: 760px) { .activity-toggle { min-height: 44px; } }
</style>
