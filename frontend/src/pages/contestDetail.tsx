import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { apiUrl, staticUrl } from '../utils/api'
import { apiFetch } from '../utils/authClient'
import { fromUnknownError, readSafeErrorMessage } from '../utils/error'
import {
  MOCK_CONTESTS,
  MOCK_CONTEST_CHALLENGES,
  MOCK_CONTEST_USER_SCOREBOARD,
  MOCK_CONTEST_TEAM_SCOREBOARD,
  MOCK_MY_TEAM,
} from '../data/mockContests'

type ContestDetailData = {
  id: string
  title: string
  description: string
  cover_image?: string
  start_time?: string | null
  end_time?: string | null
  scoring_type?: string
}

type ContestStatus = 'upcoming' | 'running' | 'ended'

type ContestChallenge = {
  id: number
  title: string
  category?: string
  difficulty?: string
  points?: number
  score_override?: number | null
  is_solved?: boolean
}

type ContestScoreUser = {
  rank?: number
  user_id?: string
  username?: string
  display_name?: string
  score?: number
  solved_count?: number
  last_solve_time?: string | null
}

type ContestScoreTeam = {
  rank?: number
  team_id?: string
  id?: string
  name?: string
  score?: number
  solved_count?: number
  last_solve_time?: string | null
}

type TeamBasic = {
  id: string
  name: string
}

function getContestStatus(startTime?: string | null, endTime?: string | null): ContestStatus {
  const now = Date.now()
  const start = startTime ? new Date(startTime).getTime() : 0
  const end = endTime ? new Date(endTime).getTime() : Number.MAX_SAFE_INTEGER

  if (now < start) return 'upcoming'
  if (now > end) return 'ended'
  return 'running'
}

function statusText(status: ContestStatus): string {
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
  const d = new Date(input)
  if (Number.isNaN(d.getTime())) return '-'
  return d.toLocaleString('zh-CN', { hour12: false })
}

function asArray<T>(payload: unknown): T[] {
  if (Array.isArray(payload)) return payload as T[]
  if (payload && typeof payload === 'object') {
    const candidate = payload as Record<string, unknown>
    if (Array.isArray(candidate.items)) return candidate.items as T[]
    if (Array.isArray(candidate.data)) return candidate.data as T[]
    if (Array.isArray(candidate.list)) return candidate.list as T[]
  }
  return []
}

export default function ContestDetail() {
  const { id } = useParams()
  const contestID = id || ''
  const hasStoredAuth = Boolean(localStorage.getItem('token') || localStorage.getItem('refresh_token'))

  const [contest, setContest] = useState<ContestDetailData | null>(null)
  const [loadingContest, setLoadingContest] = useState(true)
  const [contestError, setContestError] = useState('')
  const [usingMock, setUsingMock] = useState(false)

  const [joinType, setJoinType] = useState<'user' | 'team'>('user')
  const [team, setTeam] = useState<TeamBasic | null>(null)
  const [actionLoading, setActionLoading] = useState(false)
  const [actionMessage, setActionMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  const [challenges, setChallenges] = useState<ContestChallenge[]>([])
  const [loadingChallenges, setLoadingChallenges] = useState(false)
  const [challengeError, setChallengeError] = useState('')

  const [scoreType, setScoreType] = useState<'user' | 'team'>('user')
  const [scoreLoading, setScoreLoading] = useState(false)
  const [scoreError, setScoreError] = useState('')
  const [userScores, setUserScores] = useState<ContestScoreUser[]>([])
  const [teamScores, setTeamScores] = useState<ContestScoreTeam[]>([])

  const contestStatus = useMemo(() => {
    return getContestStatus(contest?.start_time, contest?.end_time)
  }, [contest?.start_time, contest?.end_time])

  const fetchContest = useCallback(async () => {
    if (!contestID) return

    setLoadingContest(true)
    setContestError('')
    try {
      const res = await apiFetch(apiUrl(`/api/contests/${contestID}`), undefined, {
        disableAuth: true,
        disableRetry401: true,
      })
      if (!res.ok) {
        throw new Error('获取赛事详情失败')
      }
      const data = await res.json()
      setContest(data)
      setUsingMock(false)
      setContestError('')
    } catch (err) {
      console.error(err)
      const fallback = MOCK_CONTESTS.find((item) => item.id === contestID) || MOCK_CONTESTS[0] || null
      setContest(fallback)
      setUsingMock(true)
      setContestError('接口不可用，当前展示 Mock 数据')
    } finally {
      setLoadingContest(false)
    }
  }, [contestID])

  const fetchMyTeam = useCallback(async () => {
    if (usingMock) {
      setTeam(MOCK_MY_TEAM)
      return
    }

    if (!hasStoredAuth) {
      setTeam(null)
      return
    }

    try {
      const res = await apiFetch(apiUrl('/api/teams/my'))

      if (res.status === 401) {
        return
      }
      if (!res.ok) {
        setTeam(null)
        return
      }

      const data = await res.json()
      if (data?.id && data?.name) {
        setTeam({ id: data.id, name: data.name })
      } else {
        setTeam(null)
      }
    } catch (err) {
      console.error(err)
      setTeam(null)
    }
  }, [hasStoredAuth, usingMock])

  const fetchChallenges = useCallback(async () => {
    if (!contestID) return

    if (usingMock) {
      setChallenges(MOCK_CONTEST_CHALLENGES[contestID] || [])
      setChallengeError('')
      setLoadingChallenges(false)
      return
    }

    if (!hasStoredAuth) {
      setChallenges([])
      setChallengeError('')
      setLoadingChallenges(false)
      return
    }

    setLoadingChallenges(true)
    setChallengeError('')

    try {
      const res = await apiFetch(apiUrl(`/api/contests/${contestID}/challenges`))

      if (res.status === 401) {
        return
      }
      if (!res.ok) {
        setChallengeError(await readSafeErrorMessage(res, '获取赛事题目失败，请稍后重试'))
        setChallenges([])
        return
      }

      const payload = await res.json()
      setChallenges(asArray<ContestChallenge>(payload))
    } catch (err) {
      console.error(err)
      setChallengeError(fromUnknownError(err, '获取赛事题目失败，请稍后重试'))
      setChallenges([])
    } finally {
      setLoadingChallenges(false)
    }
  }, [contestID, hasStoredAuth, usingMock])

  const fetchScoreboard = useCallback(async (kind: 'user' | 'team') => {
    if (!contestID) return

    if (usingMock) {
      setScoreError('')
      setScoreLoading(false)
      if (kind === 'user') {
        setUserScores(MOCK_CONTEST_USER_SCOREBOARD[contestID] || [])
      } else {
        setTeamScores(MOCK_CONTEST_TEAM_SCOREBOARD[contestID] || [])
      }
      return
    }

    setScoreLoading(true)
    setScoreError('')

    try {
      const res = await apiFetch(apiUrl(`/api/contests/${contestID}/scoreboard?type=${kind}&limit=50`), undefined, {
        disableAuth: true,
        disableRetry401: true,
      })
      if (!res.ok) {
        setScoreError(await readSafeErrorMessage(res, '获取赛事榜单失败，请稍后重试'))
        if (kind === 'user') setUserScores([])
        if (kind === 'team') setTeamScores([])
        return
      }

      const payload = await res.json()
      if (kind === 'user') {
        setUserScores(asArray<ContestScoreUser>(payload))
      } else {
        setTeamScores(asArray<ContestScoreTeam>(payload))
      }
    } catch (err) {
      console.error(err)
      setScoreError(fromUnknownError(err, '获取赛事榜单失败，请稍后重试'))
      if (kind === 'user') setUserScores([])
      if (kind === 'team') setTeamScores([])
    } finally {
      setScoreLoading(false)
    }
  }, [contestID, usingMock])

  useEffect(() => {
    fetchContest()
  }, [fetchContest])

  useEffect(() => {
    fetchMyTeam()
    fetchChallenges()
  }, [fetchMyTeam, fetchChallenges])

  useEffect(() => {
    fetchScoreboard(scoreType)
  }, [fetchScoreboard, scoreType])

  const handleJoinOrLeave = async (mode: 'join' | 'leave') => {
    if (usingMock) {
      setActionMessage({
        type: 'success',
        text: mode === 'join' ? 'Mock 模式: 报名成功（演示）' : 'Mock 模式: 退赛成功（演示）',
      })
      return
    }

    setActionLoading(true)
    setActionMessage(null)

    const body = joinType === 'team' ? { team_id: team?.id || '' } : {}

    try {
      const res = await apiFetch(apiUrl(`/api/contests/${contestID}/${mode}`), {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(body),
      })

      if (res.status === 401) {
        return
      }
      if (!res.ok) {
        setActionMessage({
          type: 'error',
          text: await readSafeErrorMessage(res, `${mode === 'join' ? '报名' : '退赛'}失败，请稍后重试`),
        })
        return
      }

      setActionMessage({
        type: 'success',
        text: mode === 'join' ? '报名请求已提交' : '退赛请求已提交',
      })

      fetchChallenges()
    } catch (err) {
      setActionMessage({
        type: 'error',
        text: fromUnknownError(err, `${mode === 'join' ? '报名' : '退赛'}失败，请稍后重试`),
      })
    } finally {
      setActionLoading(false)
    }
  }

  if (loadingContest) {
    return <div className="p-10 text-center text-gray-500">赛事详情加载中...</div>
  }

  if (!contest) {
    return (
      <div className="p-8 rounded-xl bg-red-50 border border-red-100 text-red-600">
        {contestError || '赛事不存在'}
      </div>
    )
  }

  return (
    <div className="max-w-7xl mx-auto space-y-8">
      {usingMock && (
        <div className="p-3 rounded-lg border border-blue-100 bg-blue-50 text-blue-700 text-sm">
          当前为演示模式，赛事详情/题目/榜单均来自本地 Mock 数据。
        </div>
      )}

      <div className="bg-white border border-gray-100 rounded-2xl overflow-hidden shadow-sm">
        <div className="h-52 bg-gray-100">
          {contest.cover_image ? (
            <img src={staticUrl(contest.cover_image)} alt={contest.title} className="w-full h-full object-cover" />
          ) : (
            <div className="w-full h-full bg-gradient-to-r from-cyan-100 to-blue-100" />
          )}
        </div>

        <div className="p-6 md:p-8 space-y-4">
          <div className="flex flex-col md:flex-row md:items-start md:justify-between gap-4">
            <div>
              <h1 className="text-2xl md:text-3xl font-extrabold tracking-tight text-gray-900">{contest.title}</h1>
              <p className="text-gray-600 mt-2">{contest.description || '暂无赛事描述'}</p>
            </div>
            <span className={`text-sm px-3 py-1 rounded-full border w-fit ${statusClass(contestStatus)}`}>
              {statusText(contestStatus)}
            </span>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 text-sm">
            <div className="bg-gray-50 border border-gray-100 rounded-lg p-3">
              <div className="text-gray-500">开始时间</div>
              <div className="font-semibold text-gray-800 mt-1">{formatTime(contest.start_time)}</div>
            </div>
            <div className="bg-gray-50 border border-gray-100 rounded-lg p-3">
              <div className="text-gray-500">结束时间</div>
              <div className="font-semibold text-gray-800 mt-1">{formatTime(contest.end_time)}</div>
            </div>
            <div className="bg-gray-50 border border-gray-100 rounded-lg p-3">
              <div className="text-gray-500">计分模式</div>
              <div className="font-semibold text-gray-800 mt-1">{contest.scoring_type || 'jeopardy'}</div>
            </div>
          </div>

          <div className="border border-gray-100 rounded-xl p-4 bg-slate-50/70 space-y-3">
            <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-3">
              <div>
                <h2 className="font-bold text-gray-800">参赛操作</h2>
                <p className="text-sm text-gray-500">支持个人报名或团队报名，提交时将按赛事规则计分。</p>
              </div>

              <div className="flex items-center gap-2">
                <button
                  onClick={() => setJoinType('user')}
                  className={`px-3 py-1.5 rounded-md text-sm font-medium ${joinType === 'user' ? 'bg-blue-600 text-white' : 'bg-white border border-gray-200 text-gray-600'}`}
                >
                  个人
                </button>
                <button
                  onClick={() => setJoinType('team')}
                  disabled={!team}
                  className={`px-3 py-1.5 rounded-md text-sm font-medium ${joinType === 'team' ? 'bg-blue-600 text-white' : 'bg-white border border-gray-200 text-gray-600'} ${!team ? 'opacity-50 cursor-not-allowed' : ''}`}
                >
                  团队
                </button>
              </div>
            </div>

            {joinType === 'team' && (
              <div className="text-sm text-gray-600">
                {team ? `当前团队: ${team.name}` : '你当前不在队伍中，请先去“我的队伍”创建或加入队伍。'}
              </div>
            )}

            <div className="flex flex-wrap items-center gap-3">
              <button
                onClick={() => handleJoinOrLeave('join')}
                disabled={actionLoading || (joinType === 'team' && !team)}
                className="px-4 py-2 rounded-lg bg-emerald-600 text-white text-sm font-semibold hover:bg-emerald-700 disabled:opacity-50"
              >
                {actionLoading ? '处理中...' : '报名参赛'}
              </button>
              <button
                onClick={() => handleJoinOrLeave('leave')}
                disabled={actionLoading || (joinType === 'team' && !team)}
                className="px-4 py-2 rounded-lg bg-white border border-gray-300 text-sm font-semibold text-gray-700 hover:bg-gray-50 disabled:opacity-50"
              >
                退赛
              </button>
              {!hasStoredAuth && <span className="text-xs text-gray-500">报名/退赛需要登录</span>}
            </div>

            {actionMessage && (
              <div className={`text-sm rounded-lg px-3 py-2 border ${actionMessage.type === 'success' ? 'bg-green-50 text-green-700 border-green-100' : 'bg-red-50 text-red-700 border-red-100'}`}>
                {actionMessage.text}
              </div>
            )}
          </div>
        </div>
      </div>

      <section className="bg-white border border-gray-100 rounded-2xl p-6 md:p-8 shadow-sm space-y-4">
        <div className="flex items-center justify-between gap-3">
          <h2 className="text-xl font-bold text-gray-900">赛事题目</h2>
          <button
            onClick={fetchChallenges}
            disabled={loadingChallenges}
            className="px-3 py-1.5 rounded-md text-sm border border-gray-300 text-gray-600 hover:bg-gray-50 disabled:opacity-50"
          >
            刷新
          </button>
        </div>

        {!usingMock && !hasStoredAuth ? (
          <div className="text-sm text-gray-500">登录并报名后可查看赛事题目。</div>
        ) : loadingChallenges ? (
          <div className="text-sm text-gray-500">题目加载中...</div>
        ) : challengeError ? (
          <div className="text-sm text-red-600 bg-red-50 border border-red-100 rounded-lg px-3 py-2">{challengeError}</div>
        ) : challenges.length === 0 ? (
          <div className="text-sm text-gray-500">暂无可见题目（可能尚未报名或赛事未开始）。</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[680px] text-sm">
              <thead>
                <tr className="text-gray-500 border-b border-gray-100">
                  <th className="py-3 px-3 text-left">题目</th>
                  <th className="py-3 px-3 text-left">分类</th>
                  <th className="py-3 px-3 text-left">难度</th>
                  <th className="py-3 px-3 text-center">分值</th>
                  <th className="py-3 px-3 text-center">状态</th>
                  <th className="py-3 px-3 text-right">操作</th>
                </tr>
              </thead>
              <tbody>
                {challenges.map((item) => {
                  const score = item.score_override ?? item.points ?? 0
                  return (
                    <tr key={item.id} className="border-b border-gray-100 last:border-b-0">
                      <td className="py-3 px-3 font-semibold text-gray-800">{item.title}</td>
                      <td className="py-3 px-3 text-gray-600">{(item.category || '-').toUpperCase()}</td>
                      <td className="py-3 px-3 text-gray-600">{item.difficulty || '-'}</td>
                      <td className="py-3 px-3 text-center font-bold text-blue-600">{score}</td>
                      <td className="py-3 px-3 text-center">
                        {item.is_solved ? (
                          <span className="inline-flex px-2 py-1 rounded-full text-xs bg-green-100 text-green-700">已解出</span>
                        ) : (
                          <span className="inline-flex px-2 py-1 rounded-full text-xs bg-gray-100 text-gray-600">未解出</span>
                        )}
                      </td>
                      <td className="py-3 px-3 text-right">
                        <Link
                          to={`/problem/${item.id}?contest_id=${contestID}`}
                          className="text-blue-600 font-semibold hover:text-blue-700"
                        >
                          去提交
                        </Link>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className="bg-white border border-gray-100 rounded-2xl p-6 md:p-8 shadow-sm space-y-4">
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
          <h2 className="text-xl font-bold text-gray-900">赛事榜单</h2>
          <div className="inline-flex bg-gray-100 rounded-lg p-1 w-fit">
            <button
              onClick={() => setScoreType('user')}
              className={`px-4 py-1.5 rounded-md text-sm font-semibold ${scoreType === 'user' ? 'bg-white shadow text-blue-600' : 'text-gray-600'}`}
            >
              个人榜
            </button>
            <button
              onClick={() => setScoreType('team')}
              className={`px-4 py-1.5 rounded-md text-sm font-semibold ${scoreType === 'team' ? 'bg-white shadow text-blue-600' : 'text-gray-600'}`}
            >
              团队榜
            </button>
          </div>
        </div>

        {scoreError && <div className="text-sm text-red-600 bg-red-50 border border-red-100 rounded-lg px-3 py-2">{scoreError}</div>}

        {scoreLoading ? (
          <div className="text-sm text-gray-500">榜单加载中...</div>
        ) : scoreType === 'user' ? (
          userScores.length === 0 ? (
            <div className="text-sm text-gray-500">暂无个人榜数据</div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full min-w-[640px] text-sm">
                <thead>
                  <tr className="text-gray-500 border-b border-gray-100">
                    <th className="py-3 px-3 text-left">排名</th>
                    <th className="py-3 px-3 text-left">用户</th>
                    <th className="py-3 px-3 text-center">得分</th>
                    <th className="py-3 px-3 text-center">解题数</th>
                    <th className="py-3 px-3 text-right">最后解题</th>
                  </tr>
                </thead>
                <tbody>
                  {userScores.map((row, idx) => (
                    <tr key={`${row.user_id || row.username || idx}`} className="border-b border-gray-100 last:border-b-0">
                      <td className="py-3 px-3 font-bold text-gray-700">{row.rank || idx + 1}</td>
                      <td className="py-3 px-3">
                        <div className="font-semibold text-gray-800">{row.display_name || row.username || row.user_id || '-'}</div>
                        {row.username && <div className="text-xs text-gray-400">@{row.username}</div>}
                      </td>
                      <td className="py-3 px-3 text-center font-bold text-blue-600">{row.score || 0}</td>
                      <td className="py-3 px-3 text-center text-gray-700">{row.solved_count || 0}</td>
                      <td className="py-3 px-3 text-right text-gray-500">{formatTime(row.last_solve_time)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )
        ) : teamScores.length === 0 ? (
          <div className="text-sm text-gray-500">暂无团队榜数据</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[640px] text-sm">
              <thead>
                <tr className="text-gray-500 border-b border-gray-100">
                  <th className="py-3 px-3 text-left">排名</th>
                  <th className="py-3 px-3 text-left">团队</th>
                  <th className="py-3 px-3 text-center">得分</th>
                  <th className="py-3 px-3 text-center">解题数</th>
                  <th className="py-3 px-3 text-right">最后解题</th>
                </tr>
              </thead>
              <tbody>
                {teamScores.map((row, idx) => (
                  <tr key={`${row.team_id || row.id || idx}`} className="border-b border-gray-100 last:border-b-0">
                    <td className="py-3 px-3 font-bold text-gray-700">{row.rank || idx + 1}</td>
                    <td className="py-3 px-3 font-semibold text-gray-800">{row.name || row.team_id || row.id || '-'}</td>
                    <td className="py-3 px-3 text-center font-bold text-blue-600">{row.score || 0}</td>
                    <td className="py-3 px-3 text-center text-gray-700">{row.solved_count || 0}</td>
                    <td className="py-3 px-3 text-right text-gray-500">{formatTime(row.last_solve_time)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  )
}
