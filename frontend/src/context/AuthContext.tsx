import React, { createContext, useState, useEffect, ReactNode } from 'react'
import { apiUrl } from '../utils/api'
import { AUTH_REFRESH_FAILED_EVENT, AUTH_USER_UPDATED_EVENT, apiFetch, clearAuthTokens, refreshAccessTokenWithLock, setAuthTokens } from '../utils/authClient'
import { readSafeErrorMessage } from '../utils/error'
import { publishGlobalHttpError } from '../utils/globalErrorBus'

export interface User {
  id: string
  username: string
  email: string
  role: string
  display_name: string
  avatar_url?: string
  bio?: string
  score: number
  solved_count: number
  is_active: boolean
  is_verified: boolean
  last_login_at?: string
  created_at: string
  updated_at: string
  preferences?: string  // JSON 字符串存储用户偏好
}

export interface AuthContextType {
  user: User | null
  token: string | null
  refreshToken: string | null
  isAuthenticated: boolean
  isLoading: boolean
  login: (token: string, refreshToken: string, user: User) => void
  logout: () => void
  setUser: (user: User) => void
  refreshUser?: () => Promise<void>
}

export const AuthContext = createContext<AuthContextType>({
  user: null,
  token: null,
  refreshToken: null,
  isAuthenticated: false,
  isLoading: true,
  login: () => { },
  logout: () => { },
  setUser: () => { },
})

export const AuthProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
  const [user, setUser] = useState<User | null>(() => {
    const storedUser = localStorage.getItem('user')
    if (storedUser) {
      try {
        return JSON.parse(storedUser)
      } catch (err) {
        console.error('Failed to parse stored user:', err)
        return null
      }
    }
    return null
  })

  const [token, setToken] = useState<string | null>(() => {
    return localStorage.getItem('token')
  })

  const [refreshToken, setRefreshToken] = useState<string | null>(() => {
    return localStorage.getItem('refresh_token')
  })

  const [isLoading, setIsLoading] = useState(true)

  const syncTokenStateFromStorage = () => {
    setToken(localStorage.getItem('token'))
    setRefreshToken(localStorage.getItem('refresh_token'))
  }

  // Single cleanup entrypoint for all auth-related client state.
  const clearAuthState = () => {
    setToken(null)
    setRefreshToken(null)
    setUser(null)
    clearAuthTokens()
    localStorage.removeItem('user')
  }

  // 验证 token 有效性并刷新用户信息
  useEffect(() => {
    const initAuth = async () => {
      if (token) {
        await refreshUser()
      } else if (refreshToken) {
        const refreshed = await refreshAccessTokenWithLock()
        if (refreshed) {
          syncTokenStateFromStorage()
          await refreshUser()
        }
      }
      setIsLoading(false)
    }
    initAuth()
  }, [])

  useEffect(() => {
    const onRefreshFailed = () => {
      // 幂等处理：如果 token 对已经清空，避免重复弹窗与重复跳转。
      if (!localStorage.getItem('token') && !localStorage.getItem('refresh_token')) return

      clearAuthState()
      window.location.replace('/login')
    }

    window.addEventListener(AUTH_REFRESH_FAILED_EVENT, onRefreshFailed)
    return () => {
      window.removeEventListener(AUTH_REFRESH_FAILED_EVENT, onRefreshFailed)
    }
  }, [])

  useEffect(() => {
    const onUserUpdated = (event: Event) => {
      const customEvent = event as CustomEvent<{ user?: User }>
      if (!customEvent.detail?.user) return
      setUser(customEvent.detail.user)
    }

    window.addEventListener(AUTH_USER_UPDATED_EVENT, onUserUpdated as EventListener)
    return () => {
      window.removeEventListener(AUTH_USER_UPDATED_EVENT, onUserUpdated as EventListener)
    }
  }, [])

  const login = (newToken: string, newRefreshToken: string, newUser: User) => {
    setToken(newToken)
    setRefreshToken(newRefreshToken)
    setUser(newUser)
    setAuthTokens(newToken, newRefreshToken)
    localStorage.setItem('user', JSON.stringify(newUser))
  }

  const logout = async () => {
    const tokenForRevoke = token
    const refreshTokenForRevoke = refreshToken

    // 先清空本地状态，防止用户在等待期间继续操作
    clearAuthState()

    if (tokenForRevoke) {
      try {
        await apiFetch(apiUrl('/api/auth/logout'), {  // ← await，等待请求完成
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ refresh_token: refreshTokenForRevoke || '' }),
        }, {
          disableAuth: true,
          disableRetry401: true,
          accessTokenOverride: tokenForRevoke,
        })
      } catch {
      }
    }

    window.location.replace('/login')
  }

  const refreshUser = async () => {
    if (!token && !refreshToken) return

    try {
      const response = await apiFetch(apiUrl('/api/auth/me'))

      if (response.ok) {
        const userData = await response.json()
        // /api/auth/me 直接返回用户对象，不是嵌套在 data.user 中
        setUser(userData)
        syncTokenStateFromStorage()
        localStorage.setItem('user', JSON.stringify(userData))
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
    token,
    refreshToken,
    isAuthenticated: !!token && !!user,
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
