import React, { useState } from 'react';
import { useProblemPublish } from '../ProblemPublishContext';
import { useNavigate } from 'react-router-dom';
import { IconCheck, IconClose, IconWarn } from '../icons';

export default function Step6Review() {
  const { state, prevStep } = useProblemPublish();
  const [publishing, setPublishing] = useState(false);
  const navigate = useNavigate();

  // Guard Checks
  const isBasicInfoValid = !!state.title && !!state.description && !!state.category;
  const isFlagValid = state.flagType === 'static' ? !!state.staticFlag : (!!state.dynamicFlagEnvName || !!state.dynamicFlagFilePath);
  const isImageValid = !state.useContainer || !!state.registryImage;

  const canPublish = isBasicInfoValid && isFlagValid && isImageValid;

  const handlePublish = async () => {
    setPublishing(true);
    setTimeout(() => {
      setPublishing(false);
      navigate('/problem');
    }, 1500);
  };

  return (
    <div className="glass-card p-4 md:p-5 space-y-5 animate-fade-in">
      <div className="text-center mb-8">
        <h2 className="text-2xl md:text-3xl font-bold text-gray-900">第五步：发布前检查</h2>
        <p className="text-gray-500 mt-2">发布前进行最终校验，避免缺失配置上线。</p>
      </div>

      <div className="bg-gray-50 rounded-xl p-6 border border-gray-200 mb-8 space-y-4">
        <h3 className="text-lg font-bold text-gray-900 border-b border-gray-300 pb-3">守卫清单</h3>
        <ul className="space-y-3">
          <li className="flex items-center text-sm">
            {isBasicInfoValid ? <IconCheck className="w-4 h-4 text-green-600 mr-3" /> : <IconClose className="w-4 h-4 text-red-600 mr-3" />}
            <span className={isBasicInfoValid ? 'text-gray-700' : 'text-red-600 font-bold'}>基础信息完整（标题、分类、描述）</span>
          </li>
          <li className="flex items-center text-sm">
            {isFlagValid ? <IconCheck className="w-4 h-4 text-green-600 mr-3" /> : <IconClose className="w-4 h-4 text-red-600 mr-3" />}
            <span className={isFlagValid ? 'text-gray-700' : 'text-red-600 font-bold'}>Flag 配置有效</span>
          </li>
          <li className="flex items-center text-sm">
            <IconCheck className="w-4 h-4 text-green-600 mr-3" />
            <span className="text-gray-700">附件上传完成（{state.attachments.length} 个）</span>
          </li>
          {state.useContainer && (
            <li className="flex items-center text-sm">
              {isImageValid ? <IconCheck className="w-4 h-4 text-green-600 mr-3" /> : <IconWarn className="w-4 h-4 text-yellow-600 mr-3" />}
              <span className={isImageValid ? 'text-gray-700' : 'text-yellow-600 font-bold'}>容器镜像已就绪</span>
            </li>
          )}
          {!state.useContainer && (
            <li className="flex items-center text-sm">
              <IconCheck className="w-4 h-4 text-green-600 mr-3" />
              <span className="text-gray-700">容器步骤已跳过（本题无需容器）</span>
            </li>
          )}
        </ul>
      </div>

      <div className="bg-amber-50 rounded-xl p-6 border border-amber-100 border-dashed">
        <h3 className="text-lg font-bold text-gray-900 border-b border-amber-200 pb-3 mb-4">题目摘要</h3>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-y-4 gap-x-8 text-sm">
          <div><span className="text-gray-500 mr-2">标题：</span> <span className="font-semibold text-gray-900">{state.title || '-'}</span></div>
          <div><span className="text-gray-500 mr-2">分类：</span> <span className="font-semibold text-gray-900">{state.category}</span></div>
          <div><span className="text-gray-500 mr-2">难度：</span> <span className="text-gray-800">{state.difficulty}</span></div>
          <div><span className="text-gray-500 mr-2">分值：</span> <span className="text-gray-800">{state.baseScore} 分 {state.isDynamicScore ? `（最低 ${state.minScore}）` : ''}</span></div>
          <div><span className="text-gray-500 mr-2">Flag 类型：</span> <span className="text-gray-800">{state.flagType}</span></div>
          <div><span className="text-gray-500 mr-2">容器：</span> <span className="text-gray-800">{state.useContainer ? '启用' : '跳过'}</span></div>
          {state.useContainer && (
            <>
              <div className="col-span-2"><span className="text-gray-500 mr-2">镜像：</span> <span className="font-mono text-xs bg-white p-1 rounded border border-amber-100">{state.registryImage || '未就绪'}</span></div>
              <div><span className="text-gray-500 mr-2">网络：</span> <span className="text-gray-800">{state.accessType}（端口 {state.internalPort}）</span></div>
            </>
          )}
        </div>
      </div>

      <div className="flex justify-between mt-10 pt-6 border-t border-gray-200">
        <button
          onClick={prevStep}
          disabled={publishing}
          className="btn-secondary !py-2 !px-4 text-sm disabled:opacity-50"
        >
          返回修改
        </button>
        <button
          onClick={handlePublish}
          disabled={!canPublish || publishing}
          className={`btn-primary !py-2 !px-5 text-sm ${publishing ? 'opacity-80 cursor-wait' : ''} ${!canPublish ? 'opacity-50 cursor-not-allowed' : ''}`}
        >
          {publishing ? '发布中...' : '发布题目'}
        </button>
      </div>
    </div>
  );
}
