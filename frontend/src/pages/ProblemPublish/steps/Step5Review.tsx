import React, { useEffect, useRef, useState } from 'react';
import { useProblemPublish } from '../ProblemPublishContext';
import { useNavigate } from 'react-router-dom';
import { IconCheck, IconClose, IconWarn } from '../icons';
import { deleteProblemDraft, fetchProblemBuildTask, publishProblem } from '../../../utils/problemPublishApi';

const STEP_PROGRESS: Record<string, number> = {
  pending: 5,
  running: 10,
  queued: 12,
  uploaded: 15,
  extracting: 30,
  loading: 40,
  tagging: 55,
  building: 65,
  pushing: 82,
  verifying_registry: 92,
  success: 100,
  failed: 100,
};

const PHASE_LABEL: Record<string, string> = {
  queued: '已进入发布队列',
  preparing: '准备执行环境',
  extracting: '解包项目文件',
  loading: '导入镜像包',
  tagging: '打标签',
  building: '构建镜像',
  pushing: '推送镜像仓库',
  verifying_registry: '校验仓库镜像',
  success: '发布完成',
  failed: '发布失败',
};

const DOCKER_SAVE_FLOW = ['queued', 'preparing', 'loading', 'tagging', 'pushing', 'verifying_registry', 'success'];
const COMPOSE_FLOW = ['queued', 'preparing', 'extracting', 'building', 'pushing', 'verifying_registry', 'success'];

function getFlow(uploadType?: string): string[] {
  return uploadType === 'compose_bundle' ? COMPOSE_FLOW : DOCKER_SAVE_FLOW;
}

function getPhaseCommand(phase: string, uploadType: string | undefined, imageRef: string): string {
  if (uploadType === 'compose_bundle') {
    switch (phase) {
      case 'queued':
        return '等待发布按钮触发构建任务';
      case 'preparing':
        return '初始化构建上下文';
      case 'extracting':
        return '解压 zip/tar 到临时目录';
      case 'building':
        return 'docker compose -f <compose-file> build';
      case 'pushing':
        return 'docker compose -f <compose-file> push';
      case 'verifying_registry':
        return `docker manifest inspect ${imageRef}`;
      case 'success':
        return '发布守卫通过，题目状态切换为 ready';
      default:
        return '等待下一步...';
    }
  }

  switch (phase) {
    case 'queued':
      return '等待发布按钮触发构建任务';
    case 'preparing':
      return '初始化构建上下文';
    case 'loading':
      return 'docker load -i <uploaded-archive>';
    case 'tagging':
      return `docker tag <loaded-image> ${imageRef}`;
    case 'pushing':
      return `docker push ${imageRef}`;
    case 'verifying_registry':
      return `docker manifest inspect ${imageRef}`;
    case 'success':
      return '发布守卫通过，题目状态切换为 ready';
    default:
      return '等待下一步...';
  }
}

export default function Step6Review() {
  const { state, prevStep, updateState } = useProblemPublish();
  const [publishing, setPublishing] = useState(false);
  const [buildingHint, setBuildingHint] = useState('');
  const [modalOpen, setModalOpen] = useState(false);
  const pollRef = useRef<number | null>(null);
  const navigate = useNavigate();

  // Guard Checks
  const isBasicInfoValid = !!state.title && !!state.description && !!state.category;
  const isFlagValid = state.flagType === 'dynamic' || !!state.flagValue;
  const hasSucceededOrUploadedTask = state.imageTaskStatus === 'uploaded' || state.imageTaskStatus === 'success';
  const hasFailedTask = state.imageTaskStatus === 'failed';
  const hasKnownTask = Boolean(state.imageTaskId || state.imageTaskStatus);
  const hasContainerImage = state.imageLocked
    ? true
    : hasFailedTask
      ? false
      : hasSucceededOrUploadedTask
        ? true
        : hasKnownTask
          ? false
          : !!(state.registryImage && state.registryImage.trim());
  const isImageValid = !state.useContainer || hasContainerImage;
  const isFlagDeployComboValid = !(state.flagType === 'dynamic' && state.deployMode === 'shared');

  const canPublish = isBasicInfoValid && isFlagValid && isImageValid && isFlagDeployComboValid;

  useEffect(() => {
    return () => {
      if (pollRef.current !== null) {
        window.clearInterval(pollRef.current);
      }
    };
  }, []);

  const stopPolling = () => {
    if (pollRef.current !== null) {
      window.clearInterval(pollRef.current);
      pollRef.current = null;
    }
  };

  const rollbackDraft = async (problemID: string) => {
    try {
      await deleteProblemDraft(problemID);
    } catch {
      // noop: rollback best effort
    }
  };

  const tryFinalizePublish = async (problemID: string) => {
    const result = await publishProblem(problemID);
    if (result.published) {
      navigate(state.sourceProblemId ? `/problems/${state.sourceProblemId}` : `/problems/${problemID}`);
      return true;
    }
    return false;
  };

  const startPollingBuild = (problemID: string) => {
    stopPolling();
    pollRef.current = window.setInterval(async () => {
      try {
        const task = await fetchProblemBuildTask(problemID);
        if (!task) return;

        const step = task.status === 'success' || task.status === 'failed' ? task.status : task.current_step;
        const imageRef = task.image_repo && task.image_tag ? `${task.image_repo}:${task.image_tag}` : state.registryImage;
        updateState({
          imageTaskId: task.id,
          imageTaskStatus: step as any,
          imageTaskProgress: STEP_PROGRESS[step] ?? 15,
          imageTaskError: task.error_log,
          registryImage: imageRef,
        });

        if (task.status === 'failed') {
          stopPolling();
          setPublishing(false);
          setModalOpen(false);
          await rollbackDraft(problemID);
          setBuildingHint('镜像构建失败，请根据错误日志修复后重试发布。');
          window.alert('发布失败，草稿与上传内容已清理');
          return;
        }
        if (task.status === 'success') {
          stopPolling();
          setBuildingHint('镜像构建完成，正在完成发布...');
          const finalized = await tryFinalizePublish(problemID);
          if (!finalized) {
            setPublishing(false);
            setModalOpen(false);
            await rollbackDraft(problemID);
            window.alert('发布失败，草稿与上传内容已清理');
          } else {
            setModalOpen(false);
          }
        }
      } catch {
        stopPolling();
        setPublishing(false);
        setModalOpen(false);
      }
    }, 2000);
  };

  const handlePublish = async () => {
    if (!state.problemId) {
      window.alert('缺少 problem_id，请先完成前置步骤');
      return;
    }
    setPublishing(true);
    setModalOpen(true);
    try {
      const result = await publishProblem(state.problemId);
      if (result.published) {
        setPublishing(false);
        setModalOpen(false);
        navigate(state.sourceProblemId ? `/problems/${state.sourceProblemId}` : `/problems/${state.problemId}`);
        return;
      }
      if (result.building) {
        setBuildingHint(result.message || '镜像构建已启动，请等待...');
        startPollingBuild(state.problemId);
        return;
      }
      setPublishing(false);
    } catch (err) {
      setPublishing(false);
      setModalOpen(false);
      await rollbackDraft(state.problemId);
      const msg = err instanceof Error ? err.message : '发布失败';
      window.alert(`${msg}；草稿与上传内容已清理`);
    }
  };

  const flow = getFlow(state.uploadType);
  const currentPhase = (state.imageTaskStatus && state.imageTaskStatus !== 'running') ? state.imageTaskStatus : 'preparing';
  const activeIdx = Math.max(flow.indexOf(currentPhase), 0);
  const currentCommand = getPhaseCommand(currentPhase, state.uploadType, state.registryImage || '<repo>:<tag>');

  return (
    <div className="glass-card p-4 md:p-5 space-y-5 animate-fade-in">
      <div className="text-center mb-8">
        <h2 className="text-2xl md:text-3xl font-bold text-gray-900">第五步：发布前检查</h2>
        <p className="text-gray-500 mt-2">发布前进行最终校验，避免缺失配置上线。</p>
      </div>

      {buildingHint && (
        <div className="p-3 rounded-lg border border-amber-200 bg-amber-50 text-sm text-amber-800">
          {buildingHint}
        </div>
      )}

      {state.useContainer && !!state.imageTaskStatus && (
        <div className="p-3 rounded-lg border border-gray-200 bg-white text-sm">
          <p className="text-gray-700">镜像任务状态：<span className="font-semibold">{state.imageTaskStatus}</span></p>
          <p className="text-gray-500">进度：{state.imageTaskProgress || 0}%</p>
          {state.imageTaskError && <p className="text-red-600 mt-1 whitespace-pre-wrap">{state.imageTaskError}</p>}
        </div>
      )}

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
            {isFlagDeployComboValid ? <IconCheck className="w-4 h-4 text-green-600 mr-3" /> : <IconClose className="w-4 h-4 text-red-600 mr-3" />}
            <span className={isFlagDeployComboValid ? 'text-gray-700' : 'text-red-600 font-bold'}>dynamic flag 与 deploy_mode 组合合法</span>
          </li>
          <li className="flex items-center text-sm">
            <IconCheck className="w-4 h-4 text-green-600 mr-3" />
            <span className="text-gray-700">附件：{state.attachment ? state.attachment.filename : '未上传（可选）'}</span>
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
          <div><span className="text-gray-500 mr-2">平台分值：</span> <span className="text-gray-800">{state.platformScore} 分</span></div>
          <div><span className="text-gray-500 mr-2">Flag 类型：</span> <span className="text-gray-800">{state.flagType}</span></div>
          <div><span className="text-gray-500 mr-2">容器：</span> <span className="text-gray-800">{state.useContainer ? '启用' : '跳过'}</span></div>
          {state.useContainer && (
            <>
              <div className="col-span-2"><span className="text-gray-500 mr-2">镜像：</span> <span className="font-mono text-xs bg-white p-1 rounded border border-amber-100">{state.registryImage || '未就绪'}</span></div>
              <div><span className="text-gray-500 mr-2">部署模式：</span> <span className="text-gray-800">{state.deployMode || 'per_user'}</span></div>
              <div><span className="text-gray-500 mr-2">网络：</span> <span className="text-gray-800">{state.accessType}（内部端口 {state.internalPort}，外部范围 {state.hostPortMin}-{state.hostPortMax}）</span></div>
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
          {publishing ? '处理中...' : '发布题目'}
        </button>
      </div>

      {modalOpen && (
        <div className="fixed inset-0 z-50 bg-black/35 flex items-center justify-center p-4">
          <div className="w-full max-w-2xl rounded-2xl bg-white border border-gray-200 shadow-xl p-6 space-y-4">
            <div className="flex items-start justify-between gap-4">
              <div>
                <h3 className="text-lg font-bold text-gray-900">发布进度</h3>
                <p className="text-sm text-gray-600 mt-1">{buildingHint || '正在执行发布流程...'}</p>
              </div>
              <span className="text-xs px-2 py-1 rounded-full bg-gray-100 text-gray-600">{currentPhase}</span>
            </div>

            <div className="rounded-lg border border-gray-200 bg-gray-50 p-3">
              <p className="text-xs text-gray-500 mb-1">当前执行命令</p>
              <p className="text-xs font-mono text-gray-800 break-all">{currentCommand}</p>
            </div>

            <div className="h-2 bg-gray-100 rounded-full overflow-hidden">
              <div className="h-full bg-amber-500 transition-all duration-500" style={{ width: `${state.imageTaskProgress || (publishing ? 10 : 0)}%` }}></div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
              {flow.map((phase, idx) => {
                const done = idx < activeIdx;
                const active = idx === activeIdx;
                return (
                  <div key={phase} className={`rounded-lg border px-3 py-2 ${active ? 'border-amber-300 bg-amber-50' : done ? 'border-emerald-200 bg-emerald-50' : 'border-gray-200 bg-white'}`}>
                    <div className="flex items-center gap-2">
                      <span className={`w-2 h-2 rounded-full ${active ? 'bg-amber-500' : done ? 'bg-emerald-500' : 'bg-gray-300'}`}></span>
                      <p className={`text-sm ${active ? 'text-amber-900 font-semibold' : done ? 'text-emerald-800' : 'text-gray-700'}`}>{PHASE_LABEL[phase] || phase}</p>
                    </div>
                    <p className="text-[11px] mt-1 text-gray-500 font-mono break-all">{getPhaseCommand(phase, state.uploadType, state.registryImage || '<repo>:<tag>')}</p>
                  </div>
                );
              })}
            </div>

            {state.imageTaskError && <p className="text-xs text-red-600 whitespace-pre-wrap border border-red-200 bg-red-50 rounded-lg p-2">{state.imageTaskError}</p>}
          </div>
        </div>
      )}
    </div>
  );
}
