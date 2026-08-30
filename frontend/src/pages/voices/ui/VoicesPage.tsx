import { Link } from '@tanstack/react-router'
import { useSession } from '@/entities/session'
import { useVoices, type Voice } from '@/entities/voice'
import { CreateVoiceForm } from '@/features/create-voice'
import { DeleteVoiceButton } from '@/features/delete-voice'
import { RenameVoiceField } from '@/features/rename-voice'
import { RestoreVoiceButton } from '@/features/restore-voice'
import { SetDefaultVoiceButton } from '@/features/set-default-voice'
import { Badge, Button, Notice } from '@/shared/ui'

/** The account's voices (PRD §3.4): the active ones first, then the tombstones. Composition only —
 *  every action is its own feature, and the rows are links into one voice's profile. */
export function VoicesPage() {
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { active, deleted, isPending, isError, isFetching, refetch } = useVoices(ownerId)

  return (
    <main className="mx-auto w-full max-w-2xl px-4 py-8 sm:px-6">
      <h1 className="text-2xl font-semibold tracking-tight">말투</h1>
      <p className="text-content-secondary max-w-measure mt-2 text-sm leading-relaxed">
        말투마다 프로필과 학습 기록이 따로 쌓여요. 새 글은 기본 말투로 시작하고, 글마다 다른 말투를
        고를 수 있어요.
      </p>

      {isError && (
        <Notice tone="danger" role="alert" className="mt-8">
          <span>말투 목록을 불러오지 못했어요.</span>
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
          <section aria-labelledby="active-voices-heading" className="mt-8">
            <h2 id="active-voices-heading" className="text-lg font-semibold tracking-tight">
              사용 중
            </h2>
            <ul className="divide-divider mt-3 divide-y">
              {active.map((voice) => (
                <VoiceRow key={voice.id} ownerId={ownerId} voice={voice} />
              ))}
            </ul>
          </section>

          <section aria-labelledby="create-voice-heading" className="mt-10">
            <h2 id="create-voice-heading" className="text-lg font-semibold tracking-tight">
              새 말투
            </h2>
            <CreateVoiceForm ownerId={ownerId} className="mt-3" />
          </section>

          {deleted.length > 0 && (
            <section aria-labelledby="deleted-voices-heading" className="mt-12">
              <h2 id="deleted-voices-heading" className="text-lg font-semibold tracking-tight">
                삭제된 말투
              </h2>
              <p className="text-content-secondary mt-2 text-sm leading-relaxed">
                글과 학습 기록은 그대로 남아 있어요. 복원하면 다시 고를 수 있고, 같은 이름의 말투가
                이미 있으면 먼저 이름을 바꿔 주세요.
              </p>
              <ul className="divide-divider mt-3 divide-y">
                {deleted.map((voice) => (
                  <DeletedVoiceRow key={voice.id} ownerId={ownerId} voice={voice} />
                ))}
              </ul>
            </section>
          )}
        </>
      )}
    </main>
  )
}

/** The name is the link into the voice; the pencil beside it renames in place. The actions sit on
 *  their own row so the phone gets three full-height targets instead of a crushed strip (§4.1). */
function VoiceRow({ ownerId, voice }: { ownerId: string; voice: Voice }) {
  return (
    <li className="py-3">
      <RenameVoiceField ownerId={ownerId} voice={voice}>
        <div className="flex min-h-11 flex-wrap items-center gap-2">
          <Link
            to="/voices/$voiceId"
            params={{ voiceId: voice.id }}
            className="text-link-fg hover:text-link-fg-hover inline-flex min-h-11 min-w-0 items-center text-sm underline"
          >
            <span className="truncate">{voice.name}</span>
          </Link>
          {voice.isDefault && <Badge tone="accent">기본</Badge>}
        </div>
      </RenameVoiceField>
      {!voice.isDefault && (
        <div className="mt-2 flex flex-wrap gap-2">
          <SetDefaultVoiceButton ownerId={ownerId} voiceId={voice.id} />
          <DeleteVoiceButton ownerId={ownerId} voice={voice} />
        </div>
      )}
    </li>
  )
}

function DeletedVoiceRow({ ownerId, voice }: { ownerId: string; voice: Voice }) {
  return (
    <li className="py-3">
      <RenameVoiceField ownerId={ownerId} voice={voice}>
        <div className="flex min-h-11 flex-wrap items-center gap-2">
          <Link
            to="/voices/$voiceId"
            params={{ voiceId: voice.id }}
            className="text-link-fg hover:text-link-fg-hover inline-flex min-h-11 min-w-0 items-center text-sm underline"
          >
            <span className="truncate">{voice.name}</span>
          </Link>
          <Badge tone="warning">삭제됨</Badge>
        </div>
      </RenameVoiceField>
      <div className="mt-2 flex flex-wrap gap-2">
        <RestoreVoiceButton ownerId={ownerId} voiceId={voice.id} />
      </div>
    </li>
  )
}
