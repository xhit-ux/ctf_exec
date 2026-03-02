import React from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'

export default function Header() {
  const { user, isAuthenticated, logout } = useAuth()
  const navigate = useNavigate()

  const handleLogout = () => {
    logout()
    navigate('/')
  }

  return (
    <header className="bg-white shadow">
      <div className="max-w-7xl mx-auto py-4 px-6 flex items-center justify-between gap-4">
        <Link to="/" className="flex items-center gap-4 text-inherit no-underline">
          <h1 className="text-2xl font-semibold">CTF Platform</h1>
          <div className="hidden sm:block text-sm text-gray-500">练习系统</div>
        </Link>

        <div className="flex items-center gap-4">
          <nav className="hidden sm:flex gap-4 text-sm font-medium text-gray-600">
            <Link to="/scoreboard" className="hover:text-blue-600 transition-colors">排行榜</Link>
          </nav>

          <div className="flex items-center gap-3">
            {isAuthenticated && user ? (
              <>
                <Link to="/profile" className="flex items-center gap-2 hover:opacity-80 transition">
                  {user.avatar_url ? (
                    <img
                      src={user.avatar_url.startsWith('static/') ||user.avatar_url.startsWith('..\\static\\')? `http://localhost:8080/${user.avatar_url}` : user.avatar_url}
                      alt={user.username}
                      className="w-8 h-8 rounded-full object-cover border-2 border-gray-200"
                    />
                  ) : (
                    <div className="w-8 h-8 rounded-full bg-blue-500 text-white flex items-center justify-center text-sm font-semibold">
                      {(user.display_name?.[0] || user.username?.[0] || '?').toUpperCase()}
                    </div>
                  )}
                  <span className="text-sm font-semibold text-gray-700">{user.display_name}</span>
                </Link>
                <Link to="/submissions" className="text-sm text-gray-600 hover:text-blue-600">我的提交</Link>
                {user.role === 'admin' && (
                  <Link to="/admin" className="text-sm text-gray-600 hover:text-blue-600">管理面板</Link>
                )}
                <button
                  onClick={handleLogout}
                  className="text-sm bg-red-50 text-red-600 px-3 py-1 rounded hover:bg-red-100 transition-colors"
                >
                  退出登录
                </button>
              </>
            ) : (
              <>
                <Link
                  to="/login"
                  className="text-sm px-3 py-1 rounded border border-gray-300 hover:border-blue-500 hover:text-blue-500 transition-colors"
                >
                  登录
                </Link>
                <Link
                  to="/register"
                  className="text-sm px-3 py-1 rounded bg-blue-600 text-white hover:bg-blue-700 transition-colors"
                >
                  注册
                </Link>
              </>
            )}
          </div>
        </div>
      </div>
    </header>
  )
}
