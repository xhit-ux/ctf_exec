import { apiUrl } from './api'

const ACCESS_TOKEN_KEY = 'token'
const REFRESH_TOKEN_KEY = 'refresh_token'
export const AUTH_REFRESH_FAILED_EVENT = 'auth:refresh-failed'

let refreshInFlight: Promise<string | null> | null = null
let refreshFailureBroadcasted = false

function normalizeUrl(input: RequestInfo | URL): string {
  if (typeof input === 'string') return input
  if (input instanceof URL) return input.toString()
  return input.url
}

function shouldBypassAutoRefresh(url: string): boolean {
  return url.includes('/api/auth/login') ||
    url.includes('/api/auth/register') ||
    url.includes('/api/auth/send-register-code') ||
    url.includes('/api/auth/refresh')
}

function readAccessToken(): string | null {
  return localStorage.getItem(ACCESS_TOKEN_KEY)
}

function readRefreshToken(): string | null {
  return localStorage.getItem(REFRESH_TOKEN_KEY)
}

function writeTokenPair(accessToken: string, refreshToken: string) {
  localStorage.setItem(ACCESS_TOKEN_KEY, accessToken)
  localStorage.setItem(REFRESH_TOKEN_KEY, refreshToken)
  refreshFailureBroadcasted = false
}

function clearTokenPair() {
  localStorage.removeItem(ACCESS_TOKEN_KEY)
  localStorage.removeItem(REFRESH_TOKEN_KEY)
}

async function performRefresh(refreshToken: string): Promise<string | null> {
  const response = await fetch(apiUrl('/api/auth/refresh'), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ refresh_token: refreshToken }),
  })

  if (!response.ok) {
    clearTokenPair()
    return null
  }

  const data = await response.json()
  if (!data?.token || !data?.refresh_token) {
    clearTokenPair()
    return null
  }

  writeTokenPair(data.token, data.refresh_token)
  return data.token
}

export async function refreshAccessTokenWithLock(): Promise<string | null> {
  const refreshToken = readRefreshToken()
  if (!refreshToken) return null

  if (!refreshInFlight) {
    refreshInFlight = performRefresh(refreshToken).finally(() => {
      refreshInFlight = null
    })
  }

  return refreshInFlight
}

export function setAuthTokens(accessToken: string, refreshToken: string) {
  writeTokenPair(accessToken, refreshToken)
}

export function clearAuthTokens() {
  clearTokenPair()
}

export type ApiFetchOptions = {
  disableAuth?: boolean
  disableRetry401?: boolean
  accessTokenOverride?: string
}

function broadcastRefreshFailed(url: string) {
  if (refreshFailureBroadcasted) return
  refreshFailureBroadcasted = true
  window.dispatchEvent(new CustomEvent(AUTH_REFRESH_FAILED_EVENT, {
    detail: { url },
  }))
}

export async function apiFetch(input: RequestInfo | URL, init?: RequestInit, options?: ApiFetchOptions): Promise<Response> {
  const url = normalizeUrl(input)
  const headers = new Headers(init?.headers)

  if (options?.accessTokenOverride && !headers.has('Authorization')) {
    headers.set('Authorization', `Bearer ${options.accessTokenOverride}`)
  } else if (!options?.disableAuth && !headers.has('Authorization')) {
    const token = readAccessToken()
    if (token) {
      headers.set('Authorization', `Bearer ${token}`)
    }
  }

  const execute = (accessToken?: string) => {
    const requestHeaders = new Headers(headers)
    if (accessToken && !options?.disableAuth && !options?.accessTokenOverride) {
      requestHeaders.set('Authorization', `Bearer ${accessToken}`)
    }

    return fetch(input, {
      ...init,
      headers: requestHeaders,
    })
  }

  const response = await execute()
  if (response.status !== 401) return response

  if (options?.disableRetry401 || shouldBypassAutoRefresh(url)) {
    return response
  }

  const newAccessToken = await refreshAccessTokenWithLock()
  if (!newAccessToken) {
    broadcastRefreshFailed(url)
    return response
  }

  return execute(newAccessToken)
}
