import React, { useEffect, useState } from 'react'
import { useAuth } from '../../context/AuthContext'
import { fromUnknownError } from '../../utils/error'
import { ContestScoreboardEntry, fetchContestScoreboard } from '../../utils/contestApi'
import { useContestContext } from './ContestLayout'

function formatTime(input?: string | null): string {
  if (!input) return '-'
  const date = new Date(input)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('zh-CN', { hour12: false })
}

export default function ContestScoreboard() {
  const { contest } = useContestContext()
  const { user } = useAuth()

  const [type, setType] = useState<'user' | 'team'>('user')
  const [items, setItems] = useState<ContestScoreboardEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let active = true

    const run = async () => {
      setLoading(true)
      setError('')
      try {
        const list = await fetchContestScoreboard(contest.id, type)
        if (active) setItems(list)
      } catch (err) {
        if (active) setError(fromUnknownError(err, '获取赛事榜单失败'))
      } finally {
        if (active) setLoading(false)
      }
    }

    run()
    return () => {
      active = false
    }
  }, [contest.id, type])

  return (
    <section className="rounded-2xl border border-gray-100 bg-white p-6 shadow-sm space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-xl font-bold text-gray-900">赛事榜单</h2>
        <div className="inline-flex rounded-lg bg-gray-100 p-1">
          <button type="button" onClick={() => setType('user')} className={`px-3 py-1.5 rounded-md text-sm font-semibold ${type === 'user' ? 'bg-white text-blue-600 shadow' : 'text-gray-600'}`}>个人榜</button>
          <button type="button" onClick={() => setType('team')} className={`px-3 py-1.5 rounded-md text-sm font-semibold ${type === 'team' ? 'bg-white text-blue-600 shadow' : 'text-gray-600'}`}>团队榜</button>
        </div>
      </div>

      {loading ? (
        <div className="text-sm text-gray-500">榜单加载中...</div>
      ) : error ? (
        <div className="rounded-lg border border-red-100 bg-red-50 px-3 py-2 text-sm text-red-600">{error}</div>
      ) : items.length === 0 ? (
        <div className="text-sm text-gray-500">暂无榜单数据</div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[680px] text-sm">
            <thead>
              <tr className="border-b border-gray-100 text-gray-500">
                <th className="px-3 py-3 text-left">排名</th>
                <th className="px-3 py-3 text-left">对象</th>
                <th className="px-3 py-3 text-center">得分</th>
                <th className="px-3 py-3 text-center">解题数</th>
                <th className="px-3 py-3 text-right">最后解题</th>
              </tr>
            </thead>
            <tbody>
              {items.map((item) => {
                const rowKey = `${type}-${item.user_id || item.team_id || item.rank}`
                const isSelf = type === 'user' && !!user?.id && item.user_id === user.id
                return (
                  <tr key={rowKey} className={`border-b border-gray-100 last:border-b-0 ${isSelf ? 'bg-blue-50/60' : ''}`}>
                    <td className="px-3 py-3 font-bold text-gray-700">{item.rank}</td>
                    <td className="px-3 py-3">
                      <div className="font-semibold text-gray-800">{type === 'user' ? (item.display_name || item.username || item.user_id || '-') : (item.name || item.team_id || '-')}</div>
                      {type === 'user' && item.username && <div className="text-xs text-gray-400">@{item.username}</div>}
                    </td>
                    <td className="px-3 py-3 text-center font-bold text-blue-600">{item.score}</td>
                    <td className="px-3 py-3 text-center text-gray-700">{item.solved_count}</td>
                    <td className="px-3 py-3 text-right text-gray-500">{formatTime(item.last_solve_time)}</td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </section>
  )
}
