import { useSession } from '@/entities/session'
import { PURPOSE_LIMITS, usePurposes, type Purpose } from '@/entities/purpose'
import { CreatePurposeForm } from '@/features/create-purpose'
import { DeletePurposeButton } from '@/features/delete-purpose'
import { EditablePurposeField, useUpdatePurpose } from '@/features/edit-purpose'
import { Badge, Button, Notice } from '@/shared/ui'

/** The account's 용도 briefs (plan 11). Composition only — every action is its own feature.
 *
 *  Nothing on this screen calls a model or enqueues a job: a purpose is authored text, and
 *  reading, editing or deleting one is a plain CRUD round trip ([I5]). */
export function PurposesPage() {
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { purposes, isPending, isError, isFetching, refetch } = usePurposes(ownerId)

  return (
    <main className="mx-auto w-full max-w-2xl px-4 py-8 sm:px-6">
      <h1 className="text-2xl font-semibold tracking-tight">용도</h1>
      <p className="text-content-secondary max-w-measure mt-2 text-sm leading-relaxed">
        용도는 글의 종류와 구성을 정해요. 글마다 하나를 고르면 AI가 그 지침대로 씁니다. 문체와
        종결어미는 그대로 말투 프로필을 따라요.
      </p>

      {isError && (
        <Notice tone="danger" role="alert" className="mt-8">
          <span>용도 목록을 불러오지 못했어요.</span>
          <Button
            variant="ghost"
            onClick={refetch}
            pending={isFetching}
            className="text-notice-danger-fg underline"
          >
            다시 시도
          </Button>
        </Notice>
      )}
      {!isError && isPending && (
        <p role="status" className="text-content-tertiary mt-8 text-sm">
          불러오는 중…
        </p>
      )}

      {!isError && !isPending && (
        <>
          {purposes.length === 0 ? (
            <EmptyState />
          ) : (
            <section aria-labelledby="purposes-heading" className="mt-8">
              <h2 id="purposes-heading" className="text-lg font-semibold tracking-tight">
                저장된 용도
              </h2>
              <ul className="divide-divider mt-3 divide-y">
                {purposes.map((purpose) => (
                  <PurposeRow key={purpose.id} ownerId={ownerId} purpose={purpose} />
                ))}
              </ul>
            </section>
          )}

          <section aria-labelledby="create-purpose-heading" className="mt-10">
            <h2 id="create-purpose-heading" className="text-lg font-semibold tracking-tight">
              새 용도
            </h2>
            <CreatePurposeForm ownerId={ownerId} className="mt-3" />
          </section>
        </>
      )}
    </main>
  )
}

/** The worked example is copy, not a row: nothing here creates a purpose the user did not
 *  author (plan 11 — no seeded presets, no guessing). */
function EmptyState() {
  return (
    <section aria-labelledby="purposes-empty-heading" className="mt-8">
      <h2 id="purposes-empty-heading" className="text-lg font-semibold tracking-tight">
        아직 저장된 용도가 없어요
      </h2>
      <p className="text-content-secondary max-w-measure mt-2 text-sm leading-relaxed">
        메모에 매번 다시 쓰던 설명을 여기에 한 번만 저장해 두세요. 예를 들어 이런 식이에요.
      </p>
      <dl className="text-content-secondary mt-4 space-y-2 text-sm leading-relaxed">
        <div>
          <dt className="text-content-tertiary text-xs font-medium">이름</dt>
          <dd className="text-content-primary">정보성 식당 리뷰</dd>
        </div>
        <div>
          <dt className="text-content-tertiary text-xs font-medium">어떤 글인가요</dt>
          <dd>식사를 제공받고 쓰는 방문 리뷰</dd>
        </div>
        <div>
          <dt className="text-content-tertiary text-xs font-medium">작성 지침</dt>
          <dd className="whitespace-pre-wrap">
            {'사진마다 무엇인지 설명하세요.\n일기체로 쓰지 마세요.\n방문 정보를 마지막에 적으세요.'}
          </dd>
        </div>
      </dl>
    </section>
  )
}

/** One saved purpose: three read-first fields, the assignment count, and the delete.
 *
 *  Each field saves on its own so two of them edited from two places cannot overwrite each
 *  other (spec/policy/purposes.md). One mutation hook serves all three — they never run at
 *  the same time, and sharing it keeps one refusal message under one field. */
function PurposeRow({ ownerId, purpose }: { ownerId: string; purpose: Purpose }) {
  const update = useUpdatePurpose(ownerId, purpose.id)
  return (
    <li className="py-4">
      <EditablePurposeField
        label="이름"
        value={purpose.name}
        limit={PURPOSE_LIMITS.name}
        placeholder="예: 정보성 식당 리뷰"
        save={update.saveName}
        errorMessage={update.errorMessage}
        pending={update.isPending}
      />
      <EditablePurposeField
        label="어떤 글인가요"
        value={purpose.description}
        limit={PURPOSE_LIMITS.description}
        multiline
        optional
        placeholder="예: 식사를 제공받고 쓰는 방문 리뷰"
        emptyText="설명 없음"
        save={update.saveDescription}
        errorMessage={update.errorMessage}
        pending={update.isPending}
        className="mt-4"
      />
      <EditablePurposeField
        label="작성 지침"
        value={purpose.instructions}
        limit={PURPOSE_LIMITS.instructions}
        multiline
        placeholder="예: 사진마다 무엇인지 설명하세요."
        save={update.saveInstructions}
        errorMessage={update.errorMessage}
        pending={update.isPending}
        className="mt-4"
      />
      <div className="mt-4 flex flex-wrap items-center gap-2">
        <Badge tone="neutral">글 {purpose.postCount}개</Badge>
        <DeletePurposeButton ownerId={ownerId} purpose={purpose} />
      </div>
    </li>
  )
}
