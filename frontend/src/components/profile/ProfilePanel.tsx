import React from 'react'

interface ProfilePanelProps {
  title: string
  description?: string
  action?: React.ReactNode
  children: React.ReactNode
  className?: string
  contentClassName?: string
}

export default function ProfilePanel({
  title,
  description,
  action,
  children,
  className = '',
  contentClassName = '',
}: ProfilePanelProps) {
  return (
    <section
      className={`overflow-hidden rounded-[28px] border border-slate-200/80 bg-white shadow-[0_20px_60px_-45px_rgba(15,23,42,0.45)] ${className}`}
    >
      <div className="flex flex-col gap-3 border-b border-slate-100 px-5 py-5 sm:px-6">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="space-y-1">
            <h2 className="text-lg font-semibold tracking-tight text-slate-950">{title}</h2>
            {description && <p className="text-sm leading-6 text-slate-500">{description}</p>}
          </div>
          {action && <div className="shrink-0">{action}</div>}
        </div>
      </div>

      <div className={`px-5 py-5 sm:px-6 sm:py-6 ${contentClassName}`}>{children}</div>
    </section>
  )
}
