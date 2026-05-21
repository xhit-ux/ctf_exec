import React, { useEffect, useState } from 'react'
import { useParams, useSearchParams, Link } from 'react-router-dom'
import { Submission } from '../types'
import { apiUrl } from '../utils/api'
import { apiFetch } from '../utils/authClient'
import { readSafeErrorMessage } from '../utils/error'
import { useAuth } from '../context/AuthContext'
import {
  categoryLabel,
  difficultyLabel,
  normalizeProblem,
} from '../utils/problemView'
import type { ProblemHint, ProblemPrerequisiteRef, ProblemView } from '../utils/problemView'
import {
  destroyProblemInstance,
  fetchProblemInstance,
  startProblemInstance,
} from '../utils/problemInstanceApi'
import { unlockProblemHint } from '../utils/problemHintsApi'

type InstanceState = {
  status: 'idle' | 'starting' | 'running' | 'stopping' | 'failed'
  endpoint: string
  note: string
}

function formatAttachmentSize(size?: number): string {
  if (!Number.isFinite(size)) return ''
  if (!size || size <= 0) return ''
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${(size / (1024 * 1024)).toFixed(1)} MB`
}

function formatRemaining(seconds?: number | null): string {
  if (seconds === undefined || seconds === null || !Number.isFinite(seconds)) return ''
  if (seconds <= 0) return '已过期'
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  if (m === 0) return `${s}s`
  return `${m}m ${s.toString().padStart(2, '0')}s`
}

function PrerequisitesSection({ items }: { items: ProblemPrerequisiteRef[] }) {
  if (!items || items.length === 0) return null
  const allSolved = items.every((it) => it.is_solved)
  return (
    <div
      className={`bg-white p-4 rounded-xl shadow-sm border ${
        allSolved ? 'border-emerald-100' : 'border-amber-200'
      }`}
    >
      <div className="flex items-center justify-between mb-2">
        <h4 className="font-medium">前置题目</h4>
        <span
          className={`text-xs px-2 py-0.5 rounded-full ${
            allSolved
              ? 'bg-emerald-50 text-emerald-700 border border-emerald-200'
              : 'bg-amber-50 text-amber-700 border border-amber-200'
          }`}
        >
          {allSolved ? '已全部解出' : '尚未全部解出'}
        </span>
      </div>
      <ul className="space-y-2 text-sm">
        {items.map((it) => (
          <li
            key={it.id}
            className="flex items-center justify-between gap-3 px-3 py-2 rounded-lg border border-gray-100 bg-gray-50"
          >
            <div className="min-w-0">
              <div className="truncate text-gray-900">{it.title || it.id}</div>
              <div className="text-xs text-gray-500">
                {it.is_solved ? '✓ 已解出' : '○ 未解出'}
              </div>
            </div>
            <Link
              to={`/problems/${it.id}`}
              className="text-xs px-3 py-1.5 rounded-md border border-gray-200 hover:bg-gray-100"
            >
              查看
            </Link>
          </li>
        ))}
      </ul>
    </div>
  )
}

function HintSection({
  hints,
  onUnlock,
  pending,
}: {
  hints: ProblemHint[]
  onUnlock: (hintID: string) => void
  pending: string | null
}) {
  if (!hints || hints.length === 0) return null
  return (
    <div className="bg-white p-4 rounded-xl shadow-sm">
      <h4 className="font-medium mb-3">提示（解锁后扣除积分）</h4>
      <ul className="space-y-2">
        {hints.map((hint, idx) => (
          <li
            key={hint.id || idx}
            className="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2"
          >
            <div className="flex items-center justify-between gap-3">
              <div className="text-sm text-gray-700">
                提示 {idx + 1}
                <span className="ml-2 text-xs text-gray-500">扣 {hint.cost} 分</span>
              </div>
              {hint.is_unlocked ? (
                <span className="text-xs px-2 py-0.5 rounded-full bg-emerald-50 text-emerald-700 border border-emerald-200">
                  已解锁
                </span>
              ) : (
                <button
                  onClick={() => onUnlock(hint.id)}
                  disabled={pending === hint.id}
                  className="text-xs px-3 py-1.5 rounded-md bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-60"
                >
                  {pending === hint.id ? '解锁中...' : '解锁'}
                </button>
              )}
            </div>
            {hint.is_unlocked && hint.content && (
              <p className="text-sm text-gray-700 mt-2 whitespace-pre-wrap">{hint.content}</p>
            )}
          </li>
        ))}
      </ul>
    </div>
  )
}

export default function ProblemDetail() {
  const { user } = useAuth()
  const { id, problemId } = useParams()
  const problemID = String(problemId ?? id ?? '').trim()
  const [searchParams] = useSearchParams()
  const contestID = searchParams.get('contest_id') || undefined

  const [problem, setProblem] = useState<ProblemView | null>(null)
  const [hints, setHints] = useState<ProblemHint[]>([])
  const [pendingHint, setPendingHint] = useState<string | null>(null)
  const [flag, setFlag] = useState('')
  const [msg, setMsg] = useState<string | null>(null)
  const [submissions, setSubmissions] = useState<Submission[]>([])
  const [submissionsLoading, setSubmissionsLoading] = useState(false)
  const [instanceState, setInstanceState] = useState<InstanceState>({
    status: 'idle',
    endpoint: '',
    note: '',
  })
  const canEditProblem = user?.role === 'root' || user?.role === 'site_admin'
	const [countdown, setCountdown] = useState<number | null>(null)
	useEffect(() => { if (countdown !== null && countdown > 0) { const t = setInterval(() => setCountdown(p => (p && p > 0 ? p - 1 : 0)), 1000); return () => clearInterval(t) } }, [countdown !== null])
	useEffect(() => { const s = problem?.instance?.remaining_seconds; if (s && s > 0) setCountdown(s) }, [problem?.instance?.remaining_seconds])

  useEffect(() => {
    if (!problemID) {
      setProblem(null)
      return
    }

    setSubmissions([])
    setMsg(null)
    setFlag('')

    apiFetch(apiUrl(`/api/problems/${encodeURIComponent(problemID)}`))
      .then((res) => {
        if (res.ok) return res.json()
        throw new Error('Failed to fetch problem')
      })
      .then((data) => {
        const view = normalizeProblem(data)
        setProblem(view)
        setHints(view.hints ?? [])
      })
      .catch((err) => console.error(err))
  }, [problemID])

  useEffect(() => {
    setInstanceState({ status: 'idle', endpoint: '', note: '' })
  }, [problemID])

  useEffect(() => {
    if (!problemID || !problem || problem.type === 'static') {
      setInstanceState({ status: 'idle', endpoint: '', note: '' })
      return
    }

    // 优先使用后端 problem.instance 作为初始态。
    if (problem.instance && problem.instance.status === 'running') {
      const endpoint =
        problem.connection_info?.http ||
        problem.connection_info?.tcp ||
        problem.instance.access_url ||
        ''
      setInstanceState({
        status: 'running',
        endpoint,
        note: endpoint ? '实例运行中，可直接访问。' : '实例运行中，访问地址暂未返回。',
      })
      return
    }

    let cancelled = false
    fetchProblemInstance(problemID)
      .then((resp) => {
        if (cancelled) return
        const serverStatus = String(resp.status || '').toLowerCase()
        const endpoint = resp.instance?.access_url || ''
        if (serverStatus === 'running') {
          setInstanceState({
            status: 'running',
            endpoint,
            note: endpoint ? '实例运行中，可直接访问。' : '实例运行中，访问地址暂未返回。',
          })
          return
        }
        if (serverStatus === 'failed') {
          setInstanceState({ status: 'failed', endpoint: '', note: '实例创建失败，可重试启动。' })
          return
        }
        setInstanceState({ status: 'idle', endpoint: '', note: '' })
      })
      .catch(() => {
        if (cancelled) return
        setInstanceState({ status: 'idle', endpoint: '', note: '' })
      })

    return () => {
      cancelled = true
    }
  }, [problemID, problem])

  useEffect(() => {
    if (!problemID) return

    let cancelled = false
    setSubmissionsLoading(true)

    apiFetch(apiUrl('/api/submissions/me'))
      .then(async (res) => {
        if (!res.ok) {
          throw new Error('Failed to fetch submissions')
        }
        return res.json()
      })
      .then((data) => {
        if (cancelled) return
        const source = Array.isArray(data) ? data : []
        const mapped: Submission[] = source.map((item: any) => ({
          problemId: item.problem_id,
          problemTitle: item.problem_title,
          flag: item.flag,
          correct: Boolean(item.is_correct),
          time: item.created_at,
        }))
        const currentProblemSubmissions = mapped.filter(
          (s) => String(s.problemId) === problemID,
        )
        setSubmissions(currentProblemSubmissions)
      })
      .catch((err) => {
        if (!cancelled) {
          console.error('Failed to fetch problem submissions', err)
          setSubmissions([])
        }
      })
      .finally(() => {
        if (!cancelled) {
          setSubmissionsLoading(false)
        }
      })

    return () => {
      cancelled = true
    }
  }, [problemID])

  if (!problem) return <div className="p-10 text-center">题目加载中或未找到...</div>

  const isContainerProblem = problem.type !== 'static'
  const isLocked = Boolean(problem.is_locked)

  const submit = async () => {
    if (isLocked) {
      setMsg('前置题目尚未全部解出，无法提交。')
      return
    }
    try {
      const response = await apiFetch(apiUrl('/api/submissions'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          problem_id: problem.id,
          flag,
          ...(contestID ? { contest_id: contestID } : {}),
        }),
      })

      if (response.ok) {
        const result = await response.json()
        const isCorrect = result.is_correct
        const record: Submission = {
          problemId: problem.id,
          flag,
          correct: isCorrect,
          time:
            typeof result.created_at === 'string'
              ? result.created_at
              : new Date().toISOString(),
        }
        setSubmissions((s) => [record, ...s])
        setMsg(isCorrect ? '提交成功，Flag 正确！' : '提交已记录，Flag 错误')
      } else if (response.status === 401) {
        return
      } else {
        const userFriendlyMsg = await readSafeErrorMessage(response, '提交失败，请稍后重试')
        setMsg(`提交失败: ${userFriendlyMsg}`)
      }
    } catch (err) {
      console.error('网络错误', err)
      setMsg('网络错误，请重试')
    }
  }

  const startRealInstance = async () => {
    if (!isContainerProblem || instanceState.status === 'starting' || instanceState.status === 'stopping')
      return
    setInstanceState({ status: 'starting', endpoint: '', note: '' })

    try {
      const resp = await startProblemInstance(problem.id)
      const endpoint = resp.instance?.access_url || ''
      const status = String(resp.status || '').toLowerCase()

      if (status === 'running') {
        setInstanceState({
          status: 'running',
          endpoint,
          note: endpoint ? '实例已启动。' : '实例已启动，访问地址待同步。',
        })
        return
      }
      if (status === 'pending') {
        setInstanceState({ status: 'starting', endpoint: '', note: '实例正在创建，请稍后刷新状态。' })
        return
      }
      setInstanceState({ status: 'failed', endpoint: '', note: '实例启动失败，请重试。' })
    } catch (err) {
      const message = err instanceof Error ? err.message : '实例启动失败'
      setInstanceState({ status: 'failed', endpoint: '', note: message })
    }
  }

  const destroyRealInstance = async () => {
    if (!isContainerProblem || instanceState.status === 'stopping' || instanceState.status === 'starting')
      return
    setInstanceState((prev) => ({ ...prev, status: 'stopping', note: '正在销毁实例...' }))
    try {
      await destroyProblemInstance(problem.id)
      setInstanceState({ status: 'idle', endpoint: '', note: '' })
    } catch (err) {
      const message = err instanceof Error ? err.message : '实例销毁失败'
      setInstanceState((prev) => ({ ...prev, status: 'failed', note: message }))
    }
  }

  const handleUnlockHint = async (hintID: string) => {
    if (!hintID || pendingHint) return
    setPendingHint(hintID)
    try {
      const updated = await unlockProblemHint(hintID)
      setHints((prev) =>
        prev.map((h) =>
          h.id === hintID ? { ...h, ...updated, is_unlocked: true } : h,
        ),
      )
    } catch (err) {
      const message = err instanceof Error ? err.message : '解锁提示失败'
      window.alert(message)
    } finally {
      setPendingHint(null)
    }
  }

  return (
    <div className="w-full max-w-6xl mx-auto">
      {contestID && (
        <div className="mb-4 text-sm bg-blue-50 border border-blue-100 text-blue-700 rounded-lg px-4 py-3 flex items-center justify-between gap-3">
          <span>当前提交将按赛事上下文计分（Contest ID: {contestID}）。</span>
          <Link
            to={`/contest/${contestID}`}
            className="font-semibold hover:underline whitespace-nowrap"
          >
            返回赛事
          </Link>
        </div>
      )}

      {isLocked && (
        <div className="mb-4 text-sm bg-amber-50 border border-amber-100 text-amber-800 rounded-lg px-4 py-3">
          本题受前置题目锁定，请先解出所有前置题。
        </div>
      )}

      <div className="mb-4 flex items-center justify-between gap-3">
        <h2 className="text-xl font-medium">{problem.title}</h2>
        {canEditProblem && (
          <Link
            to={`/problems/${problem.id}/edit`}
            className="inline-flex items-center rounded-lg border border-blue-200 bg-blue-50 px-4 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-100"
          >
            编辑题目
          </Link>
        )}
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-[minmax(0,1fr)_320px] gap-5">
        <section className="space-y-5">
          <div className="prose max-w-none text-gray-700 bg-white rounded-xl shadow-sm p-5">
            <h3>题目描述</h3>
            <p>{problem.description || '暂无题目描述。'}</p>
          </div>

          <div className="bg-white p-5 rounded-xl shadow-sm space-y-4">
            <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
              <div>
                <div className="text-sm text-gray-500">分类</div>
                <div className="font-medium">{categoryLabel(problem.category)}</div>
              </div>
              <div>
                <div className="text-sm text-gray-500">题目类型</div>
                <div className="font-medium">
                  {problem.type === 'static' ? '静态题' : '容器题'}
                </div>
              </div>
              <div>
                <div className="text-sm text-gray-500">分值</div>
                <div className="font-medium">{problem.points}</div>
              </div>
              <div>
                <div className="text-sm text-gray-500">难度</div>
                <div className="font-medium">
                  {problem.difficulty} ({difficultyLabel(problem.difficulty)})
                </div>
              </div>
              <div>
                <div className="text-sm text-gray-500">状态</div>
                <div className="font-medium">{problem.is_solved ? '已解出' : '未解出'}</div>
              </div>
              <div>
                <div className="text-sm text-gray-500">解题人数</div>
                <div className="font-medium">{problem.solves_count ?? problem.solve_count ?? 0}</div>
              </div>
              <div className="col-span-2 md:col-span-1">
                <div className="text-sm text-gray-500 mb-1">标签</div>
                {problem.tags.length === 0 ? (
                  <div className="text-sm text-gray-400">暂无标签</div>
                ) : (
                  <div className="flex flex-wrap gap-2">
                    {problem.tags.map((tag) => (
                      <span
                        key={tag}
                        className="text-xs px-2 py-1 rounded-full bg-slate-100 text-slate-700 border border-slate-200 font-medium"
                      >
                        # {tag}
                      </span>
                    ))}
                  </div>
                )}
              </div>
            </div>

            <div className="pt-3 border-t border-gray-100">
              <label className="block text-sm font-medium text-gray-700">提交 Flag</label>
              <div className="mt-2 flex gap-2 items-center">
                <input
                  value={flag}
                  onChange={(e) => setFlag(e.target.value)}
                  className="flex-1 min-w-0 border rounded px-3 py-2 disabled:bg-gray-100"
                  placeholder="FLAG{...}"
                  disabled={isLocked}
                />
                <button
                  onClick={submit}
                  disabled={isLocked}
                  className="bg-blue-600 text-white px-4 py-2 rounded whitespace-nowrap flex-shrink-0 hover:bg-blue-700 disabled:opacity-60"
                >
                  提交
                </button>
              </div>
              {msg && <div className="mt-2 text-sm text-gray-700">{msg}</div>}
            </div>
          </div>

          {problem.prerequisites && problem.prerequisites.length > 0 && (
            <PrerequisitesSection items={problem.prerequisites} />
          )}

          {hints.length > 0 && (
            <HintSection hints={hints} onUnlock={handleUnlockHint} pending={pendingHint} />
          )}

          {problem.attachment && (
            <div className="bg-white p-4 rounded-xl shadow-sm">
              <h4 className="font-medium mb-3">题目附件</h4>
              <div className="flex items-center justify-between gap-3 p-3 rounded-lg border border-gray-200 bg-gray-50">
                <div className="min-w-0">
                  <div className="text-sm font-medium text-gray-800 truncate">
                    {problem.attachment.name}
                  </div>
                  {formatAttachmentSize(problem.attachment.size) && (
                    <div className="text-xs text-gray-500 mt-0.5">
                      {formatAttachmentSize(problem.attachment.size)}
                    </div>
                  )}
                  {problem.attachment.sha256 && (
                    <div className="text-[11px] font-mono text-gray-400 mt-0.5 truncate">
                      SHA256: {problem.attachment.sha256.slice(0, 16)}…
                    </div>
                  )}
                </div>
                <a
                  href={problem.attachment.url}
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex items-center px-3 py-1.5 rounded-md bg-blue-600 text-white text-sm hover:bg-blue-700 whitespace-nowrap"
                >
                  下载
                </a>
              </div>
            </div>
          )}

          {isContainerProblem && (
            <div className="bg-white p-4 rounded-xl shadow-sm border border-blue-100">
              <div className="mb-2 flex items-center justify-between">
                <h4 className="font-medium text-gray-900">容器实例</h4>
                <span className="text-xs px-2 py-0.5 rounded-full bg-blue-50 text-blue-700 border border-blue-200">
                  k3s
                </span>
              </div>

              {instanceState.status === 'idle' && (
                <p className="text-sm text-gray-500">
                  当前无实例，点击下方按钮启动容器实例。
                </p>
              )}
              {instanceState.status === 'starting' && (
                <p className="text-sm text-gray-500">
                  正在启动容器实例（Pod -&gt; Service -&gt; Ingress）...
                </p>
              )}
              {instanceState.status === 'stopping' && (
                <p className="text-sm text-gray-500">
                  正在销毁容器实例（Ingress -&gt; Service -&gt; Pod）...
                </p>
              )}
              {instanceState.status === 'running' && (
                <div className="space-y-2">
                  <div className="text-xs text-gray-500">连接信息</div>
                  {problem.connection_info?.http && (
                    <a
                      href={problem.connection_info.http}
                      target="_blank"
                      rel="noreferrer"
                      className="block px-3 py-2 rounded-lg bg-blue-50 border border-blue-100 text-sm font-mono text-blue-700 hover:bg-blue-100 break-all"
                    >
                      HTTP: {problem.connection_info.http}
                    </a>
                  )}
                  {problem.connection_info?.tcp && (
                    <div className="px-3 py-2 rounded-lg bg-gray-50 border border-gray-200 text-sm font-mono break-all">
                      TCP: {problem.connection_info.tcp}
                    </div>
                  )}
                  {!problem.connection_info?.http && !problem.connection_info?.tcp && instanceState.endpoint && (
                    <div className="px-3 py-2 rounded-lg bg-gray-50 border border-gray-200 text-sm font-mono break-all">
                      {instanceState.endpoint}
                    </div>
                  )}
                  {                    (countdown !== null && countdown > 0) && (
                      <div className="text-xs text-gray-500">
                        剩余时间：{formatRemaining(countdown)}
                      </div>
                    )}
                  <div className="text-xs text-gray-500">{instanceState.note}</div>
                </div>
              )}
              {instanceState.status === 'failed' && (
                <div className="text-sm text-red-600 bg-red-50 border border-red-100 rounded-lg px-3 py-2">
                  {instanceState.note || '实例状态异常，请重试。'}
                </div>
              )}

              <div className="mt-3 grid grid-cols-1 sm:grid-cols-2 gap-2">
                <button
                  onClick={startRealInstance}
                  disabled={
                    instanceState.status === 'starting' || instanceState.status === 'stopping'
                  }
                  className="w-full bg-blue-600 text-white px-4 py-2 rounded hover:bg-blue-700 disabled:opacity-60"
                >
                  {instanceState.status === 'starting'
                    ? '启动中...'
                    : instanceState.status === 'running'
                      ? '续期/重启实例'
                      : '启动容器实例'}
                </button>
                <button
                  onClick={destroyRealInstance}
                  disabled={
                    instanceState.status === 'idle' ||
                    instanceState.status === 'starting' ||
                    instanceState.status === 'stopping'
                  }
                  className="w-full bg-gray-700 text-white px-4 py-2 rounded hover:bg-gray-800 disabled:opacity-60"
                >
                  {instanceState.status === 'stopping' ? '销毁中...' : '销毁实例'}
                </button>
              </div>
            </div>
          )}
        </section>

        <aside className="xl:sticky xl:top-24 self-start">
          <div className="bg-white p-4 rounded-xl shadow-sm">
            <h4 className="font-medium mb-2">历史提交（最近 20 条）</h4>
            {submissionsLoading ? (
              <div className="text-sm text-gray-500">加载提交记录中...</div>
            ) : submissions.length === 0 ? (
              <div className="text-sm text-gray-500">尚无提交记录</div>
            ) : (
              <ul className="space-y-2 text-sm">
                {submissions.slice(0, 20).map((s, idx) => (
                  <li
                    key={idx}
                    className={`px-3 py-2 rounded ${s.correct ? 'bg-green-50' : 'bg-red-50'}`}
                  >
                    <div className="flex justify-between gap-3">
                      <div className="truncate">{s.flag}</div>
                      <div className="text-xs text-gray-500 whitespace-nowrap">
                        {new Date(s.time).toLocaleString()}
                      </div>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </aside>
      </div>
    </div>
  )
}
