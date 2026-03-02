import React from 'react'
import { NavLink } from 'react-router-dom'

import { useAuth } from '../context/AuthContext'

function Item({ to, children }: { to: string; children: React.ReactNode }) {
  return (
    <NavLink
      to={to}
      className={({ isActive }) =>
        'block px-3 py-2 rounded hover:bg-gray-50 ' + (isActive ? 'bg-gray-100 font-medium' : '')
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
    <div className="bg-white rounded-lg p-4 shadow-sm">
      <div className="mb-4">
        <div className="text-sm font-medium">练习者</div>
        <div className={`text-xs ${level.color}`}>
          等级 · {score} · {level.title}
        </div>
      </div>
      <nav className="space-y-2">
        <Item to="/">题目列表</Item>
        <Item to="/submissions">我的提交</Item>
        <Item to="/profile">个人信息</Item>
        <Item to="/team">我的队伍</Item>
        {user?.role === 'admin' && <Item to="/admin">管理</Item>}
      </nav>
      <div className="mt-4 pt-4 border-t text-xs text-gray-500">TODO//每日一题？每日鸡汤？</div>
    </div>
  )
}
