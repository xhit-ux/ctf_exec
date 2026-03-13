import React from 'react'

export default function ContestHistory() {
  const mockContests = [
    { id: 1, name: '第五届全国大学生信息安全竞赛', date: '2025-10-20', rank: 3, score: 2450, total_teams: 120, award: '一等奖' },
    { id: 2, name: 'DEFCON CTF Qualifier 2025', date: '2025-05-15', rank: 45, score: 1200, total_teams: 500, award: '优胜奖' },
    { id: 3, name: '校校联合网络攻防演练', date: '2024-12-01', rank: 1, score: 3600, total_teams: 50, award: '特等奖' },
  ]

  return (
    <div className="mt-8 border-t border-gray-100 pt-8">
      <h3 className="text-lg font-bold text-gray-800 mb-6 flex items-center gap-2">
        <svg className="w-5 h-5 text-indigo-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 002-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
        </svg>
        参赛历史与战绩 (演示数据)
      </h3>
      
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {mockContests.map(contest => (
          <div key={contest.id} className="bg-white border border-gray-100 rounded-xl p-5 shadow-sm hover:shadow-md transition-shadow relative overflow-hidden group">
            {contest.rank <= 3 && (
               <div className="absolute top-0 right-0 bg-yellow-400 text-yellow-900 text-[10px] font-bold px-3 py-1 rounded-bl-lg z-10 shadow-sm">
                 🏆 TOP 3
               </div>
            )}
            <h4 className="font-bold text-gray-900 mb-1 truncate pr-8" title={contest.name}>{contest.name}</h4>
            <p className="text-xs text-gray-500 mb-4">{contest.date}</p>
            
            <div className="flex items-center justify-between mt-auto">
               <div>
                  <p className="text-2xl font-black text-indigo-600">#{contest.rank}</p>
                  <p className="text-[10px] text-gray-400 font-medium uppercase">/ {contest.total_teams} 参赛队伍</p>
               </div>
               <div className="text-right">
                  <span className={`inline-block px-2.5 py-1 rounded-md text-xs font-bold ${
                    contest.award.includes('一等') || contest.award.includes('特等') ? 'bg-red-50 text-red-600' :
                    contest.award.includes('二等') ? 'bg-orange-50 text-orange-600' : 'bg-blue-50 text-blue-600'
                  }`}>
                    {contest.award}
                  </span>
                  <p className="text-xs font-bold text-gray-700 mt-1">{contest.score} pt</p>
               </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
