import React, { useEffect, useLayoutEffect, useRef, useState } from "react"
import { Button, CommentEditor, DisplayError, Form, SignInModal } from "@fider/components"
import { DiscussionComment, DiscussionOwner } from "@fider/models"
import { useFider } from "@fider/hooks"
import { Failure } from "@fider/services"
import { CommentDraft, commentDrafts } from "@fider/services/commentDrafts"
import { editComment, submitComment } from "@fider/services/discussion"
import { newSubmissionID } from "@fider/services/postSubmission"
import { CommentAttachments } from "./CommentAttachments"

interface CommentComposerProps {
  owner: DiscussionOwner
  parentId?: number
  comment?: DiscussionComment
  images: boolean
  onSaved: (comment: DiscussionComment) => void
  onClose?: () => void
}

export function CommentComposer(props: CommentComposerProps) {
  const fider = useFider()
  const scope = `${fider.session.tenant.id}:${props.owner.kind}:${props.owner.id}:${props.comment ? `edit:${props.comment.id}` : props.parentId || "root"}`
  const [draft, setDraft] = useState<CommentDraft>()
  const [otherDrafts, setOtherDrafts] = useState<CommentDraft[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<Failure>()
  const [storageError, setStorageError] = useState(false)
  const [signIn, setSignIn] = useState(false)
  const restoredDraft = useRef<CommentDraft>()
  const currentDraft = useRef<CommentDraft>()
  const mounted = useRef(false)

  useLayoutEffect(() => {
    mounted.current = true

    return () => {
      mounted.current = false
    }
  }, [])

  function emptyDraft(): CommentDraft {
    return {
      scope,
      submissionId: newSubmissionID(),
      content: props.comment?.content || "",
      attachments: [],
      parentId: props.parentId,
      state: "draft",
      updatedAt: Date.now(),
    }
  }

  const openDraft = (saved?: CommentDraft) => {
    restoredDraft.current = saved
    const next = saved || emptyDraft()
    currentDraft.current = next
    setDraft(next)
  }

  useEffect(() => {
    let active = true
    const unsubscribe = commentDrafts.subscribe(scope, setStorageError)

    commentDrafts.load(scope).then((drafts) => {
      if (!active) {
        return
      }

      openDraft(drafts[0])
      setOtherDrafts(drafts.slice(1))
    }).catch(() => {
      if (active) {
        setStorageError(true)
        const drafts = commentDrafts.recover(scope).filter((value): value is CommentDraft => value.state !== "completed")
        openDraft(drafts[0])
        setOtherDrafts(drafts.slice(1))
      }
    })

    return () => {
      active = false
      unsubscribe()
    }
  }, [scope])

  const change = (changes: Partial<Pick<CommentDraft, "content" | "attachments">>) => {
    const previous = currentDraft.current
    if (!previous) {
      return
    }

    const submissionId = previous.submissionId === restoredDraft.current?.submissionId
      ? newSubmissionID()
      : previous.submissionId
    const next = { ...previous, ...changes, submissionId, updatedAt: Date.now() }
    currentDraft.current = next
    setDraft(next)
    void commentDrafts.save(next, restoredDraft.current).catch(() => {})
  }

  const finish = (comment: DiscussionComment) => {
    props.onSaved(comment)

    if (mounted.current) {
      openDraft()
      props.onClose?.()
    }
  }

  const send = async () => {
    if (!draft || busy) {
      return
    }

    setBusy(true)
    setError(undefined)

    try {
      let payload
      try {
        payload = await commentDrafts.prepare(draft)
      } catch {
        setError({ errors: [{ field: "attachments", message: "An image could not be read. Retry or remove it." }] })
        return
      }

      let submission: CommentDraft = { ...draft, state: "pending", updatedAt: Date.now() }

      try {
        const saved = await commentDrafts.save(submission, restoredDraft.current)

        if (saved.state === "completed") {
          finish(saved.comment)
          return
        }

        if (saved !== submission) {
          payload = await commentDrafts.prepare(saved)
        }
        submission = saved
        setStorageError(false)
      } catch {
        setStorageError(true)
      }

      setDraft(submission)
      currentDraft.current = submission

      const result = props.comment
        ? await editComment(props.comment.id, payload)
        : await submitComment(props.owner, payload)

      if (!result.ok) {
        if (result.status && result.status < 500) {
          const saved = await commentDrafts.rejected(submission).catch(() => {
            setStorageError(true)
            return { ...submission, state: "draft" as const }
          })

          if (saved.state === "completed") {
            finish(saved.comment)
            return
          }

          openDraft(saved)
        }

        setError(result.error)

        if (result.status === 401) {
          setSignIn(true)
        }

        return
      }

      await commentDrafts.complete(submission, result.data, props.comment ? "update" : "create")
        .catch(() => setStorageError(true))

      finish(result.data)
    } catch (cause) {
      setError({
        errors: [{ message: "Could not confirm your comment. Your draft is retained; retry to recover the result." }],
        cause,
      })
    } finally {
      setBusy(false)
    }
  }

  if (!draft) {
    return <p role="status">Loading saved drafts…</p>
  }

  const pending = draft.state === "pending"
  const maxImages = fider.session.tenant.generalSettings?.maxImagesPerComment ?? 2

  return (
    <div className="my-3 min-w-0">
      <SignInModal isOpen={signIn} onClose={() => setSignIn(false)} />
      {storageError && <p role="status" className="text-warning mb-2">Browser storage is unavailable. Keep this page open to retain your draft.</p>}
      {otherDrafts.length > 0 && (
        <div className="flex gap-2 flex-wrap mb-2">
          {otherDrafts.map((saved) => (
            <Button key={saved.submissionId} size="small" disabled={busy} onClick={() => {
              setOtherDrafts((previous) => [...previous.filter((value) => value.submissionId !== saved.submissionId), draft])
              openDraft(saved)
              setError(undefined)
            }}>
              Restore {saved.state === "pending" ? "unconfirmed comment" : "draft"}
            </Button>
          ))}
        </div>
      )}
      <Form error={error}>
        <CommentEditor
          key={`editor:${restoredDraft.current?.submissionId || draft.submissionId}`}
          initialValue={draft.content}
          onChange={(content) => change({ content })}
          readOnly={busy || pending}
          placeholder={props.parentId ? "Write a reply" : "Leave a comment"}
        />
        <DisplayError error={error} fields={["content", "parentId", "submissionId", "attachments"]} />
        {(props.images || !!props.comment?.attachments?.length || !!draft.attachments.length) && (
          <CommentAttachments
            key={`attachments:${restoredDraft.current?.submissionId || draft.submissionId}`}
            existing={props.comment?.attachments}
            attachments={draft.attachments}
            allowUploads={props.images}
            maxUploads={maxImages}
            disabled={busy || pending}
            onChange={(attachments) => change({ attachments })}
          />
        )}
        <div className="flex gap-2 flex-wrap mt-2">
          <Button variant="primary" loading={busy} disabled={busy || !draft.content.trim()} onClick={send}>
            {pending ? "Retry submission" : props.comment ? "Save changes" : props.parentId ? "Reply" : "Submit comment"}
          </Button>
          {pending && !props.comment && (
            <Button disabled={busy} onClick={() => {
              setOtherDrafts((previous) => [...previous, draft])
              openDraft()
              setError(undefined)
            }}>
              Write another comment
            </Button>
          )}
          {props.onClose && <Button disabled={busy} variant="tertiary" onClick={props.onClose}>Cancel</Button>}
        </div>
      </Form>
    </div>
  )
}
