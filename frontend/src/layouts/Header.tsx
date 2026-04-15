import React, { useEffect, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { staticUrl } from '../utils/api'

function getLevelInfo(score: number = 0) {
  if (score < 1000) return { title: '新手', color: 'text-zinc-600', bg: 'bg-zinc-100' }
  if (score < 3000) return { title: '入门', color: 'text-emerald-700', bg: 'bg-emerald-100' }
  if (score < 4500) return { title: '进阶', color: 'text-amber-700', bg: 'bg-amber-100' }
  if (score < 5500) return { title: '熟练', color: 'text-orange-700', bg: 'bg-orange-100' }
  if (score < 6000) return { title: '专家', color: 'text-rose-700', bg: 'bg-rose-100' }
  return { title: '大师', color: 'text-red-700', bg: 'bg-red-100' }
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

  useEffect(() => {
    if (!menuOpen) {
      document.body.style.overflow = ''
      document.body.style.touchAction = ''
      return
    }

    document.body.style.overflow = 'hidden'
    document.body.style.touchAction = 'none'

    return () => {
      document.body.style.overflow = ''
      document.body.style.touchAction = ''
    }
  }, [menuOpen])

  const NavLinkItem = ({ to, children }: { to: string, children: React.ReactNode }) => {
    const isActive = location.pathname === to || (to !== '/' && location.pathname.startsWith(to))
    return (
      <Link
        to={to}
        onClick={() => setMenuOpen(false)}
        className={`block px-3 py-2.5 rounded-lg text-sm font-medium transition-colors ${
          isActive 
            ? 'bg-amber-100 text-amber-900' 
            : 'text-gray-600 hover:bg-amber-50 hover:text-gray-900'
        }`}
      >
        {children}
      </Link>
    )
  }

  return (
    <header className="sticky top-0 z-[100] border-b border-amber-100/80 bg-white/88 backdrop-blur-xl">
      <div className="max-w-7xl mx-auto px-4">
        <div className="flex justify-between h-16">
          {/* Logo & Desktop Nav */}
          <div className="flex">
            <Link to="/" className="flex flex-shrink-0 items-center gap-2 mr-8 no-underline">
              <span className="h-8 w-8 rounded-lg bg-gradient-to-br from-amber-500 to-orange-600 text-white font-bold text-sm flex items-center justify-center shadow-sm">CTF</span>
              <h1 className="text-xl font-bold text-slate-900 tracking-tight">
                CTF Platform
              </h1>
            </Link>
            <nav className="header-desktop-nav">
              <NavLinkItem to="/">首页</NavLinkItem>
              <NavLinkItem to="/problem">题目大厅</NavLinkItem>
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
                <div className="header-desktop-meta flex-col items-end">
                  <span className="text-sm font-semibold text-gray-700">{user.display_name}</span>
                  <div className="flex items-center gap-1.5 mt-0.5">
                    <span className={`text-[10px] px-1.5 py-0.5 rounded-full font-bold ${level.bg} ${level.color}`}>
                      {level.title}
                    </span>
                    <span className="text-xs text-gray-500 font-medium">{score} 分</span>
                  </div>
                </div>
                
                {/* Profile menu dropdown (simplified) */}
                <div className="relative flex items-center">
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
                  className="header-desktop-logout text-sm text-gray-500 hover:text-red-600 transition-colors ml-2"
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
                  className="text-sm font-medium text-gray-700 hover:text-amber-700 transition-colors"
                >
                  登录
                </Link>
                <Link
                  to="/register"
                  className="btn-primary text-sm"
                >
                  注册
                </Link>
              </div>
            )}
            
            {/* Mobile menu button */}
            <div className="header-mobile-menu items-center">
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
        className={`mobile-backdrop header-mobile-only transition-opacity ${menuOpen ? 'opacity-100' : 'opacity-0 pointer-events-none'}`}
        onClick={() => setMenuOpen(false)}
        aria-hidden="true"
      />
      
      {/* Side Panel */}
      <aside className={`mobile-panel header-mobile-only ${menuOpen ? 'translate-x-0 opacity-100' : 'translate-x-[110%] opacity-0 pointer-events-none'}`} aria-hidden={!menuOpen}>
        <div className="flex items-center justify-between px-5 h-14 border-b border-amber-100/80">
          <span className="font-bold text-gray-800 tracking-wide text-base">菜单</span>
          <button onClick={() => setMenuOpen(false)} className="p-2 -mr-2 text-gray-400 hover:text-gray-600 hover:bg-white/80 rounded-full transition-colors" aria-label="关闭菜单">
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div className="px-4 py-4 space-y-4 overflow-y-auto flex-1 overscroll-contain">
          {isAuthenticated && user && (
            <div className="soft-card px-3 py-2.5 flex items-center gap-3">
              {user.avatar_url ? (
                <img
                  src={staticUrl(user.avatar_url)}
                  alt={user.username}
                  className="w-10 h-10 rounded-full object-cover border border-gray-200"
                />
              ) : (
                <div className="w-10 h-10 rounded-full bg-amber-500 text-white flex items-center justify-center text-sm font-bold">
                  {(user.display_name?.[0] || user.username?.[0] || '?').toUpperCase()}
                </div>
              )}
              <div className="min-w-0">
                <div className="text-sm font-semibold text-slate-800 truncate">{user.display_name || user.username}</div>
                <div className="text-xs text-slate-500">{score} 分 · {level.title}</div>
              </div>
            </div>
          )}

          <div className="soft-card p-2.5 space-y-1.5">
            <div className="px-2 text-[11px] font-semibold tracking-[0.14em] uppercase text-slate-500">导航</div>
            <NavLinkItem to="/">首页</NavLinkItem>
            <NavLinkItem to="/problem">题目大厅</NavLinkItem>
            <NavLinkItem to="/contests">赛事大厅</NavLinkItem>
            <NavLinkItem to="/scoreboard">排行榜</NavLinkItem>
          </div>

          {isAuthenticated && (
            <div className="soft-card p-2.5 space-y-1.5">
              <div className="px-2 text-[11px] font-semibold tracking-[0.14em] uppercase text-slate-500">我的</div>
              <NavLinkItem to="/submissions">提交记录</NavLinkItem>
              <NavLinkItem to="/team">我的队伍</NavLinkItem>
              <NavLinkItem to="/profile">个人中心</NavLinkItem>
              {canAccessSiteAdmin(user?.role) && <NavLinkItem to="/admin">管理面板</NavLinkItem>}
            </div>
          )}
        </div>

        {isAuthenticated && (
           <div className="p-4 border-t border-amber-100/80 bg-white/55">
             <button
                onClick={handleLogout}
               className="btn-danger w-full"
             >
                退出登录
              </button>
          </div>
        )}
        {!isAuthenticated && (
          <div className="p-4 border-t border-amber-100/80 bg-white/55 space-y-2">
            <Link to="/login" onClick={() => setMenuOpen(false)} className="btn-secondary w-full">登录</Link>
            <Link to="/register" onClick={() => setMenuOpen(false)} className="btn-primary w-full">注册</Link>
          </div>
        )}
      </aside>
    </header>
  )
}
