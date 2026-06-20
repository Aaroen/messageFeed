<script setup lang="ts">
import type { StyleValue } from 'vue'

type ClassValue = string | Record<string, boolean> | Array<string | Record<string, boolean>>

withDefaults(
  defineProps<{
    sourceMounted?: boolean
    sourceUnderDetail?: boolean
    sourceStyle?: StyleValue
    sourceTitleRevealMounted?: boolean
    sourceTitleRevealVisible?: boolean
    sourceTitleRevealStyle?: StyleValue
    sourceNameMorphMounted?: boolean
    sourceNameMorphVisible?: boolean
    sourceNameMorphStyle?: StyleValue
    detailOpen?: boolean
    detailClass?: ClassValue
    detailMotionSettling?: boolean
    detailReturningFeed?: boolean
    detailStyle?: StyleValue
  }>(),
  {
    sourceMounted: false,
    sourceUnderDetail: false,
    sourceStyle: undefined,
    sourceTitleRevealMounted: false,
    sourceTitleRevealVisible: false,
    sourceTitleRevealStyle: undefined,
    sourceNameMorphMounted: false,
    sourceNameMorphVisible: false,
    sourceNameMorphStyle: undefined,
    detailOpen: false,
    detailClass: '',
    detailMotionSettling: false,
    detailReturningFeed: false,
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
    <slot name="source-title-reveal" />
  </div>

  <div
    v-if="sourceNameMorphMounted"
    v-show="sourceNameMorphVisible"
    class="detail-source-morph"
    :style="sourceNameMorphStyle"
  >
    <slot name="source-name-morph" />
  </div>

  <section
    v-if="detailOpen"
    class="reader-overlay reader-overlay--detail"
    :class="[
      detailClass,
      {
        'reader-overlay--motion-settling': detailMotionSettling,
        'reader-overlay--returning-feed': detailReturningFeed,
      },
    ]"
    :style="detailStyle"
  >
    <slot name="detail" />
  </section>
</template>
