import React, { useEffect, useRef, useState } from "react"
import { Button } from "@fider/components/common/Button"
import { CommentEditor } from "@fider/components/common/form/CommentEditor"
import { DisplayError } from "@fider/components/common/form/DisplayError"
import { Form } from "@fider/components/common/form/Form"
import { SignInModal } from "@fider/components/auth/SignInModal"
import { DiscussionComment, DiscussionOwner } from "@fider/models"
import { useFider } from "@fider/hooks/use-fider"
import { useAccountDraft } from "@fider/hooks/useAccountDraft"
import { useDraftSubmission } from "@fider/hooks/useDraftSubmission"
import { DraftStatus } from "@fider/components/common/DraftStatus"
import { DraftPicker } from "@fider/components/common/DraftPicker"
import { analytics } from "@fider/services/analytics"
import { DraftImage } from "@fider/services/draftImages"
import { editComment, submitComment } from "@fider/services/discussion"
import { CommentAttachments } from "./CommentAttachments"

interface CommentComposerProps {
  owner: DiscussionOwner
  parentId?: number
  comment?: DiscussionComment
  images: boolean
  onSaved: (comment: DiscussionComment) => void
  onClose?: () => void
}

interface CommentContent {
  content: string
  attachments: DraftImage[]
  parentId?: number
}

export function CommentComposer(props: CommentComposerProps) {
  const fider = useFider()
  const scope = `${fider.session.tenant.id}:${props.owner.kind}:${props.owner.id}:${props.comment ? `edit:${props.comment.id}` : props.parentId || "root"}`
  const account = `${fider.session.tenant.id}:${fider.session.isAuthenticated ? fider.session.user.id : "anonymous"}`

  return <CommentDraftEditor key={`${account}:${scope}`} {...props} scope={scope} account={account} />
}

function CommentDraftEditor(props: CommentComposerProps & { scope: string; account: string }) {
  const fider = useFider()
  const editable = useAccountDraft<CommentContent>({
    kind: "comment",
    scope: props.scope,
    initial: { content: props.comment?.content || "", attachments: [], parentId: props.parentId },
  })
  const [signIn, setSignIn] = useState(false)
  const restored = useRef(false)
  const edited = useRef(false)
  const submission = useDraftSubmission({
    editor: editable,
    send: (payload, signal) => props.comment ? editComment(props.comment.id, payload, signal) : submitComment(props.owner, payload, signal),
    onUnauthorized: () => setSignIn(true),
    onSaved: (comment, { firstCompletion, mounted }) => {
      if (firstCompletion) analytics.event("comment", props.comment ? "update" : "create")
      props.onSaved(comment)
      if (mounted) props.onClose?.()
    },
  })
  const { busy, error } = submission
  const pending = "submission" in submission.state ? submission.state.submission : undefined

  useEffect(() => {
    if (!editable.loaded || restored.current) return

    restored.current = true
    if (!edited.current && editable.pending[0]) submission.select(editable.pending[0])
  }, [editable.loaded, editable.pending])

  const value = pending?.payload || editable.value
  const maxImages = fider.session.tenant.generalSettings?.maxImagesPerComment ?? 2

  const selectedSubmission = pending?.draft.id
  const pendingChoices = editable.pending.filter(item => item.id !== selectedSubmission)

  return (
    <div className="my-3 min-w-0">
      <SignInModal isOpen={signIn} onClose={() => setSignIn(false)} />
      {pendingChoices.length > 0 && <div className="my-2">
        <DraftPicker<CommentContent>
          drafts={pendingChoices}
          label="Pending submissions"
          action="Continue pending submission"
          disabled={busy}
          onSelect={async saved => { submission.select(saved) }}
        />
      </div>}
      {(!pending || editable.status === "error") && <DraftStatus {...editable} />}
      <Form error={error}>
        <CommentEditor
          key={`editor:${pending?.payload.submissionId || editable.restored}`}
          initialValue={value.content}
          onChange={content => {
            edited.current = true
            editable.change({ content })
          }}
          readOnly={busy || !!pending}
          placeholder={props.parentId ? "Write a reply" : "Leave a comment"}
        />
        <DisplayError error={error} fields={["content", "parentId", "submissionId", "attachments"]} />
        {(props.images || !!props.comment?.attachments?.length || !!value.attachments.length) && <CommentAttachments
          existing={props.comment?.attachments}
          attachments={value.attachments}
          allowUploads={props.images}
          maxUploads={maxImages}
          disabled={busy || !!pending}
          onChange={attachments => {
            edited.current = true
            editable.change({ attachments })
          }}
        />}
        <div className="flex gap-2 flex-wrap mt-2">
          <Button variant="primary" loading={busy} disabled={busy || !value.content.trim()} onClick={() => submission.submit()}>
            {pending ? "Retry submission" : props.comment ? "Save changes" : props.parentId ? "Reply" : "Submit comment"}
          </Button>
          {pending && !props.comment && <Button disabled={busy} onClick={submission.startNew}>Write another comment</Button>}
          {props.onClose && <Button disabled={busy} variant="tertiary" onClick={props.onClose}>Cancel</Button>}
        </div>
      </Form>
    </div>
  )
}
