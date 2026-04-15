import React, { createContext, useEffect, useRef, useState, ReactNode } from 'react'
import { apiUrl } from '../utils/api'
import { AUTH_REFRESH_FAILED_EVENT, apiFetch, clearAuthTokens, setAuthTokens } from '../utils/authClient'
import { readSafeErrorMessage } from '../utils/error'
import { publishGlobalHttpError } from '../utils/globalErrorBus'

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
  }

  // 验证 token 有效性并刷新用户信息
  useEffect(() => {
    if (initRanRef.current) return
    initRanRef.current = true

    const initAuth = async () => {
      try {
        // 迁移清理历史版本遗留的本地用户缓存。
        localStorage.removeItem('user')
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
  }

  const logout = async () => {
    // 先清空本地状态，防止用户在等待期间继续操作
    clearAuthState()

    try {
      await apiFetch(apiUrl('/api/auth/logout'), {  // ← await，等待请求完成
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      }, {
        disableAuth: true,
        disableRetry401: true,
      })
    } catch {
    }

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
