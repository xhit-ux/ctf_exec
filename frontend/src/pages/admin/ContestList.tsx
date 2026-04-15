import React, { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { fromUnknownError } from '../../utils/error'
import { Contest, addContestAdmin, createContest, fetchAdminContests } from '../../utils/contestApi'

type AdminDraft = {
  title: string
  description: string
  cover_image: string
  visibility: 'public' | 'invite_only' | 'private'
  start_time: string
  end_time: string
  scoring_type: string
}

const initialDraft: AdminDraft = {
  title: '',
  description: '',
  cover_image: '',
  visibility: 'public',
  start_time: '',
  end_time: '',
  scoring_type: 'jeopardy',
}

type DraftAdmin = {
  username: string
  role: 'admin' | 'observer'
}

export default function ContestList() {
  const navigate = useNavigate()
  const [rows, setRows] = useState<Contest[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [query, setQuery] = useState('')
  const [visibility, setVisibility] = useState<'' | 'public' | 'invite_only' | 'private'>('')

  const [draft, setDraft] = useState<AdminDraft>(initialDraft)
  const [draftAdmins, setDraftAdmins] = useState<DraftAdmin[]>([])
  const [pendingAdminUsername, setPendingAdminUsername] = useState('')
  const [pendingAdminRole, setPendingAdminRole] = useState<'admin' | 'observer'>('observer')
  const [creating, setCreating] = useState(false)
  const [createMsg, setCreateMsg] = useState('')

  const load = async () => {
    setLoading(true)
    setError('')
    try {
      const data = await fetchAdminContests({ page: 1, page_size: 100, keyword: query.trim() || undefined, visibility })
      setRows(data.items)
    } catch (err) {
      setError(fromUnknownError(err, '获取赛事管理列表失败'))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  const addPendingAdmin = () => {
    const username = pendingAdminUsername.trim()
    if (!username) return
    if (draftAdmins.some((item) => item.username === username)) return
    setDraftAdmins((prev) => [...prev, { username, role: pendingAdminRole }])
    setPendingAdminUsername('')
  }

  const removePendingAdmin = (username: string) => {
    setDraftAdmins((prev) => prev.filter((item) => item.username !== username))
  }

  const handleCreate = async () => {
    setCreateMsg('')
    if (!draft.title.trim()) {
      setCreateMsg('赛事标题不能为空')
      return
    }

    setCreating(true)
    try {
      const created = await createContest({
        title: draft.title.trim(),
        description: draft.description,
        cover_image: draft.cover_image,
        visibility: draft.visibility,
        start_time: draft.start_time ? new Date(draft.start_time).toISOString() : null,
        end_time: draft.end_time ? new Date(draft.end_time).toISOString() : null,
        scoring_type: draft.scoring_type,
      })

      for (const admin of draftAdmins) {
        await addContestAdmin(created.id, admin)
      }

      setCreateMsg('赛事创建成功，正在跳转赛事管理页...')
      setDraft(initialDraft)
      setDraftAdmins([])
      await load()
      navigate(`/contest/${created.id}/admin`)
    } catch (err) {
      setCreateMsg(fromUnknownError(err, '创建赛事失败'))
    } finally {
      setCreating(false)
    }
  }

  return (
    <div className="space-y-6">
      <section className="glass-card p-6 space-y-4">
        <h1 className="text-2xl font-extrabold text-gray-900">全站赛事管理</h1>

        <div className="grid grid-cols-1 gap-3 md:grid-cols-4">
          <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="关键词" className="glass-input" />
          <select value={visibility} onChange={(event) => setVisibility(event.target.value as '' | 'public' | 'invite_only' | 'private')} className="glass-input">
            <option value="">全部可见性</option>
            <option value="public">public</option>
            <option value="invite_only">invite_only</option>
            <option value="private">private</option>
          </select>
          <button type="button" onClick={load} className="btn-primary">筛选</button>
        </div>

        {error && <div className="rounded-lg border border-red-100 bg-red-50 px-3 py-2 text-sm text-red-600">{error}</div>}

        {loading ? (
          <div className="text-sm text-gray-500">加载中...</div>
        ) : (
          <div className="glass-table-wrap overflow-x-auto">
            <table className="w-full min-w-[760px] text-sm">
              <thead>
                <tr className="border-b border-gray-100 text-gray-500">
                  <th className="px-3 py-3 text-left">标题</th>
                  <th className="px-3 py-3 text-left">可见性</th>
                  <th className="px-3 py-3 text-left">计分模式</th>
                  <th className="px-3 py-3 text-right">操作</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((row) => (
                  <tr key={row.id} className="border-b border-gray-100 last:border-b-0">
                    <td className="px-3 py-3 font-semibold text-gray-800">{row.title}</td>
                    <td className="px-3 py-3 text-gray-600">{row.visibility}</td>
                    <td className="px-3 py-3 text-gray-600">{row.scoring_type || 'jeopardy'}</td>
                    <td className="px-3 py-3 text-right">
                      <div className="inline-flex gap-2">
                        <Link to={`/admin/contests/${row.id}/edit`} className="text-sm font-semibold text-amber-700">编辑</Link>
                        <Link to={`/contest/${row.id}/admin`} className="text-sm font-semibold text-emerald-700">赛事管理</Link>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className="glass-card p-6 space-y-4">
        <h2 className="text-xl font-bold text-gray-900">创建赛事</h2>

        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          <input value={draft.title} onChange={(event) => setDraft((prev) => ({ ...prev, title: event.target.value }))} placeholder="标题" className="glass-input" />
          <input value={draft.cover_image} onChange={(event) => setDraft((prev) => ({ ...prev, cover_image: event.target.value }))} placeholder="封面路径" className="glass-input" />
          <select value={draft.visibility} onChange={(event) => setDraft((prev) => ({ ...prev, visibility: event.target.value as 'public' | 'invite_only' | 'private' }))} className="glass-input">
            <option value="public">public</option>
            <option value="invite_only">invite_only</option>
            <option value="private">private</option>
          </select>
          <select value={draft.scoring_type} onChange={(event) => setDraft((prev) => ({ ...prev, scoring_type: event.target.value }))} className="glass-input">
            <option value="jeopardy">jeopardy</option>
            <option value="dynamic">dynamic</option>
          </select>
          <input type="datetime-local" value={draft.start_time} onChange={(event) => setDraft((prev) => ({ ...prev, start_time: event.target.value }))} className="glass-input" />
          <input type="datetime-local" value={draft.end_time} onChange={(event) => setDraft((prev) => ({ ...prev, end_time: event.target.value }))} className="glass-input" />
          <textarea value={draft.description} onChange={(event) => setDraft((prev) => ({ ...prev, description: event.target.value }))} placeholder="描述" className="md:col-span-2 min-h-[100px] glass-input" />
        </div>

        <div className="space-y-2">
          <h3 className="text-sm font-semibold text-gray-800">指定赛事管理员（可选）</h3>
          <div className="flex flex-wrap gap-2">
            <input value={pendingAdminUsername} onChange={(event) => setPendingAdminUsername(event.target.value)} placeholder="username" className="w-80 glass-input" />
            <select value={pendingAdminRole} onChange={(event) => setPendingAdminRole(event.target.value as 'admin' | 'observer')} className="glass-input w-auto">
              <option value="admin">admin</option>
              <option value="observer">observer</option>
            </select>
            <button type="button" onClick={addPendingAdmin} className="btn-secondary">添加到待提交列表</button>
          </div>

          {draftAdmins.length > 0 && (
            <div className="flex flex-wrap gap-2">
              {draftAdmins.map((item) => (
                <span key={item.username} className="inline-flex items-center gap-2 rounded-full bg-gray-100 px-3 py-1 text-xs text-gray-700">
                  {item.username} ({item.role})
                  <button type="button" onClick={() => removePendingAdmin(item.username)} className="text-red-600">x</button>
                </span>
              ))}
            </div>
          )}
        </div>

        <button type="button" disabled={creating} onClick={handleCreate} className="btn-primary disabled:opacity-50">{creating ? '创建中...' : '创建赛事'}</button>
        {createMsg && <div className="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-700">{createMsg}</div>}
      </section>
    </div>
  )
}
