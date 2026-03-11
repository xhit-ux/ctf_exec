import React from 'react'
import { NavLink } from 'react-router-dom'

import { useAuth } from '../context/AuthContext'

function Item({ to, children }: { to: string; children: React.ReactNode }) {
  return (
    <NavLink
      to={to}
      className={({ isActive }) =>
        'block px-3 py-2.5 rounded-lg font-medium transition-all duration-200 ' +
        (isActive
          ? 'bg-blue-100 text-blue-700 shadow-sm'
          : 'text-gray-700 hover:bg-gray-100 hover:text-gray-900')
      }
    >
      {children}
    </NavLink>
  )
}

function getLevelInfo(score: number = 0) {
  if (score < 1000) {
    return { title: '新手', color: 'text-gray-500' }
  }
  if (score < 3000) {
    return { title: '入门', color: 'text-green-500' }
  }
  if (score < 4500) {
    return { title: '进阶', color: 'text-cyan-500' }
  }
  if (score < 5500) {
    return { title: '熟练', color: 'text-blue-500' }
  }
  if (score < 6000) {
    return { title: '专家', color: 'text-purple-500' }
  }
  return { title: '大师', color: 'text-red-500' }
}

export default function Sidebar() {
  const { user } = useAuth()
  const score = useAuth().user?.score ?? 0
  const level = getLevelInfo(score)

  return (
    <div>
      <div className="mb-6 pb-4 border-b">
        <div className="text-sm font-semibold text-gray-900">练习者</div>
        <div className={`text-xs ${level.color} font-medium mt-1`}>
          {level.title} · {score} 分
        </div>
      </div>
      <nav className="space-y-1">
        <Item to="/">📋 题目列表</Item>
        <Item to="/submissions">✅ 提交记录</Item>
        <Item to="/profile">👤 个人信息</Item>
        <Item to="/team">👥 队伍信息</Item>
        {user?.role === 'admin' && <Item to="/admin">⚙️ 管理</Item>}
      </nav>
      <div className="mt-6 pt-4 border-t text-xs text-gray-400">
        <p>✨ 每日一题正在计划中</p>
      </div>
    </div>
  )
}
