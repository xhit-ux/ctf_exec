import React, { createContext, useEffect, useRef, useState, ReactNode } from 'react'
import { apiUrl } from '../utils/api'
import { AUTH_REFRESH_FAILED_EVENT, apiFetch, clearAuthTokens, refreshAccessTokenWithLock, setAuthTokens, shouldPreRefreshBeforeBootstrap } from '../utils/authClient'
import { readSafeErrorMessage } from '../utils/error'
import { publishGlobalHttpError } from '../utils/globalErrorBus'
import { wsHeartbeat } from '../utils/wsHeartbeat'

export interface User {
  id: string
  username: string
  email?: string
  role: string
  display_name: string
  avatar_url?: string
  bio?: string
  score: number
  solved_count: number
  is_active: boolean
  is_verified?: boolean
  last_login_at?: string
  last_active_at?: string
  created_at?: string
  updated_at?: string
  preferences?: string  // JSON 字符串存储用户偏好
}

export interface AuthContextType {
  user: User | null
  isAuthenticated: boolean
  isLoading: boolean
  login: (user: User) => void
  logout: () => void
  setUser: (user: User) => void
  refreshUser?: (silent?: boolean) => Promise<void>
}

export const AuthContext = createContext<AuthContextType>({
  user: null,
  isAuthenticated: false,
  isLoading: true,
  login: () => { },
  logout: () => { },
  setUser: () => { },
})

export const AuthProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
  const [user, setUser] = useState<User | null>(null)

  const [isLoading, setIsLoading] = useState(true)
  const initRanRef = useRef(false)

  // Single cleanup entrypoint for all auth-related client state.
  const clearAuthState = () => {
    setUser(null)
    clearAuthTokens()
    wsHeartbeat.disconnect()
  }

  // 验证 token 有效性并刷新用户信息
  useEffect(() => {
    if (initRanRef.current) return
    initRanRef.current = true

    const initAuth = async () => {
      try {
        // 迁移清理历史版本遗留的本地用户缓存。
        localStorage.removeItem('user')

        if (shouldPreRefreshBeforeBootstrap()) {
          await refreshAccessTokenWithLock()
        }

        await refreshUser(true)
      } finally {
        setIsLoading(false)
      }
    }
    initAuth()
  }, [])

  useEffect(() => {
    const onRefreshFailed = () => {
      clearAuthState()
      window.location.replace('/login')
    }

    window.addEventListener(AUTH_REFRESH_FAILED_EVENT, onRefreshFailed)
    return () => {
      window.removeEventListener(AUTH_REFRESH_FAILED_EVENT, onRefreshFailed)
    }
  }, [])

  const login = (newUser: User) => {
    setUser(newUser)
    // 登录成功后立即建立 WS 心跳，作为后续 presence 与互踢检测的载体。
    wsHeartbeat.connect()
  }

  const logout = async () => {
    // 先调用服务端撤销会话，再清空本地状态，避免「本地已登出但服务端会话仍然存活」造成的
    // 攻击面：单会话角色（admin）此时仍可被旧 access/refresh token 复用。
    //
    // 失败兜底：服务端不可达时仍然要继续清本地并跳转登录页（用户感知到「已登出」），
    // 同时往全局错误总线发一条提示，避免静默吞错。
    let serverLogoutFailed = false
    try {
      const response = await apiFetch(apiUrl('/api/auth/logout'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      }, {
        // logout 自身不重试 401（服务端 401 时同样意味着本地状态可清）
        disableRetry401: true,
        suppressRefreshFailedEvent: true,
        // 401 / 网络错误不弹全局错误，由本函数统一处理
        disableGlobalError: true,
      })
      if (!response.ok && response.status !== 401) {
        serverLogoutFailed = true
      }
    } catch {
      serverLogoutFailed = true
    }

    if (serverLogoutFailed) {
      publishGlobalHttpError({
        status: 0,
        message: '登出请求未完成，本地登录状态已清除；如担心账户安全请尽快修改密码',
        url: apiUrl('/api/auth/logout'),
        method: 'POST',
      })
    }

    clearAuthState()
    window.location.replace('/login')
  }

  const refreshUser = async (silent = false) => {
    try {
      const response = await apiFetch(apiUrl('/api/auth/me'), undefined, {
        suppressRefreshFailedEvent: silent,
      })

      if (response.ok) {
        const accessExpiresInSecs = Number(response.headers.get('X-Access-Expires-In-Secs'))
        if (Number.isFinite(accessExpiresInSecs) && accessExpiresInSecs > 0) {
          setAuthTokens(accessExpiresInSecs)
        }

        const userData = await response.json()
        // /api/auth/me 直接返回用户对象，不是嵌套在 data.user 中
        setUser(userData)
        // 刷新页面后用户依旧在线：开启 WS 心跳以恢复 presence 上报。
        wsHeartbeat.connect()
      } else {
        if (response.status === 401) {
          // 主动清理 auth 状态，避免依赖事件派发时序。
          clearAuthState()
          return
        } else {
          const message = await readSafeErrorMessage(response.clone(), '刷新用户信息失败，请稍后重试')
          publishGlobalHttpError({
            status: response.status,
            message,
            url: apiUrl('/api/auth/me'),
            method: 'GET',
          })
        }
      }
    } catch {
      publishGlobalHttpError({
        status: 0,
        message: '网络连接异常，请检查网络后重试',
        url: apiUrl('/api/auth/me'),
        method: 'GET',
      })
    }
  }

  const value: AuthContextType = {
    user,
    isAuthenticated: !!user,
    isLoading,
    login,
    logout,
    setUser,
    refreshUser,
  }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export const useAuth = () => {
  const context = React.useContext(AuthContext)
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider')
  }
  return context
}
