import React from 'react'

interface ContestHistoryProps {
  embedded?: boolean
}

export default function ContestHistory({ embedded = false }: ContestHistoryProps) {
  const mockContests = [
    { id: 1, name: '第五届全国大学生信息安全竞赛', date: '2025-10-20', rank: 3, score: 2450, total_teams: 120, award: '一等奖' },
    { id: 2, name: 'DEFCON CTF Qualifier 2025', date: '2025-05-15', rank: 45, score: 1200, total_teams: 500, award: '优胜奖' },
    { id: 3, name: '校校联合网络攻防演练', date: '2024-12-01', rank: 1, score: 3600, total_teams: 50, award: '特等奖' },
  ]

  return (
    <div className={embedded ? 'space-y-6' : 'mt-8 border-t border-slate-100 pt-8'}>
      {!embedded && (
        <h3 className="flex items-center gap-2 text-lg font-semibold tracking-tight text-slate-950">
          <svg className="w-5 h-5 text-sky-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 002-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
          </svg>
          参赛历史与战绩
        </h3>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {mockContests.map(contest => (
          <div key={contest.id} className="relative overflow-hidden rounded-[24px] border border-slate-200/80 bg-white p-5 shadow-[0_20px_60px_-45px_rgba(15,23,42,0.45)] transition hover:-translate-y-0.5 hover:border-slate-300">
            {contest.rank <= 3 && (
              <div className="absolute right-0 top-0 rounded-bl-2xl bg-amber-300 px-3 py-1 text-[10px] font-black uppercase tracking-[0.18em] text-amber-900 shadow-sm">
                Top 3
              </div>
            )}
            <div className="space-y-5">
              <div>
                <p className="text-xs font-semibold uppercase tracking-[0.22em] text-slate-400">{contest.date}</p>
                <h4 className="mt-2 pr-8 text-lg font-semibold tracking-tight text-slate-950" title={contest.name}>
                  {contest.name}
                </h4>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="rounded-2xl bg-slate-50 px-4 py-3">
                  <p className="text-xs uppercase tracking-[0.22em] text-slate-400">排名</p>
                  <p className="mt-1 text-2xl font-semibold text-sky-700">#{contest.rank}</p>
                  <p className="mt-1 text-xs text-slate-500">共 {contest.total_teams} 支队伍</p>
                </div>
                <div className="rounded-2xl bg-slate-50 px-4 py-3 text-right">
                  <p className="text-xs uppercase tracking-[0.22em] text-slate-400">积分</p>
                  <p className="mt-1 text-2xl font-semibold text-slate-950">{contest.score}</p>
                  <p className="mt-1 text-xs text-slate-500">points</p>
                </div>
              </div>

              <div className="flex items-center justify-between">
                <span className={`inline-flex rounded-full px-3 py-1 text-xs font-semibold ${
                  contest.award.includes('一等') || contest.award.includes('特等')
                    ? 'bg-rose-50 text-rose-600'
                    : contest.award.includes('二等')
                    ? 'bg-orange-50 text-orange-600'
                    : 'bg-sky-50 text-sky-600'
                }`}>
                  {contest.award}
                </span>
                <span className="text-sm font-medium text-slate-500">演示数据</span>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
