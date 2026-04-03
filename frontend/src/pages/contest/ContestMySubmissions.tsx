import React, { useEffect, useMemo, useState } from 'react'
import { fromUnknownError } from '../../utils/error'
import { ContestSubmissionItem, fetchContestMySubmissions } from '../../utils/contestApi'
import { useContestContext } from './ContestLayout'

function formatTime(input?: string): string {
  if (!input) return '-'
  const date = new Date(input)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('zh-CN', { hour12: false })
}

export default function ContestMySubmissions() {
  const { contest, isParticipant, isAdmin } = useContestContext()
  const [rows, setRows] = useState<ContestSubmissionItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [resultFilter, setResultFilter] = useState<'all' | 'correct' | 'wrong'>('all')

  useEffect(() => {
    let active = true

    const run = async () => {
      if (!isParticipant && !isAdmin) {
        setRows([])
        setLoading(false)
        return
      }

      setLoading(true)
      setError('')
      try {
        const data = await fetchContestMySubmissions(contest.id, 1, 100)
        if (active) setRows(data.items)
      } catch (err) {
        if (active) setError(fromUnknownError(err, '获取我的提交失败'))
      } finally {
        if (active) setLoading(false)
      }
    }

    run()
    return () => {
      active = false
    }
  }, [contest.id, isAdmin, isParticipant])

  const filteredRows = useMemo(() => {
    if (resultFilter === 'all') return rows
    return rows.filter((item) => resultFilter === 'correct' ? item.is_correct : !item.is_correct)
  }, [resultFilter, rows])

  return (
    <section className="rounded-2xl border border-gray-100 bg-white p-6 shadow-sm space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-xl font-bold text-gray-900">我的提交</h2>
        <select value={resultFilter} onChange={(event) => setResultFilter(event.target.value as 'all' | 'correct' | 'wrong')} className="rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm">
          <option value="all">全部</option>
          <option value="correct">正确</option>
          <option value="wrong">错误</option>
        </select>
      </div>

      {(!isParticipant && !isAdmin) ? (
        <div className="rounded-lg border border-amber-100 bg-amber-50 px-3 py-2 text-sm text-amber-700">报名后可查看赛事提交记录。</div>
      ) : loading ? (
        <div className="text-sm text-gray-500">提交记录加载中...</div>
      ) : error ? (
        <div className="rounded-lg border border-red-100 bg-red-50 px-3 py-2 text-sm text-red-600">{error}</div>
      ) : filteredRows.length === 0 ? (
        <div className="text-sm text-gray-500">暂无提交记录</div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[760px] text-sm">
            <thead>
              <tr className="border-b border-gray-100 text-gray-500">
                <th className="px-3 py-3 text-left">时间</th>
                <th className="px-3 py-3 text-left">题目</th>
                <th className="px-3 py-3 text-center">结果</th>
                <th className="px-3 py-3 text-center">类型</th>
                <th className="px-3 py-3 text-right">IP</th>
              </tr>
            </thead>
            <tbody>
              {filteredRows.map((row) => (
                <tr key={row.id} className="border-b border-gray-100 last:border-b-0">
                  <td className="px-3 py-3 text-gray-600">{formatTime(row.created_at)}</td>
                  <td className="px-3 py-3 font-semibold text-gray-800">{row.problem_title || `#${row.problem_id}`}</td>
                  <td className="px-3 py-3 text-center">{row.is_correct ? <span className="rounded-full bg-green-100 px-2 py-1 text-xs text-green-700">正确</span> : <span className="rounded-full bg-red-100 px-2 py-1 text-xs text-red-700">错误</span>}</td>
                  <td className="px-3 py-3 text-center text-gray-600">{row.team_id ? '团队' : '个人'}</td>
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
