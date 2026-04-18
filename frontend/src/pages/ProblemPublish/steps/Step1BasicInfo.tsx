import React from 'react';
import { useProblemPublish } from '../ProblemPublishContext';
import { ProblemCategory } from '../types';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { IconChip, IconGlobe, IconKey, IconPuzzle, IconTerminal } from '../icons';

const CATEGORIES: { id: ProblemCategory; label: string; desc: string; icon: React.ReactNode; color: string }[] = [
  { id: 'Web', label: 'Web', desc: 'Web 类题目。', icon: <IconGlobe className="w-5 h-5" />, color: 'border-sky-500 bg-sky-50' },
  { id: 'Pwn', label: 'Pwn', desc: '二进制利用题目。', icon: <IconTerminal className="w-5 h-5" />, color: 'border-orange-500 bg-orange-50' },
  { id: 'Crypto', label: 'Crypto', desc: '密码学题目。', icon: <IconKey className="w-5 h-5" />, color: 'border-violet-500 bg-violet-50' },
  { id: 'Reverse', label: 'Reverse', desc: '逆向工程题目。', icon: <IconChip className="w-5 h-5" />, color: 'border-emerald-500 bg-emerald-50' },
  { id: 'Misc', label: 'Misc', desc: '杂项题目。', icon: <IconPuzzle className="w-5 h-5" />, color: 'border-amber-500 bg-amber-50' },
];

const FIXED_TAG_MAP: Record<ProblemCategory, string> = {
  Web: 'web',
  Pwn: 'pwn',
  Crypto: 'crypto',
  Reverse: 'reverse',
  Misc: 'misc',
};

const FIXED_TAGS = Object.values(FIXED_TAG_MAP);

export default function Step1BasicInfo() {
  const { state, updateState, nextStep } = useProblemPublish();
  const fixedTag = state.category ? FIXED_TAG_MAP[state.category] : '';
  const customTags = state.tags.filter((tag) => !FIXED_TAGS.includes(tag));

  const handleCategorySelect = (category: ProblemCategory) => {
    const nextFixedTag = FIXED_TAG_MAP[category];
    const nonFixedTags = state.tags.filter((tag) => !FIXED_TAGS.includes(tag));
    updateState({
      category,
      useContainer: true,
      tags: [nextFixedTag, ...nonFixedTags],
    });
  };

  const handleNext = () => {
    console.log('Mock: 保存基础信息');
    nextStep();
  };

  return (
    <div className="glass-card p-4 md:p-5 space-y-5 animate-fade-in">
      <div>
        <h2 className="text-xl md:text-2xl font-bold text-gray-900">第一步：基础信息</h2>
        <p className="text-gray-500 mt-1 text-sm">本步骤包含题型选择与通用字段填写。</p>
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-gray-700">题型分类</label>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
          {CATEGORIES.map((cat) => (
            <button
              key={cat.id}
              type="button"
              onClick={() => handleCategorySelect(cat.id)}
              className={`text-left rounded-xl p-3 border-2 transition-all duration-200 ${
                state.category === cat.id
                  ? `${cat.color} shadow-sm`
                  : 'border-gray-200 hover:border-gray-300 bg-white'
              }`}
            >
              <div className="mb-2 text-gray-700">{cat.icon}</div>
              <h3 className="text-sm font-bold text-gray-900">{cat.label}</h3>
              <p className="text-xs text-gray-500 mt-1">{cat.desc}</p>
            </button>
          ))}
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
        <div className="col-span-2">
          <label className="block text-sm font-medium text-gray-700">题目标题</label>
          <input
            type="text"
            value={state.title}
            onChange={(e) => updateState({ title: e.target.value })}
            className="field-input mt-1"
            placeholder="例如：Easy SQLi"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700">难度</label>
          <select
            value={state.difficulty}
            onChange={(e) => updateState({ difficulty: e.target.value as any })}
            className="field-select mt-1"
          >
            <option value="Easy">简单</option>
            <option value="Medium">中等</option>
            <option value="Hard">困难</option>
            <option value="Expert">专家</option>
          </select>
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700">基础分值</label>
          <input
            type="number"
            value={state.baseScore}
            onChange={(e) => updateState({ baseScore: Number(e.target.value) })}
            className="field-input mt-1"
          />
        </div>
      </div>

      <div className="col-span-2 pt-4 border-t border-gray-100">
        <div className="flex items-center space-x-3 mb-4">
          <input
            type="checkbox"
            id="dynamic_score"
            checked={state.isDynamicScore}
            onChange={(e) => updateState({ isDynamicScore: e.target.checked })}
            className="h-4 w-4 rounded border-gray-300 text-amber-600 focus:ring-amber-500"
          />
          <label htmlFor="dynamic_score" className="text-sm font-medium text-gray-700">
            启用动态分值
          </label>
        </div>

        {state.isDynamicScore && (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 bg-amber-50 items-center p-4 rounded-lg border border-amber-100">
            <div>
              <label className="block text-sm font-medium text-gray-700">最低分</label>
              <input type="number" value={state.minScore} onChange={e => updateState({minScore: Number(e.target.value)})} className="field-input mt-1" />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700">每次衰减</label>
              <input type="number" value={state.decayDec} onChange={e => updateState({decayDec: Number(e.target.value)})} className="field-input mt-1" />
            </div>
          </div>
        )}
      </div>

      <div className="col-span-2">
        <label className="block text-sm font-medium text-gray-700 mb-1">题目描述（Markdown）</label>
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
          <div>
            <p className="text-xs text-gray-500 mb-1">编辑区</p>
            <textarea
              rows={12}
              className="field-input w-full font-mono text-sm"
              value={state.description}
              onChange={(e) => updateState({ description: e.target.value })}
              placeholder={'# 题目说明\n\n- 目标\n- 已知条件\n- 提交要求'}
            />
          </div>
          <div>
            <p className="text-xs text-gray-500 mb-1">预览区</p>
            <div className="min-h-[286px] rounded-xl border border-gray-200 bg-white px-4 py-3 text-sm leading-6 text-gray-700 overflow-auto">
              <div className="space-y-2 [&_h1]:text-xl [&_h1]:font-semibold [&_h1]:mb-2 [&_h2]:text-lg [&_h2]:font-semibold [&_h2]:mb-2 [&_h3]:text-base [&_h3]:font-semibold [&_p]:my-1.5 [&_ul]:list-disc [&_ul]:pl-5 [&_ol]:list-decimal [&_ol]:pl-5 [&_code]:bg-amber-50 [&_code]:px-1 [&_code]:rounded [&_pre]:bg-gray-900 [&_pre]:text-gray-100 [&_pre]:p-3 [&_pre]:rounded-lg">
                <ReactMarkdown remarkPlugins={[remarkGfm]}>
                  {state.description || '在左侧输入 Markdown 内容，这里实时预览效果。'}
                </ReactMarkdown>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className="col-span-2">
        <label className="block text-sm font-medium text-gray-700 mb-1">标签（逗号分隔）</label>
        {fixedTag && (
          <p className="text-xs text-gray-500 mb-1.5">
            固定标签：
            <span className="ml-1 inline-flex items-center rounded-full bg-amber-50 border border-amber-200 px-2 py-0.5 text-[11px] text-amber-800 font-medium">
              {fixedTag}
            </span>
          </p>
        )}
        <input
          type="text"
          value={customTags.join(', ')}
          onChange={(e) => {
            const parsed = e.target.value.split(',').map(s => s.trim()).filter(Boolean);
            updateState({ tags: fixedTag ? [fixedTag, ...parsed] : parsed });
          }}
          className="field-input"
          placeholder="例如：sqli, xss, web"
        />
      </div>

      <div className="flex justify-end mt-8 pt-6 border-t border-gray-200">
        <button
          onClick={handleNext}
          disabled={!state.title || !state.category}
          className="btn-primary !py-2 !px-4 text-sm disabled:opacity-50"
        >
          下一步：Flag 配置
        </button>
      </div>
    </div>
  );
}
