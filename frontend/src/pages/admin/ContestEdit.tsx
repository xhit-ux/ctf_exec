import React, { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { fromUnknownError } from '../../utils/error'
import { Contest, ContestAdmin, addContestAdmin, fetchContestAdmins, fetchContestDetail, removeContestAdmin, updateContest } from '../../utils/contestApi'

function toDatetimeLocal(input?: string | null): string {
  if (!input) return ''
  const d = new Date(input)
  if (Number.isNaN(d.getTime())) return ''
  const pad = (v: number) => String(v).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export default function ContestEdit() {
  const { contestId } = useParams()

  const [contest, setContest] = useState<Contest | null>(null)
  const [admins, setAdmins] = useState<ContestAdmin[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [msg, setMsg] = useState('')
  const [busy, setBusy] = useState(false)

  const [newAdminUsername, setNewAdminUsername] = useState('')
  const [newAdminRole, setNewAdminRole] = useState<'admin' | 'observer'>('observer')

  const [form, setForm] = useState({
    title: '',
    description: '',
    cover_image: '',
    visibility: 'public' as 'public' | 'invite_only' | 'private',
    scoring_type: 'jeopardy',
    start_time: '',
    end_time: '',
  })

  const load = async () => {
    if (!contestId) {
      setError('缺少赛事ID')
      return
    }

    setLoading(true)
    setError('')
    try {
      const [detail, adminsData] = await Promise.all([
        fetchContestDetail(contestId),
        fetchContestAdmins(contestId),
      ])
      setContest(detail)
      setAdmins(adminsData)
      setForm({
        title: detail.title,
        description: detail.description || '',
        cover_image: detail.cover_image || '',
        visibility: detail.visibility,
        scoring_type: detail.scoring_type || 'jeopardy',
        start_time: toDatetimeLocal(detail.start_time),
        end_time: toDatetimeLocal(detail.end_time),
      })
    } catch (err) {
      setError(fromUnknownError(err, '获取赛事编辑数据失败'))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [contestId])

  const save = async () => {
    if (!contestId) return
    setBusy(true)
    setMsg('')
    setError('')
    try {
      await updateContest(contestId, {
        title: form.title,
        description: form.description,
        cover_image: form.cover_image,
        visibility: form.visibility,
        scoring_type: form.scoring_type,
        start_time: form.start_time ? new Date(form.start_time).toISOString() : null,
        end_time: form.end_time ? new Date(form.end_time).toISOString() : null,
      })
      setMsg('赛事信息已保存')
      await load()
    } catch (err) {
      setError(fromUnknownError(err, '保存失败'))
    } finally {
      setBusy(false)
    }
  }

  const addAdmin = async () => {
    if (!contestId) return
    if (!newAdminUsername.trim()) return
    setBusy(true)
    setError('')
    setMsg('')
    try {
      await addContestAdmin(contestId, { username: newAdminUsername.trim(), role: newAdminRole })
      setAdmins(await fetchContestAdmins(contestId))
      setNewAdminUsername('')
      setMsg('已添加管理员')
    } catch (err) {
      setError(fromUnknownError(err, '添加管理员失败'))
    } finally {
      setBusy(false)
    }
  }

  const deleteAdmin = async (username: string) => {
    if (!contestId) return
    if (!username.trim()) {
      setError('管理员用户名缺失，无法移除')
      return
    }

    setBusy(true)
    setError('')
    setMsg('')
    try {
      await removeContestAdmin(contestId, username)
      setAdmins(await fetchContestAdmins(contestId))
      setMsg('已移除管理员')
    } catch (err) {
      setError(fromUnknownError(err, '移除管理员失败'))
    } finally {
      setBusy(false)
    }
  }

  if (loading) {
    return <div className="p-8 text-center text-gray-500">加载中...</div>
  }

  if (error && !contest) {
    return <div className="rounded-lg border border-red-100 bg-red-50 px-3 py-2 text-sm text-red-600">{error}</div>
  }

  return (
    <div className="space-y-5">
      <div className="flex items-center justify-between gap-3">
        <h1 className="text-2xl font-extrabold text-gray-900">编辑赛事</h1>
        {contest && <Link className="text-sm font-semibold text-amber-700" to={`/contest/${contest.id}/admin`}>进入赛事管理页</Link>}
      </div>

      {error && <div className="rounded-lg border border-red-100 bg-red-50 px-3 py-2 text-sm text-red-600">{error}</div>}
      {msg && <div className="rounded-lg border border-green-100 bg-green-50 px-3 py-2 text-sm text-green-600">{msg}</div>}

      <section className="glass-card p-6 space-y-3">
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          <input value={form.title} onChange={(event) => setForm((prev) => ({ ...prev, title: event.target.value }))} className="glass-input" placeholder="标题" />
          <input value={form.cover_image} onChange={(event) => setForm((prev) => ({ ...prev, cover_image: event.target.value }))} className="glass-input" placeholder="封面" />
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
          <textarea value={form.description} onChange={(event) => setForm((prev) => ({ ...prev, description: event.target.value }))} className="md:col-span-2 min-h-[120px] glass-input" placeholder="描述" />
        </div>

        <button type="button" disabled={busy} onClick={save} className="btn-primary disabled:opacity-50">保存</button>
      </section>

      <section className="glass-card p-6 space-y-3">
        <h2 className="text-lg font-bold text-gray-900">管理员分配</h2>

        <div className="flex flex-wrap items-center gap-2">
          <input value={newAdminUsername} onChange={(event) => setNewAdminUsername(event.target.value)} placeholder="username" className="w-72 glass-input" />
          <select value={newAdminRole} onChange={(event) => setNewAdminRole(event.target.value as 'admin' | 'observer')} className="glass-input w-auto">
            <option value="admin">admin</option>
            <option value="observer">observer</option>
          </select>
          <button type="button" disabled={busy} onClick={addAdmin} className="btn-primary disabled:opacity-50">添加</button>
        </div>

        <div className="glass-table-wrap overflow-x-auto">
          <table className="w-full min-w-[640px] text-sm">
            <thead>
              <tr className="border-b border-gray-100 text-gray-500">
                  <th className="px-3 py-3 text-left">用户名</th>
                <th className="px-3 py-3 text-center">角色</th>
                <th className="px-3 py-3 text-right">操作</th>
              </tr>
            </thead>
            <tbody>
              {admins.map((admin) => (
                <tr key={admin.id} className="border-b border-gray-100 last:border-b-0">
                  <td className="px-3 py-3 text-gray-700">{admin.username || admin.user_id}</td>
                  <td className="px-3 py-3 text-center"><span className="rounded-full bg-amber-100 px-2 py-1 text-xs text-amber-700">{admin.role}</span></td>
                  <td className="px-3 py-3 text-right"><button type="button" disabled={busy || !(admin.username || '').trim()} onClick={() => deleteAdmin(admin.username || '')} className="text-sm font-semibold text-red-600 disabled:opacity-50">移除</button></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  )
}
