import React, { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { fromUnknownError } from '../../utils/error'
import { ContestChallenge, fetchContestChallenges } from '../../utils/contestApi'
import { useContestContext } from './ContestLayout'

export default function ContestProblems() {
  const { contest, isParticipant, isAdmin } = useContestContext()
  const [items, setItems] = useState<ContestChallenge[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [category, setCategory] = useState('all')

  useEffect(() => {
    let active = true

    const run = async () => {
      if (!isParticipant && !isAdmin) {
        setLoading(false)
        setItems([])
        return
      }

      setLoading(true)
      setError('')
      try {
        const list = await fetchContestChallenges(contest.id)
        if (active) setItems(list)
      } catch (err) {
        if (active) setError(fromUnknownError(err, '获取赛事题目失败'))
      } finally {
        if (active) setLoading(false)
      }
    }

    run()
    return () => {
      active = false
    }
  }, [contest.id, isAdmin, isParticipant])

  const categories = useMemo(() => {
    const set = new Set<string>()
    items.forEach((item) => {
      if (item.category) set.add(item.category)
    })
    return ['all', ...Array.from(set).sort()]
  }, [items])

  const filtered = useMemo(() => {
    if (category === 'all') return items
    return items.filter((item) => item.category === category)
  }, [category, items])

  return (
    <section className="glass-card p-6 space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-xl font-bold text-gray-900">赛事题目</h2>
        <select value={category} onChange={(event) => setCategory(event.target.value)} className="glass-input w-auto">
          {categories.map((name) => <option key={name} value={name}>{name === 'all' ? '全部分类' : name}</option>)}
        </select>
      </div>

      {(!isParticipant && !isAdmin) ? (
        <div className="rounded-lg border border-amber-100 bg-amber-50 px-3 py-2 text-sm text-amber-700">请先报名后查看赛事题目。</div>
      ) : loading ? (
        <div className="text-sm text-gray-500">题目加载中...</div>
      ) : error ? (
        <div className="rounded-lg border border-red-100 bg-red-50 px-3 py-2 text-sm text-red-600">{error}</div>
      ) : filtered.length === 0 ? (
        <div className="text-sm text-gray-500">暂无题目</div>
      ) : (
        <div className="glass-table-wrap overflow-x-auto">
          <table className="w-full min-w-[720px] text-sm">
            <thead>
              <tr className="border-b border-gray-100 text-gray-500">
                <th className="px-3 py-3 text-left">题目</th>
                <th className="px-3 py-3 text-left">分类</th>
                <th className="px-3 py-3 text-left">难度</th>
                <th className="px-3 py-3 text-center">分值</th>
                <th className="px-3 py-3 text-center">状态</th>
                <th className="px-3 py-3 text-right">操作</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((item) => (
                <tr key={item.id} className="border-b border-gray-100 last:border-b-0">
                  <td className="px-3 py-3 font-semibold text-gray-800">{item.title}</td>
                  <td className="px-3 py-3 text-gray-600">{item.category || '-'}</td>
                  <td className="px-3 py-3 text-gray-600">{item.difficulty || '-'}</td>
                  <td className="px-3 py-3 text-center font-bold text-amber-700">
                    {item.current_score}
                    {item.is_dynamic_score && (
                      <span className="ml-1 text-[10px] font-normal text-amber-500" title="动态衰减">
                        ↘
                      </span>
                    )}
                  </td>
                  <td className="px-3 py-3 text-center">
                    {item.is_locked ? (
                      <span className="rounded-full bg-amber-100 px-2 py-1 text-xs text-amber-700">已锁定</span>
                    ) : item.is_solved ? (
                      <span className="rounded-full bg-green-100 px-2 py-1 text-xs text-green-700">已解出</span>
                    ) : (
                      <span className="rounded-full bg-gray-100 px-2 py-1 text-xs text-gray-600">未解出</span>
                    )}
                  </td>
                  <td className="px-3 py-3 text-right">
                    <Link
                      className="font-semibold text-amber-700 hover:text-amber-800"
                      to={`/problems/${item.problem_id}?contest_id=${contest.id}`}
                    >
                      进入题目
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  )
}
