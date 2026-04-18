import React, { useState, useEffect } from 'react';
import { useProblemPublish } from '../ProblemPublishContext';
// A mock sha256 function to simulate the frontend hashing preview
const sha256 = async (message: string) => {
  if (!message) return '';
  const msgBuffer = new TextEncoder().encode(message);
  const hashBuffer = await crypto.subtle.digest('SHA-256', msgBuffer);
  const hashArray = Array.from(new Uint8Array(hashBuffer));
  return hashArray.map(b => b.toString(16).padStart(2, '0')).join('');
};

export default function Step3FlagConfig() {
  const { state, updateState, nextStep, prevStep } = useProblemPublish();
  const [hashPreview, setHashPreview] = useState('');

  useEffect(() => {
    if (state.flagType === 'static' && state.staticFlag) {
      sha256(state.staticFlag).then(setHashPreview);
    } else {
      setHashPreview('');
    }
  }, [state.staticFlag, state.flagType]);

  const handleNext = () => {
    console.log('Mock: 保存 Flag 配置');
    nextStep();
  };

  return (
    <div className="glass-card p-4 md:p-5 space-y-5 animate-fade-in">
      <div>
        <h2 className="text-xl md:text-2xl font-bold text-gray-900">第二步：Flag 配置</h2>
        <p className="text-gray-500 mt-1 text-sm">
          {state.flagType === 'static' 
            ? '静态题使用固定 Flag，提交时仅存储哈希值。' 
            : '动态题按实例或用户生成 Flag，前端只配置注入规则。'}
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
            onClick={() => updateState({ flagType: 'dynamic' })}
            className={`px-3 py-1.5 text-sm rounded-md transition-colors ${state.flagType === 'dynamic' ? 'bg-amber-100 text-amber-800' : 'text-gray-600 hover:bg-gray-50'}`}
          >
            动态 Flag
          </button>
        </div>
      </div>

      {state.flagType === 'static' ? (
        <div className="p-5 bg-gray-50 border border-gray-200 rounded-lg space-y-5">
          <div>
            <label className="block text-sm font-medium text-gray-700">Flag 明文</label>
            <input 
              type="text" 
              value={state.staticFlag || ''}
              onChange={(e) => updateState({ staticFlag: e.target.value })}
              className="field-input mt-1 font-mono"
              placeholder="flag{this_is_the_answer}"
            />
            {hashPreview && (
              <p className="mt-2 text-xs text-gray-500 flex items-center break-all">
                <span className="font-semibold mr-2">SHA256:</span> {hashPreview}
              </p>
            )}
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">校验模式</label>
            <div className="mt-2 space-y-2 sm:flex sm:items-center sm:space-y-0 sm:space-x-8">
              <div className="flex items-center">
                <input id="exact" name="match_mode" type="radio" checked={state.staticFlagMatchMode === 'exact'} onChange={() => updateState({ staticFlagMatchMode: 'exact' })} className="focus:ring-amber-500 h-4 w-4 text-amber-600 border-gray-300" />
                <label htmlFor="exact" className="ml-3 block text-sm font-medium text-gray-700">精确匹配</label>
              </div>
              <div className="flex items-center">
                <input id="ignore_case" name="match_mode" type="radio" checked={state.staticFlagMatchMode === 'ignore_case'} onChange={() => updateState({ staticFlagMatchMode: 'ignore_case' })} className="focus:ring-amber-500 h-4 w-4 text-amber-600 border-gray-300" />
                <label htmlFor="ignore_case" className="ml-3 block text-sm font-medium text-gray-700">忽略大小写</label>
              </div>
              <div className="flex items-center">
                <input id="regex" name="match_mode" type="radio" checked={state.staticFlagMatchMode === 'regex'} onChange={() => updateState({ staticFlagMatchMode: 'regex' })} className="focus:ring-amber-500 h-4 w-4 text-amber-600 border-gray-300" />
                <label htmlFor="regex" className="ml-3 block text-sm font-medium text-gray-700">正则</label>
              </div>
            </div>
          </div>
        </div>
      ) : (
        <div className="p-5 bg-amber-50 border border-amber-100 rounded-lg space-y-5">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-3">注入方式</label>
            <div className="grid grid-cols-2 gap-4">
              <label className={`cursor-pointer flex flex-col p-4 rounded-lg border-2 ${state.dynamicFlagInjection === 'env' || !state.dynamicFlagInjection ? 'border-amber-500 bg-white shadow-sm' : 'border-gray-200 bg-transparent'}`} onClick={() => updateState({ dynamicFlagInjection: 'env' })}>
                <span className="font-bold text-gray-900">环境变量</span>
                <span className="text-sm text-gray-500 mt-1">通过容器环境变量注入</span>
              </label>
              <label className={`cursor-pointer flex flex-col p-4 rounded-lg border-2 ${state.dynamicFlagInjection === 'file' ? 'border-amber-500 bg-white shadow-sm' : 'border-gray-200 bg-transparent'}`} onClick={() => updateState({ dynamicFlagInjection: 'file' })}>
                <span className="font-bold text-gray-900">文件写入</span>
                <span className="text-sm text-gray-500 mt-1">写入容器指定文件路径</span>
              </label>
            </div>
          </div>

          {(state.dynamicFlagInjection === 'env' || !state.dynamicFlagInjection) ? (
            <div>
              <label className="block text-sm font-medium text-gray-700">环境变量名</label>
              <input 
                type="text" 
                value={state.dynamicFlagEnvName || 'FLAG'}
                onChange={(e) => updateState({ dynamicFlagEnvName: e.target.value })}
                className="field-input mt-1 font-mono"
                placeholder="例如：FLAG"
              />
            </div>
          ) : (
            <div>
              <label className="block text-sm font-medium text-gray-700">容器内文件路径</label>
              <input 
                type="text" 
                value={state.dynamicFlagFilePath || '/flag'}
                onChange={(e) => updateState({ dynamicFlagFilePath: e.target.value })}
                className="field-input mt-1 font-mono"
                placeholder="/home/ctf/flag.txt"
              />
            </div>
          )}

          <div>
            <label className="block text-sm font-medium text-gray-700">Flag 前缀（可选）</label>
            <input 
              type="text" 
              value={state.dynamicFlagPrefix || 'flag{'}
              onChange={(e) => updateState({ dynamicFlagPrefix: e.target.value })}
              className="field-input mt-1 font-mono"
            />
            <p className="mt-2 text-xs text-gray-500">
              示例预览：<code className="bg-white px-1 py-0.5 rounded border border-amber-100">{state.dynamicFlagPrefix || ''}8d2f1b4a-e9c...{'}'}</code>
            </p>
          </div>
        </div>
      )}

      <div className="flex justify-between mt-8 pt-6 border-t border-gray-200">
        <button
          onClick={prevStep}
          className="btn-secondary !py-2 !px-4 text-sm"
        >
          上一步
        </button>
        <button
          onClick={handleNext}
          disabled={state.flagType === 'static' && !state.staticFlag}
          className="btn-primary !py-2 !px-4 text-sm disabled:opacity-50"
        >
          下一步：附件上传
        </button>
      </div>
    </div>
  );
}
