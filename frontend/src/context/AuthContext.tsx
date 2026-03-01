import React, { createContext, useState, useEffect, ReactNode } from 'react'

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
  isAuthenticated: boolean
  isLoading: boolean
  login: (token: string, user: User) => void
  logout: () => void
  setUser: (user: User) => void
  refreshUser?: () => Promise<void>
}

export const AuthContext = createContext<AuthContextType>({
  user: null,
  token: null,
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

  const [isLoading, setIsLoading] = useState(true)

  // 验证 token 有效性并刷新用户信息
  useEffect(() => {
    const initAuth = async () => {
      if (token) {
        await refreshUser()
      }
      setIsLoading(false)
    }
    initAuth()
  }, [])

  const login = (newToken: string, newUser: User) => {
    setToken(newToken)
    setUser(newUser)
    localStorage.setItem('token', newToken)
    localStorage.setItem('user', JSON.stringify(newUser))
  }

  const logout = () => {
    setToken(null)
    setUser(null)
    localStorage.removeItem('token')
    localStorage.removeItem('user')
  }

  const refreshUser = async () => {
    if (!token) return

    try {
      const response = await fetch('http://localhost:8080/api/auth/me', {
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
        // 如果 token 无效，清除登录状态
        console.error('Token invalid, logging out')
        logout()
      }
    } catch (err) {
      console.error('Failed to refresh user:', err)
    }
  }

  const value: AuthContextType = {
    user,
    token,
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
