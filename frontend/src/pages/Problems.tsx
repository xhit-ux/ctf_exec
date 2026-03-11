import React, { useMemo, useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import ProblemCard from '../components/ProblemCard'
import { useAuth } from '../context/AuthContext'
import { Problem } from '../types'

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

    fetch('http://localhost:8080/api/problems', {
      headers: { Authorization: `Bearer ${token}` }
    })
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

  if (!isAuthenticated) return <div className="p-10 text-center">请先登录查看题目</div>
  if (loading) return <div className="p-10 text-center">加载中...</div>

  return (
    <div>
      <div className="mb-6">
        <h2 className="text-2xl font-bold text-gray-900">题目列表</h2>
        <p className="text-gray-500 text-sm mt-1">共 {filtered.length} 道题（全部 {problems.length}）</p>
      </div>

      {/* Floating Filter Panel */}
      <div className="hidden lg:block fixed right-10 top-24 w-72 z-40">
        <div className="bg-white p-5 rounded-lg shadow-lg border border-gray-200 max-h-[calc(100vh-120px)] overflow-y-auto">
          <div className="space-y-5">
            <div>
              <h3 className="text-sm font-semibold text-gray-900 mb-3">类别</h3>
              <div className="flex flex-wrap gap-2">
                {categories.map((c) => (
                  <button
                    key={c}
                    onClick={() => setCategory(c)}
                    className={`px-3 py-2 rounded text-xs font-medium transition-all whitespace-nowrap flex-shrink-0 ${
                      category === c
                        ? 'bg-blue-600 text-white shadow-md'
                        : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
                    }`}
                  >
                    {c === 'all' ? '全部' : c.toUpperCase()}
                  </button>
                ))}
              </div>
            </div>

            <div className="border-t pt-4">
              <h3 className="text-sm font-semibold text-gray-900 mb-3">难度</h3>
              <select
                value={difficulty}
                onChange={(e) => setDifficulty(e.target.value as any)}
                className="w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              >
                <option value="all">全部难度</option>
                <option value="简单">简单</option>
                <option value="中等">中等</option>
                <option value="困难">困难</option>
              </select>
            </div>

            <div className="border-t pt-4">
              <h3 className="text-sm font-semibold text-gray-900 mb-3">搜索</h3>
              <input
                placeholder="搜索题目/标签"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                className="w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>

            {allTags.length > 0 && (
              <div className="border-t pt-4">
                <h3 className="text-sm font-semibold text-gray-900 mb-3">标签</h3>
                <div className="flex flex-wrap gap-2">
                  {allTags.map((t) => (
                    <button
                      key={t}
                      onClick={() => setSelectedTag(selectedTag === t ? null : t)}
                      className={`text-xs px-2.5 py-1.5 rounded transition-all whitespace-nowrap flex-shrink-0 ${
                        selectedTag === t
                          ? 'bg-blue-600 text-white shadow-md'
                          : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
                      }`}
                    >
                      {t}
                    </button>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Main Grid - offset for fixed right panel */}
      <main className="lg:pr-40">
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5">
          {filtered.map((p: Problem) => (
            <Link key={p.id} to={`/problem/${p.id}`} className="h-full">
              <ProblemCard p={p} />
            </Link>
          ))}
        </div>
        
        {filtered.length === 0 && (
          <div className="text-center py-12">
            <p className="text-gray-500 text-lg">没有找到匹配的题目</p>
          </div>
        )}
      </main>

      {/* Mobile Filter Toggle */}
      <div className="lg:hidden mb-4 p-4 bg-white rounded-lg shadow-sm">
        <div className="text-sm font-medium mb-3">快速筛选</div>
        <div className="space-y-3">
          <div>
            <label className="text-xs text-gray-600 block mb-2">类别</label>
            <div className="flex flex-wrap gap-2">
              {categories.map((c) => (
                <button
                  key={c}
                  onClick={() => setCategory(c)}
                  className={`px-2 py-1 rounded text-xs ${category === c ? 'bg-blue-600 text-white' : 'bg-gray-100'}`}
                >
                  {c === 'all' ? '全' : c}
                </button>
              ))}
            </div>
          </div>
          <div>
            <label className="text-xs text-gray-600 block mb-2">难度</label>
            <select
              value={difficulty}
              onChange={(e) => setDifficulty(e.target.value as any)}
              className="w-full border rounded px-2 py-1 text-xs"
            >
              <option value="all">全部</option>
              <option value="简单">简单</option>
              <option value="中等">中等</option>
              <option value="困难">困难</option>
            </select>
          </div>
          <input
            placeholder="搜索"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            className="w-full border rounded px-2 py-1 text-xs"
          />
        </div>
      </div>
    </div>
  )
}
