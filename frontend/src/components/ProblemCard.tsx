import React from 'react'
import { Problem } from '../types'

export default function ProblemCard({ p }: { p: Problem }) {
  return (
    <div className={`block p-4 bg-white border rounded shadow-sm hover:shadow-md h-full relative overflow-hidden ${p.is_solved ? 'border-green-200' : ''}`}>
      {p.is_solved && (
        <div className="absolute inset-0 bg-green-500/10 pointer-events-none z-0"></div>
      )}
      <div className="flex items-start justify-between gap-4 relative z-10">
        <div className="flex-1">
          <div className="flex items-center gap-2">
            <div className="font-medium text-gray-900 text-lg">{p.title}</div>
            {p.is_solved && (
              <span className="flex items-center justify-center w-5 h-5 bg-green-500 rounded-full text-white">
                <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="3" d="M5 13l4 4L19 7" />
                </svg>
              </span>
            )}
          </div>
          <div className="text-sm text-gray-500 mt-1">{p.category.toUpperCase()} · {p.difficulty}</div>
          <div className="mt-3">
            <div className="flex flex-col gap-2">
              {(() => {
                const mid = Math.ceil(p.tags.length / 2)
                const rows = [p.tags.slice(0, mid), p.tags.slice(mid)]
                return rows.map((row, idx) => (
                  <div key={idx} className="flex gap-2">
                    {row.map((t) => (
                      <span key={t} className="text-xs px-2 py-1 rounded text-gray-600 bg-gray-100 whitespace-nowrap flex-shrink-0">{t}</span>
                    ))}
                  </div>
                ))
              })()}
            </div>
          </div>
        </div>
        <div className="flex flex-col items-end">
          <div className={`text-sm text-white px-3 py-1 rounded ${p.is_solved ? 'bg-green-600' : 'bg-blue-600'}`}>{p.points}</div>
        </div>
      </div>
    </div>
  )
}
