import React, { useEffect, useState } from 'react'
import { Navigate, useLocation, useNavigate, useParams } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'
import { useProblemPublish, ProblemPublishProvider } from './ProblemPublishContext'
import { initialProblemState } from './types'
import Step1BasicInfo from './steps/Step1BasicInfo'
import Step2FlagConfig from './steps/Step2FlagConfig'
import Step3Attachments from './steps/Step3Attachments'
import Step4ContainerEnv from './steps/Step4ContainerEnv'
import Step5Review from './steps/Step5Review'
import { createProblemDraft, fetchProblemPublishDraft } from '../../utils/problemPublishApi'

function parseDifficulty(value?: string): 'Easy' | 'Medium' | 'Hard' | 'Expert' {
  switch ((value || '').toLowerCase()) {
    case 'easy':
      return 'Easy'
    case 'medium':
      return 'Medium'
    case 'hard':
      return 'Hard'
    default:
      return 'Expert'
  }
}

function parseNumber(value?: string | number | null): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value
  }
  if (typeof value === 'string') {
    const parsed = Number.parseInt(value.replace(/[^0-9.-]/g, ''), 10)
    return Number.isFinite(parsed) ? parsed : undefined
  }
  return undefined
}

function WizardSteps() {
  const { currentStep, state } = useProblemPublish()

  const steps = [
    { id: 1, label: '基础信息', hint: '分类、标题与分值' },
    { id: 2, label: 'Flag 配置', hint: '静态或动态策略' },
    { id: 3, label: '附件上传', hint: '资料与哈希校验' },
    { id: 4, label: '容器环境', hint: '镜像与资源限制' },
    { id: 5, label: '发布检查', hint: '守卫校验与确认' },
  ]

  return (
    <div className="w-full max-w-[1200px] mx-auto px-3 md:px-4 py-4">
      <aside className="hidden lg:block fixed top-24 w-[170px] z-20" style={{ left: 'max(12px, calc(50% - 600px + 12px))' }}>
        <ul className="relative h-[calc(100vh-380px)] min-h-[420px] max-h-[500px] grid grid-rows-5 pt-6 pb-6">
          {steps.map((step) => {
            const isActive = currentStep === step.id
            const isCompleted = currentStep > step.id
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
                  />
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
            )
          })}
        </ul>
      </aside>

      <div className="lg:ml-[190px]">
        <div className="page-header mb-3">
          <div className="page-header__main">
            <h1>{state.isEditingExisting ? '编辑题目草稿' : '题目发布向导'}</h1>
            <p>{state.isEditingExisting ? '已加载原题内容到草稿中，完成修改后直接发布回原题。' : '按步骤完成题目配置，支持草稿式编辑与发布前守卫检查。'}</p>
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
  )
}

function ProblemPublishBootstrap() {
  const { problemId } = useParams<{ problemId?: string }>()
  const navigate = useNavigate()
  const location = useLocation()
  const { isAuthenticated } = useAuth()
  const { state, updateState, resetState, setStep } = useProblemPublish()
  const [bootstrapping, setBootstrapping] = useState(true)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)

  useEffect(() => {
    let active = true
    const isEditRoute = Boolean(problemId && problemId !== 'new')

    const loadDraft = async () => {
      if (!isAuthenticated) {
        setBootstrapping(false)
        return
      }

      setBootstrapping(true)
      setErrorMessage(null)
      resetState()

      try {
        if (!problemId || problemId === 'new') {
          setStep(1)
          setBootstrapping(false)
          return
        }

        const detail = await fetchProblemPublishDraft(problemId)
        if (!active) {
          return
        }

        if (isEditRoute && !detail.source_problem_id) {
          const draftProblemID = await createProblemDraft({
            category: detail.category as any,
            type: detail.type,
            source_problem_id: detail.id,
          })
          if (!active) {
            return
          }
          // 保持 bootstrapping=true，navigate 触发 problemId 变更后
          // useEffect 第二周期会 fetch draft 数据并填充表单
          navigate(`/problems/${draftProblemID}/edit`, { replace: true })
          return
        }

        updateState({
          ...initialProblemState,
          problemId: detail.id,
          sourceProblemId: detail.source_problem_id ?? undefined,
          isEditingExisting: true,
          attachmentLocked: Boolean(detail.attachment?.id),
          imageLocked: Boolean(detail.container?.image_name || detail.container?.image_tag || detail.build_task),
          title: detail.title ?? '',
          author: detail.author ?? '',
          category: detail.category as any,
          difficulty: parseDifficulty(detail.difficulty),
          platformScore: detail.platform_score ?? 0,
          description: detail.description ?? '',
          tags: detail.tags ?? [],
          status: detail.status === 'hidden' ? 'hidden' : 'ready',
          flagType: detail.flag?.flag_type ?? 'static',
          flagRule: detail.flag?.flag_rule ?? 'exact',
          flagValue: detail.flag?.flag_example ?? detail.flag?.flag_pattern ?? '',
          flagExample: detail.flag?.flag_example ?? '',
          flagInject: detail.flag?.flag_inject ?? 'env',
          flagEnvKey: detail.flag?.flag_env_key ?? '',
          flagSidecarPattern: detail.flag?.flag_sidecar_pattern ?? '',
          attachment: detail.attachment
            ? {
                id: detail.attachment.id,
                filename: detail.attachment.file_name ?? detail.attachment.file_path ?? 'attachment',
                size: detail.attachment.file_size ?? 0,
                sha256: detail.attachment.sha256 ?? '',
                status: 'success',
              }
            : null,
          imageTaskId: detail.build_task?.id,
          imageTaskStatus: detail.build_task?.status as any,
          imageTaskProgress: detail.build_task?.status === 'success' ? 100 : detail.build_task?.status ? 15 : 0,
          imageTaskError: detail.build_task?.error_log ?? undefined,
          uploadType: 'docker_save',
          registryImage:
            detail.container?.image_name && detail.container?.image_tag
              ? `${detail.container.image_name}:${detail.container.image_tag}`
              : detail.container?.image_name ?? undefined,
          internalPort: detail.container?.challenge_port ?? 0,
          sidecarImage: detail.container?.sidecar_image ?? '',
          sidecarPort: detail.container?.sidecar_port ?? 0,
          cpuLimit: parseNumber(detail.container?.cpu_limit) ?? 500,
          memoryLimit: parseNumber(detail.container?.memory_limit) ?? 512,
          ttlSeconds: detail.container?.ttl_seconds ?? 0,
          ingressHostTpl: detail.container?.ingress_host_tpl ?? '',
          useContainer: detail.type !== 'static',
          enableSidecar: Boolean(detail.container?.sidecar_image || detail.container?.sidecar_port),
          deployMode: detail.deploy_mode === 'shared' ? 'shared' : 'per_user',
          hostPortMin: 20000,
          hostPortMax: 30000,
        })
        setStep(1)
      } catch (err) {
        if (!active) {
          return
        }
        setErrorMessage(err instanceof Error ? err.message : '加载草稿失败')
      } finally {
        if (active) {
          setBootstrapping(false)
        }
      }
    }

    void loadDraft()

    return () => {
      active = false
    }
  }, [isAuthenticated, problemId, resetState, setStep, updateState])

  useEffect(() => {
    if (!isAuthenticated) {
      return
    }
    if (problemId !== 'new' && problemId !== undefined) {
      return
    }
    if (location.pathname === '/problems/new') {
      navigate('/problems/new', { replace: true })
    }
  }, [isAuthenticated, location.pathname, navigate, problemId])

  if (!isAuthenticated) {
    return <Navigate to="/login" replace state={{ from: location }} />
  }

  if (bootstrapping) {
    return (
      <div className="app-shell">
        <div className="mx-auto max-w-3xl px-4 py-16 text-center text-slate-600">
          正在加载题目草稿...
        </div>
      </div>
    )
  }

  if (errorMessage) {
    return (
      <div className="app-shell">
        <div className="mx-auto max-w-3xl px-4 py-16">
          <div className="rounded-2xl border border-rose-200 bg-rose-50 px-5 py-4 text-rose-800 shadow-sm">
            <p className="font-medium">{errorMessage}</p>
          </div>
        </div>
      </div>
    )
  }

  return <WizardSteps />
}

export default function ProblemPublish() {
  return (
    <ProblemPublishProvider>
      <div className="app-shell">
        <ProblemPublishBootstrap />
      </div>
    </ProblemPublishProvider>
  )
}
