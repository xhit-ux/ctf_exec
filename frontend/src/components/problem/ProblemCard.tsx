import React from 'react'
import { categoryLabel, ProblemView } from '../../utils/problemView'

export default function ProblemCard({ p }: { p: ProblemView }) {
  const tags = p.tags

  const difficultyColor = {
    '简单': 'bg-green-100 text-green-800',
    '中等': 'bg-yellow-100 text-yellow-800',
    '困难': 'bg-red-100 text-red-800',
    '专家': 'bg-purple-100 text-purple-800',
  }[p.difficulty] || 'bg-gray-100 text-gray-800'

  return (
    <div
      className={`flex flex-col h-full p-6 bg-white border-2 rounded-xl transition-all duration-300 hover:shadow-xl hover:-translate-y-1 cursor-pointer group overflow-hidden relative ${
        p.is_solved
          ? 'border-green-300 bg-gradient-to-br from-white to-green-50'
          : 'border-gray-200 hover:border-amber-300'
      }`}
    >
      {/* Solved overlay gradient */}
      {p.is_solved && (
        <div className="absolute inset-0 bg-gradient-to-r from-green-500/0 via-green-500/5 to-green-500/0 pointer-events-none" />
      )}

      {/* Header: Title and Solved Badge */}
      <div className="flex items-start justify-between gap-3 mb-3 relative z-10">
        <div className="flex-1">
          <h3 className="font-bold text-lg text-gray-900 group-hover:text-amber-700 transition-colors line-clamp-2">
            {p.title}
          </h3>
        </div>
        {p.is_solved && (
          <div className="flex-shrink-0">
            <div className="inline-flex items-center justify-center w-6 h-6 bg-green-500 rounded-full shadow-md">
              <svg className="w-4 h-4 text-white" fill="currentColor" viewBox="0 0 20 20">
                <path
                  fillRule="evenodd"
                  d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
                  clipRule="evenodd"
                />
              </svg>
            </div>
          </div>
        )}
      </div>

      {/* Category & Difficulty */}
      <div className="flex items-center gap-2 mb-4 relative z-10">
        <span className="text-xs font-medium text-gray-600 uppercase tracking-wider">
          {categoryLabel(p.category)}
        </span>
        <span
          className={`text-xs font-semibold px-2 py-1 rounded-full ${difficultyColor}`}
        >
          {p.difficulty}
        </span>
      </div>

      {/* Tags */}
      {tags.length > 0 && (
        <div className="mb-4 relative z-10">
          <div className="flex flex-wrap gap-1.5">
            {tags.slice(0, 3).map((t) => (
              <span
                key={t}
                className="text-xs px-2 py-1 rounded-full bg-amber-50 text-gray-700 border border-amber-100 font-medium"
              >
                #{t}
              </span>
            ))}
            {tags.length > 3 && (
              <span className="text-xs px-2 py-1 rounded-full bg-gray-100 text-gray-600 font-medium">
                +{tags.length - 3}
              </span>
            )}
          </div>
        </div>
      )}

      {/* Footer: Points */}
      <div className="mt-auto pt-3 relative z-10 flex items-center justify-between border-t border-gray-100">
        <div className={`text-xs font-semibold text-gray-500 uppercase tracking-wider ${p.is_solved ? 'text-green-600' : ''}`}>
          {p.is_solved ? '✓ 已解决' : '未解决'}
        </div>
        <div className={`text-base font-bold px-4 py-2 rounded-lg ${
          p.is_solved ? 'bg-green-100 text-green-700' : 'bg-amber-100 text-amber-800'
        }`}>
          {p.points}
        </div>
      </div>
    </div>
  )
}
