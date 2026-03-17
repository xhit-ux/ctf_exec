import React, { useMemo } from 'react'

type DayActivity = {
  date: Date
  count: number
  level: number
}

type GridCell = {
  row: number
  column: number
  day: DayActivity | null
}

export default function SubmissionHeatmap() {
  const weeks = 53
  const daysPerWeek = 7
  const cellSize = 14
  const cellGap = 3
  const columnWidth = cellSize + cellGap

  const getLevel = (count: number) => {
    if (count >= 8) return 4
    if (count >= 5) return 3
    if (count >= 3) return 2
    if (count >= 1) return 1
    return 0
  }

  const generateMockCount = (seed: number) => {
    const rand = (Math.sin(seed * 12.9898) * 43758.5453) % 1
    const normalized = Math.abs(rand)
    if (normalized < 0.62) return 0
    if (normalized < 0.8) return (seed % 2) + 1
    if (normalized < 0.92) return (seed % 3) + 3
    return (seed % 5) + 6
  }

  const toMondayIndex = (day: number) => (day + 6) % 7

  const addDays = (date: Date, days: number) => {
    const next = new Date(date)
    next.setDate(next.getDate() + days)
    return next
  }

  const startOfWeekMonday = (date: Date) => {
    const d = new Date(date)
    d.setHours(0, 0, 0, 0)
    d.setDate(d.getDate() - toMondayIndex(d.getDay()))
    return d
  }

  const oneYearGrid = useMemo(() => {
    const today = new Date()
    today.setHours(0, 0, 0, 0)

    const oneYearAgo = new Date(today)
    oneYearAgo.setFullYear(oneYearAgo.getFullYear() - 1)
    oneYearAgo.setDate(oneYearAgo.getDate() + 1)

    const gridStart = startOfWeekMonday(oneYearAgo)

    const cells: GridCell[][] = Array.from({ length: weeks }, (_, column) =>
      Array.from({ length: daysPerWeek }, (_, row) => {
        const date = addDays(gridStart, column * daysPerWeek + row)

        if (date < oneYearAgo || date > today) {
          return { row, column, day: null }
        }

        const daySeed = Number(
          `${date.getFullYear()}${String(date.getMonth() + 1).padStart(2, '0')}${String(
            date.getDate()
          ).padStart(2, '0')}`
        )
        const count = generateMockCount(daySeed)

        return {
          row,
          column,
          day: {
            date,
            count,
            level: getLevel(count)
          }
        }
      })
    )

    const effectiveWeeks = Math.floor((today.getTime() - gridStart.getTime()) / (1000 * 60 * 60 * 24 * 7)) + 1

    return {
      cells: cells.slice(0, effectiveWeeks),
      oneYearAgo,
      today
    }
  }, [])

  const weekMonthLabels = useMemo(() => {
    return oneYearGrid.cells.map((week, columnIndex) => {
      const validDays = week.map((cell) => cell.day).filter((day): day is DayActivity => Boolean(day))
      const monthStartDay = validDays.find((day) => day.date.getDate() === 1)
      if (columnIndex === 0 && validDays[0]) {
        return `${validDays[0].date.getMonth() + 1}月`
      }
      if (monthStartDay) {
        return `${monthStartDay.date.getMonth() + 1}月`
      }
      return ''
    })
  }, [oneYearGrid.cells])

  const monthMarkers = useMemo(
    () =>
      weekMonthLabels
        .map((label, index) => (label ? { label, index } : null))
        .filter((marker): marker is { label: string; index: number } => marker !== null),
    [weekMonthLabels]
  )

  const gridPixelWidth = Math.max(oneYearGrid.cells.length * columnWidth - cellGap, 0)
  const flatCells = useMemo(() => oneYearGrid.cells.flat(), [oneYearGrid.cells])

  const getColorClass = (level: number) => {
    switch (level) {
      case 1:
        return 'bg-[#9be9a8]'
      case 2:
        return 'bg-[#40c463]'
      case 3:
        return 'bg-[#30a14e]'
      case 4:
        return 'bg-[#216e39]'
      default:
        return 'bg-[#ebedf0]'
    }
  }

  const getWeekday = (date: Date) => {
    const weekday = ['周一', '周二', '周三', '周四', '周五', '周六', '周日']
    return weekday[toMondayIndex(date.getDay())]
  }

  const formatTooltip = (day: DayActivity) => {
    const month = day.date.getMonth() + 1
    const date = day.date.getDate()
    if (day.count > 0) {
      return `${month}月${date}日 ${getWeekday(day.date)}，${day.count} 次提交`
    }
    return `${month}月${date}日 ${getWeekday(day.date)}，无提交`
  }

  return (
    <div className="w-full">
      <h3 className="text-lg font-bold text-gray-800 mb-4 flex items-center gap-2">
        <svg className="w-5 h-5 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
        </svg>
        活跃度 (最近一年)
      </h3>

      <div className="bg-[#f6f8fa] border border-[#d0d7de] p-5 rounded-lg overflow-x-auto">
        <div className="min-w-max">
          <div className="mb-2 pl-10">
            <div className="relative h-5" style={{ width: `${gridPixelWidth}px` }}>
              {monthMarkers.map((marker) => (
                <span
                  key={`${marker.label}-${marker.index}`}
                  className="absolute top-0 text-[11px] leading-none text-gray-500 whitespace-nowrap"
                  style={{ left: `${marker.index * columnWidth}px` }}
                >
                  {marker.label}
                </span>
              ))}
            </div>
          </div>

          <div className="flex items-start gap-2">
            <div className="flex w-8 flex-col gap-[3px] text-xs text-gray-500">
              {['Mon', '', 'Wed', '', 'Fri', '', ''].map((label, idx) => (
                <div key={`${label}-${idx}`} className="h-[14px] leading-[14px]">
                  {label}
                </div>
              ))}
            </div>

            <div
              className="grid"
              style={{
                gridTemplateRows: `repeat(${daysPerWeek}, ${cellSize}px)`,
                gridTemplateColumns: `repeat(${oneYearGrid.cells.length}, ${cellSize}px)`,
                rowGap: `${cellGap}px`,
                columnGap: `${cellGap}px`
              }}
            >
              {flatCells.map((cell) => {
                if (!cell.day) {
                  return (
                    <div
                      key={`${cell.column}-${cell.row}`}
                      className="w-[14px] h-[14px]"
                      style={{
                        gridColumnStart: cell.column + 1,
                        gridRowStart: cell.row + 1
                      }}
                    ></div>
                  )
                }

                const day = cell.day

                return (
                  <div
                    key={`${cell.column}-${cell.row}`}
                    className={`w-[14px] h-[14px] rounded-[2px] ${getColorClass(day.level)} ${
                      day.level > 0
                        ? 'cursor-pointer hover:ring-2 hover:ring-gray-400 hover:ring-offset-1 hover:ring-offset-[#f6f8fa]'
                        : 'cursor-default'
                    } transition-all`}
                    style={{
                      gridColumnStart: cell.column + 1,
                      gridRowStart: cell.row + 1
                    }}
                    title={day.count > 0 ? formatTooltip(day) : undefined}
                  ></div>
                )
              })}
            </div>
          </div>

          <div className="flex items-center justify-start gap-2 mt-4 text-xs text-gray-600 pl-10">
            <span>少</span>
            <div className={`w-3 h-3 rounded-[2px] ${getColorClass(0)}`}></div>
            <div className={`w-3 h-3 rounded-[2px] ${getColorClass(1)}`}></div>
            <div className={`w-3 h-3 rounded-[2px] ${getColorClass(2)}`}></div>
            <div className={`w-3 h-3 rounded-[2px] ${getColorClass(3)}`}></div>
            <div className={`w-3 h-3 rounded-[2px] ${getColorClass(4)}`}></div>
            <span>多</span>
          </div>
        </div>
      </div>
    </div>
  )
}
