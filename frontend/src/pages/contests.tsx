import React, { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiUrl, staticUrl } from '../utils/api'
import { apiFetch } from '../utils/authClient'
import { MOCK_CONTESTS } from '../data/mockContests'

type ContestItem = {
  id: string
  title: string
  description: string
  cover_image?: string
  start_time?: string | null
  end_time?: string | null
  scoring_type?: string
}

type ContestStatus = 'upcoming' | 'running' | 'ended'

function getContestStatus(startTime?: string | null, endTime?: string | null): ContestStatus {
  const now = Date.now()
  const start = startTime ? new Date(startTime).getTime() : 0
  const end = endTime ? new Date(endTime).getTime() : Number.MAX_SAFE_INTEGER

  if (now < start) return 'upcoming'
  if (now > end) return 'ended'
  return 'running'
}

function statusLabel(status: ContestStatus): string {
  if (status === 'upcoming') return '未开始'
  if (status === 'running') return '进行中'
  return '已结束'
}

function statusClass(status: ContestStatus): string {
  if (status === 'upcoming') return 'bg-amber-50 text-amber-700 border-amber-200'
  if (status === 'running') return 'bg-emerald-50 text-emerald-700 border-emerald-200'
  return 'bg-gray-100 text-gray-600 border-gray-200'
}

function formatTime(input?: string | null): string {
  if (!input) return '-'
  const t = new Date(input)
  if (Number.isNaN(t.getTime())) return '-'
  return t.toLocaleString('zh-CN', { hour12: false })
}

export default function Contests() {
  const [contests, setContests] = useState<ContestItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [usingMock, setUsingMock] = useState(false)
  const [query, setQuery] = useState('')
  const [statusFilter, setStatusFilter] = useState<'all' | ContestStatus>('all')

  useEffect(() => {
    let canceled = false

    const load = async () => {
        setLoading(true)
        setError('')
      try {
        const res = await apiFetch(apiUrl('/api/contests?limit=50'), undefined, {
          disableAuth: true,
          disableRetry401: true,
        })
        if (!res.ok) {
          throw new Error('获取赛事列表失败')
        }

        const data = await res.json()
        const list = Array.isArray(data) ? data : []
        if (!canceled) {
          setContests(list)
          setUsingMock(false)
        }
      } catch (err) {
        console.error(err)
        if (!canceled) {
          setError('接口不可用，当前展示 Mock 数据')
          setContests(MOCK_CONTESTS)
          setUsingMock(true)
        }
      } finally {
        if (!canceled) {
          setLoading(false)
        }
      }
    }

    load()
    return () => {
      canceled = true
    }
  }, [])

  const filtered = useMemo(() => {
    return contests.filter((contest) => {
      const status = getContestStatus(contest.start_time, contest.end_time)
      if (statusFilter !== 'all' && status !== statusFilter) return false

      const text = `${contest.title} ${contest.description || ''}`.toLowerCase()
      if (query && !text.includes(query.toLowerCase())) return false

      return true
    })
  }, [contests, query, statusFilter])

  return (
    <div className="max-w-7xl mx-auto space-y-6">
      <div className="flex flex-col lg:flex-row lg:items-end lg:justify-between gap-4">
        <div>
          <h1 className="text-3xl font-extrabold tracking-tight text-gray-900">赛事中心</h1>
          <p className="text-gray-500 mt-2">查看赛事详情、报名状态和比赛时间窗口</p>
        </div>

        <div className="flex flex-col sm:flex-row gap-3 w-full lg:w-auto">
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="搜索赛事标题或简介"
            className="w-full sm:w-72 bg-white border border-gray-200 rounded-lg px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
          />
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value as 'all' | ContestStatus)}
            className="bg-white border border-gray-200 rounded-lg px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
          >
            <option value="all">全部状态</option>
            <option value="upcoming">未开始</option>
            <option value="running">进行中</option>
            <option value="ended">已结束</option>
          </select>
        </div>
      </div>

      {error && <div className="p-4 rounded-lg border border-red-100 bg-red-50 text-red-600">{error}</div>}
      {usingMock && <div className="p-3 rounded-lg border border-blue-100 bg-blue-50 text-blue-700 text-sm">当前为演示模式，数据来自本地 Mock。</div>}

      {loading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">
          {Array.from({ length: 6 }).map((_, idx) => (
            <div key={idx} className="h-64 rounded-2xl bg-white border border-gray-100 shadow-sm animate-pulse" />
          ))}
        </div>
      ) : filtered.length === 0 ? (
        <div className="bg-white border border-gray-100 rounded-2xl p-10 text-center text-gray-500">暂无符合条件的赛事</div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">
          {filtered.map((contest) => {
            const status = getContestStatus(contest.start_time, contest.end_time)
            return (
              <Link
                key={contest.id}
                to={`/contest/${contest.id}`}
                className="group block rounded-2xl overflow-hidden border border-gray-100 bg-white shadow-sm hover:shadow-md transition"
              >
                <div className="h-36 bg-gray-100 overflow-hidden">
                  {contest.cover_image ? (
                    <img
                      src={staticUrl(contest.cover_image)}
                      alt={contest.title}
                      className="h-full w-full object-cover group-hover:scale-105 transition duration-300"
                    />
                  ) : (
                    <div className="h-full w-full bg-gradient-to-r from-blue-100 to-cyan-100" />
                  )}
                </div>

                <div className="p-5 space-y-3">
                  <div className="flex items-center justify-between gap-3">
                    <h2 className="font-bold text-gray-900 line-clamp-1">{contest.title}</h2>
                    <span className={`text-xs px-2 py-1 rounded-full border ${statusClass(status)}`}>
                      {statusLabel(status)}
                    </span>
                  </div>

                  <p className="text-sm text-gray-600 line-clamp-2 min-h-[40px]">
                    {contest.description || '暂无赛事描述'}
                  </p>

                  <div className="text-xs text-gray-500 space-y-1 pt-2 border-t border-gray-100">
                    <div>开始: {formatTime(contest.start_time)}</div>
                    <div>结束: {formatTime(contest.end_time)}</div>
                  </div>
                </div>
              </Link>
            )
          })}
        </div>
      )}
    </div>
  )
}
