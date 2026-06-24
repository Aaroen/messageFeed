<script setup lang="ts">
import {
  IconCheckCircle,
  IconEyeInvisible,
  IconStar,
} from '@arco-design/web-vue/es/icon'

const props = withDefaults(
  defineProps<{
    isRead?: boolean
    isFavorite?: boolean
    isHidden?: boolean
    busyKey?: string
    compact?: boolean
  }>(),
  {
    isRead: false,
    isFavorite: false,
    isHidden: false,
    busyKey: '',
    compact: false,
  },
)

const emit = defineEmits<{
  (event: 'toggle-read'): void
  (event: 'toggle-favorite'): void
  (event: 'toggle-hidden'): void
}>()

function buttonTitle(kind: 'read' | 'favorite' | 'hidden') {
  if (kind === 'read') {
    return props.isRead ? '取消已读' : '标记已读'
  }
  if (kind === 'favorite') {
    return props.isFavorite ? '取消收藏' : '收藏'
  }
  return props.isHidden ? '取消隐藏' : '隐藏'
}
</script>

<template>
  <div class="item-action-bar" :class="{ 'item-action-bar--compact': compact }">
    <button
      class="item-action-bar__button"
      :class="{ 'item-action-bar__button--active': isRead }"
      type="button"
      :title="buttonTitle('read')"
      :aria-label="buttonTitle('read')"
      :aria-pressed="isRead"
      :disabled="busyKey === 'read'"
      @pointerdown.stop
      @touchstart.stop
      @click.prevent.stop="emit('toggle-read')"
    >
      <IconCheckCircle />
    </button>
    <button
      class="item-action-bar__button"
      :class="{ 'item-action-bar__button--active': isFavorite }"
      type="button"
      :title="buttonTitle('favorite')"
      :aria-label="buttonTitle('favorite')"
      :aria-pressed="isFavorite"
      :disabled="busyKey === 'favorite'"
      @pointerdown.stop
      @touchstart.stop
      @click.prevent.stop="emit('toggle-favorite')"
    >
      <IconStar />
    </button>
    <button
      class="item-action-bar__button"
      :class="{ 'item-action-bar__button--active': isHidden }"
      type="button"
      :title="buttonTitle('hidden')"
      :aria-label="buttonTitle('hidden')"
      :aria-pressed="isHidden"
      :disabled="busyKey === 'hidden'"
      @pointerdown.stop
      @touchstart.stop
      @click.prevent.stop="emit('toggle-hidden')"
    >
      <IconEyeInvisible />
    </button>
  </div>
</template>

<style scoped>
.item-action-bar {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.item-action-bar__button {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border: 1px solid var(--mf-border);
  border-radius: 9px;
  background: var(--mf-surface);
  color: var(--mf-text-muted);
  cursor: pointer;
  transition:
    transform var(--motion-fast) var(--ease-standard),
    border-color var(--motion-fast) var(--ease-standard),
    background var(--motion-fast) var(--ease-standard),
    color var(--motion-fast) var(--ease-standard);
}

.item-action-bar--compact .item-action-bar__button {
  width: 30px;
  height: 30px;
}

.item-action-bar__button svg {
  width: 16px;
  height: 16px;
}

.item-action-bar__button:hover,
.item-action-bar__button--active {
  border-color: rgba(37, 99, 235, 0.34);
  color: var(--mf-primary-strong);
  background: rgba(37, 99, 235, 0.08);
}

.item-action-bar__button:disabled {
  cursor: wait;
  opacity: 0.55;
  transform: none;
}
</style>
