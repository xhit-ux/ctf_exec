import React, { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'
import { fromUnknownError } from '../../utils/error'
import {
  ContestAdmin as ContestAdminRecord,
  ContestChallenge,
  ContestSubmissionItem,
  addContestAdmin,
  addContestChallenge,
  createAdminProblem,
  fetchContestAdmins,
  fetchContestChallenges,
  fetchContestSubmissions,
  removeContestAdmin,
  removeContestChallenge,
  updateContest,
} from '../../utils/contestApi'
import { useContestContext } from './ContestLayout'

type AdminTab = 'info' | 'challenges' | 'admins' | 'audit'

function toDatetimeLocal(input?: string | null): string {
  if (!input) return ''
  const d = new Date(input)
  if (Number.isNaN(d.getTime())) return ''
  const pad = (v: number) => String(v).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export default function ContestAdmin() {
  const { user } = useAuth()
  const { contest, isAdmin, adminRole, canEditAdminModules, refreshContest } = useContestContext()

  const [tab, setTab] = useState<AdminTab>('info')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [busy, setBusy] = useState(false)

  const [form, setForm] = useState({
    title: contest.title,
    description: contest.description || '',
    cover_image: contest.cover_image || '',
    visibility: contest.visibility,
    start_time: toDatetimeLocal(contest.start_time),
    end_time: toDatetimeLocal(contest.end_time),
    scoring_type: contest.scoring_type || 'jeopardy',
  })

  const [challenges, setChallenges] = useState<ContestChallenge[]>([])
  const [admins, setAdmins] = useState<ContestAdminRecord[]>([])
  const [auditRows, setAuditRows] = useState<ContestSubmissionItem[]>([])
  const [newChallengeID, setNewChallengeID] = useState('')
  const [newScoreOverride, setNewScoreOverride] = useState('')
  const [newAdminUsername, setNewAdminUsername] = useState('')
  const [newAdminRole, setNewAdminRole] = useState<'admin' | 'observer'>('observer')
  const [newProblem, setNewProblem] = useState({
    title: '',
    category: 'misc',
    difficulty: '简单',
    points: 100,
    description: '',
    flag: '',
    tags: '',
  })

  useEffect(() => {
    setForm({
      title: contest.title,
      description: contest.description || '',
      cover_image: contest.cover_image || '',
      visibility: contest.visibility,
      start_time: toDatetimeLocal(contest.start_time),
      end_time: toDatetimeLocal(contest.end_time),
      scoring_type: contest.scoring_type || 'jeopardy',
    })
  }, [contest])

  useEffect(() => {
    if (!isAdmin) return

    const load = async () => {
      try {
        const [challengeRows, adminRows, audit] = await Promise.all([
          fetchContestChallenges(contest.id),
          fetchContestAdmins(contest.id),
          fetchContestSubmissions(contest.id, { page: 1, page_size: 50 }),
        ])
        setChallenges(challengeRows)
        setAdmins(adminRows)
        setAuditRows(audit.items)
      } catch (err) {
        setError(fromUnknownError(err, '加载赛事管理数据失败'))
      }
    }

    load()
  }, [contest.id, isAdmin])

  if (!isAdmin) {
    return <div className="rounded-lg border border-red-100 bg-red-50 px-3 py-2 text-sm text-red-600">你没有该赛事的管理权限。</div>
  }

  const resetMsg = () => {
    setError('')
    setSuccess('')
  }

  const saveContestInfo = async () => {
    resetMsg()
    setBusy(true)
    try {
      await updateContest(contest.id, {
        title: form.title.trim(),
        description: form.description,
        cover_image: form.cover_image,
        visibility: form.visibility,
        start_time: form.start_time ? new Date(form.start_time).toISOString() : null,
        end_time: form.end_time ? new Date(form.end_time).toISOString() : null,
        scoring_type: form.scoring_type,
      })
      await refreshContest()
      setSuccess('赛事信息已更新')
    } catch (err) {
      setError(fromUnknownError(err, '更新赛事失败'))
    } finally {
      setBusy(false)
    }
  }

  const handleAddChallenge = async () => {
    resetMsg()
    const challengeID = Number(newChallengeID)
    if (!Number.isInteger(challengeID) || challengeID <= 0) {
      setError('请输入正确的题目 ID')
      return
    }
    setBusy(true)
    try {
      const scoreOverride = newScoreOverride.trim() === '' ? undefined : Number(newScoreOverride)
      await addContestChallenge(contest.id, challengeID, scoreOverride)
      setChallenges(await fetchContestChallenges(contest.id))
      setNewChallengeID('')
      setNewScoreOverride('')
      setSuccess('题目已加入赛事')
    } catch (err) {
      setError(fromUnknownError(err, '添加题目失败'))
    } finally {
      setBusy(false)
    }
  }

  const handleCreateContestProblem = async () => {
    resetMsg()

    if (!newProblem.title.trim() || !newProblem.category.trim() || !newProblem.difficulty.trim() || !newProblem.flag.trim()) {
      setError('请完整填写题目标题、分类、难度和 Flag')
      return
    }
    if (!Number.isFinite(Number(newProblem.points)) || Number(newProblem.points) < 0) {
      setError('分值必须是大于等于 0 的数字')
      return
    }

    setBusy(true)
    try {
      await createAdminProblem({
        title: newProblem.title.trim(),
        category: newProblem.category.trim(),
        points: Number(newProblem.points),
        difficulty: newProblem.difficulty.trim(),
        description: newProblem.description,
        flag: newProblem.flag.trim(),
        tags: newProblem.tags
          .split(',')
          .map((item) => item.trim())
          .filter(Boolean),
        scope: 'contest',
        contest_id: contest.id,
      })

      setNewProblem({
        title: '',
        category: 'misc',
        difficulty: '简单',
        points: 100,
        description: '',
        flag: '',
        tags: '',
      })
      setChallenges(await fetchContestChallenges(contest.id))
      setSuccess('题目已发布到当前赛事')
    } catch (err) {
      setError(fromUnknownError(err, '发布题目失败'))
    } finally {
      setBusy(false)
    }
  }

  const handleRemoveChallenge = async (challengeID: number) => {
    resetMsg()
    setBusy(true)
    try {
      await removeContestChallenge(contest.id, challengeID)
      setChallenges(await fetchContestChallenges(contest.id))
      setSuccess('题目已移除')
    } catch (err) {
      setError(fromUnknownError(err, '移除题目失败'))
    } finally {
      setBusy(false)
    }
  }

  const canMutateAdmins = user?.role === 'root' || user?.role === 'site_admin'

  const handleAddAdmin = async () => {
    resetMsg()
    if (!newAdminUsername.trim()) {
      setError('请输入用户名')
      return
    }
    setBusy(true)
    try {
      await addContestAdmin(contest.id, { username: newAdminUsername.trim(), role: newAdminRole })
      setAdmins(await fetchContestAdmins(contest.id))
      setNewAdminUsername('')
      setSuccess('已添加赛事管理员')
    } catch (err) {
      setError(fromUnknownError(err, '添加赛事管理员失败'))
    } finally {
      setBusy(false)
    }
  }

  const handleRemoveAdmin = async (username: string) => {
    if (!username.trim()) {
      setError('管理员用户名缺失，无法移除')
      return
    }

    resetMsg()
    setBusy(true)
    try {
      await removeContestAdmin(contest.id, username)
      setAdmins(await fetchContestAdmins(contest.id))
      setSuccess('已移除赛事管理员')
    } catch (err) {
      setError(fromUnknownError(err, '移除赛事管理员失败'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="rounded-2xl border border-gray-100 bg-white p-6 shadow-sm space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-xl font-bold text-gray-900">赛事管理</h2>
        <div className="flex items-center gap-2">
          <Link
            to={`/problem/publish?contest_id=${contest.id}&scope=contest`}
            className="rounded-lg border border-blue-200 bg-blue-50 px-3 py-1.5 text-xs font-semibold text-blue-700 hover:bg-blue-100"
          >
            进入赛事发布空间
          </Link>
          <span className="rounded-full bg-gray-100 px-3 py-1 text-xs text-gray-600">当前权限: {adminRole || 'admin'}</span>
        </div>
      </div>

      <div className="flex flex-wrap gap-2">
        <button type="button" onClick={() => setTab('info')} className={`px-3 py-1.5 rounded-md text-sm font-semibold ${tab === 'info' ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-700'}`}>赛事信息</button>
        <button type="button" onClick={() => setTab('challenges')} className={`px-3 py-1.5 rounded-md text-sm font-semibold ${tab === 'challenges' ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-700'}`}>题目编排</button>
        <button type="button" onClick={() => setTab('admins')} className={`px-3 py-1.5 rounded-md text-sm font-semibold ${tab === 'admins' ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-700'}`}>管理员</button>
        <button type="button" onClick={() => setTab('audit')} className={`px-3 py-1.5 rounded-md text-sm font-semibold ${tab === 'audit' ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-700'}`}>提交审计</button>
      </div>

      {error && <div className="rounded-lg border border-red-100 bg-red-50 px-3 py-2 text-sm text-red-600">{error}</div>}
      {success && <div className="rounded-lg border border-green-100 bg-green-50 px-3 py-2 text-sm text-green-600">{success}</div>}

      {tab === 'info' && (
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          <input value={form.title} onChange={(event) => setForm((prev) => ({ ...prev, title: event.target.value }))} className="rounded-lg border border-gray-200 px-3 py-2 text-sm" placeholder="赛事标题" />
          <input value={form.cover_image} onChange={(event) => setForm((prev) => ({ ...prev, cover_image: event.target.value }))} className="rounded-lg border border-gray-200 px-3 py-2 text-sm" placeholder="封面地址" />
          <select value={form.visibility} onChange={(event) => setForm((prev) => ({ ...prev, visibility: event.target.value as 'public' | 'invite_only' | 'private' }))} className="rounded-lg border border-gray-200 px-3 py-2 text-sm">
            <option value="public">public</option>
            <option value="invite_only">invite_only</option>
            <option value="private">private</option>
          </select>
          <select value={form.scoring_type} onChange={(event) => setForm((prev) => ({ ...prev, scoring_type: event.target.value }))} className="rounded-lg border border-gray-200 px-3 py-2 text-sm">
            <option value="jeopardy">jeopardy</option>
            <option value="dynamic">dynamic</option>
          </select>
          <input type="datetime-local" value={form.start_time} onChange={(event) => setForm((prev) => ({ ...prev, start_time: event.target.value }))} className="rounded-lg border border-gray-200 px-3 py-2 text-sm" />
          <input type="datetime-local" value={form.end_time} onChange={(event) => setForm((prev) => ({ ...prev, end_time: event.target.value }))} className="rounded-lg border border-gray-200 px-3 py-2 text-sm" />
          <textarea value={form.description} onChange={(event) => setForm((prev) => ({ ...prev, description: event.target.value }))} className="md:col-span-2 rounded-lg border border-gray-200 px-3 py-2 text-sm min-h-[120px]" placeholder="赛事描述" />
          <div className="md:col-span-2">
            <button type="button" disabled={!canEditAdminModules || busy} onClick={saveContestInfo} className="px-4 py-2 rounded-lg bg-blue-600 text-white text-sm font-semibold disabled:opacity-50">保存赛事信息</button>
          </div>
        </div>
      )}

      {tab === 'challenges' && (
        <div className="space-y-4">
          <div className="rounded-lg border border-blue-100 bg-blue-50/40 p-4 space-y-3">
            <div className="text-sm font-semibold text-blue-800">发布新题目到当前赛事</div>
            <div className="grid grid-cols-1 gap-2 md:grid-cols-2">
              <input value={newProblem.title} onChange={(event) => setNewProblem((prev) => ({ ...prev, title: event.target.value }))} placeholder="题目标题" className="rounded-lg border border-gray-200 px-3 py-2 text-sm" />
              <input value={newProblem.category} onChange={(event) => setNewProblem((prev) => ({ ...prev, category: event.target.value }))} placeholder="分类（如 web/pwn/rev）" className="rounded-lg border border-gray-200 px-3 py-2 text-sm" />
              <input value={String(newProblem.points)} onChange={(event) => setNewProblem((prev) => ({ ...prev, points: Number(event.target.value) || 0 }))} placeholder="分值" className="rounded-lg border border-gray-200 px-3 py-2 text-sm" />
              <input value={newProblem.difficulty} onChange={(event) => setNewProblem((prev) => ({ ...prev, difficulty: event.target.value }))} placeholder="难度" className="rounded-lg border border-gray-200 px-3 py-2 text-sm" />
              <input value={newProblem.flag} onChange={(event) => setNewProblem((prev) => ({ ...prev, flag: event.target.value }))} placeholder="Flag（明文，仅提交时哈希）" className="rounded-lg border border-gray-200 px-3 py-2 text-sm md:col-span-2" />
              <input value={newProblem.tags} onChange={(event) => setNewProblem((prev) => ({ ...prev, tags: event.target.value }))} placeholder="标签（逗号分隔）" className="rounded-lg border border-gray-200 px-3 py-2 text-sm md:col-span-2" />
              <textarea value={newProblem.description} onChange={(event) => setNewProblem((prev) => ({ ...prev, description: event.target.value }))} placeholder="题目描述" className="rounded-lg border border-gray-200 px-3 py-2 text-sm min-h-[96px] md:col-span-2" />
            </div>
            <div>
              <button type="button" disabled={!canEditAdminModules || busy} onClick={handleCreateContestProblem} className="px-4 py-2 rounded-lg bg-emerald-600 text-white text-sm font-semibold disabled:opacity-50">发布到当前赛事</button>
            </div>
          </div>

          <div className="text-xs text-gray-500">兼容旧流程：如需将已有题目加入赛事，仍可使用下方按题目 ID 编排。</div>
          <div className="flex flex-wrap items-center gap-2">
            <input value={newChallengeID} onChange={(event) => setNewChallengeID(event.target.value)} placeholder="题目 ID" className="w-32 rounded-lg border border-gray-200 px-3 py-2 text-sm" />
            <input value={newScoreOverride} onChange={(event) => setNewScoreOverride(event.target.value)} placeholder="覆盖分值(可空)" className="w-40 rounded-lg border border-gray-200 px-3 py-2 text-sm" />
            <button type="button" disabled={!canEditAdminModules || busy} onClick={handleAddChallenge} className="px-4 py-2 rounded-lg bg-blue-600 text-white text-sm font-semibold disabled:opacity-50">添加题目</button>
          </div>

          <div className="overflow-x-auto">
            <table className="w-full min-w-[680px] text-sm">
              <thead>
                <tr className="border-b border-gray-100 text-gray-500">
                  <th className="px-3 py-3 text-left">ID</th>
                  <th className="px-3 py-3 text-left">标题</th>
                  <th className="px-3 py-3 text-center">分值</th>
                  <th className="px-3 py-3 text-right">操作</th>
                </tr>
              </thead>
              <tbody>
                {challenges.map((item) => (
                  <tr key={item.id} className="border-b border-gray-100 last:border-b-0">
                    <td className="px-3 py-3 text-gray-600">#{item.id}</td>
                    <td className="px-3 py-3 font-semibold text-gray-800">{item.title}</td>
                    <td className="px-3 py-3 text-center font-bold text-blue-600">{item.score_override ?? item.points ?? 0}</td>
                    <td className="px-3 py-3 text-right"><button type="button" disabled={!canEditAdminModules || busy} onClick={() => handleRemoveChallenge(item.id)} className="text-sm font-semibold text-red-600 disabled:opacity-50">移除</button></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {tab === 'admins' && (
        <div className="space-y-4">
          <div className="flex flex-wrap items-center gap-2">
            <input value={newAdminUsername} onChange={(event) => setNewAdminUsername(event.target.value)} placeholder="用户名" className="w-72 rounded-lg border border-gray-200 px-3 py-2 text-sm" />
            <select value={newAdminRole} onChange={(event) => setNewAdminRole(event.target.value as 'admin' | 'observer')} className="rounded-lg border border-gray-200 px-3 py-2 text-sm">
              <option value="observer">observer</option>
              <option value="admin">admin</option>
            </select>
            <button type="button" disabled={!canMutateAdmins || busy} onClick={handleAddAdmin} className="px-4 py-2 rounded-lg bg-blue-600 text-white text-sm font-semibold disabled:opacity-50">添加管理员</button>
          </div>

          <div className="overflow-x-auto">
            <table className="w-full min-w-[700px] text-sm">
              <thead>
                <tr className="border-b border-gray-100 text-gray-500">
                  <th className="px-3 py-3 text-left">用户名</th>
                  <th className="px-3 py-3 text-center">角色</th>
                  <th className="px-3 py-3 text-right">操作</th>
                </tr>
              </thead>
              <tbody>
                {admins.map((item) => (
                  <tr key={item.id} className="border-b border-gray-100 last:border-b-0">
                    <td className="px-3 py-3 text-gray-700">{item.username || item.user_id}</td>
                    <td className="px-3 py-3 text-center"><span className="rounded-full bg-gray-100 px-2 py-1 text-xs text-gray-700">{item.role}</span></td>
                    <td className="px-3 py-3 text-right"><button type="button" disabled={!canMutateAdmins || busy || !(item.username || '').trim()} onClick={() => handleRemoveAdmin(item.username || '')} className="text-sm font-semibold text-red-600 disabled:opacity-50">移除</button></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {tab === 'audit' && (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[760px] text-sm">
            <thead>
              <tr className="border-b border-gray-100 text-gray-500">
                <th className="px-3 py-3 text-left">时间</th>
                <th className="px-3 py-3 text-left">用户</th>
                <th className="px-3 py-3 text-left">题目</th>
                <th className="px-3 py-3 text-center">结果</th>
                <th className="px-3 py-3 text-right">IP</th>
              </tr>
            </thead>
            <tbody>
              {auditRows.map((row) => (
                <tr key={row.id} className="border-b border-gray-100 last:border-b-0">
                  <td className="px-3 py-3 text-gray-600">{new Date(row.created_at).toLocaleString('zh-CN', { hour12: false })}</td>
                  <td className="px-3 py-3 font-semibold text-gray-800">{row.display_name || row.username || row.user_id || '-'}</td>
                  <td className="px-3 py-3 text-gray-700">{row.problem_title || `#${row.problem_id}`}</td>
                  <td className="px-3 py-3 text-center">{row.is_correct ? <span className="rounded-full bg-green-100 px-2 py-1 text-xs text-green-700">正确</span> : <span className="rounded-full bg-red-100 px-2 py-1 text-xs text-red-700">错误</span>}</td>
                  <td className="px-3 py-3 text-right text-gray-500">{row.ip_address || '-'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  )
}
