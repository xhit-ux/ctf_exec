import React, { useMemo, useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import ProblemCard from '../components/problem/ProblemCard'
import { useAuth } from '../context/AuthContext'
import { Problem } from '../types'
import { apiUrl } from '../utils/api'
import { apiFetch } from '../utils/authClient'

const categories = ['all', 'web', 'pwn', 'crypto', 'reverse', 'misc'] as const

export default function Problems() {
  const [category, setCategory] = useState<typeof categories[number]>('all')
  const [difficulty, setDifficulty] = useState<'all' | '简单' | '中等' | '困难'>('all')
  const [query, setQuery] = useState('')
  const [selectedTag, setSelectedTag] = useState<string | null>(null)

  const [problems, setProblems] = useState<Problem[]>([])
  const [loading, setLoading] = useState(true)
  const { token, isAuthenticated } = useAuth()

  useEffect(() => {
    if (!isAuthenticated || !token) {
      setLoading(false)
      return
    }

    apiFetch(apiUrl('/api/problems'))
      .then(async res => {
        if (!res.ok) throw new Error(res.statusText)
        return res.json()
      })
      .then(data => {
        if (Array.isArray(data)) {
          setProblems(data)
        } else {
          console.error('API response is not an array:', data)
          setProblems([])
        }
        setLoading(false)
      })
      .catch(err => {
        console.error(err)
        setLoading(false)
      })
  }, [isAuthenticated, token])

  const allTags = useMemo(() => {
    const s = new Set<string>()
    problems.forEach((p) => {
      const tags = Array.isArray(p.tags) ? p.tags : []
      tags.forEach((t) => s.add(t))
    })
    return Array.from(s)
  }, [problems])

  const filtered = useMemo(() => {
    return problems.filter((p) => {
      if (category !== 'all' && p.category !== category) return false
      if (difficulty !== 'all' && p.difficulty !== difficulty) return false
      const tags = Array.isArray(p.tags) ? p.tags : []
      if (selectedTag && !tags.includes(selectedTag)) return false
      if (query && !(`${p.title} ${tags.join(' ')}`.toLowerCase().includes(query.toLowerCase()))) return false
      return true
    })
  }, [category, difficulty, query, selectedTag, problems])

  if (!isAuthenticated) return <div className="p-10 text-center font-medium text-gray-600 bg-white rounded-xl shadow-sm border border-gray-100">请先登录查看题目</div>
  if (loading) return <div className="p-10 text-center font-medium text-gray-600">加载中...</div>

  return (
    <div className="flex flex-col lg:flex-row gap-8 w-full max-w-7xl mx-auto">
      {/* Main Content Area */}
      <div className="flex-1 min-w-0">
        <div className="mb-6 flex items-baseline gap-4">
          <h2 className="text-3xl font-extrabold text-gray-900 tracking-tight">题目大厅</h2>
          <p className="text-gray-500 text-sm font-medium">当前 {filtered.length} 题 / 共 {problems.length} 题</p>
        </div>

        {/* Mobile Filter Toggle */}
        <div className="lg:hidden mb-6 p-5 bg-white rounded-xl shadow-sm border border-gray-100 transition-all">
          <div className="text-sm font-bold text-gray-800 mb-4 flex items-center gap-2">
            <svg className="w-4 h-4 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z" />
            </svg>
            快速筛选
          </div>
          <div className="space-y-4">
            <div>
              <div className="flex flex-wrap gap-2">
                {categories.map((c) => (
                  <button
                    key={c}
                    onClick={() => setCategory(c)}
                    className={`px-3 py-1.5 rounded-md text-xs font-semibold uppercase tracking-wider transition-colors ${category === c ? 'bg-indigo-600 text-white shadow' : 'bg-gray-100 text-gray-600 hover:bg-gray-200'}`}
                  >
                    {c === 'all' ? '全部' : c}
                  </button>
                ))}
              </div>
            </div>
            <div className="flex gap-3">
              <select
                value={difficulty}
                onChange={(e) => setDifficulty(e.target.value as any)}
                className="flex-1 bg-gray-50 border border-gray-200 rounded-md px-3 py-2 text-sm text-gray-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 transition-shadow"
              >
                <option value="all">全部难度</option>
                <option value="简单">简单</option>
                <option value="中等">中等</option>
                <option value="困难">困难</option>
              </select>
              <input
                placeholder="搜索题目..."
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                className="flex-2 bg-gray-50 border border-gray-200 rounded-md px-3 py-2 text-sm text-gray-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 transition-shadow w-full"
              />
            </div>
          </div>
        </div>

        {filtered.length === 0 ? (
          <div className="text-center py-20 bg-white rounded-2xl border border-gray-100 shadow-sm">
            <div className="text-gray-300 mb-3">
              <svg className="w-16 h-16 mx-auto" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M9.172 16.172a4 4 0 015.656 0M9 10h.01M15 10h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
            <p className="text-gray-500 text-lg font-medium">没有找到匹配的题目，换个姿势试试？</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">
            {filtered.map((p: Problem) => (
              <Link key={p.id} to={`/problem/${p.id}`} className="h-full block transform transition-transform hover:-translate-y-1 hover:shadow-lg rounded-xl">
                <ProblemCard p={p} />
              </Link>
            ))}
          </div>
        )}
      </div>

      {/* Desktop Filter Sidebar */}
      <div className="hidden lg:block w-72 shrink-0">
        <div className="sticky top-24 bg-white p-6 rounded-2xl shadow-sm border border-gray-100">
          <div className="flex items-center gap-2 mb-6 text-gray-800">
            <svg className="w-5 h-5 text-indigo-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z" />
            </svg>
            <h3 className="text-lg font-bold">题目筛选</h3>
          </div>

          <div className="space-y-6">
            <div>
              <h4 className="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-3">方向类别</h4>
              <div className="flex flex-wrap gap-2">
                {categories.map((c) => (
                  <button
                    key={c}
                    onClick={() => setCategory(c)}
                    className={`px-3 py-1.5 rounded-lg text-sm font-semibold transition-all ${
                      category === c
                        ? 'bg-indigo-600 text-white shadow-md'
                        : 'bg-indigo-50 text-indigo-600 hover:bg-indigo-100'
                    }`}
                  >
                    {c === 'all' ? '全部领域' : c.toUpperCase()}
                  </button>
                ))}
              </div>
            </div>

            <div className="pt-5 border-t border-gray-100">
              <h4 className="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-3">挑战难度</h4>
              <div className="relative">
                <select
                  value={difficulty}
                  onChange={(e) => setDifficulty(e.target.value as any)}
                  className="w-full appearance-none bg-gray-50 border border-gray-200 text-gray-700 py-2.5 px-4 pr-8 rounded-lg outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 transition-shadow"
                >
                  <option value="all">👑 所有难度</option>
                  <option value="简单">🟢 简单 (Easy)</option>
                  <option value="中等">🟡 中等 (Medium)</option>
                  <option value="困难">🔴 困难 (Hard)</option>
                </select>
                <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center px-2 text-gray-500">
                  <svg className="fill-current h-4 w-4" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20"><path d="M9.293 12.95l.707.707L15.657 8l-1.414-1.414L10 10.828 5.757 6.586 4.343 8z"/></svg>
                </div>
              </div>
            </div>

            <div className="pt-5 border-t border-gray-100">
              <h4 className="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-3">关键词</h4>
              <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <svg className="h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                     <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                  </svg>
                </div>
                <input
                  placeholder="搜索题目名称、描述、标签"
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  className="w-full pl-9 pr-3 py-2.5 bg-gray-50 border border-gray-200 rounded-lg text-sm text-gray-900 placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 transition-shadow"
                />
              </div>
            </div>

            {allTags.length > 0 && (
              <div className="pt-5 border-t border-gray-100">
                <h4 className="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-3">技能标签</h4>
                <div className="flex flex-wrap gap-2">
                  {allTags.map((t) => (
                    <button
                      key={t}
                      onClick={() => setSelectedTag(selectedTag === t ? null : t)}
                      className={`text-xs px-2.5 py-1 rounded-md font-medium transition-all ${
                        selectedTag === t
                          ? 'bg-gray-800 text-white shadow'
                          : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
                      }`}
                    >
                      # {t}
                    </button>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
