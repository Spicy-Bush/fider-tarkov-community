import React, { useEffect, useCallback } from "react"

import { LockStatus } from "./components/LockStatus"
import { ArchiveStatus } from "./components/ArchiveStatus"
import { HiddenStatus } from "./components/HiddenStatus"
import { PostLockingModal } from "./components/PostLockingModal"
import { Post, Tag, Vote, ReportReason } from "@fider/models"
import { isPostLocked, isPostArchived, isPostHidden } from "@fider/models/post"
import * as postActions from "@fider/services/actions/post"
import * as notify from "@fider/services/notify"
import { Fider } from "@fider/services/fider"
import { formatDate } from "@fider/services/utils"
import { heroiconsDotsHorizontal as IconDotsHorizontal, heroiconsChevronUp as IconChevronUp } from "@fider/icons.generated"

import { Button } from "@fider/components/common/Button"
import { UserName } from "@fider/components/common/UserName"
import { Moment } from "@fider/components/common/Moment"
import { Markdown } from "@fider/components/common/Markdown"
import { Input } from "@fider/components/common/form/Input"
import { Form } from "@fider/components/common/form/Form"
import { TextArea } from "@fider/components/common/form/TextArea"
import { MultiImageUploader } from "@fider/components/common/form/MultiImageUploader"
import { Icon } from "@fider/components/common/Icon"
import { Avatar } from "@fider/components/common/Avatar"
import { Dropdown } from "@fider/components/common/Dropdown"
import { ImageGallery } from "@fider/components/common/ImageGallery"
import { ReportModal } from "@fider/components/moderation/ReportModal"
import { ReportButton } from "@fider/components/moderation/ReportButton"
import { ResponseDetails } from "@fider/components/post/ShowPostResponse"
import { DiscussionPanel } from "./components/DiscussionPanel"

import { heroiconsX as IconX, heroiconsThumbsup as IconThumbsUp } from "@fider/icons.generated"
import { HStack, VStack } from "@fider/components/layout/Stack"
import { Trans } from "@lingui/react/macro"
import { i18n } from "@lingui/core"
import { TagsPanel } from "./components/TagsPanel"
import { usePostVote } from "@fider/hooks/usePostVote"
import { VoteSection } from "./components/VoteSection"
import { DeletePostModal } from "./components/DeletePostModal"
import { ResponseModal } from "./components/ResponseModal"
import { VotesPanel } from "./components/VotesPanel"
import { SponsorSpot } from "@fider/components/sponsorship/SponsorProvider"
import { useShowPostState } from "@fider/pages/ShowPost/hooks/useShowPostState"
import { prepareDraftImages } from "@fider/services/draftImages"

interface ReportStatus {
  hasReportedPost: boolean
  dailyLimitReached: boolean
}

interface ShowPostPageProps {
  post: Post
  subscribed: boolean
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
    initialAttachments: props.attachments,
  })
  
  const vote = usePostVote(props.post)
  
  const handleCommentAdded = useCallback(() => {
    void vote.refresh()

    if (isPostArchived(props.post)) {
      window.location.reload()
    }
  }, [props.post, vote.refresh])

  const handleCopyEvent = useCallback(() => {
    const selection = window.getSelection()
    if (selection && selection.toString().trim()) {
      state.setHasCopiedContent(true)
    }
  }, [state.setHasCopiedContent])

  useEffect(() => {
    state.setNewTitle(props.post.title)
    state.setNewDescription(props.post.description)
  }, [props.post.number])

  useEffect(() => {
    document.addEventListener("copy", handleCopyEvent)
    const canonicalPath = `/posts/${props.post.number}/${props.post.slug}`
    if (window.location.pathname !== canonicalPath) {
      window.history.replaceState({}, document.title, canonicalPath + window.location.search + window.location.hash)
    }

    return () => {
      document.removeEventListener("copy", handleCopyEvent)
    }
  }, [handleCopyEvent, props.post.number, props.post.slug])

  const handleScrollToTop = useCallback(() => {
    window.scrollTo({ top: 0, behavior: "smooth" })
  }, [])

  const saveChanges = useCallback(async () => {
    const result = await postActions.updatePost(props.post.number, state.newTitle, state.newDescription, await prepareDraftImages(state.attachments))
    if (result.ok) {
      location.reload()
    } else {
      state.setError(result.error)
    }
  }, [props.post.number, state.newTitle, state.newDescription, state.attachments, state.setError])

  const canDeletePost = props.post.permissions.delete

  const onActionSelected = useCallback(
    (action: "copy" | "delete" | "status" | "edit" | "lock" | "unlock" | "report" | "archive" | "unarchive" | "hide" | "unhide") => async () => {
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
        const result = await postActions.archivePost(props.post.number)
        if (result.ok) {
          notify.success(<Trans id="showpost.archive.success">Post has been archived</Trans>)
          location.reload()
        }
      } else if (action === "unarchive") {
        const result = await postActions.unarchivePost(props.post.number)
        if (result.ok) {
          notify.success(<Trans id="showpost.unarchive.success">Post has been unarchived</Trans>)
          location.reload()
        }
      } else if (action === "hide") {
        const result = await postActions.hidePost(props.post.id)
        if (result.ok) {
          notify.success(<Trans id="showpost.hide.success">Post has been hidden</Trans>)
          location.reload()
        }
      } else if (action === "unhide") {
        const result = await postActions.unhidePost(props.post.id)
        if (result.ok) {
          notify.success(<Trans id="showpost.unhide.success">Post has been unhidden</Trans>)
          location.reload()
        }
      }
    },
    [state.startEdit, state.openModal, props.post.number, props.post.id]
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
                            <Moment locale={Fider.currentLocale} date={vote.lastActivityAt} showTooltip={false} />
                          </span>
                        </VStack>
                      </HStack>
                    )}
                  </VStack>

                  {!state.editMode && (
                    <HStack spacing={1} className="items-center">
                      <ReportButton
                        allowed={props.post.permissions.report}
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
                        {props.post.permissions.respond.length > 0 && (
                          <Dropdown.ListItem onClick={onActionSelected("status")}>
                            <Trans id="action.respond">Respond</Trans>
                          </Dropdown.ListItem>
                        )}
                        {props.post.permissions.edit && (
                          <>
                            <Dropdown.ListItem onClick={onActionSelected("edit")}>
                              <Trans id="action.edit">Edit</Trans>
                            </Dropdown.ListItem>
                            {props.post.permissions.lock && (
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
                            {props.post.permissions.archive && (
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
                            {props.post.permissions.moderate && (
                              <>
                                {!isPostHidden(props.post) ? (
                                  <Dropdown.ListItem onClick={onActionSelected("hide")}>
                                    <Trans id="action.hide">Hide</Trans>
                                  </Dropdown.ListItem>
                                ) : (
                                  <Dropdown.ListItem onClick={onActionSelected("unhide")}>
                                    <Trans id="action.unhide">Unhide</Trans>
                                  </Dropdown.ListItem>
                                )}
                              </>
                            )}
                          </>
                        )}
                        {canDeletePost && (
                          <Dropdown.ListItem onClick={onActionSelected("delete")} className="text-danger">
                            <Trans id="action.delete">Delete</Trans>
                          </Dropdown.ListItem>
                        )}
                      </Dropdown>
                    </HStack>
                  )}
                </HStack>

                <div className="grow">
                  {state.editMode ? (
                    <Form error={state.error}>
                      <Input field="title" maxLength={100} value={state.newTitle} onChange={state.setNewTitle} />
                    </Form>
                  ) : (
                    <>
                      <h1 className="text-large" data-morph={`post-${props.post.number}`}>{props.post.title}</h1>
                      {isPostLocked(props.post) && <LockStatus post={props.post} />}
                      {isPostArchived(props.post) && <ArchiveStatus post={props.post} />}
                      {isPostHidden(props.post) && <HiddenStatus post={props.post} />}
                    </>
                  )}
                </div>

                <DeletePostModal
                  onModalClose={state.closeModal}
                  showModal={state.isModalOpen("delete")}
                  post={props.post}
                />
                <VStack>
                  {state.editMode ? (
                    <Form error={state.error}>
                      <TextArea field="description" value={state.newDescription} onChange={state.setNewDescription} />
                      <MultiImageUploader
                        field="attachments"
                        value={state.attachments}
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
                      <VoteSection post={props.post} />
                    </div>
                  ) : (
                    <HStack>
                      <Button variant="primary" onClick={saveChanges} disabled={!props.post.permissions.edit}>
                        <Icon sprite={IconThumbsUp} />{" "}
                        <span>
                          <Trans id="action.save">Save</Trans>
                        </span>
                      </Button>
                      <Button variant="tertiary" onClick={state.cancelEdit}>
                        <Icon sprite={IconX} />
                        <span>
                          <Trans id="action.cancel">Cancel</Trans>
                        </span>
                      </Button>
                    </HStack>
                  )}
                </VStack>

                <ResponseDetails status={props.post.status} response={props.post.response} previousStatus={props.post.archivedSettings?.previousStatus} />
              </VStack>

              <SponsorSpot position="before" />
              <DiscussionPanel
                post={props.post}
                subscribed={props.subscribed}
                onCommentAdded={handleCommentAdded}
              />
              <SponsorSpot position="after" />
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
            <VotesPanel post={props.post} votes={props.votes} revision={vote.revision} />
          </div>
        </div>
      </div>
      {props.post.permissions.respond.length > 0 && (
        <ResponseModal
          onCloseModal={state.closeModal}
          showModal={state.isModalOpen("response")}
          post={props.post}
          tags={props.tags}
          attachments={props.attachments}
          hasCopiedContent={state.hasCopiedContent}
        />
      )}
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
