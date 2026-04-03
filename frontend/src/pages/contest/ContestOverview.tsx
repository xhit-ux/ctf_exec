import React, { useEffect, useMemo, useState } from 'react'
import { useAuth } from '../../context/AuthContext'
import { apiUrl } from '../../utils/api'
import { apiFetch } from '../../utils/authClient'
import { fromUnknownError } from '../../utils/error'
import { fetchContestParticipants, joinContest, leaveContest } from '../../utils/contestApi'
import { useContestContext } from './ContestLayout'

type TeamBasic = {
  id: string
  name: string
}

export default function ContestOverview() {
  const { contest, refreshAccess } = useContestContext()
  const { isAuthenticated } = useAuth()

  const [team, setTeam] = useState<TeamBasic | null>(null)
  const [joinType, setJoinType] = useState<'user' | 'team'>('user')
  const [actionLoading, setActionLoading] = useState(false)
  const [actionError, setActionError] = useState('')
  const [actionSuccess, setActionSuccess] = useState('')
  const [participantsCount, setParticipantsCount] = useState(0)

  const phase = useMemo(() => {
    const now = Date.now()
    const start = contest.start_time ? new Date(contest.start_time).getTime() : 0
    const end = contest.end_time ? new Date(contest.end_time).getTime() : Number.MAX_SAFE_INTEGER
    if (now < start) return '未开始'
    if (now > end) return '已结束'
    return '进行中'
  }, [contest.end_time, contest.start_time])

  useEffect(() => {
    let active = true

    const loadTeam = async () => {
      if (!isAuthenticated) {
        setTeam(null)
        return
      }

      try {
        const response = await apiFetch(apiUrl('/api/teams/my'))
        if (!response.ok) {
          if (active) setTeam(null)
          return
        }

        const payload = await response.json() as TeamBasic
        if (active && payload?.id) {
          setTeam(payload)
        }
      } catch {
        if (active) setTeam(null)
      }
    }

    const loadCount = async () => {
      if (!isAuthenticated) return
      try {
        const data = await fetchContestParticipants(contest.id, 1, 1)
        if (active) setParticipantsCount(data.total)
      } catch {
        if (active) setParticipantsCount(0)
      }
    }

    loadTeam()
    loadCount()

    return () => {
      active = false
    }
  }, [contest.id, isAuthenticated])

  const handleJoin = async () => {
    setActionLoading(true)
    setActionError('')
    setActionSuccess('')
    try {
      if (joinType === 'team') {
        if (!team?.id) {
          throw new Error('请选择团队报名或先加入团队')
        }
        await joinContest(contest.id, { team_id: team.id })
      } else {
        await joinContest(contest.id)
      }
      setActionSuccess('报名成功')
      await refreshAccess()
    } catch (err) {
      setActionError(fromUnknownError(err, '报名失败'))
    } finally {
      setActionLoading(false)
    }
  }

  const handleLeave = async () => {
    setActionLoading(true)
    setActionError('')
    setActionSuccess('')
    try {
      if (joinType === 'team') {
        if (!team?.id) {
          throw new Error('当前未加入团队')
        }
        await leaveContest(contest.id, { team_id: team.id })
      } else {
        await leaveContest(contest.id)
      }
      setActionSuccess('退赛成功')
      await refreshAccess()
    } catch (err) {
      setActionError(fromUnknownError(err, '退赛失败'))
    } finally {
      setActionLoading(false)
    }
  }

  return (
    <section className="rounded-2xl border border-gray-100 bg-white p-6 shadow-sm space-y-5">
      <div>
        <h2 className="text-xl font-bold text-gray-900">赛事概览</h2>
        <p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-gray-600">{contest.description || '暂无赛事描述'}</p>
      </div>

      <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4 text-sm">
        <div className="rounded-lg border border-gray-100 bg-gray-50 p-3"><div className="text-gray-500">赛事阶段</div><div className="mt-1 font-semibold text-gray-800">{phase}</div></div>
        <div className="rounded-lg border border-gray-100 bg-gray-50 p-3"><div className="text-gray-500">可见性</div><div className="mt-1 font-semibold text-gray-800">{contest.visibility}</div></div>
        <div className="rounded-lg border border-gray-100 bg-gray-50 p-3"><div className="text-gray-500">计分模式</div><div className="mt-1 font-semibold text-gray-800">{contest.scoring_type || 'jeopardy'}</div></div>
        <div className="rounded-lg border border-gray-100 bg-gray-50 p-3"><div className="text-gray-500">参赛人数</div><div className="mt-1 font-semibold text-gray-800">{participantsCount}</div></div>
      </div>

      <div className="rounded-xl border border-gray-100 bg-slate-50 p-4 space-y-3">
        <div className="text-sm font-semibold text-gray-800">报名操作</div>
        <div className="flex items-center gap-2">
          <button type="button" onClick={() => setJoinType('user')} className={`px-3 py-1.5 rounded-md text-sm ${joinType === 'user' ? 'bg-blue-600 text-white' : 'bg-white border border-gray-200 text-gray-700'}`}>个人</button>
          <button type="button" onClick={() => setJoinType('team')} className={`px-3 py-1.5 rounded-md text-sm ${joinType === 'team' ? 'bg-blue-600 text-white' : 'bg-white border border-gray-200 text-gray-700'}`}>团队</button>
          {joinType === 'team' && <span className="text-xs text-gray-500">{team ? `当前团队: ${team.name}` : '你当前没有团队'}</span>}
        </div>

        <div className="flex flex-wrap gap-3">
          <button type="button" disabled={!isAuthenticated || actionLoading} onClick={handleJoin} className="px-4 py-2 rounded-lg bg-emerald-600 text-white text-sm font-semibold disabled:opacity-50">{actionLoading ? '处理中...' : '报名'}</button>
          <button type="button" disabled={!isAuthenticated || actionLoading} onClick={handleLeave} className="px-4 py-2 rounded-lg bg-white border border-gray-300 text-sm font-semibold text-gray-700 disabled:opacity-50">退赛</button>
          {!isAuthenticated && <span className="text-xs text-gray-500">请先登录后报名</span>}
        </div>

        {actionError && <div className="rounded-lg border border-red-100 bg-red-50 px-3 py-2 text-sm text-red-600">{actionError}</div>}
        {actionSuccess && <div className="rounded-lg border border-green-100 bg-green-50 px-3 py-2 text-sm text-green-600">{actionSuccess}</div>}
      </div>
    </section>
  )
}
