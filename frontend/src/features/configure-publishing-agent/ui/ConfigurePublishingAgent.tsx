import { useMemo, useState } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { publishingAgentsQueryKey, type PublishingAgent } from '@/entities/publishing-agent'
import { publishingClientFor, PublishVisibility } from '@/shared/api'
import { Button, FieldLabel, Notice, Select, TextField } from '@/shared/ui'

export function ConfigurePublishingAgent({
  ownerId,
  agent,
}: {
  ownerId: string
  agent: PublishingAgent
}) {
  const [label, setLabel] = useState(agent.label)
  const [categoryId, setCategoryId] = useState(agent.defaultCategoryId)
  const [visibility, setVisibility] = useState(agent.defaultVisibility)
  const transport = useTransport()
  const client = useMemo(() => publishingClientFor(transport), [transport])
  const queryClient = useQueryClient()
  const update = useMutation({
    mutationFn: () =>
      client.updatePublishingAgent({
        agentId: agent.id,
        label: label.trim(),
        defaultCategoryId: categoryId,
        defaultVisibility: visibility,
      }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: publishingAgentsQueryKey(ownerId) }),
  })
  if (!agent.ready) return null
  return (
    <div className="mt-4 grid gap-4">
      <div>
        <FieldLabel htmlFor={`agent-label-${agent.id}`}>연결 이름</FieldLabel>
        <TextField
          id={`agent-label-${agent.id}`}
          value={label}
          onChange={(event) => setLabel(event.target.value)}
          type="text"
          autoComplete="off"
          autoCapitalize="off"
          autoCorrect="off"
          enterKeyHint="done"
          className="mt-2"
        />
      </div>
      <div>
        <FieldLabel htmlFor={`agent-category-${agent.id}`}>기본 카테고리</FieldLabel>
        <Select
          id={`agent-category-${agent.id}`}
          value={categoryId}
          onChange={(event) => setCategoryId(event.target.value)}
          className="mt-2"
        >
          {agent.categories.map((category) => (
            <option key={category.id} value={category.id}>
              {category.name}
            </option>
          ))}
        </Select>
      </div>
      <div>
        <FieldLabel htmlFor={`agent-visibility-${agent.id}`}>기본 공개 설정</FieldLabel>
        <Select
          id={`agent-visibility-${agent.id}`}
          value={visibility}
          onChange={(event) => setVisibility(Number(event.target.value) as PublishVisibility)}
          className="mt-2"
        >
          <option value={PublishVisibility.PUBLIC}>전체 공개</option>
          <option value={PublishVisibility.PRIVATE}>비공개</option>
        </Select>
      </div>
      <Button
        variant="secondary"
        onClick={() => update.mutate()}
        pending={update.isPending}
        disabled={!label.trim() || !categoryId}
      >
        기본값 저장
      </Button>
      {update.isError && <Notice tone="danger">기본값을 저장하지 못했어요.</Notice>}
    </div>
  )
}
