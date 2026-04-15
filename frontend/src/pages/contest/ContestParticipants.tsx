import React, { useEffect, useMemo, useState } from 'react'
import { fromUnknownError } from '../../utils/error'
import { ContestParticipantItem, fetchContestParticipants } from '../../utils/contestApi'
import { TAB_ACCENT_PALETTE, randomTabAccent, tabAccentStyle } from '../../utils/tabAccent'
import { useContestContext } from './ContestLayout'

function formatTime(input?: string): string {
  if (!input) return '-'
  const date = new Date(input)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('zh-CN', { hour12: false })
}

export default function ContestParticipants() {
  const { contest, isParticipant, isAdmin } = useContestContext()

  const [rows, setRows] = useState<ContestParticipantItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [kind, setKind] = useState<'all' | 'user' | 'team'>('all')
  const [tabAccent, setTabAccent] = useState(TAB_ACCENT_PALETTE[0])

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
        const data = await fetchContestParticipants(contest.id, 1, 200)
        if (active) setRows(data.items)
      } catch (err) {
        if (active) setError(fromUnknownError(err, '获取参赛者失败'))
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
    if (kind === 'all') return rows
    if (kind === 'team') return rows.filter((item) => !!item.team_id)
    return rows.filter((item) => !!item.user_id)
  }, [kind, rows])

  const handleKindChange = (nextKind: 'all' | 'user' | 'team') => {
    setKind(nextKind)
    setTabAccent((prev) => randomTabAccent(prev))
  }

  return (
    <section className="glass-card p-6 space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-xl font-bold text-gray-900">参赛者列表</h2>
        <div className="inline-flex rounded-lg glass-soft p-1 gap-1">
          <button type="button" onClick={() => handleKindChange('all')} className={`tab-pill ${kind === 'all' ? 'tab-pill--active' : ''}`} style={kind === 'all' ? tabAccentStyle(tabAccent) : undefined}>全部</button>
          <button type="button" onClick={() => handleKindChange('user')} className={`tab-pill ${kind === 'user' ? 'tab-pill--active' : ''}`} style={kind === 'user' ? tabAccentStyle(tabAccent) : undefined}>个人</button>
          <button type="button" onClick={() => handleKindChange('team')} className={`tab-pill ${kind === 'team' ? 'tab-pill--active' : ''}`} style={kind === 'team' ? tabAccentStyle(tabAccent) : undefined}>团队</button>
        </div>
      </div>

      {(!isParticipant && !isAdmin) ? (
        <div className="rounded-lg border border-amber-100 bg-amber-50 px-3 py-2 text-sm text-amber-700">报名后可查看参赛者列表。</div>
      ) : loading ? (
        <div className="text-sm text-gray-500">参赛者加载中...</div>
      ) : error ? (
        <div className="rounded-lg border border-red-100 bg-red-50 px-3 py-2 text-sm text-red-600">{error}</div>
      ) : filteredRows.length === 0 ? (
        <div className="text-sm text-gray-500">暂无参赛者</div>
      ) : (
        <div className="glass-table-wrap overflow-x-auto">
          <table className="w-full min-w-[760px] text-sm">
            <thead>
              <tr className="border-b border-gray-100 text-gray-500">
                <th className="px-3 py-3 text-left">对象</th>
                <th className="px-3 py-3 text-left">类型</th>
                <th className="px-3 py-3 text-center">状态</th>
                <th className="px-3 py-3 text-right">报名时间</th>
              </tr>
            </thead>
            <tbody>
              {filteredRows.map((row) => (
                <tr key={row.id} className="border-b border-gray-100 last:border-b-0">
                  <td className="px-3 py-3 font-semibold text-gray-800">{row.team_name || row.display_name || row.username || row.team_id || row.user_id || '-'}</td>
                  <td className="px-3 py-3 text-gray-600">{row.team_id ? '团队' : '个人'}</td>
                  <td className="px-3 py-3 text-center"><span className="rounded-full bg-amber-100 px-2 py-1 text-xs text-amber-700">{row.status}</span></td>
                  <td className="px-3 py-3 text-right text-gray-500">{formatTime(row.registered_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  )
}
