import { useId, useState, type ChangeEvent } from 'react'
import { Link } from '@tanstack/react-router'
import {
  NO_PURPOSE_LABEL,
  NO_PURPOSE_VALUE,
  usePurposes,
  type PurposeRef,
} from '@/entities/purpose'
import { Button, FieldLabel, FieldMessage, Select } from '@/shared/ui'
import { RUNNING_JOB_NOTE, assignmentFailureMessage } from '../model/assignment'

interface PostPurposeSelectProps {
  ownerId: string
  /** The assignment as the editor shows it; '' is 없음. */
  value: string
  /** The post's purpose as the server reports it — undefined for a draft with no post yet.
   *  Listed even while the directory loads, so a post that plainly has one is never shown
   *  as 없음 for a beat. */
  current?: PurposeRef
  /** True while an AI job could still be running. The select stays usable either way; this
   *  only decides whether the note about the frozen brief is shown. */
  jobRunning?: boolean
  onSelect: (purposeId: string) => Promise<void> | void
  className?: string
}

/** The optional 용도 of a post: a native select wearing the field well (design-language §7),
 *  defaulting to 없음, beside the required voice select. */
export function PostPurposeSelect({
  ownerId,
  value,
  current,
  jobRunning = false,
  onSelect,
  className,
}: PostPurposeSelectProps) {
  const id = useId()
  const hintId = `${id}-hint`
  const errorId = `${id}-error`
  const { purposes, isPending, isError, isFetching, refetch } = usePurposes(ownerId)
  const [applying, setApplying] = useState(false)
  const [error, setError] = useState('')

  // The post's own purpose is listed even while the directory is still loading; a select
  // showing 없음 under a post that has one reads as if the assignment were lost.
  const unlisted =
    current && current.id && !purposes.some((purpose) => purpose.id === current.id)
      ? current
      : undefined
  const selected = purposes.find((purpose) => purpose.id === value)
  const describedBy = [jobRunning ? hintId : '', error || isError ? errorId : '']
    .filter(Boolean)
    .join(' ')

  const apply = async (purposeId: string) => {
    setApplying(true)
    setError('')
    try {
      await onSelect(purposeId)
    } catch (cause) {
      setError(assignmentFailureMessage(cause))
    } finally {
      setApplying(false)
    }
  }

  const onChange = (event: ChangeEvent<HTMLSelectElement>) => {
    const next = event.target.value
    if (next === value) return
    // No confirmation sheet, unlike the voice: changing a purpose moves no content, drops no
    // baseline and costs no learning, so there is nothing to warn about.
    void apply(next)
  }

  return (
    <div className={className}>
      <div className="flex items-center gap-3">
        <FieldLabel htmlFor={id} className="shrink-0">
          용도
        </FieldLabel>
        <span className="min-w-0 flex-1">
          <Select
            id={id}
            value={value}
            onChange={onChange}
            disabled={applying || isError || (isPending && !unlisted)}
            aria-invalid={error || isError ? true : undefined}
            aria-describedby={describedBy || undefined}
          >
            <option value={NO_PURPOSE_VALUE}>{NO_PURPOSE_LABEL}</option>
            {unlisted && <option value={unlisted.id}>{unlisted.name || unlisted.id}</option>}
            {purposes.map((purpose) => (
              <option key={purpose.id} value={purpose.id}>
                {purpose.name}
              </option>
            ))}
          </Select>
        </span>
      </div>
      {selected?.description && (
        <p className="text-content-secondary mt-2 text-sm break-words">{selected.description}</p>
      )}
      {jobRunning && (
        <p id={hintId} role="status" className="text-content-secondary mt-2 text-sm">
          {RUNNING_JOB_NOTE}
        </p>
      )}
      {/* A failed directory read must not render as "you have no 용도": the select would be
          enabled with 없음 alone, and the only thing the user could do is clear a purpose they
          still have. Disabled, said out loud, with a retry. */}
      {isError && (
        <FieldMessage id={errorId} className="mt-2">
          용도 목록을 불러오지 못했어요.{' '}
          <Button
            variant="ghost"
            onClick={refetch}
            pending={isFetching}
            className="text-field-error underline"
          >
            다시 시도
          </Button>
        </FieldMessage>
      )}
      {error && (
        <FieldMessage id={errorId} className="mt-2">
          {error}
        </FieldMessage>
      )}
      <p className="mt-2 text-sm">
        <Link to="/purposes" className="text-content-secondary underline underline-offset-2">
          용도 관리
        </Link>
      </p>
    </div>
  )
}
