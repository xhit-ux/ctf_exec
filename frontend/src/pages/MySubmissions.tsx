import React, { useEffect, useState } from 'react'


import { useAuth } from '../context/AuthContext'
import { Submission } from '../types'

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
        const res = await fetch('http://localhost:8080/api/submissions/me', {
          headers: {
            'Authorization': `Bearer ${token}`
          }
        })
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

  if (!isAuthenticated) return <div className="p-4 text-center text-gray-500">请先登录</div>
  if (loading) return <div className="p-4 text-center text-gray-500">加载中...</div>

  return (
    <div>
      <h2 className="text-xl font-medium mb-4">我的提交</h2>

      {subs.length === 0 ? (
        <div className="text-sm text-gray-500">尚无提交记录</div>
      ) : (
        <div className="bg-white p-4 rounded shadow-sm">
          <table className="w-full text-sm">
            <thead className="text-left text-xs text-gray-500">
              <tr>
                <th>题目</th>
                <th>Flag</th>
                <th>结果</th>
                <th>时间</th>
              </tr>
            </thead>
            <tbody>
              {subs.map((s, idx) => {
                return (
                  <tr key={idx} className={`${s.correct ? 'bg-green-50' : 'bg-red-50'}`}>
                    <td className="py-2">{s.problemTitle || `#${s.problemId}`}</td>
                    <td>{s.flag}</td>
                    <td>{s.correct ? '正确' : '错误'}</td>
                    <td className="text-xs text-gray-500">{new Date(s.time).toLocaleString()}</td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
