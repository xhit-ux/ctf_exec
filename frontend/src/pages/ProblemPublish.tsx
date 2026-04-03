import React, { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { useAuth } from '../context/AuthContext'
import { fromUnknownError } from '../utils/error'
import { createAdminProblem, fetchAdminProblem, updateAdminProblem } from '../utils/contestApi'

type ProblemFormState = {
  title: string
  type: 'static' | 'dynamic'
  category: string
  points: number
  difficulty: string
  description: string
  flag: string
  tags: string[]
  tagInput: string
  flagMode: 'exact' | 'case_insensitive' | 'contains' | 'regex'
  flagRuleValue: string
  flagExample: string
  dockerImage: string
  runCommand: string
  scope: 'platform' | 'contest'
  contestID: string
  status: 'draft' | 'review' | 'published'
  visibleFrom: string
  visibleTo: string
}

const defaultForm: ProblemFormState = {
  title: '',
  type: 'static',
  category: 'web',
  points: 100,
  difficulty: '简单',
  description: '',
  flag: '',
  tags: [],
  tagInput: '',
  flagMode: 'exact',
  flagRuleValue: '',
  flagExample: '',
  dockerImage: '',
  runCommand: '',
  scope: 'platform',
  contestID: '',
  status: 'published',
  visibleFrom: '',
  visibleTo: '',
}

function splitRule(rule?: string): { mode: ProblemFormState['flagMode']; value: string } {
  const normalized = (rule || '').trim()
  if (normalized === '' || normalized === 'exact') return { mode: 'exact', value: '' }
  if (normalized === 'case_insensitive') return { mode: 'case_insensitive', value: '' }
  if (normalized.startsWith('contains:')) return { mode: 'contains', value: normalized.slice('contains:'.length) }
  if (normalized.startsWith('regex:')) return { mode: 'regex', value: normalized.slice('regex:'.length) }
  return { mode: 'exact', value: '' }
}

function toDatetimeLocal(input?: string | null): string {
  if (!input) return ''
  const d = new Date(input)
  if (Number.isNaN(d.getTime())) return ''
  const pad = (v: number) => String(v).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function appendTags(existing: string[], raw: string): string[] {
  const pieces = raw
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)

  if (pieces.length === 0) return existing

  const seen = new Set(existing.map((item) => item.toLowerCase()))
  const next = [...existing]
  for (const item of pieces) {
    const key = item.toLowerCase()
    if (!seen.has(key)) {
      seen.add(key)
      next.push(item)
    }
  }
  return next
}

export default function ProblemPublish() {
  const { user } = useAuth()
  const { id } = useParams()
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()

  const isEditing = Number.isInteger(Number(id)) && Number(id) > 0
  const contestIDFromQuery = (searchParams.get('contest_id') || '').trim()
  const scopeFromQuery = (searchParams.get('scope') || '').trim().toLowerCase()

  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [form, setForm] = useState<ProblemFormState>(defaultForm)

  const role = user?.role || ''
  const isSiteAdmin = role === 'root' || role === 'site_admin'
  const isContestAdmin = role === 'contest_admin'

  const resolvedScope = useMemo<'platform' | 'contest'>(() => {
    if (isContestAdmin) return 'contest'
    if (scopeFromQuery === 'contest' || contestIDFromQuery) return 'contest'
    return 'platform'
  }, [contestIDFromQuery, isContestAdmin, scopeFromQuery])

  useEffect(() => {
    if (!isEditing) return

    const load = async () => {
      setLoading(true)
      setError('')
      try {
        const data = await fetchAdminProblem(Number(id))
        const rule = splitRule(data.flag_rule)
        setForm({
          title: data?.title || '',
          type: data?.type || 'static',
          category: data?.category || 'web',
          points: Number(data?.points) || 100,
          difficulty: data?.difficulty || '简单',
          description: data?.description || '',
          flag: '',
          tags: Array.isArray(data?.tags) ? data.tags : [],
          tagInput: '',
          flagMode: rule.mode,
          flagRuleValue: rule.value,
          flagExample: data?.flag_example || '',
          dockerImage: data?.docker_image || '',
          runCommand: data?.run_command || '',
          scope: data?.scope || resolvedScope,
          contestID: data?.contest_id || contestIDFromQuery,
          status: (data?.status || 'published') as 'draft' | 'review' | 'published',
          visibleFrom: toDatetimeLocal(data?.visible_from),
          visibleTo: toDatetimeLocal(data?.visible_to),
        })
      } catch (err) {
        setError(fromUnknownError(err, '加载题目详情失败'))
      } finally {
        setLoading(false)
      }
    }

    load()
  }, [id, isEditing, contestIDFromQuery, resolvedScope])

  useEffect(() => {
    if (isEditing) return
    setForm((prev) => ({
      ...prev,
      scope: resolvedScope,
      contestID: contestIDFromQuery,
      status: isContestAdmin ? 'draft' : prev.status,
    }))
  }, [contestIDFromQuery, isContestAdmin, isEditing, resolvedScope])

  const flagRule = useMemo(() => {
    if (form.flagMode === 'contains' || form.flagMode === 'regex') {
      return `${form.flagMode}:${form.flagRuleValue.trim()}`
    }
    return form.flagMode
  }, [form.flagMode, form.flagRuleValue])

  const canDirectPublish = isSiteAdmin

  const submit = async (targetStatus?: 'draft' | 'review' | 'published') => {
    setError('')
    setSuccess('')

    if (!form.title.trim() || !form.category.trim() || !form.difficulty.trim()) {
      setError('题目标题、分类、难度不能为空')
      return
    }

    if (!Number.isFinite(Number(form.points)) || Number(form.points) < 0) {
      setError('分值必须是大于等于 0 的数字')
      return
    }

    if (!isEditing && !form.flag.trim() && !(form.flagMode === 'contains' || form.flagMode === 'regex')) {
      setError('创建题目时必须填写 Flag')
      return
    }

    if ((isContestAdmin || form.scope === 'contest') && !form.contestID.trim() && !isEditing) {
      setError('contest_admin 发布题目时必须通过赛事上下文进入（缺少 contest_id）')
      return
    }

    if ((form.flagMode === 'contains' || form.flagMode === 'regex') && !form.flagRuleValue.trim()) {
      setError('contains/regex 模式必须填写规则内容')
      return
    }

    const statusToSubmit = targetStatus || form.status
    if (!canDirectPublish && statusToSubmit === 'published') {
      setError('当前角色不能直接发布，请先保存草稿或提交审核')
      return
    }

    const normalizedFlag = form.flag.trim() || form.flagRuleValue.trim()

    setSaving(true)
    try {
      const payload = {
        title: form.title.trim(),
        category: form.category.trim(),
        type: form.type,
        points: Number(form.points),
        difficulty: form.difficulty.trim(),
        description: form.description,
        tags: form.tags,
        flag: normalizedFlag,
        flag_rule: flagRule,
        flag_example: form.flagExample.trim(),
        docker_image: form.type === 'dynamic' ? form.dockerImage.trim() : '',
        run_command: form.type === 'dynamic' ? form.runCommand.trim() : '',
        scope: isContestAdmin ? 'contest' : form.scope,
        contest_id: (isContestAdmin || form.scope === 'contest') ? form.contestID.trim() : undefined,
        status: statusToSubmit,
        visible_from: form.visibleFrom ? new Date(form.visibleFrom).toISOString() : null,
        visible_to: form.visibleTo ? new Date(form.visibleTo).toISOString() : null,
      }

      if (isEditing) {
        await updateAdminProblem(Number(id), payload)

        setSuccess('题目更新成功')
        return
      }

      await createAdminProblem(payload)

      setSuccess(statusToSubmit === 'published' ? '题目发布成功' : statusToSubmit === 'review' ? '已提交审核' : '草稿已保存')
      setForm({ ...defaultForm, scope: resolvedScope, contestID: contestIDFromQuery, status: isContestAdmin ? 'draft' : 'published' })
      if ((isContestAdmin || form.scope === 'contest') && form.contestID.trim()) {
        navigate(`/contest/${form.contestID.trim()}/admin`, { replace: true })
      }
    } catch (err) {
      setError(fromUnknownError(err, isEditing ? '更新题目失败' : '发布题目失败'))
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return <div className="rounded-lg bg-white p-6 text-sm text-gray-600">加载中...</div>
  }

  return (
    <section className="mx-auto w-full max-w-4xl space-y-5 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">{isEditing ? '编辑题目' : '发布题目'}</h1>
          <p className="mt-1 text-sm text-gray-500">
            {(isContestAdmin ? 'contest' : form.scope) === 'contest'
              ? '当前发布域: contest（题目会先进入赛事域）'
              : '当前发布域: platform（题目发布后进入题目大厅）'}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Link to="/problem" className="rounded-lg border border-gray-200 px-3 py-2 text-sm font-semibold text-gray-700 hover:bg-gray-50">返回题目大厅</Link>
          {!isEditing && contestIDFromQuery && (
            <Link to={`/contest/${contestIDFromQuery}/admin`} className="rounded-lg border border-blue-200 bg-blue-50 px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-100">返回赛事管理</Link>
          )}
        </div>
      </div>

      {isContestAdmin && !contestIDFromQuery && !isEditing && (
        <div className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-700">
          contest_admin 需从赛事管理空间进入发布页。请先前往
          <Link to="/contests" className="ml-1 font-semibold underline">赛事大厅</Link>
          选择对应赛事。
        </div>
      )}

      {error && <div className="rounded-lg border border-red-100 bg-red-50 px-3 py-2 text-sm text-red-600">{error}</div>}
      {success && <div className="rounded-lg border border-green-100 bg-green-50 px-3 py-2 text-sm text-green-600">{success}</div>}

      <div className="grid grid-cols-1 gap-3 rounded-xl border border-gray-100 bg-gray-50 p-4 md:grid-cols-2">
        <input value={form.title} onChange={(event) => setForm((prev) => ({ ...prev, title: event.target.value }))} className="rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm" placeholder="题目标题" />
        <select value={form.type} onChange={(event) => setForm((prev) => ({ ...prev, type: event.target.value as 'static' | 'dynamic' }))} className="rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm">
          <option value="static">static（静态题）</option>
          <option value="dynamic">dynamic（动态题）</option>
        </select>
        <input value={form.category} onChange={(event) => setForm((prev) => ({ ...prev, category: event.target.value }))} className="rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm" placeholder="分类（web/pwn/reverse/crypto/misc）" />
        <input type="number" value={form.points} onChange={(event) => setForm((prev) => ({ ...prev, points: Number(event.target.value) }))} className="rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm" placeholder="分值" />
        <select value={form.difficulty} onChange={(event) => setForm((prev) => ({ ...prev, difficulty: event.target.value }))} className="rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm">
          <option value="简单">简单</option>
          <option value="中等">中等</option>
          <option value="困难">困难</option>
        </select>
        {!isContestAdmin && (
          <select value={form.scope} onChange={(event) => setForm((prev) => ({ ...prev, scope: event.target.value as 'platform' | 'contest' }))} className="rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm">
            <option value="platform">platform</option>
            <option value="contest">contest</option>
          </select>
        )}
        {(isContestAdmin || form.scope === 'contest') && (
          <input value={form.contestID} onChange={(event) => setForm((prev) => ({ ...prev, contestID: event.target.value }))} className="rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm md:col-span-2" placeholder="赛事 ID（contest_id）" />
        )}
        <div className="md:col-span-2 rounded-lg border border-gray-200 bg-white p-2">
          <div className="mb-2 flex flex-wrap gap-2">
            {form.tags.map((tag) => (
              <span key={tag} className="inline-flex items-center gap-1 rounded-full bg-blue-50 px-2 py-1 text-xs font-medium text-blue-700">
                #{tag}
                <button
                  type="button"
                  className="text-blue-500"
                  onClick={() => setForm((prev) => ({ ...prev, tags: prev.tags.filter((item) => item !== tag) }))}
                >
                  x
                </button>
              </span>
            ))}
          </div>
          <input
            value={form.tagInput}
            onChange={(event) => setForm((prev) => ({ ...prev, tagInput: event.target.value }))}
            onKeyDown={(event) => {
              if (event.key === 'Enter' || event.key === ',') {
                event.preventDefault()
                setForm((prev) => ({ ...prev, tags: appendTags(prev.tags, prev.tagInput), tagInput: '' }))
              }
            }}
            onBlur={() => setForm((prev) => ({ ...prev, tags: appendTags(prev.tags, prev.tagInput), tagInput: '' }))}
            className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm"
            placeholder="输入标签，按 Enter 或逗号添加"
          />
        </div>
      </div>

      <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
        <div className="space-y-2 rounded-xl border border-gray-100 p-4">
          <h3 className="text-sm font-semibold text-gray-800">Flag / 答案设置</h3>
          <select value={form.flagMode} onChange={(event) => setForm((prev) => ({ ...prev, flagMode: event.target.value as ProblemFormState['flagMode'] }))} className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm">
            <option value="exact">完全匹配</option>
            <option value="case_insensitive">忽略大小写</option>
            <option value="contains">子串匹配</option>
            <option value="regex">正则匹配</option>
          </select>
          {(form.flagMode === 'contains' || form.flagMode === 'regex') && (
            <input value={form.flagRuleValue} onChange={(event) => setForm((prev) => ({ ...prev, flagRuleValue: event.target.value }))} className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm" placeholder={form.flagMode === 'contains' ? '子串关键词' : '正则表达式'} />
          )}
          <input value={form.flag} onChange={(event) => setForm((prev) => ({ ...prev, flag: event.target.value }))} className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm" placeholder={isEditing ? '如需更新校验种子请输入；留空则沿用/由规则值回填' : 'Flag（exact/case_insensitive 建议必填）'} />
          <input value={form.flagExample} onChange={(event) => setForm((prev) => ({ ...prev, flagExample: event.target.value }))} className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm" placeholder="Flag 示例（可选）" />
        </div>

        <div className="space-y-2 rounded-xl border border-gray-100 p-4">
          <h3 className="text-sm font-semibold text-gray-800">发布设置</h3>
          <select value={form.status} onChange={(event) => setForm((prev) => ({ ...prev, status: event.target.value as ProblemFormState['status'] }))} className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm">
            <option value="draft">draft</option>
            <option value="review">review</option>
            <option value="published" disabled={!canDirectPublish}>published</option>
          </select>
          <input type="datetime-local" value={form.visibleFrom} onChange={(event) => setForm((prev) => ({ ...prev, visibleFrom: event.target.value }))} className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm" />
          <input type="datetime-local" value={form.visibleTo} onChange={(event) => setForm((prev) => ({ ...prev, visibleTo: event.target.value }))} className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm" />
          <p className="text-xs text-gray-500">contest 题目可见性仍受赛事时间窗控制；platform 题目按 visible_from/visible_to 控制。</p>
        </div>

        {form.type === 'dynamic' && (
          <div className="space-y-2 rounded-xl border border-gray-100 p-4 md:col-span-2">
            <h3 className="text-sm font-semibold text-gray-800">网络配置（dynamic）</h3>
            <input value={form.dockerImage} onChange={(event) => setForm((prev) => ({ ...prev, dockerImage: event.target.value }))} className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm" placeholder="docker_image，例如 ctf/web-lab:latest" />
            <textarea value={form.runCommand} onChange={(event) => setForm((prev) => ({ ...prev, runCommand: event.target.value }))} className="min-h-[88px] w-full rounded-lg border border-gray-200 px-3 py-2 text-sm" placeholder="run_command，例如 python3 /app/server.py" />
          </div>
        )}

        <div className="md:col-span-2 grid grid-cols-1 gap-3 lg:grid-cols-2">
          <div className="space-y-2 rounded-xl border border-gray-100 p-4">
            <h3 className="text-sm font-semibold text-gray-800">题目描述（Markdown）</h3>
            <textarea value={form.description} onChange={(event) => setForm((prev) => ({ ...prev, description: event.target.value }))} className="min-h-[220px] w-full rounded-lg border border-gray-200 px-3 py-2 text-sm" placeholder="支持 # 标题、**加粗**、`代码`" />
            <div className="text-xs text-gray-500">字数: {form.description.length}</div>
          </div>
          <div className="space-y-2 rounded-xl border border-gray-100 p-4">
            <h3 className="text-sm font-semibold text-gray-800">实时预览</h3>
            <div className="min-h-[220px] rounded-lg border border-gray-200 bg-gray-50 p-3 text-sm text-gray-700 prose prose-sm max-w-none">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{form.description || '_暂无内容_'}</ReactMarkdown>
            </div>
          </div>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <button type="button" disabled={saving} onClick={() => submit('draft')} className="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-semibold text-gray-700 disabled:opacity-50">
          {saving ? '提交中...' : '保存草稿'}
        </button>
        <button type="button" disabled={saving} onClick={() => submit('review')} className="rounded-lg bg-amber-500 px-4 py-2 text-sm font-semibold text-white disabled:opacity-50">
          {saving ? '提交中...' : '提交审核'}
        </button>
        <button type="button" disabled={saving || !canDirectPublish} onClick={() => submit('published')} className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-semibold text-white disabled:opacity-50">
          {saving ? '提交中...' : isEditing ? '保存并发布' : '立即发布'}
        </button>
        {!isSiteAdmin && !isContestAdmin && (
          <span className="text-xs text-red-500">当前角色无发布权限</span>
        )}
      </div>

      {isEditing && (
        <div className="rounded-lg border border-blue-100 bg-blue-50 px-3 py-2 text-xs text-blue-700">
          编辑模式下留空 Flag 不会清空数据库，仅在你填写时才更新校验种子。
        </div>
      )}
    </section>
  )
}
