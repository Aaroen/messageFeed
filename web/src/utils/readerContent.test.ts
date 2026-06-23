import { describe, expect, it } from 'vitest'

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

  it('formats item dates for Chinese locale display', () => {
    expect(formatItemDate('2026-06-23T04:05:00.000Z')).toMatch(/06\/23.*12:05|06\/23.*04:05/)
  })
})
