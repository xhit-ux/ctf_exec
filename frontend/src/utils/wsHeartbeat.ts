// WebSocket 心跳客户端 + 任务状态推送接收。
//
// 设计要点：
//   - 浏览器 WebSocket 自动响应服务端 ping 帧。
//   - 服务端通过 cookie access token 鉴权（credentials: 'include'）。
//   - onclose 指数退避重连；refresh 失败后停止重连。
//   - 支持注册 message handler 接收后端推送的业务事件（如 task_status）。

import { apiUrl } from './api'

type ConnectState = 'idle' | 'connecting' | 'open' | 'closed'

const DEFAULT_HEARTBEAT_INTERVAL_MS = 30_000
const RECONNECT_BASE_DELAY_MS = 1_000
const RECONNECT_MAX_DELAY_MS = 30_000
const CLOSE_POLICY_VIOLATION = 1008

export type WSMessageHandler = (data: any) => void

class WSHeartbeat {
  private ws: WebSocket | null = null
  private state: ConnectState = 'idle'
  private reconnectAttempts = 0
  private reconnectTimer: number | null = null
  private aliveCheckTimer: number | null = null
  private intentionallyClosed = false
  private opts: Required<{ heartbeatIntervalMs: number; debug: boolean }>
  private lastMessageAt = 0
  private messageHandlers: Set<WSMessageHandler> = new Set()

  constructor(opts: { heartbeatIntervalMs?: number; debug?: boolean } = {}) {
    this.opts = {
      heartbeatIntervalMs: opts.heartbeatIntervalMs ?? DEFAULT_HEARTBEAT_INTERVAL_MS,
      debug: opts.debug ?? false,
    }
  }

  addMessageListener(fn: WSMessageHandler) { this.messageHandlers.add(fn) }
  removeMessageListener(fn: WSMessageHandler) { this.messageHandlers.delete(fn) }

  private buildWsUrl(): string {
    const httpUrl = apiUrl('/api/ws/connect')
    if (/^https?:\/\//i.test(httpUrl)) {
      return httpUrl.replace(/^http/i, (m) => (m.toLowerCase() === 'https' ? 'wss' : 'ws'))
    }
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    return `${protocol}//${window.location.host}${httpUrl.startsWith('/') ? httpUrl : `/${httpUrl}`}`
  }

  private log(...args: unknown[]) {
    if (this.opts.debug) console.debug('[ws]', ...args)
  }

  private scheduleReconnect() {
    if (this.intentionallyClosed || this.reconnectTimer !== null) return
    const attempts = Math.min(this.reconnectAttempts, 10)
    const delay = Math.min(RECONNECT_BASE_DELAY_MS * Math.pow(2, attempts), RECONNECT_MAX_DELAY_MS) + Math.random() * 1_000
    this.log('schedule reconnect in', Math.round(delay), 'ms')
    this.reconnectAttempts += 1
    this.reconnectTimer = window.setTimeout(() => { this.reconnectTimer = null; this.connect() }, delay)
  }

  private clearAliveCheck() {
    if (this.aliveCheckTimer !== null) { window.clearInterval(this.aliveCheckTimer); this.aliveCheckTimer = null }
  }

  private startAliveCheck() {
    this.clearAliveCheck()
    this.lastMessageAt = Date.now()
    const timeoutMs = this.opts.heartbeatIntervalMs * 3
    this.aliveCheckTimer = window.setInterval(() => {
      if (Date.now() - this.lastMessageAt > timeoutMs) {
        this.log('alive check timeout, force reconnect')
        this.forceReopen()
      }
    }, this.opts.heartbeatIntervalMs)
  }

  private forceReopen() {
    if (this.ws) { try { this.ws.close() } catch { /* noop */ } }
  }

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

    socket.onmessage = (event: MessageEvent) => {
      this.lastMessageAt = Date.now()
      try {
        const data = JSON.parse(event.data)
        if (data && data.type === 'task_status') {
          console.log('[ws] received task_status:', data.status, 'task:', data.task_id, 'listeners:', this.messageHandlers.size)
          for (const fn of this.messageHandlers) {
            try { fn(data) } catch (e) { console.error('[ws] listener error:', e) }
          }
        }
      } catch { /* non-JSON (ping), ignore */ }
    }

    socket.onerror = (event) => { this.log('error', event) }

    socket.onclose = (event) => {
      this.log('close', event.code, event.reason)
      this.state = 'closed'
      this.clearAliveCheck()
      this.ws = null
      if (this.intentionallyClosed) return
      if (event.code === CLOSE_POLICY_VIOLATION) return
      this.scheduleReconnect()
    }
  }

  disconnect() {
    this.intentionallyClosed = true
    if (this.reconnectTimer !== null) { window.clearTimeout(this.reconnectTimer); this.reconnectTimer = null }
    this.clearAliveCheck()
    if (this.ws) { try { this.ws.close(1000, 'client logout') } catch { /* noop */ }; this.ws = null }
    this.state = 'idle'
    this.reconnectAttempts = 0
  }
}

const debugFlag = (() => {
  if (typeof import.meta === 'undefined') return false
  // @ts-ignore
  return Boolean(import.meta.env?.VITE_WS_DEBUG)
})()

export const wsHeartbeat = new WSHeartbeat({ debug: debugFlag })
