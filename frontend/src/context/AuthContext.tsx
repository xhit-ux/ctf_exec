import React, { createContext, useState, useEffect, ReactNode } from 'react'
import { apiUrl } from '../utils/api'

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

  // 验证 token 有效性并刷新用户信息
  useEffect(() => {
    const initAuth = async () => {
      if (token) {
        await refreshUser()
      } else if (refreshToken) {
        const refreshed = await refreshAccessToken(refreshToken)
        if (refreshed) {
          await refreshUser()
        }
      }
      setIsLoading(false)
    }
    initAuth()
  }, [])

  const login = (newToken: string, newRefreshToken: string, newUser: User) => {
    setToken(newToken)
    setRefreshToken(newRefreshToken)
    setUser(newUser)
    localStorage.setItem('token', newToken)
    localStorage.setItem('refresh_token', newRefreshToken)
    localStorage.setItem('user', JSON.stringify(newUser))
  }

  const refreshAccessToken = async (tokenForRefresh: string): Promise<boolean> => {
    try {
      const response = await fetch(apiUrl('/api/auth/refresh'), {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ refresh_token: tokenForRefresh }),
      })

      if (!response.ok) return false

      const data = await response.json()
      if (!data?.token || !data?.refresh_token) return false

      setToken(data.token)
      setRefreshToken(data.refresh_token)
      localStorage.setItem('token', data.token)
      localStorage.setItem('refresh_token', data.refresh_token)
      return true
    } catch {
      return false
    }
  }

  const logout = () => {
    const tokenForRevoke = token
    const refreshTokenForRevoke = refreshToken
    setToken(null)
    setRefreshToken(null)
    setUser(null)
    localStorage.removeItem('token')
    localStorage.removeItem('refresh_token')
    localStorage.removeItem('user')

    if (tokenForRevoke) {
      void fetch(apiUrl('/api/auth/logout'), {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${tokenForRevoke}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ refresh_token: refreshTokenForRevoke || '' }),
      })
    }
  }

  const refreshUser = async () => {
    if (!token) return

    try {
      const response = await fetch(apiUrl('/api/auth/me'), {
        headers: {
          'Authorization': `Bearer ${token}`,
        },
      })

      if (response.ok) {
        const userData = await response.json()
        // /api/auth/me 直接返回用户对象，不是嵌套在 data.user 中
        setUser(userData)
        localStorage.setItem('user', JSON.stringify(userData))
      } else {
        const currentRefresh = localStorage.getItem('refresh_token')
        if (!currentRefresh) {
          logout()
          return
        }

        const refreshed = await refreshAccessToken(currentRefresh)
        if (!refreshed) {
          console.error('Token refresh failed, logging out')
          logout()
          return
        }

        const retriedToken = localStorage.getItem('token')
        if (!retriedToken) {
          logout()
          return
        }

        const retryResponse = await fetch(apiUrl('/api/auth/me'), {
          headers: {
            'Authorization': `Bearer ${retriedToken}`,
          },
        })

        if (!retryResponse.ok) {
          logout()
          return
        }

        const userData = await retryResponse.json()
        setUser(userData)
        localStorage.setItem('user', JSON.stringify(userData))
      }
    } catch (err) {
      console.error('Failed to refresh user:', err)
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
