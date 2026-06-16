import React, { useEffect, useMemo, useState } from 'react';
import { useProblemPublish } from '../ProblemPublishContext';
import { IconUpload } from '../icons';
import { saveProblemBasic, saveProblemContainer, uploadProblemImage } from '../../../utils/problemPublishApi';

export default function Step4ContainerEnv() {
  const { state, updateState, nextStep, prevStep, setSubmitting } = useProblemPublish();
  const [uploading, setUploading] = useState(false);
  const [savingDeployMode, setSavingDeployMode] = useState(false);

  const toProblemType = (): 'static' | 'container_web' | 'container_tcp' => {
    if (!state.useContainer) return 'static';
    if (state.category === 'Web') return 'container_web';
    return 'container_tcp';
  };

  const toDifficulty = (): 'easy' | 'medium' | 'hard' | 'expert' => {
    const raw = String(state.difficulty || '').toLowerCase();
    if (raw === 'easy' || raw === 'medium' || raw === 'hard') {
      return raw;
    }
    return 'expert';
  };

  const persistDeployMode = async (deployMode: 'shared' | 'per_user'): Promise<boolean> => {
    if (!state.problemId || !state.category || !state.title.trim()) {
      return true;
    }
    setSavingDeployMode(true);
    try {
      await saveProblemBasic(state.problemId, {
        title: state.title.trim(),
        author: state.author?.trim() || undefined,
        category: state.category,
        type: toProblemType(),
        description: state.description,
        platform_score: Number(state.platformScore) || 0,
        difficulty: toDifficulty(),
        deploy_mode: deployMode,
        use_kata: state.useKata,
        tags: state.tags,
      });
      return true;
    } catch (err) {
      const msg = err instanceof Error ? err.message : '保存部署模式失败';
      window.alert(msg);
      return false;
    } finally {
      setSavingDeployMode(false);
    }
  };

  const parseImageRef = (value: string): { repo: string; tag: string } => {
    const trimmed = value.trim();
    if (!trimmed) {
      return { repo: '', tag: '' };
    }
    const lastColon = trimmed.lastIndexOf(':');
    const lastSlash = trimmed.lastIndexOf('/');
    const hasExplicitTag = lastColon > lastSlash;
    if (hasExplicitTag && lastColon > 0 && lastColon < trimmed.length - 1) {
      return {
        repo: trimmed.slice(0, lastColon),
        tag: trimmed.slice(lastColon + 1),
      };
    }
    return { repo: trimmed, tag: dateTag() };
  };

  const persistContainerConfig = async (options?: { fallbackRepo?: string }): Promise<boolean> => {
    if (!state.problemId || !state.useContainer) {
      return true;
    }
    const imageRef = String(state.registryImage || '').trim();
    const parsed = parseImageRef(imageRef);
    const repoToUse = parsed.repo || String(options?.fallbackRepo || '').trim();
    if (!repoToUse) {
      window.alert('请先填写镜像仓库地址（可在上传后自动回填）');
      return false;
    }
    if (!state.internalPort || state.internalPort <= 0) {
      window.alert('请填写合法的内部端口');
      return false;
    }
    const sidecarImage = String(state.sidecarImage || '').trim();
    const sidecarPort = Number(state.sidecarPort || 0);
    if (state.enableSidecar && (!sidecarImage || sidecarPort <= 0)) {
      window.alert('启用 sidecar 后，必须同时填写 sidecar 镜像和端口');
      return false;
    }

    setSubmitting(true);
    try {
      await saveProblemContainer(state.problemId, {
        image_name: repoToUse,
        image_tag: parsed.tag || dateTag(),
        challenge_port: state.internalPort,
        sidecar_image: state.enableSidecar ? sidecarImage || null : null,
        sidecar_port: state.enableSidecar ? (sidecarPort > 0 ? sidecarPort : null) : null,
        cpu_limit: `${state.cpuLimit || 500}m`,
        memory_limit: `${state.memoryLimit || 512}Mi`,
        env: {},
        ttl_seconds: 0,
        ingress_host_tpl:
          (state.accessType || 'ingress') === 'ingress'
            ? (state.ingressHostTpl || '{id}.chal.local')
            : null,
      });
      return true;
    } catch (err) {
      const msg = err instanceof Error ? err.message : '保存容器配置失败';
      window.alert(msg);
      return false;
    } finally {
      setSubmitting(false);
    }
  };

  const isDynamicFlag = state.flagType === 'dynamic';

  useEffect(() => {
    if (isDynamicFlag && state.deployMode === 'shared') {
      updateState({ deployMode: 'per_user' });
    }
  }, [isDynamicFlag, state.deployMode, updateState]);

  const handleDeployModeSelect = (deployMode: 'shared' | 'per_user') => {
    const previousMode = state.deployMode || 'per_user';
    if (isDynamicFlag && deployMode === 'shared') {
      window.alert('dynamic flag 仅支持 per_user 部署模式');
      return;
    }
    if (previousMode === deployMode || savingDeployMode) {
      return;
    }
    updateState({ deployMode });
    void persistDeployMode(deployMode).then((ok) => {
      if (!ok) {
        updateState({ deployMode: previousMode });
      }
    });
  };

  const handleNext = async () => {
    if (savingDeployMode || uploading) {
      return;
    }
    const deployMode = state.deployMode || 'per_user';
    if (isDynamicFlag && deployMode === 'shared') {
      window.alert('dynamic flag 不支持 shared 部署，请切换为 per_user');
      return;
    }
    const ok = await persistDeployMode(deployMode);
    if (!ok) {
      return;
    }
    if (state.useContainer) {
      const saved = await persistContainerConfig();
      if (!saved) {
        return;
      }
    }
    nextStep();
  };

  const dateTag = (): string => {
    return new Date().toISOString().slice(0, 10);
  };

  const handleUploadFile = async (file: File) => {
    if (!state.problemId) {
      window.alert('请先完成基础信息并保存草稿');
      return;
    }
    if (!state.useContainer) {
      window.alert('当前已关闭容器模式，请先启用容器后再上传镜像');
      return;
    }
    if (state.imageLocked) {
      window.alert('当前草稿已继承原题镜像配置，不能直接重新上传。请先新建草稿。');
      return;
    }
    if (!state.internalPort || state.internalPort <= 0) {
      window.alert('请先填写内部端口');
      return;
    }
    if (!state.hostPortMin || !state.hostPortMax || state.hostPortMin > state.hostPortMax) {
      window.alert('请填写合法的可用端口范围');
      return;
    }

    const parsedImageRef = parseImageRef(String(state.registryImage || ''));
    const repo = parsedImageRef.repo || `ctf-exec/${state.problemId}`;

    setSubmitting(true);
    setUploading(true);
    try {
      const beforeUploadSaved = await persistContainerConfig({ fallbackRepo: repo });
      if (!beforeUploadSaved) {
        return;
      }
      const task = await uploadProblemImage(state.problemId, {
        file,
        imageRepo: repo,
        imageTag: '',
        uploadType: state.uploadType || 'docker_save',
        hostPortMin: state.hostPortMin,
        hostPortMax: state.hostPortMax,
      });

      const imageRef =
        task.image_repo && task.image_tag
          ? `${task.image_repo}:${task.image_tag}`
          : (state.registryImage && state.registryImage.trim())
            ? state.registryImage
            : `${repo}:${dateTag()}`;
      updateState({
        imageTaskId: task.id,
        imageTaskStatus: 'uploaded',
        imageTaskProgress: 100,
        imageTaskError: undefined,
        registryImage: imageRef,
      });
    } catch (err) {
      const msg = err instanceof Error ? err.message : '上传镜像文件失败';
      updateState({ imageTaskStatus: 'failed', imageTaskError: msg, imageTaskProgress: 100 });
      window.alert(msg);
    } finally {
      setUploading(false);
      setSubmitting(false);
    }
  };

  const hasReadyImage = useMemo(() => {
    if (!state.useContainer) {
      return true;
    }
    if (state.imageLocked) {
      return true;
    }
    const hasFailedTask = state.imageTaskStatus === 'failed';
    if (hasFailedTask) {
      return false;
    }
    if (state.imageTaskStatus === 'uploaded' || state.imageTaskStatus === 'success') {
      return true;
    }
    const hasKnownTask = Boolean(state.imageTaskId || state.imageTaskStatus);
    if (hasKnownTask) {
      return false;
    }
    return Boolean(String(state.registryImage || '').trim());
  }, [state.imageLocked, state.imageTaskId, state.imageTaskStatus, state.registryImage, state.useContainer]);

  return (
    <div className="glass-card p-4 md:p-5 space-y-4 animate-fade-in">
      <div>
        <h2 className="text-xl md:text-2xl font-bold text-gray-900">第四步：容器环境</h2>
        <p className="text-gray-500 mt-1 text-sm">所有题型默认都支持容器。如本题无需容器，可在本步骤直接跳过。</p>
      </div>

      <div className="p-3 border border-amber-200 bg-amber-50 rounded-lg flex items-center justify-between gap-3">
        <div>
          <p className="text-sm font-semibold text-amber-800">本题是否使用容器环境</p>
          <p className="text-xs text-amber-700 mt-0.5">关闭后将跳过镜像、网络和资源配置。</p>
        </div>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => updateState({ useContainer: true })}
            className={`px-3 py-1.5 text-xs rounded-md border ${state.useContainer ? 'bg-amber-600 text-white border-amber-600' : 'bg-white text-gray-700 border-gray-300'}`}
          >
            使用容器
          </button>
          <button
            type="button"
            onClick={() => updateState({
              useContainer: false,
              imageTaskStatus: undefined,
              imageTaskProgress: 0,
              registryImage: undefined,
            })}
            className={`px-3 py-1.5 text-xs rounded-md border ${!state.useContainer ? 'bg-gray-700 text-white border-gray-700' : 'bg-white text-gray-700 border-gray-300'}`}
          >
            无需容器，跳过
          </button>
        </div>
      </div>

      {state.imageLocked && (
        <div className="rounded-xl border border-sky-200 bg-sky-50 px-4 py-3 text-sm text-sky-800">
          当前草稿已继承原题镜像配置，镜像地址与上传文件仅支持查看，不支持直接替换。
        </div>
      )}

      {state.useContainer && (
      <div className="space-y-6 mt-4">

      <div className="p-4 border border-gray-200 rounded-lg shadow-sm bg-gray-50">
        <h3 className="font-bold text-gray-900 mb-2">1. 镜像上传</h3>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-3 mb-3">
          <div>
            <label className="block text-xs font-medium text-gray-700">上传类型</label>
            <select
              value={state.uploadType || 'docker_save'}
              onChange={(e) => updateState({ uploadType: e.target.value as any })}
              disabled={state.imageLocked}
              className="field-select mt-1"
            >
              <option value="docker_save">docker save 镜像包（load）</option>
              <option value="compose_bundle">compose 项目包（解包后 compose）</option>
            </select>
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-700">可用端口最小值</label>
              <input
              type="number"
              value={state.hostPortMin || 20000}
              onChange={(e) => updateState({ hostPortMin: Number(e.target.value) })}
              disabled={state.imageLocked}
              className="field-input mt-1"
              placeholder="20000"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-700">可用端口最大值</label>
              <input
              type="number"
              value={state.hostPortMax || 30000}
              onChange={(e) => updateState({ hostPortMax: Number(e.target.value) })}
              disabled={state.imageLocked}
              className="field-input mt-1"
              placeholder="30000"
            />
          </div>
        </div>
        {!state.imageTaskStatus || state.imageTaskStatus === 'failed' ? (
          <div>
            <div className={`border-2 border-dashed border-gray-300 bg-white rounded-xl p-4 text-center relative mt-2 transition ${state.imageLocked ? 'opacity-70 cursor-not-allowed' : 'cursor-pointer hover:bg-gray-50'}`}>
              <input 
                type="file" 
                accept=".tar,.gz,.tgz"
                disabled={state.imageLocked}
                className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
                onChange={(e) => {
                  const file = e.target.files?.[0];
                  if (file) {
                    void handleUploadFile(file);
                  }
                  e.currentTarget.value = '';
                }}
              />
              <span className="inline-flex items-center justify-center w-9 h-9 rounded-full bg-amber-50 text-amber-700">
                <IconUpload className="w-4 h-4" />
              </span>
              <p className="mt-1.5 font-medium text-gray-800 text-sm">拖拽或点击上传 challenge.tar</p>
              <p className="text-xs text-gray-500 mt-1">支持 docker save 镜像包或 compose 打包文件</p>
            </div>
            <p className="text-xs text-gray-400 mt-2">默认会以日期作为 tag，例如 2026-04-19。</p>
            <input 
              type="text" 
              value={state.registryImage || ''}
              onChange={(e) => updateState({ registryImage: e.target.value })}
              disabled={state.imageLocked}
              className="field-input mt-2"
              placeholder="例如：reg.ctf.local/ctf/web-sqli-2026"
            />
          </div>
        ) : (
          <div className="bg-white p-2.5 border border-amber-200 rounded-lg animate-fade-in">
            <div className="flex items-center justify-between gap-2 mb-1.5">
              <div className="flex items-center space-x-2 min-w-0">
                {state.imageTaskStatus === 'success' ? <span className="text-green-600 text-base">●</span> : <span className="text-amber-600 text-base">●</span>}
                <div>
                  <p className="font-semibold text-gray-900 text-sm">镜像包：已上传并暂存</p>
                  <p className="text-xs text-gray-500">构建会在点击发布后执行</p>
                </div>
              </div>
              {state.imageTaskStatus === 'uploaded' && !state.imageLocked && (
                <button onClick={() => updateState({ imageTaskStatus: undefined, imageTaskProgress: 0, registryImage: undefined })} className="text-xs text-amber-700 hover:text-amber-800 underline">重新上传</button>
              )}
            </div>
            {state.registryImage && (
              <p className="text-[11px] font-mono mt-2 text-gray-600 truncate">{state.registryImage}</p>
            )}
            {state.imageTaskError && (
              <p className="text-[11px] mt-2 text-red-600 whitespace-pre-wrap">{state.imageTaskError}</p>
            )}
          </div>
        )}
      </div>

      <div className="p-4 border border-gray-200 rounded-lg shadow-sm">
         <h3 className="font-bold text-gray-900 mb-3">2. 网络配置</h3>
         <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700">内部端口</label>
              <input type="number" value={state.internalPort || ''} onChange={e => updateState({internalPort: Number(e.target.value)})} className="field-input mt-1" placeholder="80" />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700">访问方式</label>
              <select value={state.accessType || ''} onChange={e => updateState({accessType: e.target.value as any})} className="field-select mt-1">
                <option value="ingress">Ingress（HTTP 子域名）</option>
                <option value="nodeport">NodePort（TCP/UDP）</option>
              </select>
            </div>
         </div>
         {(state.accessType || 'ingress') === 'ingress' && (
           <div className="mt-4">
             <label className="block text-sm font-medium text-gray-700">Ingress Host 模板</label>
             <input
               type="text"
               value={state.ingressHostTpl || ''}
               onChange={(e) => updateState({ ingressHostTpl: e.target.value })}
               className="field-input mt-1"
               placeholder="{id}.chal.local"
             />
             <p className="text-xs text-gray-500 mt-1">可用占位符由后端模板渲染逻辑决定，留空时使用默认模板。</p>
           </div>
         )}
      </div>

      <div className="p-4 border border-gray-200 rounded-lg shadow-sm">
         <h3 className="font-bold text-gray-900 mb-3">3. 资源配额</h3>
         <div className="space-y-4">
           <div>
             <label className="flex justify-between text-sm font-medium text-gray-700">
               <span>CPU 限额</span>
               <span className="font-mono bg-gray-100 px-2 rounded">{state.cpuLimit || 500}m</span>
             </label>
             <input type="range" min="100" max="2000" step="100" value={state.cpuLimit || 500} onChange={e => updateState({cpuLimit: Number(e.target.value)})} className="w-full h-2 bg-gray-200 rounded-lg appearance-none cursor-pointer mt-2" />
           </div>
           <div>
             <label className="flex justify-between text-sm font-medium text-gray-700">
               <span>内存限额</span>
               <span className="font-mono bg-gray-100 px-2 rounded">{state.memoryLimit || 512} Mi</span>
             </label>
             <input type="range" min="128" max="2048" step="128" value={state.memoryLimit || 512} onChange={e => updateState({memoryLimit: Number(e.target.value)})} className="w-full h-2 bg-gray-200 rounded-lg appearance-none cursor-pointer mt-2" />
           </div>
           <div className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2">
             <p className="text-xs text-amber-800">
               实例 TTL 统一由服务端环境变量 <span className="font-mono">DEFAULT_CONTAINER_TTL</span> 控制（留空默认 3600 秒），题目级不再可配置。
             </p>
           </div>
         </div>
      </div>

      <div className="p-4 border border-gray-200 rounded-lg shadow-sm">
         <h3 className="font-bold text-gray-900 mb-3">4. 部署与安全</h3>
         {isDynamicFlag && (
           <p className="mb-3 text-xs text-amber-700">当前 Flag 类型为 dynamic，部署模式已强制为 per_user。</p>
         )}
         <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <label className={`cursor-pointer flex flex-col p-4 rounded-lg border-2 ${state.deployMode === 'shared' ? 'border-amber-500 bg-amber-50' : 'border-gray-200'} ${savingDeployMode || isDynamicFlag ? 'opacity-60' : ''}`} onClick={() => handleDeployModeSelect('shared')}>
             <span className="font-bold text-gray-900">shared（共享容器）</span>
             <span className="text-sm text-gray-500 mt-1">适合无状态 Web 场景，成本更低。</span>
           </label>
          <label className={`cursor-pointer flex flex-col p-4 rounded-lg border-2 ${state.deployMode === 'per_user' ? 'border-amber-500 bg-amber-50' : 'border-gray-200'} ${savingDeployMode ? 'opacity-60' : ''}`} onClick={() => handleDeployModeSelect('per_user')}>
             <span className="font-bold text-gray-900">per_user（独立容器）</span>
             <span className="text-sm text-gray-500 mt-1">每位选手独立实例，推荐 Pwn 与动态 Flag。</span>
           </label>
         </div>
      </div>

      <div className="p-4 border border-gray-200 rounded-lg shadow-sm">
         <div className="flex items-center justify-between mb-3">
           <div>
             <h3 className="font-bold text-gray-900">5. Sidecar（可选）</h3>
             <p className="text-xs text-gray-500 mt-0.5">用于动态 Flag 注入或题面访客桥接，未启用则保持空。</p>
           </div>
           <label className="inline-flex items-center cursor-pointer gap-2">
             <input
               type="checkbox"
               checked={state.enableSidecar}
               onChange={(e) => updateState({ enableSidecar: e.target.checked })}
               className="h-4 w-4 accent-amber-500"
             />
             <span className="text-sm text-gray-700">启用 sidecar</span>
           </label>
         </div>
         {state.enableSidecar && (
           <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
             <div>
               <label className="block text-sm font-medium text-gray-700">Sidecar 镜像</label>
               <input
                 type="text"
                 value={state.sidecarImage || ''}
                 onChange={(e) => updateState({ sidecarImage: e.target.value })}
                 className="field-input mt-1"
                 placeholder="例如：reg.ctf.local/ctf/web-flag-injector:2026-04"
               />
             </div>
             <div>
               <label className="block text-sm font-medium text-gray-700">Sidecar 端口</label>
               <input
                 type="number"
                 value={state.sidecarPort || ''}
                 onChange={(e) => updateState({ sidecarPort: Number(e.target.value) })}
                 className="field-input mt-1"
                 placeholder="9999"
               />
             </div>
           </div>
         )}
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
          onClick={() => { void handleNext(); }}
          disabled={!hasReadyImage || savingDeployMode || uploading}
          className="btn-primary !py-2 !px-4 text-sm disabled:opacity-50"
        >
          {uploading ? '处理中...' : '下一步：发布检查'}
        </button>
      </div>
    </div>
  );
}
