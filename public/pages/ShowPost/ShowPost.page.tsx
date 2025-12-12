import React, { useEffect, useCallback, useState } from "react"

import { LockStatus } from "./components/LockStatus"
import { ArchiveStatus } from "./components/ArchiveStatus"
import { PostLockingModal } from "./components/PostLockingModal"
import { Comment, Post, Tag, Vote, PostStatus, isPostLocked, isPostArchived, ReportReason } from "@fider/models"
import { actions, clearUrlHash, Fider, notify, formatDate, postPermissions } from "@fider/services"
import { heroiconsDotsHorizontal as IconDotsHorizontal, heroiconsChevronUp as IconChevronUp } from "@fider/icons.generated"

import {
  Button,
  UserName,
  Moment,
  Markdown,
  Input,
  Form,
  TextArea,
  MultiImageUploader,
  Icon,
  Avatar,
  Dropdown,
  ImageGallery,
  ReportModal,
  ReportButton,
} from "@fider/components"
import { ResponseDetails } from "@fider/components/post/ShowPostResponse"
import { SentimentBar } from "@fider/components/post/SentimentBar"
import { DiscussionPanel } from "./components/DiscussionPanel"

import { heroiconsX as IconX, heroiconsThumbsup as IconThumbsUp } from "@fider/icons.generated"
import { HStack, VStack } from "@fider/components/layout"
import { Trans } from "@lingui/react/macro"
import { i18n } from "@lingui/core"
import { TagsPanel } from "./components/TagsPanel"
import { VoteSection } from "./components/VoteSection"
import { DeletePostModal } from "./components/DeletePostModal"
import { ResponseModal } from "./components/ResponseModal"
import { VotesPanel } from "./components/VotesPanel"
import { useShowPostState } from "./hooks"

interface ReportStatus {
  hasReportedPost: boolean
  reportedCommentIds: number[]
  dailyLimitReached: boolean
}

interface ShowPostPageProps {
  post: Post
  subscribed: boolean
  comments: Comment[]
  tags: Tag[]
  votes: Vote[]
  attachments: string[]
  reportStatus?: ReportStatus
  reportReasons?: ReportReason[]
}

const ShowPostPage: React.FC<ShowPostPageProps> = (props) => {
  const state = useShowPostState({
    initialTitle: props.post.title,
    initialDescription: props.post.description,
  })
  
  const [upvotes, setUpvotes] = useState(props.post.upvotes || 0)
  const [downvotes, setDownvotes] = useState(props.post.downvotes || 0)
  
  useEffect(() => {
    setUpvotes(props.post.upvotes || 0)
    setDownvotes(props.post.downvotes || 0)
  }, [props.post.upvotes, props.post.downvotes])
  
  const handleVoteChange = useCallback((newUpvotes: number, newDownvotes: number) => {
    setUpvotes(newUpvotes)
    setDownvotes(newDownvotes)
  }, [])

  const handleCopyEvent = useCallback(() => {
    const selection = window.getSelection()
    if (selection && selection.toString().trim()) {
      state.setHasCopiedContent(true)
    }
  }, [state.setHasCopiedContent])

  const scrollToHighlightedComment = useCallback(() => {
    if (state.highlightedComment) {
      setTimeout(() => {
        const element = document.getElementById(`comment-${state.highlightedComment}`)
        if (element) {
          element.scrollIntoView({ behavior: "smooth", block: "start" })
        }
      }, 100)
    }
  }, [state.highlightedComment])

  const handleHashChange = useCallback(
    (e?: Event) => {
      const hash = window.location.hash
      const result = /#comment-([0-9]+)/.exec(hash)

      let newHighlightedComment: number | undefined
      if (result === null) {
        newHighlightedComment = undefined
      } else {
        const id = parseInt(result[1])
        if (props.comments.map((comment) => comment.id).includes(id)) {
          newHighlightedComment = id
        } else {
          if (e?.cancelable) {
            e.preventDefault()
          } else {
            clearUrlHash(true)
          }
          notify.error(<Trans id="showpost.comment.unknownhighlighted">Unknown comment ID #{id}</Trans>)
          newHighlightedComment = undefined
        }
      }
      state.setHighlightedComment(newHighlightedComment)
    },
    [props.comments, state.setHighlightedComment]
  )

  useEffect(() => {
    state.setNewTitle(props.post.title)
    state.setNewDescription(props.post.description)
  }, [props.post.number])

  useEffect(() => {
    handleHashChange()
    window.addEventListener("hashchange", handleHashChange)
    document.addEventListener("copy", handleCopyEvent)
    const currentPath = window.location.pathname

    if (!currentPath.includes(props.post.slug)) {
      const newPath = `${currentPath}/${props.post.slug}`
      window.history.replaceState({}, document.title, newPath)
    }

    return () => {
      window.removeEventListener("hashchange", handleHashChange)
      document.removeEventListener("copy", handleCopyEvent)
    }
  }, [handleHashChange, handleCopyEvent, props.post.slug])

  useEffect(() => {
    scrollToHighlightedComment()
  }, [scrollToHighlightedComment])

  const handleScrollToTop = useCallback(() => {
    window.scrollTo({ top: 0, behavior: "smooth" })
    document.documentElement.scrollTo({ top: 0, behavior: "smooth" })
    document.body.scrollTo({ top: 0, behavior: "smooth" })
  }, [])

  const saveChanges = useCallback(async () => {
    const result = await actions.updatePost(props.post.number, state.newTitle, state.newDescription, state.attachments)
    if (result.ok) {
      location.reload()
    } else {
      state.setError(result.error)
    }
  }, [props.post.number, state.newTitle, state.newDescription, state.attachments, state.setError])

  const canDeletePost = useCallback(() => {
    const status = PostStatus.Get(props.post.status)
    if (status.closed) return false
    return postPermissions.canDelete(props.post)
  }, [props.post.status, props.post.user.role])

  const onActionSelected = useCallback(
    (action: "copy" | "delete" | "status" | "edit" | "lock" | "unlock" | "report" | "archive" | "unarchive") => async () => {
      if (action === "copy") {
        navigator.clipboard.writeText(window.location.href)
        notify.success(<Trans id="showpost.copylink.success">Link copied to clipboard</Trans>)
      } else if (action === "delete") {
        state.openModal("delete")
      } else if (action === "status") {
        state.openModal("response")
      } else if (action === "edit") {
        state.startEdit()
      } else if (action === "lock") {
        state.openModal("lock")
      } else if (action === "unlock") {
        state.openModal("unlock")
      } else if (action === "report") {
        state.openModal("report")
      } else if (action === "archive") {
        const result = await actions.archivePost(props.post.number)
        if (result.ok) {
          notify.success(<Trans id="showpost.archive.success">Post has been archived</Trans>)
          location.reload()
        }
      } else if (action === "unarchive") {
        const result = await actions.unarchivePost(props.post.number)
        if (result.ok) {
          notify.success(<Trans id="showpost.unarchive.success">Post has been unarchived</Trans>)
          location.reload()
        }
      }
    },
    [state.startEdit, state.openModal, props.post.number]
  )

  return (
    <>
      <div id="p-show-post" className="page container overflow-hidden">
        <div className="lg:grid lg:gap-6 lg:grid-cols-[2fr_6fr_1fr] lg:grid-rows-[auto] lg:items-start">
          <div className="mb-4 lg:col-start-2 lg:col-end-3 lg:row-start-1 min-w-0 bg-border tag-clipped p-px self-start">
            <div className="p-4 bg-elevated tag-clipped-inner wrap-anywhere">
              <VStack spacing={8}>
                <HStack justify="between">
                  <VStack align="start">
                    {!state.editMode && (
                      <HStack>
                        <Avatar user={props.post.user} />
                        <VStack spacing={1}>
                          <UserName user={props.post.user} />
                          <span
                            className="text-muted"
                            data-tooltip={i18n._("showpost.createdat", { message: "Created {date}", date: formatDate(Fider.currentLocale, props.post.createdAt, "full") })}
                          >
                            <Trans id="showpost.lastactivity">Last activity:</Trans>{" "}
                            <Moment locale={Fider.currentLocale} date={props.post.lastActivityAt} />
                          </span>
                        </VStack>
                      </HStack>
                    )}
                  </VStack>

                  {!state.editMode && (
                    <HStack spacing={1} className="items-center">
                      <ReportButton
                        reportedUserId={props.post.user.id}
                        size="medium"
                        hasReported={props.reportStatus?.hasReportedPost ?? false}
                        dailyLimitReached={props.reportStatus?.dailyLimitReached ?? false}
                        onReport={() => state.openModal("report")}
                      />
                      <Dropdown
                        position="left"
                        renderHandle={<Icon sprite={IconDotsHorizontal} width="24" height="24" />}
                      >
                        <Dropdown.ListItem onClick={onActionSelected("copy")}>
                          <Trans id="action.copylink">Copy link</Trans>
                        </Dropdown.ListItem>
                        {postPermissions.canEdit(props.post) && (
                          <>
                            <Dropdown.ListItem onClick={onActionSelected("edit")}>
                              <Trans id="action.edit">Edit</Trans>
                            </Dropdown.ListItem>
                            {postPermissions.canRespond() && (
                              <Dropdown.ListItem onClick={onActionSelected("status")}>
                                <Trans id="action.respond">Respond</Trans>
                              </Dropdown.ListItem>
                            )}
                            {postPermissions.canLock() && (
                              <>
                                {!isPostLocked(props.post) ? (
                                  <Dropdown.ListItem onClick={onActionSelected("lock")}>
                                    <Trans id="action.lock">Lock</Trans>
                                  </Dropdown.ListItem>
                                ) : (
                                  <Dropdown.ListItem onClick={onActionSelected("unlock")}>
                                    <Trans id="action.unlock">Unlock</Trans>
                                  </Dropdown.ListItem>
                                )}
                              </>
                            )}
                            {postPermissions.canArchive() && (
                              <>
                                {!isPostArchived(props.post) ? (
                                  <Dropdown.ListItem onClick={onActionSelected("archive")}>
                                    <Trans id="action.archive">Archive</Trans>
                                  </Dropdown.ListItem>
                                ) : (
                                  <Dropdown.ListItem onClick={onActionSelected("unarchive")}>
                                    <Trans id="action.unarchive">Unarchive</Trans>
                                  </Dropdown.ListItem>
                                )}
                              </>
                            )}
                          </>
                        )}
                        {canDeletePost() && (
                          <Dropdown.ListItem onClick={onActionSelected("delete")} className="text-danger">
                            <Trans id="action.delete">Delete</Trans>
                          </Dropdown.ListItem>
                        )}
                      </Dropdown>
                    </HStack>
                  )}
                </HStack>

                <div className="flex-grow">
                  {state.editMode ? (
                    <Form error={state.error}>
                      <Input field="title" maxLength={100} value={state.newTitle} onChange={state.setNewTitle} />
                    </Form>
                  ) : (
                    <>
                      <h1 className="text-large">{props.post.title}</h1>
                      {isPostLocked(props.post) && <LockStatus post={props.post} />}
                      {isPostArchived(props.post) && <ArchiveStatus post={props.post} />}
                    </>
                  )}
                </div>

                <DeletePostModal
                  onModalClose={state.closeModal}
                  showModal={state.isModalOpen("delete")}
                  post={props.post}
                />
                {postPermissions.canRespond() && (
                  <ResponseModal
                    onCloseModal={state.closeModal}
                    showModal={state.isModalOpen("response")}
                    post={props.post}
                    tags={props.tags}
                    attachments={props.attachments}
                    hasCopiedContent={state.hasCopiedContent}
                  />
                )}
                <VStack>
                  {state.editMode ? (
                    <Form error={state.error}>
                      <TextArea field="description" value={state.newDescription} onChange={state.setNewDescription} />
                      <MultiImageUploader
                        field="attachments"
                        bkeys={props.attachments}
                        maxUploads={Fider.session.tenant.generalSettings?.maxImagesPerPost || 3}
                        onChange={state.setAttachments}
                      />
                    </Form>
                  ) : (
                    <>
                      {props.post.description && (
                        <Markdown className="description" text={props.post.description} style="full" />
                      )}
                      {!props.post.description && (
                        <em className="text-muted">
                          <Trans id="showpost.message.nodescription">No description provided.</Trans>
                        </em>
                      )}
                      {props.attachments.length > 0 && <ImageGallery bkeys={props.attachments} />}
                    </>
                  )}
                </VStack>
                <div className="mt-2">
                  <TagsPanel post={props.post} tags={props.tags} />
                </div>

                <VStack spacing={4}>
                  {!state.editMode ? (
                    <div className="w-full">
                      <VoteSection post={props.post} onVoteChange={handleVoteChange} />
                    </div>
                  ) : (
                    <HStack>
                      <Button variant="primary" onClick={saveChanges} disabled={Fider.isReadOnly}>
                        <Icon sprite={IconThumbsUp} />{" "}
                        <span>
                          <Trans id="action.save">Save</Trans>
                        </span>
                      </Button>
                      <Button variant="tertiary" onClick={state.cancelEdit} disabled={Fider.isReadOnly}>
                        <Icon sprite={IconX} />
                        <span>
                          <Trans id="action.cancel">Cancel</Trans>
                        </span>
                      </Button>
                    </HStack>
                  )}
                  <SentimentBar post={props.post} upvotes={upvotes} downvotes={downvotes} />
                </VStack>

                <ResponseDetails status={props.post.status} response={props.post.response} />
              </VStack>

              <DiscussionPanel
                post={props.post}
                comments={props.comments}
                highlightedComment={state.highlightedComment}
                subscribed={props.subscribed}
                reportedCommentIds={props.reportStatus?.reportedCommentIds ?? []}
                dailyLimitReached={props.reportStatus?.dailyLimitReached ?? false}
                reportReasons={props.reportReasons}
              />
              <div className="mt-4 flex items-center justify-between">
                <Button variant="secondary" onClick={handleScrollToTop}>
                  <Icon sprite={IconChevronUp} />
                  <span>
                    <Trans id="returntop.button">Return to top</Trans>
                  </span>
                </Button>
              </div>
            </div>
          </div>
          <div className="lg:col-start-1 lg:col-end-2 lg:row-start-1 min-w-0 bg-elevated rounded-panel p-4 h-fit">
            <VotesPanel post={props.post} votes={props.votes} />
          </div>
        </div>
      </div>
      <PostLockingModal
        post={props.post}
        isOpen={state.isModalOpen("lock") || state.isModalOpen("unlock")}
        onClose={state.closeModal}
        mode={state.isModalOpen("lock") ? "lock" : "unlock"}
      />
      <ReportModal
        isOpen={state.isModalOpen("report")}
        onClose={state.closeModal}
        postNumber={props.post.number}
        reasons={props.reportReasons}
      />
    </>
  )
}

export default ShowPostPage
