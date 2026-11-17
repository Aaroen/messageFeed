<script setup lang="ts">
import { computed } from 'vue'
import MarkdownIt from 'markdown-it'

const props = defineProps<{ content: string }>()
const renderer = new MarkdownIt({ html: false, linkify: true, breaks: true })
const rendered = computed(() => renderer.render(props.content))
</script>

<template>
  <div class="assistant-markdown" v-html="rendered" />
</template>

<style>
.assistant-markdown { color: var(--mf-text); line-height: 1.85; overflow-wrap: anywhere; font-size: 14px; }
.assistant-markdown > :first-child { margin-top: 0; }
.assistant-markdown > :last-child { margin-bottom: 0; }
.assistant-markdown p { margin: 0 0 1em; }
.assistant-markdown h1, .assistant-markdown h2, .assistant-markdown h3 { line-height: 1.5; margin: 1.4em 0 .7em; font-weight: 650; }
.assistant-markdown h1 { font-size: 1.5em; }
.assistant-markdown h2 { font-size: 1.25em; }
.assistant-markdown h3 { font-size: 1.1em; }
.assistant-markdown pre { background: var(--mf-surface-muted, #f4f5f6); border: 1px solid var(--mf-border); padding: 15px 17px; border-radius: 10px; overflow-x: auto; white-space: pre; line-height: 1.65; }
.assistant-markdown code { font-family: ui-monospace, SFMono-Regular, Consolas, monospace; font-size: .88em; }
.assistant-markdown :not(pre) > code { padding: 2px 5px; border-radius: 4px; background: var(--mf-surface-muted, #f4f5f6); }
.assistant-markdown blockquote { margin-left: 0; padding-left: 16px; border-left: 3px solid var(--mf-border); color: var(--mf-text-muted); }
.assistant-markdown table { display: block; max-width: 100%; overflow-x: auto; border-collapse: collapse; font-size: 13px; }
.assistant-markdown th, .assistant-markdown td { border: 1px solid var(--mf-border); padding: 8px 12px; text-align: left; }
.assistant-markdown a { color: var(--mf-primary); text-decoration: underline; text-underline-offset: 3px; }
.assistant-markdown img { max-width: 100%; height: auto; border-radius: 8px; }
</style>
