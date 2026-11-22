import { spawn } from 'node:child_process'
import { createServer } from 'node:http'
import { setTimeout as delay } from 'node:timers/promises'
import { once } from 'node:events'

const chrome = 'http://127.0.0.1:9222'
const rss = 'http://127.0.0.1:1200'
const xHosts = new Set(['x.com', 'www.x.com', 'twitter.com', 'www.twitter.com'])
let rsshub
let starting
let invalidToken

async function page() {
  const response = await fetch(`${chrome}/json/list`, { signal: AbortSignal.timeout(2500) })
  if (!response.ok) throw new Error('CDP unavailable')
  const targets = await response.json()
  return targets.find((target) => target.type === 'page' && xHosts.has(URL.parse(target.url)?.hostname))
}

async function command(target, method, params) {
  const endpoint = new URL(target.webSocketDebuggerUrl)
  if (endpoint.origin !== 'ws://127.0.0.1:9222' || !endpoint.pathname.startsWith('/devtools/page/')) {
    throw new Error('Invalid CDP target')
  }
  const socket = new WebSocket(endpoint)
  try {
    return await new Promise((resolve, reject) => {
      const timeout = setTimeout(() => reject(new Error('CDP timed out')), 2500)
      const finish = (error, result) => {
        clearTimeout(timeout)
        if (error) reject(error)
        else resolve(result)
      }
      socket.addEventListener('open', () => socket.send(JSON.stringify({ id: 1, method, params })))
      socket.addEventListener('error', () => finish(new Error('CDP unavailable')))
      socket.addEventListener('close', () => finish(new Error('CDP closed')))
      socket.addEventListener('message', (event) => {
        const reply = JSON.parse(event.data)
        if (reply.id === 1) finish(reply.error && new Error('CDP rejected command'), reply.result)
      })
    })
  } finally {
    socket.close()
  }
}

async function authToken(target) {
  if (!target) return undefined
  const { cookies } = await command(target, 'Network.getCookies', { urls: ['https://x.com/'] })
  const token = cookies.find((cookie) => cookie.name === 'auth_token')?.value
  return token && token !== invalidToken ? token : undefined
}

async function startRSSHub(token) {
  if (rsshub?.token === token && rsshub.ready) return
  if (rsshub) {
    const { child } = rsshub
    const exited = once(child, 'exit')
    child.kill()
    await exited
  }
  const child = spawn(process.execPath, ['/app/dist/index.mjs'], {
    stdio: 'ignore',
    env: { ...process.env, LISTEN_INADDR_ANY: '0', PORT: '1200', REQUEST_TIMEOUT: '18000', TWITTER_AUTH_TOKEN: token },
  })
  const current = { token, child, ready: false }
  rsshub = current
  const reset = () => { if (rsshub === current) rsshub = undefined }
  child.once('error', reset)
  child.once('exit', reset)
  for (let attempt = 0; attempt < 20 && rsshub === current; attempt++) {
    const ready = await fetch(`${rss}/healthz`, { signal: AbortSignal.timeout(500) }).catch(() => null)
    if (ready?.ok) {
      current.ready = true
      return
    }
    await delay(400)
  }
  child.kill()
  throw new Error('RSSHub unavailable')
}

async function rssRequest(token, route) {
  while (starting) await starting
  if (rsshub?.token !== token || !rsshub.ready) {
    starting = startRSSHub(token)
    try {
      await starting
    } finally {
      starting = undefined
    }
  }
  return fetch(`${rss}${route}`, { signal: AbortSignal.timeout(23000), redirect: 'error' })
}

function reply(response, status, body) {
  response.writeHead(status, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' })
  response.end(JSON.stringify(body))
}

async function handle(request, response) {
  if (request.method === 'GET' && request.url === '/healthz') return reply(response, 200, {})
  const readStatus = request.method === 'GET' && request.url === '/status'
  const insertText = request.method === 'POST' && request.url === '/input'
  const readFeed = request.method === 'GET' && /^\/twitter\/(home_latest|user\/[a-z0-9_]{1,15})$/.test(request.url)
  if (!readStatus && !insertText && !readFeed) return reply(response, 404, {})
  const target = await page()
  if (insertText) {
    let body = ''
    request.setEncoding('utf8')
    for await (const chunk of request) {
      body += chunk
      if (body.length > 8192) return reply(response, 413, {})
    }
    const { text } = JSON.parse(body)
    if (typeof text !== 'string' || text.length === 0 || text.length > 2048 || /[\r\n\0]/.test(text)) {
      return reply(response, 400, {})
    }
    if (!target) return reply(response, 409, {})
    const focused = await command(target, 'Runtime.evaluate', {
      expression: `document.activeElement instanceof HTMLInputElement && !document.activeElement.readOnly && !document.activeElement.disabled && !['button','checkbox','radio','file','submit','reset','hidden'].includes(document.activeElement.type)`,
      returnByValue: true,
    })
    if (focused.result?.value !== true) return reply(response, 409, {})
    await command(target, 'Input.insertText', { text })
    response.writeHead(204, { 'Cache-Control': 'no-store' }).end()
    return
  }
  const token = await authToken(target)
  if (readStatus) return reply(response, 200, { connected: Boolean(token) })
  if (!token) return reply(response, 401, {})
  const upstream = await rssRequest(token, request.url)
  if (!upstream.ok) {
    const errorPage = await upstream.text()
    if (upstream.status === 401 || /Twitter API error: 401|No valid Twitter token found|Could not authenticate you|Invalid or expired token|Bad Authentication data|Your account is locked/i.test(errorPage)) {
      invalidToken = token
      return reply(response, 401, {})
    }
    if (upstream.status === 429 || /Twitter API error: 429|Too Many Requests|Rate limit exceeded/i.test(errorPage)) return reply(response, 429, {})
    if (upstream.status === 404 || /This account doesn.t exist/i.test(errorPage)) return reply(response, 404, {})
    return reply(response, 502, {})
  }
  response.writeHead(200, { 'Content-Type': 'application/rss+xml; charset=utf-8', 'Cache-Control': 'no-store' })
  response.end(Buffer.from(await upstream.arrayBuffer()))
}

createServer((request, response) => {
  handle(request, response).catch(() => {
    if (response.headersSent) response.destroy()
    else reply(response, 502, {})
  })
}).listen(1201, '0.0.0.0')
