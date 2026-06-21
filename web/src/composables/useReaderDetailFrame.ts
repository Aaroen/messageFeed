import { computed } from 'vue'

import type { FeedItem } from '@/api/feed'
import { escapeHTML, formatItemDate, plainPreviewText, sanitizeDetailHTML } from '@/utils/readerContent'

type ReaderDetailFrameOptions = {
  item: {
    readonly value: FeedItem | null | undefined
  }
  metricsInitialDelay: number
  metricsSettledDelay: number
}

function hashFrameSource(value: string) {
  let hash = 2166136261
  for (let index = 0; index < value.length; index += 1) {
    hash ^= value.charCodeAt(index)
    hash = Math.imul(hash, 16777619)
  }
  return (hash >>> 0).toString(36)
}

export function useReaderDetailFrame(options: ReaderDetailFrameOptions) {
  const html = computed(() => options.item.value?.content_html || options.item.value?.content_snippet || '')
  const text = computed(
    () =>
      options.item.value?.content_text ||
      options.item.value?.summary ||
      options.item.value?.content_snippet ||
      '',
  )
  const previewSummary = computed(
    () =>
      plainPreviewText(
        options.item.value?.summary,
        options.item.value?.content_snippet,
        options.item.value?.content_text,
        options.item.value?.content_html,
      ) || '暂无摘要。',
  )
  const displayDate = computed(() => formatItemDate(options.item.value?.published_at || options.item.value?.fetched_at))
  const body = computed(() => {
    const source = html.value || `<p>${escapeHTML(text.value || '暂无正文。')}</p>`
    return sanitizeDetailHTML(source)
  })
  const frameId = computed(() => {
    const item = options.item.value
    return `detail-${hashFrameSource(
      [
        item?.id ?? '',
        item?.source_id ?? '',
        item?.url ?? '',
        item?.published_at ?? '',
        item?.fetched_at ?? '',
        body.value,
      ].join('\u001f'),
    )}`
  })
  const srcdoc = computed(() => `<!doctype html>
<html>
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<base target="_blank" />
<style>
  :root { color-scheme: light dark; }
  * {
    scrollbar-width: none;
    -ms-overflow-style: none;
  }
  html {
    scrollbar-width: none;
    -ms-overflow-style: none;
    touch-action: pan-y;
  }
  body {
    margin: 0;
    padding: 0;
    background: transparent;
    color: #162033;
    font: 16px/1.72 -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    overflow-wrap: anywhere;
    overflow: hidden;
    scrollbar-width: none;
    -ms-overflow-style: none;
    touch-action: pan-y;
  }
  #messagefeed-detail-body {
    display: flow-root;
    min-height: 1px;
    overflow-wrap: anywhere;
  }
  *::-webkit-scrollbar,
  html::-webkit-scrollbar,
  body::-webkit-scrollbar {
    width: 0;
    height: 0;
    display: none;
  }
  img, video, iframe { max-width: 100%; height: auto; }
  pre, code { white-space: pre-wrap; overflow-wrap: anywhere; }
  a { color: #1d4ed8; }
  blockquote { margin: 1em 0; padding-left: 1em; border-left: 3px solid #d1d5db; color: #4b5563; }
  @media (prefers-color-scheme: dark) {
    body { color: #dbe6f3; background: transparent; }
    a { color: #93c5fd; }
    blockquote { border-left-color: #475569; color: #a9b6c6; }
  }
</style>
</head>
<body>
<main id="messagefeed-detail-body">
${body.value}
</main>
<script>
(() => {
  let startX = 0;
  let startY = 0;
  let tracking = false;
  let intent = null;
  let metricsTicking = false;
  let resizeObserver = null;
  const frameId = '${frameId.value}';
  const preferTouchEvents = 'ontouchstart' in window || navigator.maxTouchPoints > 0;
  const post = (phase, touch) => {
    window.parent.postMessage({
      type: 'messagefeed-detail-gesture',
      frameId,
      phase,
      startX,
      startY,
      x: touch.clientX,
      y: touch.clientY,
      dx: touch.clientX - startX,
      dy: touch.clientY - startY,
      source: 'detail-frame'
    }, '*');
  };
  const scrollMetrics = () => {
    const doc = document.documentElement;
    const body = document.body;
    const content = document.getElementById('messagefeed-detail-body');
    const contentRect = content?.getBoundingClientRect();
    const scrollHeight = Math.max(
      doc.scrollHeight || 0,
      body.scrollHeight || 0,
      content?.scrollHeight || 0,
      contentRect ? Math.ceil(contentRect.height) : 0
    );
    const clientHeight = window.innerHeight || doc.clientHeight || body.clientHeight || 0;
    return {
      scrollTop: 0,
      scrollHeight,
      clientHeight
    };
  };
  const postScrollMetrics = () => {
    window.parent.postMessage({
      type: 'messagefeed-detail-scroll',
      frameId,
      ...scrollMetrics()
    }, '*');
  };
  const requestScrollMetrics = () => {
    if (metricsTicking) return;
    metricsTicking = true;
    requestAnimationFrame(() => {
      metricsTicking = false;
      postScrollMetrics();
    });
  };
  window.addEventListener('resize', () => requestAnimationFrame(postScrollMetrics), { passive: true });
  window.addEventListener('message', (event) => {
    if (event.data?.type !== 'messagefeed-detail-scroll-to') return;
    if (event.data?.frameId && event.data.frameId !== frameId) return;
    requestAnimationFrame(postScrollMetrics);
  });
  window.addEventListener('load', () => {
    requestAnimationFrame(() => {
      postScrollMetrics();
      if ('ResizeObserver' in window) {
        resizeObserver = new ResizeObserver(() => requestAnimationFrame(postScrollMetrics));
        const content = document.getElementById('messagefeed-detail-body');
        resizeObserver.observe(document.documentElement);
        resizeObserver.observe(document.body);
        if (content) resizeObserver.observe(content);
      }
      window.setTimeout(postScrollMetrics, ${options.metricsInitialDelay});
      window.setTimeout(postScrollMetrics, ${options.metricsSettledDelay});
    });
  });
  window.addEventListener('touchstart', (event) => {
    if (!preferTouchEvents) return;
    if (event.touches.length !== 1) return;
    startX = event.touches[0].clientX;
    startY = event.touches[0].clientY;
    tracking = true;
    intent = null;
    post('start', event.touches[0]);
  }, { passive: true, capture: true });
  window.addEventListener('touchmove', (event) => {
    if (!preferTouchEvents) return;
    if (!tracking || event.touches.length !== 1) return;
    const touch = event.touches[0];
    const dx = touch.clientX - startX;
    const dy = touch.clientY - startY;
    const absX = Math.abs(dx);
    const absY = Math.abs(dy);
    if (!intent) {
      if (absX > 3 && absX > absY * 0.52) {
        intent = 'horizontal';
      } else {
        post('move', touch);
        requestScrollMetrics();
        return;
      }
    }
    if (event.cancelable) {
      event.preventDefault();
    }
    post('move', touch);
  }, { passive: false, capture: true });
  window.addEventListener('touchcancel', (event) => {
    if (!preferTouchEvents) return;
    const touch = event.changedTouches[0];
    if (tracking && touch) post('cancel', touch);
    requestScrollMetrics();
    tracking = false;
    intent = null;
  }, { passive: true, capture: true });
  window.addEventListener('touchend', (event) => {
    if (!preferTouchEvents) return;
    const touch = event.changedTouches[0];
    if (!touch) return;
    if (tracking) post('end', touch);
    requestScrollMetrics();
    tracking = false;
    intent = null;
  }, { passive: true, capture: true });
  if (!preferTouchEvents && window.PointerEvent) {
    let pointerTracking = false;
    let pointerIntent = null;
    let pointerId = null;
    window.addEventListener('pointerdown', (event) => {
      if (event.pointerType !== 'touch' || !event.isPrimary) return;
      pointerId = event.pointerId;
      startX = event.clientX;
      startY = event.clientY;
      pointerTracking = true;
      pointerIntent = null;
      post('start', event);
    }, { passive: true, capture: true });
    window.addEventListener('pointermove', (event) => {
      if (!pointerTracking || event.pointerId !== pointerId || event.pointerType !== 'touch') return;
      const dx = event.clientX - startX;
      const dy = event.clientY - startY;
      const absX = Math.abs(dx);
      const absY = Math.abs(dy);
      if (!pointerIntent) {
        if (absX > 3 && absX > absY * 0.52) {
          pointerIntent = 'horizontal';
        } else {
          post('move', event);
          requestScrollMetrics();
          return;
        }
      }
      if (event.cancelable) {
        event.preventDefault();
      }
      post('move', event);
    }, { passive: false, capture: true });
    window.addEventListener('pointercancel', (event) => {
      if (pointerTracking && event.pointerId === pointerId) post('cancel', event);
      requestScrollMetrics();
      pointerTracking = false;
      pointerIntent = null;
      pointerId = null;
    }, { passive: true, capture: true });
    window.addEventListener('pointerup', (event) => {
      if (pointerTracking && event.pointerId === pointerId) post('end', event);
      requestScrollMetrics();
      pointerTracking = false;
      pointerIntent = null;
      pointerId = null;
    }, { passive: true, capture: true });
  }
})();
<\/script>
</body>
</html>`)

  return {
    previewSummary,
    displayDate,
    frameId,
    srcdoc,
  }
}
