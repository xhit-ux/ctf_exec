import React, { useMemo, useState } from 'react'
import { TAB_ACCENT_PALETTE, randomTabAccent, tabAccentStyle } from '../../utils/tabAccent'

interface PaginationProps {
  total: number
  page: number
  pageSize: number
  onPageChange: (page: number) => void
  onPageSizeChange: (size: number) => void
}

export default function Pagination({
  total,
  page,
  pageSize,
  onPageChange,
  onPageSizeChange,
}: PaginationProps) {
  const [jumpInput, setJumpInput] = useState('')
  const [tabAccent, setTabAccent] = useState(TAB_ACCENT_PALETTE[0])
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const clampedPage = Math.min(Math.max(page, 1), totalPages)

  const pageSizeOptions = useMemo(() => {
    const base = [10, 20, 50, 100]
    if (!base.includes(pageSize)) {
      return [pageSize, ...base].sort((a, b) => a - b)
    }
    return base
  }, [pageSize])

  const pageItems = useMemo(() => {
    if (totalPages <= 1) {
      return [1]
    }

    const items: Array<number | 'ellipsis'> = []
    const start = Math.max(2, clampedPage - 1)
    const end = Math.min(totalPages - 1, clampedPage + 1)

    items.push(1)

    if (start > 2) {
      items.push('ellipsis')
    }

    for (let p = start; p <= end; p++) {
      items.push(p)
    }

    if (end < totalPages - 1) {
      items.push('ellipsis')
    }

    items.push(totalPages)
    return items
  }, [clampedPage, totalPages])

  const goToPage = (nextPage: number) => {
    const target = Math.min(Math.max(nextPage, 1), totalPages)
    if (target !== clampedPage) {
      onPageChange(target)
      setTabAccent((prev) => randomTabAccent(prev))
    }
  }

  const handleJump = () => {
    const parsed = Number(jumpInput)
    if (!Number.isFinite(parsed)) {
      return
    }
    const target = Math.trunc(parsed)
    const clamped = Math.min(Math.max(target, 1), totalPages)
    if (clamped !== clampedPage) {
      onPageChange(clamped)
      setTabAccent((prev) => randomTabAccent(prev))
    }
    setJumpInput('')
  }

  const handlePageSizeChange = (value: number) => {
    if (value === pageSize) {
      return
    }
    onPageSizeChange(value)
    onPageChange(1)
  }

  const btnBase = 'h-9 min-w-9 px-3 rounded-md border text-sm transition'
  const btnDisabled = 'opacity-50 pointer-events-none'

  return (
    <div className="flex flex-col gap-3 border-t bg-gray-50 px-4 py-3 md:flex-row md:items-center md:justify-between">
      <div className="flex items-center gap-2">
        <button
          type="button"
          aria-label="Previous page"
          onClick={() => goToPage(clampedPage - 1)}
          disabled={clampedPage <= 1}
          className={`${btnBase} bg-white hover:bg-gray-100 ${clampedPage <= 1 ? btnDisabled : ''}`}
        >
          上一页
        </button>

        <div className="hidden items-center gap-1 sm:flex">
          {pageItems.map((item, idx) => {
            if (item === 'ellipsis') {
              return (
                <span key={`ellipsis-${idx}`} className="px-1 text-sm text-gray-400">
                  ...
                </span>
              )
            }

            const active = item === clampedPage
            return (
              <button
                key={item}
                type="button"
                aria-current={active ? 'page' : undefined}
                onClick={() => goToPage(item)}
                className={`${btnBase} ${active ? 'tab-pill tab-pill--active border-transparent' : 'bg-white hover:bg-gray-100'}`}
                style={active ? tabAccentStyle(tabAccent) : undefined}
              >
                {item}
              </button>
            )
          })}
        </div>

        <div className="sm:hidden rounded-md border bg-white px-3 py-1.5 text-sm text-gray-600">
          {clampedPage} / {totalPages}
        </div>

        <button
          type="button"
          aria-label="Next page"
          onClick={() => goToPage(clampedPage + 1)}
          disabled={clampedPage >= totalPages}
          className={`${btnBase} bg-white hover:bg-gray-100 ${clampedPage >= totalPages ? btnDisabled : ''}`}
        >
          下一页
        </button>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <label className="text-sm text-gray-500" htmlFor="page-size-select">
          每页
        </label>
        <select
          id="page-size-select"
          value={pageSize}
          onChange={(e) => handlePageSizeChange(Number(e.target.value))}
          className="h-9 rounded-md border bg-white px-2 text-sm"
        >
          {pageSizeOptions.map((size) => (
            <option key={size} value={size}>
              {size}
            </option>
          ))}
        </select>

        <input
          type="text"
          inputMode="numeric"
          value={jumpInput}
          placeholder="页码"
          onChange={(e) => setJumpInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              handleJump()
            }
          }}
          className="h-9 w-16 rounded-md border bg-white px-2 text-sm"
        />
        <button
          type="button"
          onClick={handleJump}
          className="h-9 rounded-md border bg-white px-3 text-sm hover:bg-gray-100"
        >
          跳转
        </button>
      </div>
    </div>
  )
}
