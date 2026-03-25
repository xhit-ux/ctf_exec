import React, { useEffect, useState } from 'react'


import { useAuth } from '../context/AuthContext'
import { Submission } from '../types'
import { apiUrl } from '../utils/api'
import { apiFetch } from '../utils/authClient'

export default function MySubmissions() {
  const [subs, setSubs] = useState<Submission[]>([])
  const { token, isAuthenticated } = useAuth()
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!isAuthenticated || !token) {
      setLoading(false)
      return
    }

    const fetchSubmissions = async () => {
      try {
        const res = await apiFetch(apiUrl('/api/submissions/me'))
        if (res.ok) {
          const data = await res.json()
          // 后端返回的字段名可能与前端 Submission 类型不同，需要映射
          // Backend: id, user_id, problem_id, flag, is_correct, created_at
          // Frontend Type: problemId, flag, correct, time
          const mapped = data.map((item: any) => ({
            problemId: item.problem_id,
            problemTitle: item.problem_title,
            flag: item.flag,
            correct: item.is_correct,
            time: item.created_at
          }))
          setSubs(mapped)
        }
      } catch (err) {
        console.error('Failed to fetch submissions', err)
      } finally {
        setLoading(false)
      }
    }
    fetchSubmissions()
  }, [isAuthenticated, token])

  if (!isAuthenticated) return <div className="p-10 text-center font-medium text-gray-600 bg-white rounded-xl shadow-sm border border-gray-100 max-w-4xl mx-auto w-full">请先登录</div>
  if (loading) return <div className="p-10 text-center font-medium text-gray-600">加载中...</div>

  return (
    <div className="max-w-6xl mx-auto w-full">
      <div className="mb-6 flex items-baseline gap-4">
        <h2 className="text-3xl font-extrabold text-gray-900 tracking-tight">我的提交</h2>
        <p className="text-gray-500 text-sm font-medium">共 {subs.length} 条记录</p>
      </div>

      {subs.length === 0 ? (
        <div className="text-center py-20 bg-white rounded-2xl border border-gray-100 shadow-sm">
          <div className="text-gray-300 mb-3">
            <svg className="w-16 h-16 mx-auto" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
            </svg>
          </div>
          <p className="text-gray-500 text-lg font-medium">尚无提交记录，快去刷题吧！</p>
        </div>
      ) : (
        <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-left min-w-[600px]">
            <thead className="bg-gray-50/50 text-xs font-semibold text-gray-500 uppercase tracking-wider">
              <tr>
                <th className="px-6 py-4 border-b border-gray-100">题目</th>
                <th className="px-6 py-4 border-b border-gray-100">提交详情 (Flag)</th>
                <th className="px-6 py-4 border-b border-gray-100">判题结果</th>
                <th className="px-6 py-4 border-b border-gray-100">提交时间</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {subs.map((s, idx) => {
                return (
                  <tr key={idx} className={`hover:bg-gray-50 transition-colors ${s.correct ? 'bg-green-50/20' : 'bg-red-50/10'}`}>
                    <td className="px-6 py-4 whitespace-nowrap font-medium text-gray-900">{s.problemTitle || `#${s.problemId}`}</td>
                    <td className="px-6 py-4 max-w-xs truncate text-gray-600 font-mono text-sm">{s.flag}</td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      {s.correct ? (
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
                          <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                           <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                          </svg>
                          Correct
                        </span>
                      ) : (
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800">
                          <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                           <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                          </svg>
                          Incorrect
                        </span>
                      )}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">{new Date(s.time).toLocaleString()}</td>
                  </tr>
                )
              })}
            </tbody>
          </table>
          </div>
        </div>
      )}
    </div>
  )
}
