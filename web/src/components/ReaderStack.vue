<script setup lang="ts">
import type { StyleValue } from 'vue'

withDefaults(
  defineProps<{
    sourceMounted?: boolean
    sourceUnderDetail?: boolean
    sourceStyle?: StyleValue
    sourceTitleRevealMounted?: boolean
    sourceTitleRevealVisible?: boolean
    sourceTitleRevealStyle?: StyleValue
    sourceTitle?: string
    sourceMeta?: string
    sourceNameMorphMounted?: boolean
    sourceNameMorphVisible?: boolean
    sourceNameMorphStyle?: StyleValue
    sourceNameMorphText?: string
    detailOpen?: boolean
    detailStyle?: StyleValue
  }>(),
  {
    sourceMounted: false,
    sourceUnderDetail: false,
    sourceStyle: undefined,
    sourceTitleRevealMounted: false,
    sourceTitleRevealVisible: false,
    sourceTitleRevealStyle: undefined,
    sourceTitle: '',
    sourceMeta: '',
    sourceNameMorphMounted: false,
    sourceNameMorphVisible: false,
    sourceNameMorphStyle: undefined,
    sourceNameMorphText: '',
    detailOpen: false,
    detailStyle: undefined,
  },
)
</script>

<template>
  <section
    v-if="sourceMounted"
    class="reader-overlay reader-overlay--source"
    :class="{ 'reader-overlay--under-detail': sourceUnderDetail }"
    :style="sourceStyle"
  >
    <slot name="source" />
  </section>

  <div
    v-if="sourceTitleRevealMounted"
    v-show="sourceTitleRevealVisible"
    class="source-title-reveal"
    :style="sourceTitleRevealStyle"
    aria-hidden="true"
  >
    <span>{{ sourceTitle }}</span>
    <small>{{ sourceMeta }}</small>
  </div>

  <div
    v-if="sourceNameMorphMounted"
    v-show="sourceNameMorphVisible"
    class="detail-source-morph"
    :style="sourceNameMorphStyle"
  >
    {{ sourceNameMorphText }}
  </div>

  <section
    v-if="detailOpen"
    class="reader-overlay reader-overlay--detail"
    :style="detailStyle"
  >
    <slot name="detail" />
  </section>
</template>
