import React from 'react'
import { Link } from 'react-router-dom'

export default function Index() {
  return (
    <section className="space-y-8 stagger-fade">
      <div className="section-card p-7 sm:p-9">
        <p className="text-xs font-semibold tracking-[0.18em] text-amber-700 uppercase">CTF Platform</p>
        <h1 className="mt-3 text-4xl md:text-6xl tracking-tight font-extrabold text-slate-900">真实训练节奏，稳定题目环境</h1>
        <p className="mt-4 text-slate-600 max-w-3xl leading-relaxed">
          统一题目管理、赛事组织与队伍协作入口。你可以在同一个平台完成日常练习、参与比赛、查看排行榜，并跟踪自己的解题成长路径。
        </p>
        <div className="mt-7 flex flex-wrap items-center gap-3">
          <Link to="/problem" className="btn-primary">进入题目大厅</Link>
          <Link to="/contests" className="btn-secondary">查看赛事</Link>
          <Link to="/scoreboard" className="btn-secondary">排行榜</Link>
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        <article className="soft-card p-5">
          <h2 className="text-lg font-semibold">练习模式</h2>
          <p className="mt-2 text-sm leading-relaxed text-slate-600">按分类和难度筛选题目，持续刷题并沉淀解题经验。</p>
        </article>
        <article className="soft-card p-5">
          <h2 className="text-lg font-semibold">赛事模式</h2>
          <p className="mt-2 text-sm leading-relaxed text-slate-600">从开赛前预览到赛中榜单与提交追踪，信息透明可追溯。</p>
        </article>
        <article className="soft-card p-5">
          <h2 className="text-lg font-semibold">团队协作</h2>
          <p className="mt-2 text-sm leading-relaxed text-slate-600">通过队伍维度聚合成员表现，清晰掌握整体攻防能力。</p>
        </article>
      </div>
    </section>
  )
}
