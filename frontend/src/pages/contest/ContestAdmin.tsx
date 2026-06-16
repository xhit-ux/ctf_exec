import React, { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'
import { fromUnknownError } from '../../utils/error'
import { TAB_ACCENT_PALETTE, randomTabAccent, tabAccentStyle } from '../../utils/tabAccent'
import {
  ContestAdmin as ContestAdminRecord,
  ContestChallenge,
  ContestChallengeUpdatePayload,
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
  updateContestChallenge,
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
  const [tabAccent, setTabAccent] = useState(TAB_ACCENT_PALETTE[0])
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

  const handleTabSwitch = (nextTab: AdminTab) => {
    setTab(nextTab)
    setTabAccent((prev) => randomTabAccent(prev))
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
    const challengeID = newChallengeID.trim()
    if (!challengeID) {
      setError('请输入题目 ID（UUID）')
      return
    }
    setBusy(true)
    try {
      const contestScore =
        newScoreOverride.trim() === '' ? undefined : Number(newScoreOverride)
      await addContestChallenge(contest.id, {
        challenge_id: challengeID,
        contest_score: Number.isFinite(contestScore) ? contestScore : undefined,
        is_dynamic_score: false,
      })
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

  const handleRemoveChallenge = async (contestChallengeID: string) => {
    resetMsg()
    setBusy(true)
    try {
      await removeContestChallenge(contest.id, contestChallengeID)
      setChallenges(await fetchContestChallenges(contest.id))
      setSuccess('题目已移除')
    } catch (err) {
      setError(fromUnknownError(err, '移除题目失败'))
    } finally {
      setBusy(false)
    }
  }

  const [scoringEdit, setScoringEdit] = useState<{
    open: boolean
    target?: ContestChallenge
    form: ContestChallengeUpdatePayload
  }>({ open: false, form: {} })

  const openScoringEdit = (item: ContestChallenge) => {
    setScoringEdit({
      open: true,
      target: item,
      form: {
        contest_score: item.current_score,
        is_dynamic_score: item.is_dynamic_score,
        initial_score: item.initial_score ?? null,
        min_score: item.min_score ?? null,
        decay_constant: item.decay_constant ?? null,
        sort_order: item.sort_order ?? null,
      },
    })
  }

  const closeScoringEdit = () => setScoringEdit({ open: false, form: {} })

  const submitScoringEdit = async () => {
    if (!scoringEdit.target) return
    resetMsg()
    setBusy(true)
    try {
      await updateContestChallenge(contest.id, scoringEdit.target.id, scoringEdit.form)
      setChallenges(await fetchContestChallenges(contest.id))
      setSuccess('赛事题目计分已更新')
      closeScoringEdit()
    } catch (err) {
      setError(fromUnknownError(err, '更新赛事题目计分失败'))
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
    <section className="glass-card p-6 space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-xl font-bold text-gray-900">赛事管理</h2>
        <div className="flex items-center gap-2">
          <Link
            to={`/problem/publish?contest_id=${contest.id}&scope=contest`}
            className="btn-secondary text-xs"
          >
            进入赛事发布空间
          </Link>
          <span className="rounded-full glass-soft px-3 py-1 text-xs text-gray-600">当前权限: {adminRole || 'admin'}</span>
        </div>
      </div>

      <div className="flex flex-wrap gap-2">
        <button type="button" onClick={() => handleTabSwitch('info')} className={`tab-pill ${tab === 'info' ? 'tab-pill--active' : ''}`} style={tab === 'info' ? tabAccentStyle(tabAccent) : undefined}>赛事信息</button>
        <button type="button" onClick={() => handleTabSwitch('challenges')} className={`tab-pill ${tab === 'challenges' ? 'tab-pill--active' : ''}`} style={tab === 'challenges' ? tabAccentStyle(tabAccent) : undefined}>题目编排</button>
        <button type="button" onClick={() => handleTabSwitch('admins')} className={`tab-pill ${tab === 'admins' ? 'tab-pill--active' : ''}`} style={tab === 'admins' ? tabAccentStyle(tabAccent) : undefined}>管理员</button>
        <button type="button" onClick={() => handleTabSwitch('audit')} className={`tab-pill ${tab === 'audit' ? 'tab-pill--active' : ''}`} style={tab === 'audit' ? tabAccentStyle(tabAccent) : undefined}>提交审计</button>
      </div>

      {error && <div className="rounded-lg border border-red-100 bg-red-50 px-3 py-2 text-sm text-red-600">{error}</div>}
      {success && <div className="rounded-lg border border-green-100 bg-green-50 px-3 py-2 text-sm text-green-600">{success}</div>}

      {tab === 'info' && (
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          <input value={form.title} onChange={(event) => setForm((prev) => ({ ...prev, title: event.target.value }))} className="glass-input" placeholder="赛事标题" />
          <input value={form.cover_image} onChange={(event) => setForm((prev) => ({ ...prev, cover_image: event.target.value }))} className="glass-input" placeholder="封面地址" />
          <select value={form.visibility} onChange={(event) => setForm((prev) => ({ ...prev, visibility: event.target.value as 'public' | 'invite_only' | 'private' }))} className="glass-input">
            <option value="public">public</option>
            <option value="invite_only">invite_only</option>
            <option value="private">private</option>
          </select>
          <select value={form.scoring_type} onChange={(event) => setForm((prev) => ({ ...prev, scoring_type: event.target.value }))} className="glass-input">
            <option value="jeopardy">jeopardy</option>
            <option value="dynamic">dynamic</option>
          </select>
          <input type="datetime-local" value={form.start_time} onChange={(event) => setForm((prev) => ({ ...prev, start_time: event.target.value }))} className="glass-input" />
          <input type="datetime-local" value={form.end_time} onChange={(event) => setForm((prev) => ({ ...prev, end_time: event.target.value }))} className="glass-input" />
          <textarea value={form.description} onChange={(event) => setForm((prev) => ({ ...prev, description: event.target.value }))} className="md:col-span-2 glass-input min-h-[120px]" placeholder="赛事描述" />
          <div className="md:col-span-2">
            <button type="button" disabled={!canEditAdminModules || busy} onClick={saveContestInfo} className="btn-primary disabled:opacity-50">保存赛事信息</button>
          </div>
        </div>
      )}

      {tab === 'challenges' && (
        <div className="space-y-4">
          <div className="glass-soft p-4 space-y-3">
            <div className="text-sm font-semibold text-amber-800">发布新题目到当前赛事</div>
            <div className="grid grid-cols-1 gap-2 md:grid-cols-2">
              <input value={newProblem.title} onChange={(event) => setNewProblem((prev) => ({ ...prev, title: event.target.value }))} placeholder="题目标题" className="glass-input" />
              <input value={newProblem.category} onChange={(event) => setNewProblem((prev) => ({ ...prev, category: event.target.value }))} placeholder="分类（如 web/pwn/rev）" className="glass-input" />
              <input value={String(newProblem.points)} onChange={(event) => setNewProblem((prev) => ({ ...prev, points: Number(event.target.value) || 0 }))} placeholder="分值" className="glass-input" />
              <input value={newProblem.difficulty} onChange={(event) => setNewProblem((prev) => ({ ...prev, difficulty: event.target.value }))} placeholder="难度" className="glass-input" />
              <input value={newProblem.flag} onChange={(event) => setNewProblem((prev) => ({ ...prev, flag: event.target.value }))} placeholder="Flag（明文，仅提交时哈希）" className="glass-input md:col-span-2" />
              <input value={newProblem.tags} onChange={(event) => setNewProblem((prev) => ({ ...prev, tags: event.target.value }))} placeholder="标签（逗号分隔）" className="glass-input md:col-span-2" />
              <textarea value={newProblem.description} onChange={(event) => setNewProblem((prev) => ({ ...prev, description: event.target.value }))} placeholder="题目描述" className="glass-input min-h-[96px] md:col-span-2" />
            </div>
            <div>
              <button type="button" disabled={!canEditAdminModules || busy} onClick={handleCreateContestProblem} className="btn-primary disabled:opacity-50">发布到当前赛事</button>
            </div>
          </div>

          <div className="text-xs text-gray-500">兼容旧流程：如需将已有题目加入赛事，仍可使用下方按题目 ID 编排。</div>
          <div className="flex flex-wrap items-center gap-2">
            <input value={newChallengeID} onChange={(event) => setNewChallengeID(event.target.value)} placeholder="题目 ID" className="w-32 glass-input" />
            <input value={newScoreOverride} onChange={(event) => setNewScoreOverride(event.target.value)} placeholder="覆盖分值(可空)" className="w-40 glass-input" />
            <button type="button" disabled={!canEditAdminModules || busy} onClick={handleAddChallenge} className="btn-primary disabled:opacity-50">添加题目</button>
          </div>

          <div className="glass-table-wrap overflow-x-auto">
            <table className="w-full min-w-[760px] text-sm">
              <thead>
                <tr className="border-b border-gray-100 text-gray-500">
                  <th className="px-3 py-3 text-left">题目</th>
                  <th className="px-3 py-3 text-center">当前分</th>
                  <th className="px-3 py-3 text-center">动态衰减</th>
                  <th className="px-3 py-3 text-center">解出</th>
                  <th className="px-3 py-3 text-right">操作</th>
                </tr>
              </thead>
              <tbody>
                {challenges.map((item) => (
                  <tr key={item.id} className="border-b border-gray-100 last:border-b-0">
                    <td className="px-3 py-3 font-semibold text-gray-800">
                      <div>{item.title}</div>
                      <div className="text-[11px] font-mono text-gray-400">{item.problem_id}</div>
                    </td>
                    <td className="px-3 py-3 text-center font-bold text-amber-700">
                      {item.current_score}
                    </td>
                    <td className="px-3 py-3 text-center text-xs text-gray-600">
                      {item.is_dynamic_score
                        ? `${item.initial_score ?? '-'} → ${item.min_score ?? '-'} (N=${item.decay_constant ?? '-'})`
                        : '固定'}
                    </td>
                    <td className="px-3 py-3 text-center text-gray-600">
                      {item.solve_count ?? 0}
                    </td>
                    <td className="px-3 py-3 text-right space-x-3">
                      <button
                        type="button"
                        disabled={!canEditAdminModules || busy}
                        onClick={() => openScoringEdit(item)}
                        className="text-sm font-semibold text-amber-700 disabled:opacity-50"
                      >
                        计分配置
                      </button>
                      <button
                        type="button"
                        disabled={!canEditAdminModules || busy}
                        onClick={() => handleRemoveChallenge(item.id)}
                        className="text-sm font-semibold text-red-600 disabled:opacity-50"
                      >
                        移除
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {scoringEdit.open && scoringEdit.target && (
        <div className="fixed inset-0 z-50 bg-black/40 flex items-center justify-center p-4">
          <div className="w-full max-w-md rounded-2xl bg-white p-5 shadow-xl space-y-3">
            <div className="flex items-center justify-between">
              <h3 className="font-semibold text-gray-900">计分配置 — {scoringEdit.target.title}</h3>
              <button type="button" onClick={closeScoringEdit} className="text-gray-500 hover:text-gray-800">×</button>
            </div>
            <label className="flex items-center gap-2 text-sm text-gray-700">
              <input
                type="checkbox"
                className="h-4 w-4 accent-amber-500"
                checked={Boolean(scoringEdit.form.is_dynamic_score)}
                onChange={(e) =>
                  setScoringEdit((prev) => ({
                    ...prev,
                    form: { ...prev.form, is_dynamic_score: e.target.checked },
                  }))
                }
              />
              <span>启用 CTFd 多项式动态衰减</span>
            </label>
            <div className="grid grid-cols-2 gap-2">
              <label className="text-xs text-gray-600">
                contest_score / 固定分
                <input
                  type="number"
                  className="glass-input mt-1"
                  value={scoringEdit.form.contest_score ?? ''}
                  onChange={(e) =>
                    setScoringEdit((prev) => ({
                      ...prev,
                      form: { ...prev.form, contest_score: e.target.value === '' ? null : Number(e.target.value) },
                    }))
                  }
                />
              </label>
              <label className="text-xs text-gray-600">
                sort_order
                <input
                  type="number"
                  className="glass-input mt-1"
                  value={scoringEdit.form.sort_order ?? ''}
                  onChange={(e) =>
                    setScoringEdit((prev) => ({
                      ...prev,
                      form: { ...prev.form, sort_order: e.target.value === '' ? null : Number(e.target.value) },
                    }))
                  }
                />
              </label>
              {scoringEdit.form.is_dynamic_score && (
                <>
                  <label className="text-xs text-gray-600">
                    initial_score
                    <input
                      type="number"
                      className="glass-input mt-1"
                      value={scoringEdit.form.initial_score ?? ''}
                      onChange={(e) =>
                        setScoringEdit((prev) => ({
                          ...prev,
                          form: { ...prev.form, initial_score: e.target.value === '' ? null : Number(e.target.value) },
                        }))
                      }
                    />
                  </label>
                  <label className="text-xs text-gray-600">
                    min_score
                    <input
                      type="number"
                      className="glass-input mt-1"
                      value={scoringEdit.form.min_score ?? ''}
                      onChange={(e) =>
                        setScoringEdit((prev) => ({
                          ...prev,
                          form: { ...prev.form, min_score: e.target.value === '' ? null : Number(e.target.value) },
                        }))
                      }
                    />
                  </label>
                  <label className="col-span-2 text-xs text-gray-600">
                    decay_constant N
                    <input
                      type="number"
                      className="glass-input mt-1"
                      value={scoringEdit.form.decay_constant ?? ''}
                      onChange={(e) =>
                        setScoringEdit((prev) => ({
                          ...prev,
                          form: { ...prev.form, decay_constant: e.target.value === '' ? null : Number(e.target.value) },
                        }))
                      }
                    />
                  </label>
                </>
              )}
            </div>
            <div className="flex justify-end gap-2 pt-2">
              <button type="button" onClick={closeScoringEdit} className="btn-secondary text-sm">取消</button>
              <button type="button" onClick={submitScoringEdit} disabled={busy} className="btn-primary text-sm disabled:opacity-50">
                {busy ? '保存中...' : '保存'}
              </button>
            </div>
          </div>
        </div>
      )}

      {tab === 'admins' && (
        <div className="space-y-4">
          <div className="flex flex-wrap items-center gap-2">
            <input value={newAdminUsername} onChange={(event) => setNewAdminUsername(event.target.value)} placeholder="用户名" className="w-72 glass-input" />
            <select value={newAdminRole} onChange={(event) => setNewAdminRole(event.target.value as 'admin' | 'observer')} className="glass-input w-auto">
              <option value="observer">observer</option>
              <option value="admin">admin</option>
            </select>
            <button type="button" disabled={!canMutateAdmins || busy} onClick={handleAddAdmin} className="btn-primary disabled:opacity-50">添加管理员</button>
          </div>

          <div className="glass-table-wrap overflow-x-auto">
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
                    <td className="px-3 py-3 text-center"><span className="rounded-full bg-amber-100 px-2 py-1 text-xs text-amber-700">{item.role}</span></td>
                    <td className="px-3 py-3 text-right"><button type="button" disabled={!canMutateAdmins || busy || !(item.username || '').trim()} onClick={() => handleRemoveAdmin(item.username || '')} className="text-sm font-semibold text-red-600 disabled:opacity-50">移除</button></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {tab === 'audit' && (
        <div className="glass-table-wrap overflow-x-auto">
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
                  <td className="px-3 py-3 text-gray-700">{row.problem_title || (row.problem_id ? `#${row.problem_id}` : '已删除题目')}</td>
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
