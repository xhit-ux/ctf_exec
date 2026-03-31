import { apiUrl } from './api'
import { readSafeErrorMessage } from './error'
import { publishGlobalHttpError } from './globalErrorBus'

const ACCESS_TOKEN_KEY = 'token'
const REFRESH_TOKEN_KEY = 'refresh_token'
const USER_KEY = 'user'
export const AUTH_REFRESH_FAILED_EVENT = 'auth:refresh-failed'
export const AUTH_USER_UPDATED_EVENT = 'auth:user-updated'

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

function writeStoredUser(user: unknown) {
  if (!user) return
  localStorage.setItem(USER_KEY, JSON.stringify(user))
  window.dispatchEvent(new CustomEvent(AUTH_USER_UPDATED_EVENT, {
    detail: { user },
  }))
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
  if (data.user) {
    writeStoredUser(data.user)
  }
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
  disableGlobalError?: boolean
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
  const method = (init?.method || 'GET').toUpperCase()
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

  const executeWithNetworkFallback = async (accessToken?: string, suppressGlobalError = false) => {
    try {
      return await execute(accessToken)
    } catch {
      try {
        return await execute(accessToken)
      } catch (err) {
        if (!options?.disableGlobalError && !suppressGlobalError) {
          publishGlobalHttpError({
            status: 0,
            message: '网络连接异常，请检查网络后重试',
            url,
            method,
          })
        }
        throw err
      }
    }
  }

  let response: Response
  try {
    response = await executeWithNetworkFallback()
  } catch (err) {
    throw err
  }

  if (response.status !== 401) {
    if (!response.ok && !options?.disableGlobalError) {
      const message = await readSafeErrorMessage(response.clone())
      publishGlobalHttpError({
        status: response.status,
        message,
        url,
        method,
      })
    }

    return response
  }

  if (options?.disableRetry401 || shouldBypassAutoRefresh(url)) {
    return response
  }

  const newAccessToken = await refreshAccessTokenWithLock()
  if (!newAccessToken) {
    broadcastRefreshFailed(url)
    return response
  }

  const retriedResponse = await executeWithNetworkFallback(newAccessToken, true)
  if (!retriedResponse.ok && retriedResponse.status !== 401 && !options?.disableGlobalError) {
    const message = await readSafeErrorMessage(retriedResponse.clone())
    publishGlobalHttpError({
      status: retriedResponse.status,
      message,
      url,
      method,
    })
  }

  return retriedResponse
}
