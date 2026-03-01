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
    problems.forEach((p) => p.tags && p.tags.forEach((t) => s.add(t)))
    return Array.from(s)
  }, [problems])

  const filtered = useMemo(() => {
    return problems.filter((p) => {
      if (category !== 'all' && p.category !== category) return false
      if (difficulty !== 'all' && p.difficulty !== difficulty) return false
      if (selectedTag && !p.tags?.includes(selectedTag)) return false
      if (query && !(`${p.title} ${p.tags?.join(' ')}`.toLowerCase().includes(query.toLowerCase()))) return false
      return true
    })
  }, [category, difficulty, query, selectedTag, problems])

  if (!isAuthenticated) return <div className="p-10 text-center">请先登录查看题目</div>
  if (loading) return <div className="p-10 text-center">加载中...</div>

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-xl font-medium">题目列表</h2>
        <div className="text-sm text-gray-500">共 {filtered.length} 道题（全部 {problems.length}）</div>
      </div>

      <div className="lg:flex lg:items-start lg:gap-6">
        <aside className="mb-4 lg:mb-0 lg:w-72 flex-shrink-0">
          <div className="bg-white p-4 rounded border shadow-sm sticky top-20">
            <div className="mb-4">
              <h3 className="text-sm font-medium mb-2">类别</h3>
              <div className="flex flex-wrap gap-2">
                {categories.map((c) => (
                  <button
                    key={c}
                    onClick={() => setCategory(c)}
                    className={`px-3 py-1 rounded text-sm whitespace-nowrap flex-shrink-0 ${category === c ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-700'}`}>
                    {c === 'all' ? '全部' : c.toUpperCase()}
                  </button>
                ))}
              </div>
            </div>

            <div className="mb-4">
              <h3 className="text-sm font-medium mb-2">难度</h3>
              <select value={difficulty} onChange={(e) => setDifficulty(e.target.value as any)} className="w-full border rounded px-2 py-1 text-sm">
                <option value="all">全部难度</option>
                <option value="简单">简单</option>
                <option value="中等">中等</option>
                <option value="困难">困难</option>
              </select>
            </div>

            <div className="mb-4">
              <h3 className="text-sm font-medium mb-2">搜索</h3>
              <input placeholder="搜索题目/标签" value={query} onChange={(e) => setQuery(e.target.value)} className="w-full border rounded px-3 py-1" />
            </div>

            <div>
              <h3 className="text-sm font-medium mb-2">标签</h3>
              <div className="flex flex-col gap-2">
                {(() => {
                  const mid = Math.ceil(allTags.length / 2)
                  const rows = [allTags.slice(0, mid), allTags.slice(mid)]
                  return rows.map((row, idx) => (
                    <div key={idx} className="flex gap-2 flex-wrap">
                      {row.map((t) => (
                        <button
                          key={t}
                          onClick={() => setSelectedTag(selectedTag === t ? null : t)}
                          className={`text-xs px-2 py-1 rounded whitespace-nowrap flex-shrink-0 ${selectedTag === t ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-700'}`}>
                          {t}
                        </button>
                      ))}
                    </div>
                  ))
                })()}
              </div>
            </div>
          </div>
        </aside>

        <main className="flex-1">
          <div className="hidden lg:block mb-4 text-sm text-gray-500">共 {filtered.length} 道题（全部 {problems.length}）</div>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
            {filtered.map((p: Problem) => (
              <Link key={p.id} to={`/problem/${p.id}`}>
                <ProblemCard p={p} />
              </Link>
            ))}
          </div>
        </main>
      </div>
    </div>
  )
}
