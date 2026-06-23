const forbiddenDetailTags = new Set([
  'base',
  'embed',
  'form',
  'iframe',
  'input',
  'link',
  'math',
  'meta',
  'noscript',
  'object',
  'script',
  'style',
  'svg',
  'title',
])

export function escapeHTML(value: string) {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

export function plainPreviewText(...values: Array<string | undefined>) {
  const value = values.find((item) => item?.trim())
  if (!value) {
    return ''
  }

  const input = value.trim()
  if (typeof DOMParser === 'undefined') {
    return input.replace(/\s+/g, ' ')
  }

  const documentFragment = new DOMParser().parseFromString(input, 'text/html')
  return (documentFragment.body.textContent || '').replace(/\s+/g, ' ').trim()
}

export function sanitizeDetailHTML(value: string) {
  const input = value.trim()
  if (!input || typeof DOMParser === 'undefined') {
    return fallbackSanitizeDetailHTML(input)
  }

  const documentFragment = new DOMParser().parseFromString(input, 'text/html')
  documentFragment.body.querySelectorAll('*').forEach((element) => {
    const tagName = element.tagName.toLowerCase()
    if (forbiddenDetailTags.has(tagName)) {
      element.remove()
      return
    }

    for (const attribute of Array.from(element.attributes)) {
      if (attributeIsUnsafe(attribute)) {
        element.removeAttribute(attribute.name)
      }
    }
  })
  return documentFragment.body.innerHTML.trim()
}

function attributeIsUnsafe(attribute: Attr) {
  const name = attribute.name.toLowerCase()
  const value = attribute.value.trim()
  if (name.startsWith('on') || name === 'style' || name === 'srcset' || name.startsWith('data-')) {
    return true
  }
  if (name === 'href' || name === 'src' || name === 'xlink:href') {
    return urlValueIsUnsafe(value)
  }
  return false
}

function urlValueIsUnsafe(value: string) {
  const normalized = value.replace(/[\u0000-\u001f\u007f\s]+/g, '').toLowerCase()
  return normalized.startsWith('javascript:') || normalized.startsWith('data:') || normalized.startsWith('vbscript:')
}

function fallbackSanitizeDetailHTML(input: string) {
  return input
    .replace(/<script[\s\S]*?<\/script>/gi, '')
    .replace(/<style[\s\S]*?<\/style>/gi, '')
    .replace(/<\/?(?:html|head|body)[^>]*>/gi, '')
    .replace(/\s+on[a-z]+\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)/gi, '')
    .replace(/\s+(?:href|src)\s*=\s*(["'])\s*javascript:[\s\S]*?\1/gi, '')
    .replace(/\s+(?:href|src)\s*=\s*(["'])\s*data:[\s\S]*?\1/gi, '')
    .replace(/\s+(?:href|src)\s*=\s*(["'])\s*vbscript:[\s\S]*?\1/gi, '')
    .replace(/\s+style\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)/gi, '')
    .replace(/\s+srcset\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)/gi, '')
    .replace(/\s+data-[a-z0-9_-]+\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)/gi, '')
}

export function formatItemDate(value?: string) {
  if (!value) {
    return '时间未知'
  }
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(new Date(value))
}
