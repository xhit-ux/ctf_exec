// WebSocket 心跳客户端：与后端 /api/ws/connect 维持长连接，仅用于在线状态上报。
//
// 设计要点：
//   - 浏览器 WebSocket 自动响应服务端 ping 帧，前端无需手动发 pong（也无法发）。
//   - 服务端通过 cookie 中的 access token 完成鉴权，凭证由 credentials: 'include' 携带。
//   - onclose 时计算指数退避（带抖动）后重连；refresh 失败事件触发后停止重连。
//   - 同一 tab 内全局单例：避免多次 mount/unmount 造成的连接风暴。

import { apiUrl } from './api'

type ConnectState = 'idle' | 'connecting' | 'open' | 'closed'

// 与后端 WS_CLIENT_HEARTBEAT_INTERVAL_SEC 默认值保持一致；
// 这个值仅用于本地的存活检测兜底，实际心跳由服务端发送 ping 主导。
const DEFAULT_HEARTBEAT_INTERVAL_MS = 30_000

const RECONNECT_BASE_DELAY_MS = 1_000
const RECONNECT_MAX_DELAY_MS = 30_000

// CloseCode 1008 (Policy Violation) 通常意味着鉴权失败或连接数超限，应停止自动重连。
const CLOSE_POLICY_VIOLATION = 1008

export interface WSHeartbeatOptions {
  // 心跳间隔（毫秒）；仅作存活兜底用。
  heartbeatIntervalMs?: number
  // 调试日志开关。
  debug?: boolean
}

class WSHeartbeat {
  private ws: WebSocket | null = null
  private state: ConnectState = 'idle'
  private reconnectAttempts = 0
  private reconnectTimer: number | null = null
  private aliveCheckTimer: number | null = null
  private intentionallyClosed = false
  private opts: Required<WSHeartbeatOptions>
  private lastMessageAt = 0

  constructor(opts: WSHeartbeatOptions = {}) {
    this.opts = {
      heartbeatIntervalMs: opts.heartbeatIntervalMs ?? DEFAULT_HEARTBEAT_INTERVAL_MS,
      debug: opts.debug ?? false,
    }
  }

  private buildWsUrl(): string {
    const httpUrl = apiUrl('/api/ws/connect')

    if (/^https?:\/\//i.test(httpUrl)) {
      return httpUrl.replace(/^http/i, (m) => (m.toLowerCase() === 'https' ? 'wss' : 'ws'))
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    return `${protocol}//${window.location.host}${httpUrl.startsWith('/') ? httpUrl : `/${httpUrl}`}`
  }

  private log(...args: unknown[]) {
    if (this.opts.debug) {
      // eslint-disable-next-line no-console
      console.debug('[ws]', ...args)
    }
  }

  private scheduleReconnect() {
    if (this.intentionallyClosed) return
    if (this.reconnectTimer !== null) return

    const attempts = Math.min(this.reconnectAttempts, 10)
    const exponential = RECONNECT_BASE_DELAY_MS * Math.pow(2, attempts)
    const capped = Math.min(exponential, RECONNECT_MAX_DELAY_MS)
    const jitter = Math.random() * 1_000
    const delay = capped + jitter

    this.log('schedule reconnect in', Math.round(delay), 'ms (attempt', attempts + 1, ')')
    this.reconnectAttempts += 1
    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = null
      this.connect()
    }, delay)
  }

  private clearAliveCheck() {
    if (this.aliveCheckTimer !== null) {
      window.clearInterval(this.aliveCheckTimer)
      this.aliveCheckTimer = null
    }
  }

  private startAliveCheck() {
    this.clearAliveCheck()
    this.lastMessageAt = Date.now()
    // 兜底：超过心跳间隔 3 倍未收到任何帧（含 ping），主动重连。
    const timeoutMs = this.opts.heartbeatIntervalMs * 3
    this.aliveCheckTimer = window.setInterval(() => {
      if (Date.now() - this.lastMessageAt > timeoutMs) {
        this.log('alive check timeout, force reconnect')
        this.forceReopen()
      }
    }, this.opts.heartbeatIntervalMs)
  }

  private forceReopen() {
    if (this.ws) {
      try {
        this.ws.close()
      } catch {
        /* noop */
      }
    }
  }

  /** 发起连接。多次调用幂等：已连接或正在连接时直接返回。 */
  connect() {
    if (this.state === 'open' || this.state === 'connecting') return
    this.intentionallyClosed = false

    let socket: WebSocket
    try {
      socket = new WebSocket(this.buildWsUrl())
    } catch (err) {
      this.log('construct WebSocket failed', err)
      this.state = 'closed'
      this.scheduleReconnect()
      return
    }

    this.ws = socket
    this.state = 'connecting'

    socket.onopen = () => {
      this.log('open')
      this.state = 'open'
      this.reconnectAttempts = 0
      this.lastMessageAt = Date.now()
      this.startAliveCheck()
    }

    socket.onmessage = () => {
      this.lastMessageAt = Date.now()
    }

    socket.onerror = (event) => {
      this.log('error', event)
    }

    socket.onclose = (event) => {
      this.log('close', event.code, event.reason)
      this.state = 'closed'
      this.clearAliveCheck()
      this.ws = null

      if (this.intentionallyClosed) return
      if (event.code === CLOSE_POLICY_VIOLATION) {
        // 鉴权失败 / 连接超限：放弃自动重连，等待应用层显式调用 connect()。
        this.log('policy violation, stop auto reconnect')
        return
      }
      this.scheduleReconnect()
    }
  }

  /** 主动断开连接，并阻止后续自动重连。 */
  disconnect() {
    this.intentionallyClosed = true
    if (this.reconnectTimer !== null) {
      window.clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    this.clearAliveCheck()
    if (this.ws) {
      try {
        this.ws.close(1000, 'client logout')
      } catch {
        /* noop */
      }
      this.ws = null
    }
    this.state = 'idle'
    this.reconnectAttempts = 0
  }
}

const debugFlag = (() => {
  if (typeof import.meta === 'undefined') return false
  // @ts-ignore vite-injected env
  return Boolean(import.meta.env?.VITE_WS_DEBUG)
})()

export const wsHeartbeat = new WSHeartbeat({ debug: debugFlag })
