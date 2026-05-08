import React from 'react';
import { ProblemPublishProvider, useProblemPublish } from './ProblemPublishContext';
import Step1BasicInfo from './steps/Step1BasicInfo';
import Step2FlagConfig from './steps/Step2FlagConfig';
import Step3Attachments from './steps/Step3Attachments';
import Step4ContainerEnv from './steps/Step4ContainerEnv';
import Step5Review from './steps/Step5Review';

function WizardSteps() {
  const { currentStep } = useProblemPublish();

  const steps = [
    { id: 1, label: '基础信息', hint: '分类、标题与分值' },
    { id: 2, label: 'Flag 配置', hint: '静态或动态策略' },
    { id: 3, label: '附件上传', hint: '资料与哈希校验' },
    { id: 4, label: '容器环境', hint: '镜像与资源限制' },
    { id: 5, label: '发布检查', hint: '守卫校验与确认' },
  ];

  return (
    <div className="w-full max-w-[1200px] mx-auto px-3 md:px-4 py-4">
      <aside className="hidden lg:block fixed top-24 w-[170px] z-20" style={{ left: 'max(12px, calc(50% - 600px + 12px))' }}>

        <ul className="relative h-[calc(100vh-380px)] min-h-[420px] max-h-[500px] grid grid-rows-5 pt-6 pb-6">
          {steps.map((step) => {
            const isActive = currentStep === step.id;
            const isCompleted = currentStep > step.id;
            return (
              <li key={step.id} className="relative flex items-center">
                <div className="pl-10 pr-1 py-1">
                  {isActive && (
                    <>
                      <span className="absolute left-[14px] top-1/2 -translate-x-1/2 -translate-y-1/2 h-[114px] w-3 rounded-full pp-step-segment-glow" />
                      <span className="absolute left-[14px] top-1/2 -translate-x-1/2 -translate-y-1/2 h-[92px] w-[2px] rounded-full pp-step-segment" />
                    </>
                  )}
                  <span
                    className={`absolute left-[9px] top-1/2 -translate-y-1/2 w-[10px] h-[10px] rounded-full border ${
                      isActive
                        ? 'pp-step-node--active bg-amber-500 border-amber-500'
                        : isCompleted
                          ? 'bg-emerald-500 border-emerald-500 shadow-[0_0_0_2px_rgba(16,185,129,0.08)]'
                          : 'bg-white border-gray-300'
                    }`}
                  ></span>
                  <p className={`text-[11px] leading-4 ${isActive ? 'text-amber-700 font-semibold' : isCompleted ? 'text-emerald-700' : 'text-gray-400'}`}>
                    {String(step.id).padStart(2, '0')}
                  </p>
                  <p className={`text-sm leading-5 ${isActive ? 'text-amber-900 font-semibold' : isCompleted ? 'text-slate-800 font-medium' : 'text-gray-600'}`}>
                    {step.label}
                  </p>
                  <p className={`text-[11px] leading-4 ${isActive ? 'text-amber-700/80' : 'text-gray-400'}`}>
                    {step.hint}
                  </p>
                </div>
              </li>
            );
          })}
        </ul>
      </aside>

      <div className="lg:ml-[190px]">
        <div className="page-header mb-3">
          <div className="page-header__main">
            <h1>题目发布向导</h1>
            <p>按步骤完成题目配置，支持草稿式编辑与发布前守卫检查。</p>
          </div>
        </div>

        <div className="mt-1">
          {currentStep === 1 && <Step1BasicInfo />}
          {currentStep === 2 && <Step2FlagConfig />}
          {currentStep === 3 && <Step3Attachments />}
          {currentStep === 4 && <Step4ContainerEnv />}
          {currentStep === 5 && <Step5Review />}
        </div>
      </div>
    </div>
  );
}

export default function ProblemPublish() {
  return (
    <ProblemPublishProvider>
      <div className="app-shell">
        <WizardSteps />
      </div>
    </ProblemPublishProvider>
  );
}
