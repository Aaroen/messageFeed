<script setup lang="ts">
import type { StyleValue } from 'vue'

withDefaults(
  defineProps<{
    sourceMounted?: boolean
    sourceInteractive?: boolean
    sourceUnderDetail?: boolean
    sourceStyle?: StyleValue
    sourceTitleRevealMounted?: boolean
    sourceTitleRevealStyle?: StyleValue
    sourceTitle?: string
    sourceMeta?: string
    sourceNameMorphMounted?: boolean
    sourceNameMorphStyle?: StyleValue
    sourceNameMorphText?: string
    detailOpen?: boolean
    detailStyle?: StyleValue
    detailBackdropStyle?: StyleValue
  }>(),
  {
    sourceMounted: false,
    sourceInteractive: false,
    sourceUnderDetail: false,
    sourceStyle: undefined,
    sourceTitleRevealMounted: false,
    sourceTitleRevealStyle: undefined,
    sourceTitle: '',
    sourceMeta: '',
    sourceNameMorphMounted: false,
    sourceNameMorphStyle: undefined,
    sourceNameMorphText: '',
    detailOpen: false,
    detailStyle: undefined,
    detailBackdropStyle: undefined,
  },
)
</script>

<template>
  <section
    v-if="sourceMounted"
    class="reader-overlay reader-overlay--source"
    :class="{ 'reader-overlay--under-detail': sourceUnderDetail }"
    :style="sourceStyle"
    :aria-hidden="!sourceInteractive"
    :inert="!sourceInteractive"
  >
    <slot name="source" />
  </section>

  <div
    v-if="sourceTitleRevealMounted"
    class="source-title-reveal"
    :style="sourceTitleRevealStyle"
    aria-hidden="true"
  >
    <span>{{ sourceTitle }}</span>
    <small>{{ sourceMeta }}</small>
  </div>

  <div
    v-if="sourceNameMorphMounted"
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
    <div class="reader-overlay__backdrop" :style="detailBackdropStyle" aria-hidden="true" />
    <slot name="detail" />
  </section>
</template>
