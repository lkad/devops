import { useState, useEffect } from 'react'
import { Modal } from '@/components/ui/Modal'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { Button } from '@/components/ui/Button'
import { useToast } from '@/components/ui/Toast'
import type { Pipeline, CreatePipelineRequest } from '@/api/endpoints/pipelines'
import styles from './PipelineForm.module.css'

const AVAILABLE_STAGES = [
  'validate',
  'build',
  'test',
  'security_scan',
  'stage_deploy',
  'smoke_test',
  'prod_deploy',
  'verification',
]

const STRATEGY_OPTIONS = [
  { value: 'blue_green', label: 'Blue-Green' },
  { value: 'canary', label: 'Canary' },
  { value: 'rolling', label: 'Rolling' },
]

export interface PipelineFormProps {
  isOpen: boolean
  onClose: () => void
  onSuccess: () => void
  pipeline?: Pipeline
}

export function PipelineForm({ isOpen, onClose, onSuccess, pipeline }: PipelineFormProps) {
  const { addToast } = useToast()
  const isEdit = !!pipeline

  const [name, setName] = useState('')
  const [stages, setStages] = useState<string[]>([])
  const [strategy, setStrategy] = useState<'blue_green' | 'canary' | 'rolling'>('rolling')
  const [canarySteps, setCanarySteps] = useState('')
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [isSubmitting, setIsSubmitting] = useState(false)

  // Reset form when modal opens/closes or pipeline changes
  useEffect(() => {
    if (isOpen) {
      if (pipeline) {
        setName(pipeline.name)
        setStages(pipeline.stages)
        setStrategy(pipeline.deployConfig.strategy)
        if (pipeline.deployConfig.canary?.steps) {
          setCanarySteps(pipeline.deployConfig.canary.steps.join(', '))
        } else {
          setCanarySteps('')
        }
      } else {
        setName('')
        setStages([])
        setStrategy('rolling')
        setCanarySteps('')
      }
      setErrors({})
    }
  }, [isOpen, pipeline])

  const toggleStage = (stage: string) => {
    setStages((prev) =>
      prev.includes(stage)
        ? prev.filter((s) => s !== stage)
        : [...prev, stage]
    )
  }

  const validate = (): boolean => {
    const newErrors: Record<string, string> = {}

    if (!name.trim()) {
      newErrors.name = 'Name is required'
    }

    if (stages.length === 0) {
      newErrors.stages = 'At least one stage is required'
    }

    if (strategy === 'canary') {
      const steps = canarySteps
        .split(',')
        .map((s) => parseInt(s.trim(), 10))
        .filter((n) => !isNaN(n))

      if (steps.length === 0) {
        newErrors.canarySteps = 'At least one canary step is required'
      } else if (steps.some((s) => s <= 0 || s > 100)) {
        newErrors.canarySteps = 'Steps must be percentages between 1 and 100'
      }
    }

    setErrors(newErrors)
    return Object.keys(newErrors).length === 0
  }

  const handleSubmit = async () => {
    if (!validate()) return

    setIsSubmitting(true)

    try {
      const deployConfig: CreatePipelineRequest['deployConfig'] = {
        strategy,
      }

      if (strategy === 'canary') {
        const steps = canarySteps
          .split(',')
          .map((s) => parseInt(s.trim(), 10))
          .filter((n) => !isNaN(n))
        deployConfig.canary = { steps }
      }

      const data: CreatePipelineRequest = {
        name: name.trim(),
        stages,
        deployConfig,
      }

      if (isEdit && pipeline) {
        const { pipelinesApi } = await import('@/api/endpoints/pipelines')
        await pipelinesApi.update(pipeline.id, data)
        addToast({ type: 'success', message: 'Pipeline updated successfully' })
      } else {
        const { pipelinesApi } = await import('@/api/endpoints/pipelines')
        await pipelinesApi.create(data)
        addToast({ type: 'success', message: 'Pipeline created successfully' })
      }

      onSuccess()
      onClose()
    } catch (err) {
      addToast({ type: 'error', message: isEdit ? 'Failed to update pipeline' : 'Failed to create pipeline' })
    } finally {
      setIsSubmitting(false)
    }
  }

  const footer = (
    <>
      <Button variant="ghost" onClick={onClose} disabled={isSubmitting}>
        Cancel
      </Button>
      <Button variant="primary" onClick={handleSubmit} loading={isSubmitting}>
        {isEdit ? 'Update' : 'Create'}
      </Button>
    </>
  )

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={isEdit ? 'Edit Pipeline' : 'Create Pipeline'}
      footer={footer}
      className={styles.modal}
    >
      <div className={styles.form}>
        <Input
          label="Name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="e.g., production-deploy"
          error={errors.name}
          required
        />

        <div className={styles.field}>
          <label className={styles.label}>Stages</label>
          <div className={styles.stagesGrid}>
            {AVAILABLE_STAGES.map((stage) => (
              <button
                key={stage}
                type="button"
                className={`${styles.stageTag} ${stages.includes(stage) ? styles.stageTagSelected : ''}`}
                onClick={() => toggleStage(stage)}
              >
                {stage.replace(/_/g, ' ')}
              </button>
            ))}
          </div>
          {errors.stages && <span className={styles.error}>{errors.stages}</span>}
        </div>

        <Select
          label="Deployment Strategy"
          value={strategy}
          onChange={(e) => setStrategy(e.target.value as typeof strategy)}
          options={STRATEGY_OPTIONS}
        />

        {strategy === 'canary' && (
          <Input
            label="Canary Steps"
            value={canarySteps}
            onChange={(e) => setCanarySteps(e.target.value)}
            placeholder="e.g., 1, 5, 25, 100"
            error={errors.canarySteps}
          />
        )}
      </div>
    </Modal>
  )
}