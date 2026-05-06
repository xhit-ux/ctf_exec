import { apiUrl } from './api'
import { readSafeErrorMessage } from './error'
import { publishGlobalHttpError } from './globalErrorBus'

export const AUTH_REFRESH_FAILED_EVENT = 'auth:refresh-failed'

let refreshInFlight: Promise<boolean> | null = null
let refreshFailureBroadcasted = false
let proactiveRefreshTimer: number | null = null

const ACCESS_EXPIRES_AT_KEY = 'auth_access_expires_at_ms'

const PREEMPTIVE_REFRESH_AHEAD_SECONDS = 180
const MIN_PREEMPTIVE_REFRESH_DELAY_MS = 30_000

function normalizeUrl(input: RequestInfo | URL): string {
  if (typeof input === 'string') return input
  if (input instanceof URL) return input.toString()
  return input.url
}

function shouldBypassAutoRefresh(url: string): boolean {
  return url.includes('/api/auth/login') ||
    url.includes('/api/auth/register') ||
    url.includes('/api/auth/send-register-code') ||
    url.includes('/api/auth/send-password-reset-code') ||
    url.includes('/api/auth/reset-password') ||
    url.includes('/api/auth/refresh')
}

/**
 * onAccessTokenIssued 在新 access token 签发后调用，仅做前端态一致性维护：
 * 重置「刷新失败」广播标志位，避免上一轮过期态泄漏到本轮。
 *
 * 注意：access/refresh token 实际承载在 HttpOnly Cookie 中，JS 不直接接触；
 * 本函数和文件名中的 "Token" 仅是「token 生命周期事件」的语义。
 */
function onAccessTokenIssued() {
  refreshFailureBroadcasted = false
}

function persistAccessExpireAt(accessExpiresInSecs: number) {
  if (!Number.isFinite(accessExpiresInSecs) || accessExpiresInSecs <= 0) return
  const expiresAtMs = Date.now() + accessExpiresInSecs * 1000
  window.sessionStorage.setItem(ACCESS_EXPIRES_AT_KEY, String(expiresAtMs))
}

function clearAccessExpireAt() {
  window.sessionStorage.removeItem(ACCESS_EXPIRES_AT_KEY)
}

export function shouldPreRefreshBeforeBootstrap(): boolean {
  const raw = window.sessionStorage.getItem(ACCESS_EXPIRES_AT_KEY)
  if (!raw) return false

  const expiresAtMs = Number(raw)
  if (!Number.isFinite(expiresAtMs) || expiresAtMs <= 0) {
    clearAccessExpireAt()
    return false
  }

  // 若已进入“提前刷新窗口”则优先 refresh，避免启动时先 hit 到 401。
  return Date.now() >= expiresAtMs-PREEMPTIVE_REFRESH_AHEAD_SECONDS*1000
}

function clearProactiveRefreshTimer() {
  if (proactiveRefreshTimer !== null) {
    window.clearTimeout(proactiveRefreshTimer)
    proactiveRefreshTimer = null
  }
}

function schedulePreemptiveRefresh(accessExpiresInSecs: number) {
  if (!Number.isFinite(accessExpiresInSecs) || accessExpiresInSecs <= 0) return

  persistAccessExpireAt(accessExpiresInSecs)
  clearProactiveRefreshTimer()

  const refreshDelayMs = Math.max(
    (accessExpiresInSecs - PREEMPTIVE_REFRESH_AHEAD_SECONDS) * 1000,
    MIN_PREEMPTIVE_REFRESH_DELAY_MS,
  )

  proactiveRefreshTimer = window.setTimeout(() => {
    void refreshAccessTokenWithLock()
  }, refreshDelayMs)
}

/**
 * onAccessTokenCleared 在登出 / refresh 失败时调用：清理预刷新定时器和过期时间戳，
 * 但不会触碰任何 cookie（cookie 由后端 Set-Cookie 控制）。
 */
function onAccessTokenCleared() {
  clearProactiveRefreshTimer()
  clearAccessExpireAt()
}

async function performRefresh(): Promise<boolean> {
  const response = await fetch(apiUrl('/api/auth/refresh'), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    credentials: 'include',
  })

  if (!response.ok) {
    onAccessTokenCleared()
    return false
  }

  const data = await response.json()
  onAccessTokenIssued()

  if (typeof data?.access_expires_in_secs === 'number') {
    schedulePreemptiveRefresh(data.access_expires_in_secs)
  }

  refreshFailureBroadcasted = false
  return true
}

export async function refreshAccessTokenWithLock(): Promise<boolean> {
  if (!refreshInFlight) {
    refreshInFlight = performRefresh().finally(() => {
      refreshInFlight = null
    })
  }

  return refreshInFlight
}

export function setAuthTokens(accessExpiresInSecs?: number) {
  onAccessTokenIssued()
  if (typeof accessExpiresInSecs === 'number') {
    schedulePreemptiveRefresh(accessExpiresInSecs)
  }
}

export function clearAuthTokens() {
  onAccessTokenCleared()
}

export type ApiFetchOptions = {
  disableAuth?: boolean
  disableRetry401?: boolean
  accessTokenOverride?: string
  disableGlobalError?: boolean
  suppressRefreshFailedEvent?: boolean
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
  }

  const execute = (accessToken?: string) => {
    const requestHeaders = new Headers(headers)
    if (accessToken && !options?.disableAuth && !options?.accessTokenOverride) {
      requestHeaders.set('Authorization', `Bearer ${accessToken}`)
    }

    return fetch(input, {
      ...init,
      credentials: init?.credentials ?? 'include',
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

  const refreshed = await refreshAccessTokenWithLock()
  if (!refreshed) {
    if (!options?.suppressRefreshFailedEvent) {
      broadcastRefreshFailed(url)
    }
    return response
  }

  const retriedResponse = await executeWithNetworkFallback(undefined, true)
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
