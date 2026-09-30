import React, { useRef, useState } from "react"
import { Avatar } from "@fider/components/common/Avatar"
import { Button } from "@fider/components/common/Button"
import { Dropdown } from "@fider/components/common/Dropdown"
import { Form } from "@fider/components/common/form/Form"
import { Icon } from "@fider/components/common/Icon"
import { ImageGallery } from "@fider/components/common/ImageGallery"
import { Markdown } from "@fider/components/common/Markdown"
import { Modal } from "@fider/components/common/Modal"
import { Moment } from "@fider/components/common/Moment"
import { Reactions } from "@fider/components/post/Reactions"
import { ReportModal } from "@fider/components/moderation/ReportModal"
import { UserName } from "@fider/components/common/UserName"
import { DiscussionComment, DiscussionOwner, ReactionCount } from "@fider/models"
import * as postActions from "@fider/services/actions/post"
import * as notify from "@fider/services/notify"
import { Failure } from "@fider/services"
import { copyToClipboard, formatDate } from "@fider/services/utils"
import { deleteComment, setCommentReaction } from "@fider/services/discussion"
import { heroiconsDotsHorizontal as IconDotsHorizontal } from "@fider/icons.generated"
import { useFider } from "@fider/hooks/use-fider"
import { CommentComposer } from "./CommentComposer"

export type CommentChange = "edit" | "moderation" | "delete" | "report"

export interface CommentInteraction {
  editor?: "edit" | "reply"
  action?:
    | { kind: "pending" }
    | { kind: "failed"; error: Failure; retry: () => Promise<void> }
}

interface DiscussionCommentCardProps {
  owner: DiscussionOwner
  comment: DiscussionComment
  images: boolean
  collapsed: boolean
  highlighted: boolean
  entering: boolean
  onEntered: (id: number) => void
  interaction?: CommentInteraction
  onCreated: (comment: DiscussionComment) => void
  onChanged: (comment: DiscussionComment, change: CommentChange) => void
  onReactionsChanged: (id: number, reactions: ReactionCount[]) => void
  onEditor: (id: number, editor?: "edit" | "reply") => void
  onAction: (id: number, operation: () => Promise<void>) => Promise<void>
}

export const DiscussionCommentCard = React.memo(function DiscussionCommentCard(props: DiscussionCommentCardProps) {
  const { comment } = props
  const fider = useFider()
  const editor = props.interaction?.editor
  const setEditor = (value?: "edit" | "reply") => props.onEditor(comment.id, value)
  const [deleting, setDeleting] = useState(false)
  const [reporting, setReporting] = useState(false)
  const busy = props.interaction?.action?.kind === "pending"
  const failure = props.interaction?.action?.kind === "failed" ? props.interaction.action : undefined
  const [linkToCopy, setLinkToCopy] = useState<string>()
  const emojiSelectorRef = useRef<HTMLDivElement>(null)
  const run = (operation: () => Promise<void>) => props.onAction(comment.id, operation)

  const remove = () => run(async () => {
    const result = await deleteComment(comment.id)

    if (!result.ok) {
      throw new Error(result.error.errors?.[0]?.message || "Could not delete this comment.")
    }

    props.onChanged({ ...comment, state: "deleted" }, "delete")
    setDeleting(false)
  })

  const moderate = (hidden: boolean) => run(async () => {
    const result = hidden ? await postActions.hideComment(comment.id) : await postActions.unhideComment(comment.id)

    if (!result.ok) {
      throw new Error(result.error.errors?.[0]?.message || "Could not moderate this comment.")
    }

    props.onChanged(result.data, "moderation")
  })

  const react = (emoji: string) => {
    const active = !comment.reactionCounts?.find((reaction) => reaction.emoji === emoji)?.includesMe

    return run(async () => {
      const result = await setCommentReaction(comment.id, emoji, active)

      if (!result.ok) {
        throw new Error(result.error.errors?.[0]?.message || "Could not save your reaction.")
      }

      props.onReactionsChanged(comment.id, result.data.reactionCounts || [])
    })
  }

  const saved = (updated: DiscussionComment) => {
    if (updated.id === comment.id) {
      props.onChanged(updated, "edit")
    } else {
      props.onCreated(updated)
    }
  }

  const copyLink = async () => {
    const link = `${window.location.origin}${props.owner.url}#comment-${comment.id}`

    try {
      await copyToClipboard(link)
      setLinkToCopy(undefined)
      notify.success("Link copied to clipboard")
    } catch {
      setLinkToCopy(link)
    }
  }

  return (
    <article
      id={`comment-${comment.id}`}
      className={`discussion-comment ${props.highlighted ? "highlighted-comment" : ""} ${props.entering ? "comment-enter" : ""}`}
      onAnimationEnd={props.entering ? (event) => event.target === event.currentTarget && event.animationName === "comment-tint" && props.onEntered(comment.id) : undefined}
      aria-label={`Comment ${comment.id}`}
    >
      <div className="flex items-center gap-2 min-w-0">
        {comment.user && <Avatar user={comment.user} size="small" />}
        <div className="flex flex-1 items-center gap-x-2 gap-y-0.5 flex-wrap min-w-0 text-sm">
          {comment.user
            ? <UserName user={comment.user} />
            : <span className="text-muted">{comment.state === "deleted" ? "Deleted comment" : "Hidden comment"}</span>}
          <a href={`#comment-${comment.id}`} className="text-xs text-muted hover:text-foreground">
            <Moment locale={fider.currentLocale} date={comment.createdAt} />
          </a>
          {comment.editedAt && comment.editedBy && (
            <span className="text-xs text-muted" title={`Edited by ${comment.editedBy.name} on ${formatDate(fider.currentLocale, comment.editedAt)}`}>
              · edited
            </span>
          )}
          {comment.moderationPending && (
            <span className="rounded border border-danger/40 bg-danger-light px-1.5 py-0.5 text-xs font-medium text-danger-dark" title="Hidden from public view">
              Hidden
            </span>
          )}
        </div>
        <Dropdown label="Comment actions" position="left" renderHandle={<Icon sprite={IconDotsHorizontal} width="16" height="16" className="cursor-pointer" />}>
          <Dropdown.ListItem onClick={copyLink}>Copy link</Dropdown.ListItem>
          {comment.permissions.edit && <Dropdown.ListItem onClick={() => setEditor("edit")}>Edit</Dropdown.ListItem>}
          {comment.permissions.delete && <Dropdown.ListItem onClick={() => setDeleting(true)}>Delete</Dropdown.ListItem>}
          {comment.permissions.moderate && (
            <Dropdown.ListItem onClick={() => moderate(!comment.moderationPending)}>
              {comment.moderationPending ? "Unhide" : "Hide"}
            </Dropdown.ListItem>
          )}
          {comment.permissions.report && <Dropdown.ListItem onClick={() => setReporting(true)}>Report</Dropdown.ListItem>}
        </Dropdown>
      </div>
      <div className="discussion-comment-body">
        {linkToCopy && (
          <label className="block my-2 text-sm">
            Copy this comment link:
            <input
              aria-label="Comment link"
              readOnly
              value={linkToCopy}
              onFocus={(event) => event.currentTarget.select()}
              className="block w-full mt-1 rounded border border-border bg-background p-2"
            />
          </label>
        )}
        <Form error={failure?.error}>
          {failure && <Button disabled={busy} size="small" onClick={() => run(failure.retry)}>Retry action</Button>}
        </Form>
        {!props.collapsed && comment.state === "visible" && (
          <>
            {editor === "edit" ? (
              <CommentComposer
                owner={props.owner}
                comment={comment}
                images={props.images}
                onSaved={saved}
                onClose={() => setEditor(undefined)}
              />
            ) : (
              <div className="mt-2">
                <Markdown text={comment.content} style="full" />
                {!!comment.attachments?.length && <ImageGallery bkeys={comment.attachments} />}
              </div>
            )}
            {editor !== "edit" && (
              <div className="flex items-center flex-wrap gap-2 mt-2">
                <Reactions
                  className=""
                  reactions={comment.reactionCounts}
                  emojiSelectorRef={emojiSelectorRef}
                  toggleReaction={react}
                  disabled={!comment.permissions.react}
                  busy={busy}
                />
                {comment.permissions.reply && editor !== "reply" && (
                  <Button size="small" variant="tertiary" onClick={() => setEditor("reply")}>Reply</Button>
                )}
              </div>
            )}
            {editor === "reply" && (
              <CommentComposer
                owner={props.owner}
                parentId={comment.id}
                images={props.images}
                onSaved={saved}
                onClose={() => setEditor(undefined)}
              />
            )}
          </>
        )}
      </div>
      <ReportModal
        isOpen={reporting}
        onClose={() => setReporting(false)}
        commentId={comment.id}
        onSubmit={() => props.onChanged(comment, "report")}
      />
      <Modal.Window isOpen={deleting} onClose={() => setDeleting(false)}>
        <Modal.Header>Delete comment</Modal.Header>
        <Modal.Content>
          <p>Delete this comment? Other people’s replies will remain.</p>
          <Form error={failure?.error} />
        </Modal.Content>
        <Modal.Footer>
          <Button variant="danger" loading={busy} onClick={remove}>Delete comment</Button>
          <Button variant="tertiary" disabled={busy} onClick={() => setDeleting(false)}>Cancel</Button>
        </Modal.Footer>
      </Modal.Window>
    </article>
  )
})
