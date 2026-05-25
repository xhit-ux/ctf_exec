import React, { useState, useEffect } from 'react';
import { useProblemPublish } from '../ProblemPublishContext';
import { saveProblemFlag } from '../../../utils/problemPublishApi';
import { FlagRule } from '../types';

const sha256 = async (message: string) => {
  if (!message) return '';
  const msgBuffer = new TextEncoder().encode(message);
  const hashBuffer = await crypto.subtle.digest('SHA-256', msgBuffer);
  const hashArray = Array.from(new Uint8Array(hashBuffer));
  return hashArray.map((b) => b.toString(16).padStart(2, '0')).join('');
};

const FLAG_RULE_OPTIONS: { id: FlagRule; label: string; desc: string }[] = [
  { id: 'exact', label: '精确匹配', desc: '完整字符串匹配（推荐用于标准 flag）' },
  { id: 'case_insensitive', label: '忽略大小写', desc: '统一转小写后再比对' },
  { id: 'regex', label: '正则匹配', desc: '提交内容须匹配指定正则；仅 static flag 可选' },
  { id: 'contains', label: '包含子串', desc: '提交包含子串即视为正确；仅 static flag 可选' },
];

export default function Step3FlagConfig() {
  const { state, updateState, nextStep, prevStep, setSubmitting } = useProblemPublish();
  const [hashPreview, setHashPreview] = useState('');
  const dynamicBlockedByShared = state.deployMode === 'shared';

  useEffect(() => {
    if (
      state.flagType === 'static' &&
      (state.flagRule === 'exact' || state.flagRule === 'case_insensitive') &&
      state.flagValue
    ) {
      const normalized =
        state.flagRule === 'case_insensitive' ? state.flagValue.toLowerCase() : state.flagValue;
      sha256(normalized).then(setHashPreview);
    } else {
      setHashPreview('');
    }
  }, [state.flagValue, state.flagType, state.flagRule]);

  // dynamic 不允许 regex/contains（与后端 flag_mode_explain.md Part 4 对齐）。
  useEffect(() => {
    if (
      state.flagType === 'dynamic' &&
      (state.flagRule === 'regex' || state.flagRule === 'contains')
    ) {
      updateState({ flagRule: 'exact' });
    }
  }, [state.flagType, state.flagRule, updateState]);

  const isStaticPattern =
    state.flagType === 'static' && (state.flagRule === 'regex' || state.flagRule === 'contains');
  const isStaticPlain =
    state.flagType === 'static' && (state.flagRule === 'exact' || state.flagRule === 'case_insensitive');

  const handleNext = async () => {
    if (state.flagType === 'dynamic' && state.deployMode === 'shared') {
      window.alert('shared 部署模式不支持 dynamic flag，请先在容器步骤切换为 per_user');
      return;
    }
    if (!state.problemId) {
      window.alert('请先完成基础信息并创建草稿');
      return;
    }
    if (state.flagType === 'static') {
      if (!state.flagValue || !state.flagValue.trim()) {
        window.alert('static flag 的 flag_value 不能为空');
        return;
      }
      if (state.flagRule === 'regex') {
        try {
          new RegExp(state.flagValue);
        } catch {
          window.alert('正则表达式不合法');
          return;
        }
      }
    }
    setSubmitting(true);
    try {
      await saveProblemFlag(state.problemId, {
        flag_value: state.flagType === 'static' ? state.flagValue || '' : '',
        flag_rule: state.flagRule,
        flag_type: state.flagType,
        flag_inject: state.flagInject,
        flag_env_key: state.flagEnvKey || 'EXEC_FLAG',
        flag_example: state.flagExample || undefined,
        flag_sidecar_pattern: state.flagSidecarPattern || undefined,
      });
      nextStep();
    } catch (err) {
      const msg = err instanceof Error ? err.message : '保存 Flag 配置失败';
      window.alert(msg);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="glass-card p-4 md:p-5 space-y-5 animate-fade-in">
      <div>
        <h2 className="text-xl md:text-2xl font-bold text-gray-900">第二步：Flag 配置</h2>
        <p className="text-gray-500 mt-1 text-sm">
          v2.1 三维度：flag_type（static/dynamic）× flag_rule（exact/case_insensitive/regex/contains）× flag_inject（env/sidecar）。
        </p>
      </div>

      <div className="p-3.5 border border-gray-200 rounded-lg bg-gray-50">
        <p className="text-sm font-medium text-gray-700 mb-2">Flag 类型</p>
        <div className="inline-flex rounded-lg border border-gray-200 bg-white p-1">
          <button
            type="button"
            onClick={() => updateState({ flagType: 'static' })}
            className={`px-3 py-1.5 text-sm rounded-md transition-colors ${state.flagType === 'static' ? 'bg-amber-100 text-amber-800' : 'text-gray-600 hover:bg-gray-50'}`}
          >
            静态 Flag
          </button>
          <button
            type="button"
            onClick={() => {
              if (dynamicBlockedByShared) {
                window.alert('当前部署模式为 shared，不支持 dynamic flag');
                return;
              }
              updateState({ flagType: 'dynamic' });
            }}
            className={`px-3 py-1.5 text-sm rounded-md transition-colors ${state.flagType === 'dynamic' ? 'bg-amber-100 text-amber-800' : 'text-gray-600 hover:bg-gray-50'} ${dynamicBlockedByShared ? 'opacity-50 cursor-not-allowed' : ''}`}
          >
            动态 Flag
          </button>
        </div>
        {dynamicBlockedByShared && (
          <p className="text-xs text-amber-700 mt-2">
            当前 deploy_mode=shared，dynamic flag 已禁用。请在「容器环境」步骤切换为 per_user。
          </p>
        )}
      </div>

      <div className="p-3.5 border border-gray-200 rounded-lg bg-gray-50">
        <p className="text-sm font-medium text-gray-700 mb-2">校验规则（flag_rule）</p>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          {FLAG_RULE_OPTIONS.map((opt) => {
            const disabled =
              state.flagType === 'dynamic' && (opt.id === 'regex' || opt.id === 'contains');
            const selected = state.flagRule === opt.id;
            return (
              <button
                key={opt.id}
                type="button"
                disabled={disabled}
                onClick={() => updateState({ flagRule: opt.id })}
                className={`text-left rounded-lg p-3 border-2 transition ${
                  selected
                    ? 'border-amber-500 bg-amber-50'
                    : disabled
                    ? 'border-gray-100 bg-gray-100 opacity-50 cursor-not-allowed'
                    : 'border-gray-200 bg-white hover:border-gray-300'
                }`}
              >
                <div className="font-bold text-sm text-gray-900">{opt.label}</div>
                <div className="text-xs text-gray-500 mt-1">{opt.desc}</div>
              </button>
            );
          })}
        </div>
      </div>

      {state.flagType === 'static' && (
        <div className="p-5 bg-gray-50 border border-gray-200 rounded-lg space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700">
              {isStaticPattern ? 'Flag Pattern（正则 / 子串）' : 'Flag 明文'}
            </label>
            <input
              type="text"
              value={state.flagValue || ''}
              onChange={(e) => updateState({ flagValue: e.target.value })}
              className="field-input mt-1 font-mono"
              placeholder={
                isStaticPattern
                  ? state.flagRule === 'regex'
                    ? '^flag\\{[a-z0-9_]+\\}$'
                    : 'admin'
                  : 'flag{this_is_the_answer}'
              }
            />
            {isStaticPlain && hashPreview && (
              <p className="mt-2 text-xs text-gray-500 flex items-center break-all">
                <span className="font-semibold mr-2">SHA256 预览：</span> {hashPreview}
              </p>
            )}
            {isStaticPattern && (
              <p className="mt-2 text-xs text-amber-700">
                pattern 长度上限 512；正则将由后端预编译并加 200ms 超时防 ReDoS。
              </p>
            )}
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">Flag 示例（可选）</label>
            <input
              type="text"
              value={state.flagExample || ''}
              onChange={(e) => updateState({ flagExample: e.target.value })}
              className="field-input mt-1 font-mono"
              placeholder="flag{example_for_user}"
            />
          </div>
        </div>
      )}

      <div className="p-5 bg-amber-50 border border-amber-100 rounded-lg space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-3">注入方式（flag_inject）</label>
          <div className="grid grid-cols-2 gap-4">
            <button
              type="button"
              onClick={() => updateState({ flagInject: 'env' })}
              className={`flex flex-col p-4 rounded-lg border-2 text-left ${
                state.flagInject === 'env'
                  ? 'border-amber-500 bg-white shadow-sm'
                  : 'border-gray-200 bg-transparent'
              }`}
            >
              <span className="font-bold text-gray-900">环境变量</span>
              <span className="text-sm text-gray-500 mt-1">通过容器环境变量注入到挑战容器</span>
            </button>
            <button
              type="button"
              onClick={() => updateState({ flagInject: 'sidecar' })}
              className={`flex flex-col p-4 rounded-lg border-2 text-left ${
                state.flagInject === 'sidecar'
                  ? 'border-amber-500 bg-white shadow-sm'
                  : 'border-gray-200 bg-transparent'
              }`}
            >
              <span className="font-bold text-gray-900">Sidecar</span>
              <span className="text-sm text-gray-500 mt-1">由 sidecar 容器写入指定路径</span>
            </button>
          </div>
        </div>

        {state.flagInject === 'env' ? (
          <div>
            <label className="block text-sm font-medium text-gray-700">环境变量名（flag_env_key）</label>
            <input
              type="text"
              value={state.flagEnvKey || 'EXEC_FLAG'}
              onChange={(e) => updateState({ flagEnvKey: e.target.value })}
              className="field-input mt-1 font-mono"
              placeholder="EXEC_FLAG"
            />
          </div>
        ) : (
          <div>
            <label className="block text-sm font-medium text-gray-700">Sidecar 写入路径（flag_sidecar_pattern）</label>
            <input
              type="text"
              value={state.flagSidecarPattern || ''}
              onChange={(e) => updateState({ flagSidecarPattern: e.target.value })}
              className="field-input mt-1 font-mono"
              placeholder="/flag"
            />
          </div>
        )}
      </div>

      <div className="flex justify-between mt-8 pt-6 border-t border-gray-200">
        <button onClick={prevStep} className="btn-secondary !py-2 !px-4 text-sm">
          上一步
        </button>
        <button
          onClick={handleNext}
          disabled={state.flagType === 'static' && !state.flagValue}
          className="btn-primary !py-2 !px-4 text-sm disabled:opacity-50"
        >
          下一步：附件上传
        </button>
      </div>
    </div>
  );
}
