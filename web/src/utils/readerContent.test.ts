import { ref } from 'vue'
import { describe, expect, it } from 'vitest'

import { useReaderDetailFrame } from '@/composables/useReaderDetailFrame'
import { formatItemDate, plainPreviewText, sanitizeDetailHTML } from '@/utils/readerContent'

describe('readerContent', () => {
  it('extracts plain preview text from HTML content', () => {
    expect(plainPreviewText('<p>第一段&nbsp;<strong>重点</strong></p>')).toBe('第一段 重点')
  })

  it('sanitizes active HTML content for detail rendering', () => {
    const sanitized = sanitizeDetailHTML(`
      <article onclick="alert(1)" style="color: red">
        <script>alert(1)</script>
        <a href="javascript:alert(1)" data-track-id="1">危险链接</a>
        <img src="https://example.com/a.png" srcset="https://example.com/a2.png 2x" onerror="alert(1)">
        <iframe src="https://example.com"></iframe>
      </article>
    `)

    expect(sanitized).toContain('<article>')
    expect(sanitized).toContain('<a>危险链接</a>')
    expect(sanitized).toContain('<img src="https://example.com/a.png">')
    expect(sanitized).not.toContain('script')
    expect(sanitized).not.toContain('onclick')
    expect(sanitized).not.toContain('onerror')
    expect(sanitized).not.toContain('javascript:')
    expect(sanitized).not.toContain('srcset')
    expect(sanitized).not.toContain('style=')
    expect(sanitized).not.toContain('iframe')
    expect(sanitized).not.toContain('data-track-id')
  })

  it('converts supported source offsets to Taiwan time, including midnight', () => {
    const item = { published_at: '2026-06-23T17:05:00+01:00', fetched_at: '2026-06-24T06:00:00Z' }
    expect(formatItemDate(item)).toEqual({
      datetime: '2026-06-23T16:05:00.000Z',
      text: expect.stringMatching(/^发布于 06\/24.*00:05$/),
    })
  })

  it('labels the fetch date when the published date is absent or invalid', () => {
    const item = { fetched_at: '2026-06-23T06:00:00Z' }
    const fetched = formatItemDate(item)
    expect(fetched).toEqual({
      datetime: '2026-06-23T06:00:00.000Z',
      text: expect.stringMatching(/^拉取于 06\/23.*14:00$/),
    })
    expect(formatItemDate({ ...item, published_at: 'invalid' })).toEqual(fetched)
    expect(formatItemDate({ fetched_at: 'invalid' })).toEqual({ text: '时间未知' })
  })

  it('syncs reader iframe theme attribute and preserves detail gesture bridge script', () => {
    const darkTheme = ref(false)
    const item = ref({
      id: 42,
      source_id: 7,
      title: '测试文章',
      url: 'https://example.com/post',
      summary: '摘要',
      content_html: '<p>正文内容</p>',
      is_read: false,
      is_favorite: false,
      is_hidden: false,
      fetched_at: '2026-06-23T06:00:00Z',
    })

    const frame = useReaderDetailFrame({
      item,
      darkTheme,
      metricsInitialDelay: 120,
      metricsSettledDelay: 420,
    })

    expect(frame.srcdoc.value).toContain('<html data-theme="light">')
    expect(frame.srcdoc.value).toContain('color-scheme: light;')
    expect(frame.srcdoc.value).toContain("type: 'messagefeed-detail-gesture'")

    darkTheme.value = true

    expect(frame.srcdoc.value).toContain('<html data-theme="dark">')
    expect(frame.srcdoc.value).toContain('color-scheme: dark;')
    expect(frame.srcdoc.value).toContain("type: 'messagefeed-detail-gesture'")
  })
})
