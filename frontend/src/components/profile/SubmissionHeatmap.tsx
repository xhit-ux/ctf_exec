import React from 'react'

export default function SubmissionHeatmap() {
  // 生成过去半年的 mock 数据（简化为 16 周 x 7 天的网格）
  const weeks = 24
  const daysPerWeek = 7
  
  // 随机生成色块级别 (0-4)
  const generateMockActivity = () => {
    const activity = []
    for (let w = 0; w < weeks; w++) {
      const week = []
      for (let d = 0; d < daysPerWeek; d++) {
        // 大多是没有提交的，部分有提交
        const rand = Math.random()
        let level = 0
        if (rand > 0.9) level = 4
        else if (rand > 0.8) level = 3
        else if (rand > 0.7) level = 2
        else if (rand > 0.6) level = 1
        
        week.push(level)
      }
      activity.push(week)
    }
    return activity
  }
  
  const heatmapData = generateMockActivity()
  
  const getColorClass = (level: number) => {
    switch (level) {
      case 1: return 'bg-green-200'
      case 2: return 'bg-green-400'
      case 3: return 'bg-green-600'
      case 4: return 'bg-green-800'
      default: return 'bg-gray-100'
    }
  }

  const months = ['九月', '十月', '十一月', '十二月', '一月', '二月', '三月']

  return (
    <div className="w-full">
      <h3 className="text-lg font-bold text-gray-800 mb-4 flex items-center gap-2">
        <svg className="w-5 h-5 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
        </svg>
        活跃度 (近半年)
      </h3>
      <div className="bg-white border text-center border-gray-100 p-6 rounded-xl shadow-sm overflow-x-auto">
        <div className="min-w-max">
          <div className="flex text-xs text-gray-400 mb-2 pl-6">
             {months.map((month, i) => (
                <div key={i} className="flex-1 min-w-[50px] text-left">{month}</div>
             ))}
          </div>
          <div className="flex gap-1.5">
            <div className="flex flex-col gap-1.5 text-xs text-gray-400 justify-around pr-2 py-1">
              <span>一</span>
              <span>三</span>
              <span>五</span>
              <span>日</span>
            </div>
            {heatmapData.map((week, wIndex) => (
              <div key={wIndex} className="flex flex-col gap-1.5">
                {week.map((level, dIndex) => (
                  <div
                    key={`${wIndex}-${dIndex}`}
                    className={`w-3.5 h-3.5 rounded-sm ${getColorClass(level)} hover:ring-2 hover:ring-gray-300 transition-all cursor-pointer`}
                    title={`${level > 0 ? level * 2 + ' 次提交' : '无提交'}`}
                  ></div>
                ))}
              </div>
            ))}
          </div>
          <div className="flex items-center justify-end gap-2 mt-4 text-xs text-gray-500">
            <span>少</span>
            <div className={`w-3 h-3 rounded-sm ${getColorClass(0)}`}></div>
            <div className={`w-3 h-3 rounded-sm ${getColorClass(1)}`}></div>
            <div className={`w-3 h-3 rounded-sm ${getColorClass(2)}`}></div>
            <div className={`w-3 h-3 rounded-sm ${getColorClass(3)}`}></div>
            <div className={`w-3 h-3 rounded-sm ${getColorClass(4)}`}></div>
            <span>多</span>
          </div>
        </div>
      </div>
    </div>
  )
}
