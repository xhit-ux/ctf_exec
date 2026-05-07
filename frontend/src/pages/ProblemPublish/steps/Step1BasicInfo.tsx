import React from 'react';
import { useProblemPublish } from '../ProblemPublishContext';
import { ProblemCategory, DIFFICULTY_SCORE_RANGE, Difficulty } from '../types';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { IconChip, IconGlobe, IconKey, IconPuzzle, IconTerminal } from '../icons';
import { createProblemDraft, saveProblemBasic } from '../../../utils/problemPublishApi';

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
  const { state, updateState, nextStep, setSubmitting } = useProblemPublish();
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

  const toProblemType = (category: ProblemCategory, useContainer: boolean): 'static' | 'container_web' | 'container_tcp' => {
    if (!useContainer) return 'static';
    if (category === 'Web') return 'container_web';
    return 'container_tcp';
  };

  const toDifficulty = (raw: string): 'easy' | 'medium' | 'hard' | 'expert' => {
    switch (raw.toLowerCase()) {
      case 'easy':
        return 'easy';
      case 'medium':
        return 'medium';
      case 'hard':
        return 'hard';
      default:
        return 'expert';
    }
  };

  const handleNext = async () => {
    if (!state.category || !state.title.trim()) return;
    // 校验 platform_score 是否在难度区间内（v2.1 Part 3.1）。
    const range = DIFFICULTY_SCORE_RANGE[state.difficulty];
    const score = Number(state.platformScore) || 0;
    if (score < range.min || score > range.max) {
      window.alert(
        `platform_score 必须在 [${range.min}, ${range.max}] 区间内（当前难度=${state.difficulty}）`,
      );
      return;
    }
    setSubmitting(true);
    try {
      const problemType = toProblemType(state.category, state.useContainer);
      let problemID = state.problemId;
      if (!problemID) {
        problemID = await createProblemDraft({
          category: state.category,
          type: problemType,
        });
        updateState({ problemId: problemID });
      }

      await saveProblemBasic(problemID, {
        title: state.title.trim(),
        author: state.author?.trim() || undefined,
        category: state.category,
        type: problemType,
        description: state.description,
        platform_score: score,
        difficulty: toDifficulty(state.difficulty),
        deploy_mode: state.deployMode || 'per_user',
        use_kata: false,
        tags: state.tags,
      });

      nextStep();
    } catch (err) {
      const msg = err instanceof Error ? err.message : '保存基础信息失败';
      window.alert(msg);
    } finally {
      setSubmitting(false);
    }
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
            onChange={(e) => {
              const next = e.target.value as Difficulty;
              const range = DIFFICULTY_SCORE_RANGE[next];
              const clamp = Math.min(Math.max(state.platformScore, range.min), range.max);
              updateState({ difficulty: next, platformScore: clamp });
            }}
            className="field-select mt-1"
          >
            <option value="Easy">简单 [1, 200]</option>
            <option value="Medium">中等 [200, 500]</option>
            <option value="Hard">困难 [500, 1000]</option>
            <option value="Expert">专家 [1000, 2000]</option>
          </select>
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700">
            平台分值（platform_score）
          </label>
          <input
            type="number"
            min={DIFFICULTY_SCORE_RANGE[state.difficulty].min}
            max={DIFFICULTY_SCORE_RANGE[state.difficulty].max}
            value={state.platformScore}
            onChange={(e) => updateState({ platformScore: Number(e.target.value) })}
            className="field-input mt-1"
          />
          <p className="text-xs text-gray-500 mt-1">
            须在 [{DIFFICULTY_SCORE_RANGE[state.difficulty].min},{' '}
            {DIFFICULTY_SCORE_RANGE[state.difficulty].max}] 区间内。赛事动态计分（contest_score、衰减）请在赛事题目编排时配置。
          </p>
        </div>

        <div className="col-span-2">
          <label className="block text-sm font-medium text-gray-700">作者（author，可选）</label>
          <input
            type="text"
            value={state.author || ''}
            onChange={(e) => updateState({ author: e.target.value })}
            className="field-input mt-1"
            placeholder="题目作者展示名"
          />
        </div>
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
