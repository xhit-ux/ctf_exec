import React, { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { apiUrl } from '../utils/api'
import { apiFetch } from '../utils/authClient'
import { fromUnknownError, readSafeErrorMessage } from '../utils/error'
import { TAB_ACCENT_PALETTE, randomTabAccent, tabAccentStyle } from '../utils/tabAccent'

type AdminTab =
  | 'overview'
  | 'problem-manage'
  | 'contest'
  | 'user-status'
  | 'root-users'
  | 'root-audit'
  | 'root-site-config'

type ContestListItem = {
  id: string
  title: string
  start_time?: string | null
  end_time?: string | null
}

type ProblemListItem = {
  id: number
  title: string
  category: string
  points: number
  difficulty: string
}

type AdminProblemItem = {
  id: string
  title: string
  author?: string | null
  category: string
  type: string
  difficulty: string
  platform_score: number
  status: string
  created_by: string
  tags: string[]
  description?: string | null
}

type AdminUserListItem = {
  id: string
  username: string
  display_name: string
  email: string
  role: string
  is_active: boolean
  is_verified: boolean
  created_at: string
  updated_at: string
}

type AdminRoleChangeLogItem = {
  id: string
  operator_user_id: string
  operator_username: string
  target_user_id: string
  target_username: string
  old_role: string
  new_role: string
  reason: string
  ip: string
  created_at: string
}

type SiteConfig = {
  site_name: string
  site_notice: string
  registration_enabled: boolean
  email_verification_required: boolean
  default_contest_visibility: 'public' | 'invite_only' | 'private'
}

const contestScoringTypes = ['dynamic', 'jeopardy']

function canAccessRootOnly(role?: string): boolean {
  return role === 'root'
}

function formatDateTime(input?: string | null): string {
  if (!input) return '-'
  const date = new Date(input)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('zh-CN', { hour12: false })
}

function toISODateTime(input: string): string | null {
  const normalized = input.trim()
  if (!normalized) return null
  const date = new Date(normalized)
  if (Number.isNaN(date.getTime())) return null
  return date.toISOString()
}

function roleBadgeClass(role: string): string {
  if (role === 'root') return 'bg-red-100 text-red-700'
  if (role === 'site_admin') return 'bg-purple-100 text-purple-700'
  return 'bg-blue-100 text-blue-700'
}

const CARD_CLASS = 'glass-card p-6 space-y-4'
const INPUT_CLASS = 'glass-input'
const TABLE_WRAP_CLASS = 'glass-table-wrap overflow-x-auto'

export default function Admin() {
  const { user } = useAuth()
  const isRoot = canAccessRootOnly(user?.role)

  const [activeTab, setActiveTab] = useState<AdminTab>('overview')
  const [tabAccent, setTabAccent] = useState(TAB_ACCENT_PALETTE[0])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [successHint, setSuccessHint] = useState('')

  const [contests, setContests] = useState<ContestListItem[]>([])
  const [problems, setProblems] = useState<ProblemListItem[]>([])

  const [adminProblems, setAdminProblems] = useState<AdminProblemItem[]>([])
  const [adminProblemKeyword, setAdminProblemKeyword] = useState('')
  const [adminProblemStatus, setAdminProblemStatus] = useState('')
  const [adminProblemsLoading, setAdminProblemsLoading] = useState(false)
  const [deletingProblemID, setDeletingProblemID] = useState('')
  const [togglingProblemID, setTogglingProblemID] = useState('')

  const [contestForm, setContestForm] = useState({
    title: '',
    description: '',
    cover_image: '',
    start_time: '',
    end_time: '',
    scoring_type: contestScoringTypes[0],
  })
  const [creatingContest, setCreatingContest] = useState(false)

  const [statusUsers, setStatusUsers] = useState<AdminUserListItem[]>([])
  const [statusKeyword, setStatusKeyword] = useState('')
  const [statusPage, setStatusPage] = useState(1)
  const [statusTotalPages, setStatusTotalPages] = useState(1)
  const [statusLoading, setStatusLoading] = useState(false)
  const [statusUpdatingUserID, setStatusUpdatingUserID] = useState('')
  const [statusReasonDraft, setStatusReasonDraft] = useState<Record<string, string>>({})

  const [roleUsers, setRoleUsers] = useState<AdminUserListItem[]>([])
  const [roleKeyword, setRoleKeyword] = useState('')
  const [rolePage, setRolePage] = useState(1)
  const [roleTotalPages, setRoleTotalPages] = useState(1)
  const [roleLoading, setRoleLoading] = useState(false)
  const [updatingRoleUserID, setUpdatingRoleUserID] = useState('')
  const [roleDraft, setRoleDraft] = useState<Record<string, string>>({})
  const [roleReasonDraft, setRoleReasonDraft] = useState<Record<string, string>>({})

  const [roleLogs, setRoleLogs] = useState<AdminRoleChangeLogItem[]>([])
  const [logsPage, setLogsPage] = useState(1)
  const [logsTotalPages, setLogsTotalPages] = useState(1)
  const [logsLoading, setLogsLoading] = useState(false)

  const [siteConfig, setSiteConfig] = useState<SiteConfig>({
    site_name: 'CTF Platform',
    site_notice: '',
    registration_enabled: true,
    email_verification_required: true,
    default_contest_visibility: 'public',
  })
  const [configLoading, setConfigLoading] = useState(false)
  const [configSaving, setConfigSaving] = useState(false)

  useEffect(() => {
    let canceled = false

    const loadSummaryData = async () => {
      let keepLoading = false

      setLoading(true)
      setError('')
      try {
        const [contestRes, problemRes] = await Promise.all([
          apiFetch(apiUrl('/api/contests?limit=8'), undefined, {
            disableAuth: true,
            disableRetry401: true,
          }),
          apiFetch(apiUrl('/api/problems')),
        ])

        if (contestRes.status === 401 || problemRes.status === 401) {
          keepLoading = true
          return
        }
        if (!contestRes.ok) {
          throw new Error('获取赛事数据失败')
        }
        if (!problemRes.ok) {
          throw new Error('获取题目数据失败')
        }

        const contestData = await contestRes.json()
        const problemData = await problemRes.json()

        if (!canceled) {
          setContests(Array.isArray(contestData) ? contestData : [])
          setProblems(Array.isArray(problemData) ? problemData : [])
        }
      } catch (err) {
        if (!canceled) {
          setError(fromUnknownError(err, '管理页加载失败，请稍后重试'))
        }
      } finally {
        if (!canceled && !keepLoading) {
          setLoading(false)
        }
      }
    }

    loadSummaryData()
    return () => {
      canceled = true
    }
  }, [])

  useEffect(() => {
    let canceled = false

    const loadAdminProblems = async () => {
      if (activeTab !== 'problem-manage') return

      let keepLoading = false
      setAdminProblemsLoading(true)
      setError('')
      try {
        const params = new URLSearchParams()
        if (adminProblemKeyword.trim()) params.set('keyword', adminProblemKeyword.trim())
        if (adminProblemStatus) params.set('status', adminProblemStatus)

        const res = await apiFetch(apiUrl(`/api/admin/problems?${params.toString()}`))

        if (res.status === 401) {
          keepLoading = true
          return
        }
        if (!res.ok) {
          setError(await readSafeErrorMessage(res, '加载题目列表失败'))
          return
        }

        const data = await res.json()
        if (!canceled) {
          setAdminProblems(Array.isArray(data?.items) ? data.items : [])
        }
      } catch (err) {
        if (!canceled) setError(fromUnknownError(err, '加载题目列表失败'))
      } finally {
        if (!canceled && !keepLoading) setAdminProblemsLoading(false)
      }
    }

    loadAdminProblems()
    return () => {
      canceled = true
    }
  }, [activeTab, adminProblemKeyword, adminProblemStatus])

  useEffect(() => {
    let canceled = false

    const loadStatusUsers = async () => {
      if (activeTab !== 'user-status') return

      let keepLoading = false
      setStatusLoading(true)
      setError('')
      try {
        const params = new URLSearchParams({ page: String(statusPage), page_size: '10' })
        if (statusKeyword.trim()) params.set('keyword', statusKeyword.trim())

        const res = await apiFetch(apiUrl(`/api/admin/users?${params.toString()}`))

        if (res.status === 401) {
          keepLoading = true
          return
        }
        if (!res.ok) {
          setError(await readSafeErrorMessage(res, '加载用户列表失败'))
          return
        }

        const data = await res.json()
        if (!canceled) {
          setStatusUsers(Array.isArray(data?.items) ? data.items : [])
          setStatusTotalPages(Math.max(1, Number(data?.total_pages) || 1))
        }
      } catch (err) {
        if (!canceled) setError(fromUnknownError(err, '加载用户列表失败'))
      } finally {
        if (!canceled && !keepLoading) setStatusLoading(false)
      }
    }

    loadStatusUsers()
    return () => {
      canceled = true
    }
  }, [activeTab, statusKeyword, statusPage])

  useEffect(() => {
    let canceled = false

    const loadRoleUsers = async () => {
      if (!isRoot || activeTab !== 'root-users') return

      let keepLoading = false
      setRoleLoading(true)
      setError('')
      try {
        const params = new URLSearchParams({ page: String(rolePage), page_size: '10' })
        if (roleKeyword.trim()) params.set('keyword', roleKeyword.trim())

        const res = await apiFetch(apiUrl(`/api/admin/users?${params.toString()}`))

        if (res.status === 401) {
          keepLoading = true
          return
        }
        if (!res.ok) {
          setError(await readSafeErrorMessage(res, '加载用户列表失败'))
          return
        }

        const data = await res.json()
        if (!canceled) {
          const items = Array.isArray(data?.items) ? data.items : []
          setRoleUsers(items)
          setRoleTotalPages(Math.max(1, Number(data?.total_pages) || 1))
          setRoleDraft((prev) => {
            const next = { ...prev }
            items.forEach((item: AdminUserListItem) => {
              if (!next[item.id]) next[item.id] = item.role
            })
            return next
          })
        }
      } catch (err) {
        if (!canceled) setError(fromUnknownError(err, '加载 root 用户列表失败'))
      } finally {
        if (!canceled && !keepLoading) setRoleLoading(false)
      }
    }

    loadRoleUsers()
    return () => {
      canceled = true
    }
  }, [activeTab, roleKeyword, rolePage, isRoot])

  useEffect(() => {
    let canceled = false

    const loadRoleLogs = async () => {
      if (!isRoot || activeTab !== 'root-audit') return

      let keepLoading = false
      setLogsLoading(true)
      setError('')
      try {
        const params = new URLSearchParams({ page: String(logsPage), page_size: '10' })
        const res = await apiFetch(apiUrl(`/api/admin/users/role-change-logs?${params.toString()}`))

        if (res.status === 401) {
          keepLoading = true
          return
        }
        if (!res.ok) {
          setError(await readSafeErrorMessage(res, '加载审计日志失败'))
          return
        }

        const data = await res.json()
        if (!canceled) {
          setRoleLogs(Array.isArray(data?.items) ? data.items : [])
          setLogsTotalPages(Math.max(1, Number(data?.total_pages) || 1))
        }
      } catch (err) {
        if (!canceled) setError(fromUnknownError(err, '加载审计日志失败'))
      } finally {
        if (!canceled && !keepLoading) setLogsLoading(false)
      }
    }

    loadRoleLogs()
    return () => {
      canceled = true
    }
  }, [activeTab, logsPage, isRoot])

  useEffect(() => {
    let canceled = false

    const loadSiteConfig = async () => {
      if (!isRoot || activeTab !== 'root-site-config') return

      let keepLoading = false
      setConfigLoading(true)
      setError('')
      try {
        const res = await apiFetch(apiUrl('/api/admin/site-config'))

        if (res.status === 401) {
          keepLoading = true
          return
        }
        if (!res.ok) {
          setError(await readSafeErrorMessage(res, '加载站点配置失败'))
          return
        }

        const data = await res.json()
        if (!canceled) {
          setSiteConfig({
            site_name: data?.site_name || 'CTF Platform',
            site_notice: data?.site_notice || '',
            registration_enabled: !!data?.registration_enabled,
            email_verification_required: !!data?.email_verification_required,
            default_contest_visibility: (data?.default_contest_visibility || 'public') as SiteConfig['default_contest_visibility'],
          })
        }
      } catch (err) {
        if (!canceled) setError(fromUnknownError(err, '加载站点配置失败'))
      } finally {
        if (!canceled && !keepLoading) setConfigLoading(false)
      }
    }

    loadSiteConfig()
    return () => {
      canceled = true
    }
  }, [activeTab, isRoot])

  const statCards = useMemo(() => {
    return [
      { title: '公开题目总数', value: String(problems.length), hint: '题目发布请走独立发布页' },
      { title: '近期赛事条目', value: String(contests.length), hint: '含未开始 / 进行中 / 已结束' },
      { title: '当前角色', value: user?.role || '-', hint: isRoot ? 'root 拥有最高权限' : 'site_admin 负责站点管理' },
    ]
  }, [problems.length, contests.length, user?.role, isRoot])

  const deleteProblem = async (problemID: string, title: string) => {
    if (!window.confirm(`确定要删除题目「${title}」吗？此操作不可撤销。`)) return

    setDeletingProblemID(problemID)
    setError('')
    setSuccessHint('')
    try {
      const res = await apiFetch(apiUrl(`/api/admin/problems/${encodeURIComponent(problemID)}`), {
        method: 'DELETE',
      })

      if (!res.ok) {
        setError(await readSafeErrorMessage(res, '删除题目失败'))
        return
      }

      setAdminProblems((prev) => prev.filter((p) => p.id !== problemID))
      setSuccessHint(`题目「${title}」已删除`)
    } catch (err) {
      setError(fromUnknownError(err, '网络异常，删除题目失败'))
    } finally {
      setDeletingProblemID('')
    }
  }

  const toggleProblemVisibility = async (problemID: string, currentStatus: string, title: string) => {
    const isHidden = currentStatus === 'hidden'
    const newStatus = isHidden ? 'ready' : 'hidden'
    const action = isHidden ? '显示' : '隐藏'

    setTogglingProblemID(problemID)
    setError('')
    setSuccessHint('')
    try {
      const res = await apiFetch(apiUrl(`/api/admin/problems/${encodeURIComponent(problemID)}/visibility`), {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ status: newStatus }),
      })
      if (!res.ok) {
        setError(await readSafeErrorMessage(res, `${action}题目失败`))
        return
      }
      setAdminProblems((prev) =>
        prev.map((p) => (p.id === problemID ? { ...p, status: newStatus } : p))
      )
      setSuccessHint(`题目「${title}」已${action}`)
    } catch (err) {
      setError(fromUnknownError(err, `网络异常，${action}题目失败`))
    } finally {
      setTogglingProblemID('')
    }
  }

  const submitContest = async (e: React.FormEvent) => {
    e.preventDefault()

    const startISO = toISODateTime(contestForm.start_time)
    const endISO = toISODateTime(contestForm.end_time)
    if (contestForm.start_time && !startISO) {
      setError('开始时间格式不正确')
      return
    }
    if (contestForm.end_time && !endISO) {
      setError('结束时间格式不正确')
      return
    }

    setCreatingContest(true)
    setError('')
    setSuccessHint('')
    try {
      const res = await apiFetch(apiUrl('/api/admin/contests'), {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          ...contestForm,
          start_time: startISO,
          end_time: endISO,
        }),
      })

      if (!res.ok) {
        setError(await readSafeErrorMessage(res, '创建赛事失败'))
        return
      }

      const created = await res.json()
      setContests((prev) => [created, ...prev])
      setContestForm({
        title: '',
        description: '',
        cover_image: '',
        start_time: '',
        end_time: '',
        scoring_type: contestScoringTypes[0],
      })
      setSuccessHint(`赛事创建成功：${created?.title || ''}`)
      setActiveTab('overview')
    } catch (err) {
      setError(fromUnknownError(err, '网络异常，创建赛事失败'))
    } finally {
      setCreatingContest(false)
    }
  }

  const updateUserActiveStatus = async (target: AdminUserListItem, nextActive: boolean) => {
    const reason = (statusReasonDraft[target.id] || '').trim()

    setStatusUpdatingUserID(target.id)
    setError('')
    setSuccessHint('')
    try {
      const res = await apiFetch(apiUrl(`/api/admin/users/${target.id}/active`), {
        method: 'PATCH',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ is_active: nextActive, reason }),
      })

      if (!res.ok) {
        setError(await readSafeErrorMessage(res, '更新用户状态失败'))
        return
      }

      setStatusUsers((prev) => prev.map((u) => (u.id === target.id ? { ...u, is_active: nextActive } : u)))
      setSuccessHint(`用户状态已更新：${target.username} -> ${nextActive ? '启用' : '封禁'}`)
    } catch (err) {
      setError(fromUnknownError(err, '网络异常，更新用户状态失败'))
    } finally {
      setStatusUpdatingUserID('')
    }
  }

  const updateUserRole = async (target: AdminUserListItem) => {
    const nextRole = roleDraft[target.id] || target.role
    const reason = (roleReasonDraft[target.id] || '').trim()

    if (target.id === user?.id && nextRole !== 'root') {
      const confirmed = window.confirm('你正在尝试降低自己的 root 权限，这会导致当前账户失去最高权限。确认继续吗？')
      if (!confirmed) return
    }

    if (target.role === 'root' && nextRole !== 'root') {
      const confirmed = window.confirm('你正在降级一个 root 账户。请再次确认该操作。')
      if (!confirmed) return
    }

    setUpdatingRoleUserID(target.id)
    setError('')
    setSuccessHint('')
    try {
      const res = await apiFetch(apiUrl(`/api/admin/users/${target.id}/role`), {
        method: 'PATCH',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ role: nextRole, reason }),
      })

      if (!res.ok) {
        setError(await readSafeErrorMessage(res, '角色更新失败'))
        return
      }

      setRoleUsers((prev) => prev.map((u) => (u.id === target.id ? { ...u, role: nextRole } : u)))
      setSuccessHint(`角色更新成功：${target.username} -> ${nextRole}`)
    } catch (err) {
      setError(fromUnknownError(err, '网络异常，角色更新失败'))
    } finally {
      setUpdatingRoleUserID('')
    }
  }

  const saveSiteConfig = async (e: React.FormEvent) => {
    e.preventDefault()

    setConfigSaving(true)
    setError('')
    setSuccessHint('')
    try {
      const res = await apiFetch(apiUrl('/api/admin/site-config'), {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(siteConfig),
      })

      if (!res.ok) {
        setError(await readSafeErrorMessage(res, '站点配置保存失败'))
        return
      }

      setSuccessHint('站点配置已更新')
    } catch (err) {
      setError(fromUnknownError(err, '网络异常，站点配置保存失败'))
    } finally {
      setConfigSaving(false)
    }
  }

  const handleTabSwitch = (nextTab: AdminTab) => {
    setActiveTab(nextTab)
    setTabAccent((prev) => randomTabAccent(prev))
  }

  return (
    <div className="max-w-7xl mx-auto space-y-6">
      <div className="flex flex-col lg:flex-row lg:items-end lg:justify-between gap-4">
        <div>
          <h1 className="text-3xl font-extrabold tracking-tight text-gray-900">站点管理中心</h1>
          <p className="text-gray-500 mt-2">site_admin 负责运营管理，root 额外负责用户权限治理与站点全局配置。</p>
        </div>
        <div className="text-sm text-gray-600 glass-soft px-4 py-2 w-fit">
          当前身份: <span className="font-semibold text-gray-900">{user?.role || '-'}</span>
        </div>
      </div>

      <div className="flex flex-wrap gap-2">
        <button onClick={() => handleTabSwitch('overview')} className={`tab-pill ${activeTab === 'overview' ? 'tab-pill--active' : ''}`} style={activeTab === 'overview' ? tabAccentStyle(tabAccent) : undefined}>概览</button>
        <button onClick={() => handleTabSwitch('problem-manage')} className={`tab-pill ${activeTab === 'problem-manage' ? 'tab-pill--active' : ''}`} style={activeTab === 'problem-manage' ? tabAccentStyle(tabAccent) : undefined}>题目管理</button>
        <button onClick={() => handleTabSwitch('contest')} className={`tab-pill ${activeTab === 'contest' ? 'tab-pill--active' : ''}`} style={activeTab === 'contest' ? tabAccentStyle(tabAccent) : undefined}>新建赛事</button>
        <button onClick={() => handleTabSwitch('user-status')} className={`tab-pill ${activeTab === 'user-status' ? 'tab-pill--active' : ''}`} style={activeTab === 'user-status' ? tabAccentStyle(tabAccent) : undefined}>用户封禁/解封</button>
        {isRoot && <button onClick={() => handleTabSwitch('root-users')} className={`tab-pill ${activeTab === 'root-users' ? 'tab-pill--active' : ''}`} style={activeTab === 'root-users' ? tabAccentStyle(tabAccent) : undefined}>Root 角色治理</button>}
        {isRoot && <button onClick={() => handleTabSwitch('root-audit')} className={`tab-pill ${activeTab === 'root-audit' ? 'tab-pill--active' : ''}`} style={activeTab === 'root-audit' ? tabAccentStyle(tabAccent) : undefined}>角色变更审计</button>}
        {isRoot && <button onClick={() => handleTabSwitch('root-site-config')} className={`tab-pill ${activeTab === 'root-site-config' ? 'tab-pill--active' : ''}`} style={activeTab === 'root-site-config' ? tabAccentStyle(tabAccent) : undefined}>站点配置</button>}
      </div>

      {error && <div className="rounded-lg border border-red-200 bg-red-50 text-red-700 px-4 py-3">{error}</div>}
      {successHint && <div className="rounded-lg border border-emerald-200 bg-emerald-50 text-emerald-700 px-4 py-3">{successHint}</div>}

      {loading ? (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {Array.from({ length: 3 }).map((_, idx) => <div key={idx} className="h-28 rounded-xl glass-card animate-pulse" />)}
        </div>
      ) : (
        <>
          {activeTab === 'overview' && (
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              {statCards.map((card) => (
                <div key={card.title} className="glass-card p-5">
                  <div className="text-sm text-gray-500">{card.title}</div>
                  <div className="text-3xl font-extrabold text-gray-900 mt-2">{card.value}</div>
                  <div className="text-xs text-gray-500 mt-3">{card.hint}</div>
                </div>
              ))}
            </div>
          )}

          {activeTab === 'problem-manage' && (
            <div className={CARD_CLASS}>
              <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-3">
                <div className="flex items-center gap-3">
                  <h2 className="text-lg font-bold text-gray-900">题目管理</h2>
                  <Link to="/problems/new" className="px-3 py-1.5 rounded-md bg-amber-600 text-white text-sm font-semibold hover:bg-amber-700">
                    前往题目发布页
                  </Link>
                </div>
                <div className="flex gap-2">
                  <input
                    value={adminProblemKeyword}
                    onChange={(e) => setAdminProblemKeyword(e.target.value)}
                    placeholder="搜索标题..."
                    className="w-full lg:w-56 glass-input"
                  />
                  <select
                    value={adminProblemStatus}
                    onChange={(e) => setAdminProblemStatus(e.target.value)}
                    className="glass-input"
                  >
                    <option value="">全部状态</option>
                    <option value="draft">草稿</option>
                    <option value="ready">已发布</option>
                    <option value="hidden">隐藏</option>
                  </select>
                </div>
              </div>

              {adminProblemsLoading ? (
                <div className="text-sm text-gray-500 py-8 text-center">加载中...</div>
              ) : adminProblems.length === 0 ? (
                <div className="text-sm text-gray-500 py-8 text-center border border-gray-200 rounded-lg">
                  暂无题目，请先前往题目发布页创建草稿。
                </div>
              ) : (
                <div className={TABLE_WRAP_CLASS}>
                  <table className="min-w-full text-sm">
                    <thead className="bg-gray-50 text-gray-600">
                      <tr>
                        <th className="text-left px-3 py-2 font-semibold">标题</th>
                        <th className="text-left px-3 py-2 font-semibold">分类</th>
                        <th className="text-left px-3 py-2 font-semibold">难度</th>
                        <th className="text-left px-3 py-2 font-semibold">分值</th>
                        <th className="text-left px-3 py-2 font-semibold">状态</th>
                        <th className="text-left px-3 py-2 font-semibold">标签</th>
                        <th className="text-left px-3 py-2 font-semibold">操作</th>
                      </tr>
                    </thead>
                    <tbody>
                      {adminProblems.map((item) => (
                        <tr key={item.id} className="border-t border-gray-100">
                          <td className="px-3 py-3">
                            <div className="font-semibold text-gray-900">{item.title}</div>
                            {item.author && <div className="text-xs text-gray-500">by {item.author}</div>}
                          </td>
                          <td className="px-3 py-3">
                            <span className="inline-flex px-2 py-0.5 rounded-full text-xs font-semibold bg-gray-100 text-gray-700">{item.category}</span>
                          </td>
                          <td className="px-3 py-3 text-gray-700">{item.difficulty}</td>
                          <td className="px-3 py-3 text-gray-700">{item.platform_score}</td>
                          <td className="px-3 py-3">
                            <span className={`inline-flex px-2 py-0.5 rounded-full text-xs font-semibold ${
                              item.status === 'ready' ? 'bg-emerald-100 text-emerald-700' :
                              item.status === 'draft' ? 'bg-amber-100 text-amber-700' :
                              item.status === 'hidden' ? 'bg-red-100 text-red-700' :
                              'bg-gray-100 text-gray-700'
                            }`}>{
                              item.status === 'hidden' ? 'hidden' : item.status
                            }</span>
                          </td>
                          <td className="px-3 py-3">
                            <div className="flex flex-wrap gap-1">
                              {item.tags.slice(0, 3).map((tag) => (
                                <span key={tag} className="inline-flex px-1.5 py-0.5 rounded text-xs bg-gray-100 text-gray-600">{tag}</span>
                              ))}
                              {item.tags.length > 3 && <span className="text-xs text-gray-400">+{item.tags.length - 3}</span>}
                            </div>
                          </td>
                          <td className="px-3 py-3">
                            <div className="flex gap-2">
                              <Link to={`/problems/${item.id}/edit`} className="px-3 py-1.5 rounded-md bg-blue-600 text-white text-xs font-semibold hover:bg-blue-700">
                                编辑
                              </Link>
                              {item.status !== 'draft' && (
                                <button
                                  onClick={() => toggleProblemVisibility(item.id, item.status, item.title)}
                                  disabled={togglingProblemID === item.id}
                                  className={`px-3 py-1.5 rounded-md text-white text-xs font-semibold disabled:opacity-60 ${
                                    item.status === 'hidden'
                                      ? 'bg-emerald-600 hover:bg-emerald-700'
                                      : 'bg-amber-600 hover:bg-amber-700'
                                  }`}
                                >
                                  {togglingProblemID === item.id ? '处理中...' : item.status === 'hidden' ? '显示' : '隐藏'}
                                </button>
                              )}
                              <button
                                onClick={() => deleteProblem(item.id, item.title)}
                                disabled={deletingProblemID === item.id}
                                className="px-3 py-1.5 rounded-md bg-red-600 text-white text-xs font-semibold hover:bg-red-700 disabled:opacity-60"
                              >
                                {deletingProblemID === item.id ? '删除中...' : '删除'}
                              </button>
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          )}

          {activeTab === 'contest' && (
            <form onSubmit={submitContest} className={CARD_CLASS}>
              <div className="flex flex-wrap items-center justify-between gap-3">
                <h2 className="text-lg font-bold text-gray-900">创建赛事</h2>
                <a href="/admin/contests" className="text-sm font-semibold text-amber-700 hover:text-amber-800">
                  进入新版赛事管理页
                </a>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <input value={contestForm.title} onChange={(e) => setContestForm((prev) => ({ ...prev, title: e.target.value }))} placeholder="赛事标题" className={INPUT_CLASS} required />
                <select value={contestForm.scoring_type} onChange={(e) => setContestForm((prev) => ({ ...prev, scoring_type: e.target.value }))} className={INPUT_CLASS}>
                  {contestScoringTypes.map((item) => <option value={item} key={item}>{item}</option>)}
                </select>
                <input value={contestForm.cover_image} onChange={(e) => setContestForm((prev) => ({ ...prev, cover_image: e.target.value }))} placeholder="封面路径（可选）" className={`${INPUT_CLASS} md:col-span-2`} />
                <input type="datetime-local" value={contestForm.start_time} onChange={(e) => setContestForm((prev) => ({ ...prev, start_time: e.target.value }))} className={INPUT_CLASS} />
                <input type="datetime-local" value={contestForm.end_time} onChange={(e) => setContestForm((prev) => ({ ...prev, end_time: e.target.value }))} className={INPUT_CLASS} />
              </div>
              <textarea value={contestForm.description} onChange={(e) => setContestForm((prev) => ({ ...prev, description: e.target.value }))} placeholder="赛事描述" className={`${INPUT_CLASS} min-h-32`} />
              <button type="submit" disabled={creatingContest} className="btn-primary w-fit">{creatingContest ? '创建中...' : '创建赛事'}</button>
            </form>
          )}

          {activeTab === 'user-status' && (
            <div className={CARD_CLASS}>
              <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-3">
                <h2 className="text-lg font-bold text-gray-900">用户封禁/解封（site_admin）</h2>
                <input
                  value={statusKeyword}
                  onChange={(e) => { setStatusPage(1); setStatusKeyword(e.target.value) }}
                  placeholder="搜索用户名/显示名/邮箱"
                  className="w-full lg:w-72 glass-input"
                />
              </div>

              {statusLoading ? (
                <div className="text-sm text-gray-500">加载中...</div>
              ) : (
                <div className={TABLE_WRAP_CLASS}>
                  <table className="min-w-full text-sm">
                    <thead className="bg-gray-50 text-gray-600">
                      <tr>
                        <th className="text-left px-3 py-2 font-semibold">用户</th>
                        <th className="text-left px-3 py-2 font-semibold">角色</th>
                        <th className="text-left px-3 py-2 font-semibold">状态</th>
                        <th className="text-left px-3 py-2 font-semibold">理由</th>
                        <th className="text-left px-3 py-2 font-semibold">操作</th>
                      </tr>
                    </thead>
                    <tbody>
                      {statusUsers.map((item) => (
                        <tr key={item.id} className="border-t border-gray-100">
                          <td className="px-3 py-3">
                            <div className="font-semibold text-gray-900">{item.display_name || item.username}</div>
                            <div className="text-xs text-gray-500">@{item.username}</div>
                          </td>
                          <td className="px-3 py-3"><span className={`inline-flex px-2 py-0.5 rounded-full text-xs font-semibold ${roleBadgeClass(item.role)}`}>{item.role}</span></td>
                          <td className="px-3 py-3">{item.is_active ? <span className="text-emerald-700">启用</span> : <span className="text-red-700">封禁</span>}</td>
                          <td className="px-3 py-3"><input value={statusReasonDraft[item.id] || ''} onChange={(e) => setStatusReasonDraft((prev) => ({ ...prev, [item.id]: e.target.value }))} placeholder="可选原因" className="glass-input w-52" /></td>
                          <td className="px-3 py-3">
                            <button onClick={() => updateUserActiveStatus(item, !item.is_active)} disabled={statusUpdatingUserID === item.id} className={`px-3 py-1.5 rounded-md text-xs font-semibold text-white ${item.is_active ? 'bg-red-600 hover:bg-red-700' : 'bg-emerald-600 hover:bg-emerald-700'} disabled:opacity-60`}>
                              {statusUpdatingUserID === item.id ? '提交中...' : (item.is_active ? '封禁' : '解封')}
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}

              <div className="flex items-center justify-between">
                <div className="text-xs text-gray-500">第 {statusPage} / {statusTotalPages} 页</div>
                <div className="flex gap-2">
                  <button onClick={() => setStatusPage((prev) => Math.max(1, prev - 1))} disabled={statusPage <= 1} className="btn-secondary disabled:opacity-50">上一页</button>
                  <button onClick={() => setStatusPage((prev) => Math.min(statusTotalPages, prev + 1))} disabled={statusPage >= statusTotalPages} className="btn-secondary disabled:opacity-50">下一页</button>
                </div>
              </div>
            </div>
          )}

          {activeTab === 'root-users' && isRoot && (
            <div className={CARD_CLASS}>
              <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-3">
                <h2 className="text-lg font-bold text-gray-900">Root 用户角色治理（已过滤当前登录账户）</h2>
                <input value={roleKeyword} onChange={(e) => { setRolePage(1); setRoleKeyword(e.target.value) }} placeholder="搜索用户名/显示名/邮箱" className="w-full lg:w-72 glass-input" />
              </div>

              {roleLoading ? (
                <div className="text-sm text-gray-500">加载中...</div>
              ) : (
                <div className={TABLE_WRAP_CLASS}>
                  <table className="min-w-full text-sm">
                    <thead className="bg-gray-50 text-gray-600">
                      <tr>
                        <th className="text-left px-3 py-2 font-semibold">用户</th>
                        <th className="text-left px-3 py-2 font-semibold">当前角色</th>
                        <th className="text-left px-3 py-2 font-semibold">新角色</th>
                        <th className="text-left px-3 py-2 font-semibold">理由</th>
                        <th className="text-left px-3 py-2 font-semibold">操作</th>
                      </tr>
                    </thead>
                    <tbody>
                      {roleUsers.map((item) => (
                        <tr key={item.id} className="border-t border-gray-100">
                          <td className="px-3 py-3">
                            <div className="font-semibold text-gray-900">{item.display_name || item.username}</div>
                            <div className="text-xs text-gray-500">@{item.username}</div>
                          </td>
                          <td className="px-3 py-3"><span className={`inline-flex px-2 py-0.5 rounded-full text-xs font-semibold ${roleBadgeClass(item.role)}`}>{item.role}</span></td>
                          <td className="px-3 py-3">
                            <select value={roleDraft[item.id] || item.role} onChange={(e) => setRoleDraft((prev) => ({ ...prev, [item.id]: e.target.value }))} className="glass-input !w-auto !py-1">
                              <option value="site_admin">site_admin</option>
                              <option value="user">user</option>
                            </select>
                          </td>
                          <td className="px-3 py-3"><input value={roleReasonDraft[item.id] || ''} onChange={(e) => setRoleReasonDraft((prev) => ({ ...prev, [item.id]: e.target.value }))} placeholder="可选原因" className="glass-input w-52 !py-1" /></td>
                          <td className="px-3 py-3"><button onClick={() => updateUserRole(item)} disabled={updatingRoleUserID === item.id} className="px-3 py-1.5 rounded-md bg-red-600 text-white text-xs font-semibold hover:bg-red-700 disabled:opacity-60">{updatingRoleUserID === item.id ? '提交中...' : '更新角色'}</button></td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}

              <div className="flex items-center justify-between">
                <div className="text-xs text-gray-500">第 {rolePage} / {roleTotalPages} 页</div>
                <div className="flex gap-2">
                  <button onClick={() => setRolePage((prev) => Math.max(1, prev - 1))} disabled={rolePage <= 1} className="btn-secondary disabled:opacity-50">上一页</button>
                  <button onClick={() => setRolePage((prev) => Math.min(roleTotalPages, prev + 1))} disabled={rolePage >= roleTotalPages} className="btn-secondary disabled:opacity-50">下一页</button>
                </div>
              </div>
            </div>
          )}

          {activeTab === 'root-audit' && isRoot && (
            <div className={CARD_CLASS}>
              <h2 className="text-lg font-bold text-gray-900">角色变更历史（审计日志）</h2>
              {logsLoading ? (
                <div className="text-sm text-gray-500">加载中...</div>
              ) : roleLogs.length === 0 ? (
                <div className="text-sm text-gray-500 border border-gray-200 rounded-lg p-4">暂无角色变更记录</div>
              ) : (
                <div className={TABLE_WRAP_CLASS}>
                  <table className="min-w-full text-sm">
                    <thead className="bg-gray-50 text-gray-600">
                      <tr>
                        <th className="text-left px-3 py-2 font-semibold">时间</th>
                        <th className="text-left px-3 py-2 font-semibold">操作人</th>
                        <th className="text-left px-3 py-2 font-semibold">目标用户</th>
                        <th className="text-left px-3 py-2 font-semibold">变更</th>
                        <th className="text-left px-3 py-2 font-semibold">理由</th>
                        <th className="text-left px-3 py-2 font-semibold">IP</th>
                      </tr>
                    </thead>
                    <tbody>
                      {roleLogs.map((item) => (
                        <tr key={item.id} className="border-t border-gray-100">
                          <td className="px-3 py-3 text-gray-700">{formatDateTime(item.created_at)}</td>
                          <td className="px-3 py-3">{item.operator_username || item.operator_user_id || '-'}</td>
                          <td className="px-3 py-3">{item.target_username || item.target_user_id || '-'}</td>
                          <td className="px-3 py-3"><span className="text-gray-700">{item.old_role || '-'} {'->'} {item.new_role || '-'}</span></td>
                          <td className="px-3 py-3 text-gray-700">{item.reason || '-'}</td>
                          <td className="px-3 py-3 text-gray-500">{item.ip || '-'}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}

              <div className="flex items-center justify-between">
                <div className="text-xs text-gray-500">第 {logsPage} / {logsTotalPages} 页</div>
                <div className="flex gap-2">
                  <button onClick={() => setLogsPage((prev) => Math.max(1, prev - 1))} disabled={logsPage <= 1} className="btn-secondary disabled:opacity-50">上一页</button>
                  <button onClick={() => setLogsPage((prev) => Math.min(logsTotalPages, prev + 1))} disabled={logsPage >= logsTotalPages} className="btn-secondary disabled:opacity-50">下一页</button>
                </div>
              </div>
            </div>
          )}

          {activeTab === 'root-site-config' && isRoot && (
            <form onSubmit={saveSiteConfig} className={CARD_CLASS}>
              <h2 className="text-lg font-bold text-gray-900">站点配置（root）</h2>
              <p className="text-sm text-gray-600">对应 RBAC 规则中的“修改站点配置”。当前配置写入数据库 `site_settings`，并记录审计日志。</p>

              {configLoading ? (
                <div className="text-sm text-gray-500">加载中...</div>
              ) : (
                <>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <input value={siteConfig.site_name} onChange={(e) => setSiteConfig((prev) => ({ ...prev, site_name: e.target.value }))} placeholder="站点名称" className={INPUT_CLASS} required />
                    <select value={siteConfig.default_contest_visibility} onChange={(e) => setSiteConfig((prev) => ({ ...prev, default_contest_visibility: e.target.value as SiteConfig['default_contest_visibility'] }))} className={INPUT_CLASS}>
                      <option value="public">public</option>
                      <option value="invite_only">invite_only</option>
                      <option value="private">private</option>
                    </select>
                  </div>
                  <textarea value={siteConfig.site_notice} onChange={(e) => setSiteConfig((prev) => ({ ...prev, site_notice: e.target.value }))} placeholder="站点公告" className={`${INPUT_CLASS} min-h-28`} />
                  <div className="flex flex-wrap gap-4">
                    <label className="inline-flex items-center gap-2 text-sm text-gray-700"><input type="checkbox" checked={siteConfig.registration_enabled} onChange={(e) => setSiteConfig((prev) => ({ ...prev, registration_enabled: e.target.checked }))} />允许新用户注册</label>
                    <label className="inline-flex items-center gap-2 text-sm text-gray-700"><input type="checkbox" checked={siteConfig.email_verification_required} onChange={(e) => setSiteConfig((prev) => ({ ...prev, email_verification_required: e.target.checked }))} />要求邮箱验证</label>
                  </div>
                  <button type="submit" disabled={configSaving} className="btn-primary w-fit">{configSaving ? '保存中...' : '保存站点配置'}</button>
                </>
              )}
            </form>
          )}
        </>
      )}
    </div>
  )
}
