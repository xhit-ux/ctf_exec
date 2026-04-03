import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import { NavLink, Outlet, useNavigate, useParams } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'
import { staticUrl } from '../../utils/api'
import { Contest, ContestAdmin, fetchContestAdmins, fetchContestChallenges, fetchContestDetail } from '../../utils/contestApi'

type ContestContextValue = {
  contest: Contest
  isParticipant: boolean
  isAdmin: boolean
  adminRole: 'admin' | 'observer' | null
  canEditAdminModules: boolean
  refreshContest: () => Promise<void>
  refreshAccess: () => Promise<void>
}

const ContestContext = createContext<ContestContextValue | null>(null)

export function useContestContext(): ContestContextValue {
  const context = useContext(ContestContext)
  if (!context) {
    throw new Error('useContestContext must be used within ContestLayout')
  }
  return context
}

function formatTime(input?: string | null): string {
  if (!input) return '-'
  const date = new Date(input)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('zh-CN', { hour12: false })
}

function buildTabClass(isActive: boolean): string {
  return isActive
    ? 'px-3 py-2 rounded-lg text-sm font-semibold bg-blue-600 text-white'
    : 'px-3 py-2 rounded-lg text-sm font-semibold text-gray-600 hover:bg-gray-100'
}

export default function ContestLayout() {
  const { contestId } = useParams()
  const navigate = useNavigate()
  const { user, isAuthenticated } = useAuth()

  const [contest, setContest] = useState<Contest | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const [isParticipant, setIsParticipant] = useState(false)
  const [isAdmin, setIsAdmin] = useState(false)
  const [adminRole, setAdminRole] = useState<'admin' | 'observer' | null>(null)

  const refreshContest = useCallback(async () => {
    if (!contestId) return
    const data = await fetchContestDetail(contestId)
    setContest(data)
  }, [contestId])

  const refreshAccess = useCallback(async () => {
    if (!contestId || !isAuthenticated || !user) {
      setIsParticipant(false)
      setIsAdmin(false)
      setAdminRole(null)
      return
    }

    let participant = false
    try {
      await fetchContestChallenges(contestId)
      participant = true
    } catch {
      participant = false
    }

    let admins: ContestAdmin[] = []
    try {
      admins = await fetchContestAdmins(contestId)
    } catch {
      admins = []
    }

    const adminEntry = admins.find((item) => item.user_id === user.id)
    const elevated = user.role === 'root' || user.role === 'site_admin'

    setIsParticipant(participant)
    setIsAdmin(Boolean(adminEntry) || elevated)
    if (elevated) {
      setAdminRole('admin')
    } else {
      setAdminRole(adminEntry?.role || null)
    }
  }, [contestId, isAuthenticated, user])

  useEffect(() => {
    let active = true

    const run = async () => {
      if (!contestId) {
        setError('缺少赛事ID')
        setLoading(false)
        return
      }

      setLoading(true)
      setError('')
      try {
        await refreshContest()
      } catch (err) {
        const msg = err instanceof Error ? err.message : '获取赛事信息失败'
        if (active) setError(msg)
      }

      try {
        await refreshAccess()
      } finally {
        if (active) setLoading(false)
      }
    }

    run()
    return () => {
      active = false
    }
  }, [contestId, refreshContest, refreshAccess])

  const canEditAdminModules = adminRole === 'admin'

  const contestStatus = useMemo(() => {
    if (!contest) return 'unknown'
    const now = Date.now()
    const start = contest.start_time ? new Date(contest.start_time).getTime() : 0
    const end = contest.end_time ? new Date(contest.end_time).getTime() : Number.MAX_SAFE_INTEGER
    if (now < start) return 'upcoming'
    if (now > end) return 'ended'
    return 'running'
  }, [contest])

  const statusText = contestStatus === 'running' ? '进行中' : contestStatus === 'upcoming' ? '未开始' : contestStatus === 'ended' ? '已结束' : '-'

  if (loading) {
    return <div className="p-10 text-center text-gray-500">赛事页面加载中...</div>
  }

  if (error || !contest) {
    return (
      <div className="rounded-xl border border-red-100 bg-red-50 p-6 space-y-3">
        <div className="text-red-700 font-semibold">{error || '赛事加载失败'}</div>
        <button
          type="button"
          onClick={() => navigate('/contests')}
          className="px-4 py-2 rounded-lg bg-white border border-red-200 text-red-700 text-sm"
        >
          返回赛事列表
        </button>
      </div>
    )
  }

  return (
    <ContestContext.Provider
      value={{
        contest,
        isParticipant,
        isAdmin,
        adminRole,
        canEditAdminModules,
        refreshContest,
        refreshAccess,
      }}
    >
      <div className="space-y-6">
        <section className="overflow-hidden rounded-2xl border border-gray-100 bg-white shadow-sm">
          <div className="h-52 bg-gray-100">
            {contest.cover_image ? (
              <img src={staticUrl(contest.cover_image)} alt={contest.title} className="h-full w-full object-cover" />
            ) : (
              <div className="h-full w-full bg-gradient-to-r from-sky-100 to-cyan-100" />
            )}
          </div>

          <div className="space-y-4 p-6">
            <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
              <div>
                <h1 className="text-2xl font-extrabold text-gray-900">{contest.title}</h1>
                <p className="mt-2 text-sm text-gray-600">{contest.description || '暂无赛事描述'}</p>
              </div>
              <span className="inline-flex w-fit rounded-full border border-blue-100 bg-blue-50 px-3 py-1 text-sm text-blue-700">{statusText}</span>
            </div>

            <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
              <div className="rounded-lg border border-gray-100 bg-gray-50 p-3 text-sm">
                <div className="text-gray-500">开始时间</div>
                <div className="mt-1 font-semibold text-gray-800">{formatTime(contest.start_time)}</div>
              </div>
              <div className="rounded-lg border border-gray-100 bg-gray-50 p-3 text-sm">
                <div className="text-gray-500">结束时间</div>
                <div className="mt-1 font-semibold text-gray-800">{formatTime(contest.end_time)}</div>
              </div>
              <div className="rounded-lg border border-gray-100 bg-gray-50 p-3 text-sm">
                <div className="text-gray-500">计分模式</div>
                <div className="mt-1 font-semibold text-gray-800">{contest.scoring_type || 'jeopardy'}</div>
              </div>
            </div>
          </div>
        </section>

        <section className="rounded-2xl border border-gray-100 bg-white p-4 shadow-sm">
          <div className="flex flex-wrap gap-2">
            <NavLink to="." end className={({ isActive }) => buildTabClass(isActive)}>概览</NavLink>
            {(isParticipant || isAdmin) && <NavLink to="problems" className={({ isActive }) => buildTabClass(isActive)}>题目</NavLink>}
            <NavLink to="board" className={({ isActive }) => buildTabClass(isActive)}>榜单</NavLink>
            {(isParticipant || isAdmin) && <NavLink to="submissions" className={({ isActive }) => buildTabClass(isActive)}>我的提交</NavLink>}
            {(isParticipant || isAdmin) && <NavLink to="participants" className={({ isActive }) => buildTabClass(isActive)}>参赛者</NavLink>}
            {isAdmin && <NavLink to="admin" className={({ isActive }) => buildTabClass(isActive)}>管理</NavLink>}
          </div>
        </section>

        <Outlet />
      </div>
    </ContestContext.Provider>
  )
}
