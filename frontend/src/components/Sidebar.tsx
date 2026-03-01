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

export default function Sidebar() {
  const { user } = useAuth()

  return (
    <div className="bg-white rounded-lg p-4 shadow-sm">
      <div className="mb-4">
        <div className="text-sm font-medium">练习者</div>
        <div className="text-xs text-gray-500">等级 · 新手</div>
      </div>
      <nav className="space-y-2">
        <Item to="/">题目列表</Item>
        <Item to="/submissions">我的提交</Item>
        <Item to="/profile">个人信息</Item>
        <Item to="/team">我的队伍</Item>
        {user?.role === 'admin' && <Item to="/admin">管理</Item>}
      </nav>
      <div className="mt-4 pt-4 border-t text-xs text-gray-500">筛选 · 标签 / 难度（后续）</div>
    </div>
  )
}
