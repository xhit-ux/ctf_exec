import React, { useEffect, useMemo, useState } from 'react'
import { useParams } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { Problem, Submission } from '../types'

export default function ProblemDetail() {
  const { id } = useParams()
  const pid = Number(id)

  const [problem, setProblem] = useState<Problem | null>(null)
  const [flag, setFlag] = useState('')
  const [msg, setMsg] = useState<string | null>(null)
  const [submissions, setSubmissions] = useState<Submission[]>([])
  const { token, isAuthenticated } = useAuth()

  useEffect(() => {
    // 每次进入页面清空提交记录显式（或者是从 backend 拉取）
    setSubmissions([])
    setMsg(null)
    setFlag('')

    if (token) {
      fetch(`http://localhost:8080/api/problems/detail?id=${pid}`, {
        headers: { Authorization: `Bearer ${token}` }
      })
        .then(res => {
          if (res.ok) return res.json()
          throw new Error('Failed to fetch problem')
        })
        .then(data => setProblem(data))
        .catch(err => console.error(err))
    }
  }, [pid, token])

  if (!problem) return <div className="p-10 text-center">题目加载中或未找到...</div>

  const submit = async () => {
    if (!isAuthenticated) {
      setMsg('请先登录')
      return
    }

    try {
      const response = await fetch('http://localhost:8080/api/submissions', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({
          problem_id: problem.id,
          flag: flag,
          // Removed is_correct, backend validates it
        })
      })

      if (response.ok) {
        const result = await response.json()
        // result is the created submission from backend: { id, is_correct, ... }
        const isCorrect = result.is_correct

        const record: Submission = {
          problemId: problem.id,
          flag: flag,
          correct: isCorrect,
          time: new Date().toISOString()
        }

        setSubmissions((s) => [record, ...s])
        setMsg(isCorrect ? '提交成功，Flag 正确！' : '提交已记录，Flag 错误')
      } else {
        try {
          const errData = await response.json()
          let userFriendlyMsg = errData.error || errData.message || response.statusText

          // 根据状态码提供更友好的提示
          if (response.status === 409) {
            userFriendlyMsg = '您已经解决过这个题目，无需重复提交'
          } else if (response.status === 400) {
            userFriendlyMsg = errData.error || '请求参数错误'
          } else if (response.status === 404) {
            userFriendlyMsg = errData.error || '题目不存在'
          } else if (response.status === 429) {
            userFriendlyMsg = errData.error || '提交过于频繁，请稍后再试'
          }

          setMsg(`提交失败: ${userFriendlyMsg}`)
        } catch {
          setMsg(`提交失败: ${response.statusText || '未知错误'}`)
        }
      }
    } catch (err) {
      console.error('网络错误', err)
      setMsg('网络错误，请重试')
    }
  }

  return (
    <div>
      <h2 className="text-xl font-medium mb-4">{problem.title} · #{problem.id}</h2>

      <div className="grid grid-cols-1 lg:grid-cols-[1fr_320px] gap-6">
        <section>
          <div className="prose max-w-none text-gray-700">
            <h3>题目描述</h3>
            <p>{problem.description}</p>
            <pre className="bg-gray-100 p-3 rounded">示例附件或命令行输出</pre>
          </div>

          <div className="mt-6 bg-white p-4 rounded shadow-sm">
            <h4 className="font-medium mb-2">历史提交（最近 20 条）</h4>
            {submissions.length === 0 ? (
              <div className="text-sm text-gray-500">尚无提交记录</div>
            ) : (
              <ul className="space-y-2 text-sm">
                {submissions.filter((s) => s.problemId === problem.id).slice(0, 20).map((s, idx) => (
                  <li key={idx} className={`px-3 py-2 rounded ${s.correct ? 'bg-green-50' : 'bg-red-50'}`}>
                    <div className="flex justify-between">
                      <div className="truncate">{s.flag}</div>
                      <div className="text-xs text-gray-500">{new Date(s.time).toLocaleString()}</div>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </section>

        <aside>
          <div className="bg-white p-4 rounded shadow-sm">
            <div className="mb-3">
              <div className="text-sm text-gray-500">分类</div>
              <div className="font-medium">{problem.category.toUpperCase()}</div>
            </div>
            <div className="mb-3">
              <div className="text-sm text-gray-500">分值</div>
              <div className="font-medium">{problem.points}</div>
            </div>
            <div className="mb-3">
              <div className="text-sm text-gray-500">难度</div>
              <div className="font-medium">{problem.difficulty}</div>
            </div>

            <div className="mt-4">
              <label className="block text-sm font-medium text-gray-700">提交 Flag</label>
              <div className="mt-2 flex gap-2 items-center">
                <input
                  value={flag}
                  onChange={(e) => setFlag(e.target.value)}
                  className="flex-1 min-w-0 border rounded px-3 py-2"
                  placeholder="FLAG{...}"
                />
                <button onClick={submit} className="bg-blue-600 text-white px-4 py-2 rounded whitespace-nowrap flex-shrink-0">
                  提交
                </button>
              </div>
              {msg && <div className="mt-2 text-sm text-gray-700">{msg}</div>}
            </div>
          </div>
        </aside>
      </div>
    </div>
  )
}
