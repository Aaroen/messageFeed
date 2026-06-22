type RequestTokenOptions = {
  isActive?: () => boolean
}

export function useRequestToken(options: RequestTokenOptions = {}) {
  let currentToken = 0

  function next() {
    currentToken += 1
    return currentToken
  }

  function invalidate() {
    currentToken += 1
  }

  function set(token: number) {
    currentToken = token
  }

  function isCurrent(token?: number) {
    const active = options.isActive?.() ?? true
    return active && (token === undefined || token === currentToken)
  }

  function current() {
    return currentToken
  }

  return {
    next,
    invalidate,
    set,
    isCurrent,
    current,
  }
}
