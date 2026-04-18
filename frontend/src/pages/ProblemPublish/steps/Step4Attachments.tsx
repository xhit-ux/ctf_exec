import React from 'react';
import { useProblemPublish } from '../ProblemPublishContext';
import { IconFile, IconUpload } from '../icons';

const DENY_EXT = ['.exe', '.sh', '.bat'];

async function calcSha256(file: File): Promise<string> {
  const buf = await file.arrayBuffer();
  const hash = await crypto.subtle.digest('SHA-256', buf);
  const hashArray = Array.from(new Uint8Array(hash));
  return hashArray.map((b) => b.toString(16).padStart(2, '0')).join('');
}

export default function Step4Attachments() {
  const { state, updateState, nextStep, prevStep } = useProblemPublish();

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!e.target.files || e.target.files.length === 0) {
      return;
    }

    if (e.target.files.length > 1) {
      alert('仅支持上传单个文件。如有多个文件，请先打包后再上传。');
    }

    const file = e.target.files[0];
    const ext = file.name.slice(file.name.lastIndexOf('.')).toLowerCase();
    if (DENY_EXT.includes(ext)) {
      alert('该文件类型不允许上传为附件');
      return;
    }

    const sha = await calcSha256(file);
    const next = [{
      id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
      filename: file.name,
      size: file.size,
      sha256: sha,
      status: 'success' as const,
    }];

    updateState({ attachments: next });
    e.target.value = '';
  };

  const removeAttachment = (id: string) => {
    updateState({ attachments: state.attachments.filter(a => a.id !== id) });
  };

  const handleNext = () => {
    nextStep();
  };

  return (
    <div className="glass-card p-4 md:p-5 space-y-5 animate-fade-in">
      <div>
        <h2 className="text-xl md:text-2xl font-bold text-gray-900">第三步：附件上传</h2>
        <p className="text-gray-500 mt-1 text-sm">
          仅支持上传一份文件，前端会自动计算 SHA256。若有多个文件，请先打包后再上传。
        </p>
      </div>

      <div className="border-2 border-dashed border-gray-300 rounded-xl p-6 text-center hover:bg-gray-50 transition cursor-pointer relative">
        <input 
          type="file" 
          onChange={handleFileUpload} 
          className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
        />
        <div className="mb-2 inline-flex items-center justify-center w-10 h-10 rounded-full bg-amber-50 text-amber-700">
          <IconUpload className="w-5 h-5" />
        </div>
        <p className="text-gray-700 font-medium">点击或拖拽文件到这里上传</p>
        <p className="text-sm text-gray-500 mt-1">若需要上传多个文件，请先压缩为 zip/tar 后上传</p>
      </div>

      {state.attachments.length > 0 && (
        <div className="mt-6 space-y-3">
          <h3 className="text-sm font-semibold text-gray-900">已上传文件</h3>
          {state.attachments.map(att => (
            <div key={att.id} className="flex items-center justify-between p-2.5 bg-gray-50 border border-gray-200 rounded-lg">
              <div className="flex items-center">
                <span className="mr-2 text-gray-600">
                  <IconFile className="w-5 h-5" />
                </span>
                <div>
                  <p className="font-medium text-gray-900">{att.filename}</p>
                  <div className="flex flex-wrap gap-3 text-xs text-gray-500 mt-1">
                    <span>{(att.size / 1024 / 1024).toFixed(2)} MB</span>
                    <span className="font-mono text-gray-400">SHA256: {att.sha256.slice(0, 12)}...</span>
                  </div>
                </div>
              </div>
              <button 
                onClick={() => removeAttachment(att.id)}
                className="btn-danger !py-1.5 !px-3 text-xs"
              >
                删除
              </button>
            </div>
          ))}
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
          className="btn-primary !py-2 !px-4 text-sm disabled:opacity-50"
        >
          下一步：容器环境
        </button>
      </div>
    </div>
  );
}
