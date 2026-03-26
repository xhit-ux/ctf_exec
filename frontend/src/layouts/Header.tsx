import React, { useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { staticUrl } from '../utils/api'

function getLevelInfo(score: number = 0) {
  if (score < 1000) return { title: '新手', color: 'text-gray-500', bg: 'bg-gray-100' }
  if (score < 3000) return { title: '入门', color: 'text-green-600', bg: 'bg-green-100' }
  if (score < 4500) return { title: '进阶', color: 'text-cyan-600', bg: 'bg-cyan-100' }
  if (score < 5500) return { title: '熟练', color: 'text-blue-600', bg: 'bg-blue-100' }
  if (score < 6000) return { title: '专家', color: 'text-purple-600', bg: 'bg-purple-100' }
  return { title: '大师', color: 'text-red-600', bg: 'bg-red-100' }
}

function canAccessSiteAdmin(role?: string) {
  return role === 'root' || role === 'site_admin'
}

export default function Header() {
  const { user, isAuthenticated, logout } = useAuth()
  const location = useLocation()
  const [menuOpen, setMenuOpen] = useState(false)

  const handleLogout = () => {
    logout()
    setMenuOpen(false)
  }

  const score = user?.score ?? 0
  const level = getLevelInfo(score)

  const NavLinkItem = ({ to, children }: { to: string, children: React.ReactNode }) => {
    const isActive = location.pathname === to || (to !== '/' && location.pathname.startsWith(to))
    return (
      <Link
        to={to}
        onClick={() => setMenuOpen(false)}
        className={`block px-3 py-2.5 rounded-lg text-sm font-medium transition-colors ${
          isActive 
            ? 'bg-blue-50 text-blue-700' 
            : 'text-gray-600 hover:bg-gray-50 hover:text-gray-900'
        }`}
      >
        {children}
      </Link>
    )
  }

  return (
    <header className="bg-white border-b border-gray-200 sticky top-0 z-50">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex justify-between h-16">
          {/* Logo & Desktop Nav */}
          <div className="flex">
            <Link to="/" className="flex flex-shrink-0 items-center gap-2 mr-8 no-underline">
              <span className="text-2xl pt-1">🎯</span>
              <h1 className="text-xl font-bold bg-clip-text text-transparent bg-gradient-to-r from-pink-500 to-rose-400">
                CTF Platform
              </h1>
            </Link>
            <nav className="hidden sm:flex sm:space-x-4 sm:items-center">
              <NavLinkItem to="/">题目大厅</NavLinkItem>
              <NavLinkItem to="/contests">赛事大厅</NavLinkItem>
              <NavLinkItem to="/scoreboard">排行榜</NavLinkItem>
              {isAuthenticated && (
                <>
                  <NavLinkItem to="/submissions">提交记录</NavLinkItem>
                  <NavLinkItem to="/team">我的队伍</NavLinkItem>
                  {canAccessSiteAdmin(user?.role) && (
                    <NavLinkItem to="/admin">管理面板</NavLinkItem>
                  )}
                </>
              )}
            </nav>
          </div>

          {/* User & Actions */}
          <div className="flex items-center gap-4">
            {isAuthenticated && user ? (
              <div className="relative flex items-center gap-4">
                <div className="hidden md:flex flex-col items-end">
                  <span className="text-sm font-semibold text-gray-700">{user.display_name}</span>
                  <div className="flex items-center gap-1.5 mt-0.5">
                    <span className={`text-[10px] px-1.5 py-0.5 rounded-full font-bold ${level.bg} ${level.color}`}>
                      {level.title}
                    </span>
                    <span className="text-xs text-gray-500 font-medium">{score} 分</span>
                  </div>
                </div>
                
                {/* Profile menu dropdown (simplified) */}
                <div className="relative flex Items-center">
                  <Link to="/profile" className="flex items-center gap-2 hover:opacity-80 transition focus:outline-none">
                    {user.avatar_url ? (
                      <img
                        src={staticUrl(user.avatar_url)}
                        alt={user.username}
                        className="w-9 h-9 rounded-full object-cover border border-gray-200 shadow-sm"
                      />
                    ) : (
                      <div className="w-9 h-9 rounded-full bg-gradient-to-tr from-pink-300 to-rose-300 text-white flex items-center justify-center text-sm font-bold shadow-sm">
                        {(user.display_name?.[0] || user.username?.[0] || '?').toUpperCase()}
                      </div>
                    )}
                  </Link>
                </div>
                
                <button
                  onClick={handleLogout}
                  className="hidden sm:flex text-sm text-gray-500 hover:text-red-600 transition-colors ml-2"
                  title="退出登录"
                >
                  <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                     <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
                  </svg>
                </button>
              </div>
            ) : (
              <div className="flex items-center gap-3">
                <Link
                  to="/login"
                  className="text-sm font-medium text-gray-700 hover:text-blue-600 transition-colors"
                >
                  登录
                </Link>
                <Link
                  to="/register"
                  className="text-sm font-medium px-4 py-2 rounded-lg bg-blue-600 text-white hover:bg-blue-700 shadow-sm transition-colors"
                >
                  注册
                </Link>
              </div>
            )}
            
            {/* Mobile menu button */}
            <div className="sm:hidden flex items-center">
              <button
                onClick={() => setMenuOpen(!menuOpen)}
                className="text-gray-500 hover:text-gray-700 focus:outline-none rounded-md p-2"
              >
                <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
                </svg>
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Mobile nav Drawer */}
      {/* Overlay */}
      <div 
        className={`fixed inset-0 bg-black/30 z-[55] transition-opacity sm:hidden ${menuOpen ? 'opacity-100' : 'opacity-0 pointer-events-none'}`}
        onClick={() => setMenuOpen(false)}
      />
      
      {/* Side Panel */}
      <div className={`fixed top-0 right-0 bottom-0 w-64 bg-white z-[60] transform transition-transform duration-300 ease-out sm:hidden flex flex-col shadow-2xl ${menuOpen ? 'translate-x-0' : 'translate-x-full'}`}>
        <div className="flex items-center justify-between px-6 h-16 border-b border-gray-100">
          <span className="font-bold text-gray-800 tracking-wide text-lg">菜单</span>
          <button onClick={() => setMenuOpen(false)} className="p-2 -mr-2 text-gray-400 hover:text-gray-600 hover:bg-gray-100 rounded-full transition-colors">
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
        
        <div className="px-4 py-6 space-y-1 overflow-y-auto flex-1">
            <NavLinkItem to="/">题目大厅</NavLinkItem>
          <NavLinkItem to="/contests">赛事</NavLinkItem>
            <NavLinkItem to="/scoreboard">排行榜</NavLinkItem>
            {isAuthenticated && (
              <>
                <div className="my-4 border-t border-gray-100" />
                <NavLinkItem to="/submissions">提交记录</NavLinkItem>
                <NavLinkItem to="/team">我的队伍</NavLinkItem>
                <NavLinkItem to="/profile">个人中心</NavLinkItem>
                {canAccessSiteAdmin(user?.role) && <NavLinkItem to="/admin">管理面板</NavLinkItem>}
              </>
            )}
        </div>

        {isAuthenticated && (
          <div className="p-4 border-t border-gray-100">
             <button
                onClick={handleLogout}
                className="w-full text-center px-4 py-3 rounded-xl text-sm font-bold text-red-600 bg-red-50 hover:bg-red-100 transition-colors"
             >
                退出登录
              </button>
          </div>
        )}
      </div>
    </header>
  )
}
