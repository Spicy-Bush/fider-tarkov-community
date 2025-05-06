import "./ShowPost.page.scss"

import React from "react"

import { LockStatus } from "./components/LockStatus"
import { PostLockingModal } from "./components/PostLockingModal"
import { Comment, Post, Tag, Vote, ImageUpload, CurrentUser, PostStatus, isPostLocked, UserRole } from "@fider/models"
import { actions, clearUrlHash, Failure, Fider, notify, timeAgo } from "@fider/services"
import IconDotsHorizontal from "@fider/assets/images/heroicons-dots-horizontal.svg"
import IconChevronUp from "@fider/assets/images/heroicons-chevron-up.svg"

import {
  ResponseDetails,
  Button,
  UserName,
  Moment,
  Markdown,
  Input,
  Form,
  TextArea,
  MultiImageUploader,
  Icon,
  Header,
  Avatar,
  Dropdown,
} from "@fider/components"
import { ImageGallery } from "./components/ImageGallery"
import { DiscussionPanel } from "./components/DiscussionPanel"

import IconX from "@fider/assets/images/heroicons-x.svg"
import IconThumbsUp from "@fider/assets/images/heroicons-thumbsup.svg"
import { HStack, VStack } from "@fider/components/layout"
import { Trans } from "@lingui/react/macro"
import { TagsPanel } from "./components/TagsPanel"
import { FollowButton } from "./components/FollowButton"
import { VoteSection } from "./components/VoteSection"
import { DeletePostModal } from "./components/DeletePostModal"
import { ResponseModal } from "./components/ResponseModal"
import { VotesPanel } from "./components/VotesPanel"

interface ShowPostPageProps {
  post: Post
  subscribed: boolean
  comments: Comment[]
  tags: Tag[]
  votes: Vote[]
  attachments: string[]
}

interface ShowPostPageState {
  editMode: boolean
  newTitle: string
  showDeleteModal: boolean
  showResponseModal: boolean
  attachments: ImageUpload[]
  newDescription: string
  highlightedComment?: number
  showLockModal: boolean;
  showUnlockModal: boolean;
  error?: Failure
}

const oneHour = 3600
const canEditPost = (user: CurrentUser, post: Post) => {
  if (!user) {
    return false
  }

  // If user is collaborator or admin, they can edit any post
  if (user.isCollaborator || user.isAdministrator) {
    return true
  }

  // If user is moderator, they can only edit posts from regular users
  if (user.isModerator) {
    return post.user.role === UserRole.Visitor
  }

  // Regular users can only edit their own posts within 1 hour
  return user.id === post.user.id && timeAgo(post.createdAt) <= oneHour
}

export default class ShowPostPage extends React.Component<ShowPostPageProps, ShowPostPageState> {
  constructor(props: ShowPostPageProps) {
    super(props)

    this.state = {
      editMode: false,
      showDeleteModal: false,
      showResponseModal: false,
      newTitle: this.props.post.title,
      newDescription: this.props.post.description,
      attachments: [],
      showLockModal: false,
      showUnlockModal: false,
    }
  }

  public componentDidMount() {
    this.handleHashChange()
    window.addEventListener("hashchange", this.handleHashChange)
  }

  public componentWillUnmount() {
    window.removeEventListener("hashchange", this.handleHashChange)
  }

  private handleScrollToTop = () => {
    window.scrollTo({ top: 0, behavior: "smooth" })
  }

  private saveChanges = async () => {
    const result = await actions.updatePost(this.props.post.number, this.state.newTitle, this.state.newDescription, this.state.attachments)
    if (result.ok) {
      location.reload()
    } else {
      this.setState({
        error: result.error,
      })
    }
  }

  private setShowLockModal = (showLockModal: boolean) => {
    this.setState({ showLockModal })
  }
  
  private setShowUnlockModal = (showUnlockModal: boolean) => {
    this.setState({ showUnlockModal })
  }

  private canDeletePost = () => {
    const status = PostStatus.Get(this.props.post.status)
    if (
      !Fider.session.isAuthenticated ||
      status.closed
    ) {
      return false
    }

    const user = Fider.session.user
    if (!user) {
      return false
    }

    // If user is collaborator or admin, they can delete any post
    if (user.isCollaborator || user.isAdministrator) {
      return true
    }

    // If user is moderator, they can only delete posts from regular users
    if (user.isModerator) {
      return this.props.post.user.role === UserRole.Visitor
    }

    return false
  }

  private setNewTitle = (newTitle: string) => {
    this.setState({ newTitle })
  }

  private setNewDescription = (newDescription: string) => {
    this.setState({ newDescription })
  }

  private setAttachments = (attachments: ImageUpload[]) => {
    this.setState({ attachments })
  }

  private setShowDeleteModal = (showDeleteModal: boolean) => {
    this.setState({ showDeleteModal })
  }

  private setShowResponseModal = (showResponseModal: boolean) => {
    this.setState({ showResponseModal })
  }

  private cancelEdit = async () => {
    this.setState({ error: undefined, editMode: false })
  }

  private startEdit = async () => {
    this.setState({ editMode: true })
  }

  private handleHashChange = (e?: Event) => {
    const hash = window.location.hash
    const result = /#comment-([0-9]+)/.exec(hash)

    let highlightedComment
    if (result === null) {
      // No match
      highlightedComment = undefined
    } else {
      // Match, extract numeric ID
      const id = parseInt(result[1])
      if (this.props.comments.map((comment) => comment.id).includes(id)) {
        highlightedComment = id
      } else {
        // Unknown comment
        if (e?.cancelable) {
          e.preventDefault()
        } else {
          clearUrlHash(true)
        }
        notify.error(<Trans id="showpost.comment.unknownhighlighted">Unknown comment ID #{id}</Trans>)
        highlightedComment = undefined
      }
    }
    this.setState({ highlightedComment }, () => {
      this.scrollToHighlightedComment()
    })
  }
  
  private scrollToHighlightedComment = () => {
    const { highlightedComment } = this.state
    if (highlightedComment) {
      setTimeout(() => {
        const element = document.getElementById(`comment-${highlightedComment}`)
        if (element) {
          element.scrollIntoView({ behavior: 'smooth', block: 'start' })
        }
      }, 100)
    }
  }

  public onActionSelected = (action: "copy" | "delete" | "status" | "edit" | "lock" | "unlock") => () => {
    if (action === "copy") {
      navigator.clipboard.writeText(window.location.href)
      notify.success(<Trans id="showpost.copylink.success">Link copied to clipboard</Trans>)
    } else if (action === "delete") {
      this.setShowDeleteModal(true)
    } else if (action === "status") {
      this.setShowResponseModal(true)
    } else if (action === "edit") {
      this.startEdit()
    } else if (action === "lock") {
      this.setShowLockModal(true)
    }
    else if (action === "unlock") {
      this.setShowUnlockModal(true)
    }
  }

  public render() {
    return (
      <>
        <Header />
        <div id="p-show-post" className="page container">
          <div className="p-show-post">
            <div className="p-show-post__main-col">
              <div className="p-show-post__header-col">
                <VStack spacing={8}>
                  <HStack justify="between">
                    <VStack align="start">
                      {!this.state.editMode && (
                        <HStack>
                          <Avatar user={this.props.post.user} />
                          <VStack spacing={1}>
                            <UserName user={this.props.post.user} />
                            <Moment className="text-muted" locale={Fider.currentLocale} date={this.props.post.createdAt} />
                          </VStack>
                        </HStack>
                      )}
                    </VStack>

                    {!this.state.editMode && (
                      <Dropdown position="left" renderHandle={<Icon sprite={IconDotsHorizontal} width="24" height="24" />}>
                        <Dropdown.ListItem onClick={this.onActionSelected("copy")}>
                          <Trans id="action.copylink">Copy link</Trans>
                        </Dropdown.ListItem>
                        {Fider.session.isAuthenticated && canEditPost(Fider.session.user, this.props.post) && (
                          <>
                            <Dropdown.ListItem onClick={this.onActionSelected("edit")}>
                              <Trans id="action.edit">Edit</Trans>
                            </Dropdown.ListItem>
                            {(Fider.session.user.isCollaborator || Fider.session.user.isModerator || Fider.session.user.isAdministrator) && (
                              <>
                                <Dropdown.ListItem onClick={this.onActionSelected("status")}>
                                  <Trans id="action.respond">Respond</Trans>
                                </Dropdown.ListItem>
                              {!isPostLocked(this.props.post) ? (
                                <Dropdown.ListItem onClick={this.onActionSelected("lock")}>
                                  <Trans id="action.lock">Lock</Trans>
                                </Dropdown.ListItem>
                              ) : (
                                <Dropdown.ListItem onClick={this.onActionSelected("unlock")}>
                                  <Trans id="action.unlock">Unlock</Trans>
                                </Dropdown.ListItem>
                              )}
                            </>
                          )}
                        </>
                      )}
                      {this.canDeletePost() && (
                        <Dropdown.ListItem onClick={this.onActionSelected("delete")} className="text-red-700">
                          <Trans id="action.delete">Delete</Trans>
                        </Dropdown.ListItem>
                      )}
                    </Dropdown>
                    )}
                  </HStack>

                  <div className="flex-grow">
                    {this.state.editMode ? (
                      <Form error={this.state.error}>
                        <Input field="title" maxLength={100} value={this.state.newTitle} onChange={this.setNewTitle} />
                      </Form>
                    ) : (
                      <>
                        <h1 className="text-large">{this.props.post.title}</h1>
                        {isPostLocked(this.props.post) && <LockStatus post={this.props.post} />}
                      </>
                    )}
                  </div>

                  <DeletePostModal onModalClose={() => this.setShowDeleteModal(false)} showModal={this.state.showDeleteModal} post={this.props.post} />
                    {Fider.session.isAuthenticated && (Fider.session.user.isCollaborator || Fider.session.user.isModerator || Fider.session.user.isAdministrator) && (
                      <ResponseModal onCloseModal={() => this.setShowResponseModal(false)} showModal={this.state.showResponseModal} post={this.props.post} />
                    )}
                  <VStack>
                    {this.state.editMode ? (
                      <Form error={this.state.error}>
                        <TextArea field="description" value={this.state.newDescription} onChange={this.setNewDescription} />
                        <MultiImageUploader field="attachments" bkeys={this.props.attachments} maxUploads={3} onChange={this.setAttachments} />
                      </Form>
                    ) : (
                      <>
                        {this.props.post.description && <Markdown className="description" text={this.props.post.description} style="full" />}
                        {!this.props.post.description && (
                          <em className="text-muted">
                            <Trans id="showpost.message.nodescription">No description provided.</Trans>
                          </em>
                        )}
                        {this.props.attachments.length > 0 && (
                          <ImageGallery bkeys={this.props.attachments} />
                        )}
                      </>
                    )}
                  </VStack>
                  <div className="mt-2">
                    <TagsPanel post={this.props.post} tags={this.props.tags} />
                  </div>

                  <VStack spacing={4}>
                    {!this.state.editMode ? (
                      <HStack justify="between" align="start">
                        <VoteSection post={this.props.post} votes={this.props.post.votesCount} />
                        <FollowButton post={this.props.post} subscribed={this.props.subscribed} />
                      </HStack>
                    ) : (
                      <HStack>
                        <Button variant="primary" onClick={this.saveChanges} disabled={Fider.isReadOnly}>
                          <Icon sprite={IconThumbsUp} />{" "}
                          <span>
                            <Trans id="action.save">Save</Trans>
                          </span>
                        </Button>
                        <Button onClick={this.cancelEdit} disabled={Fider.isReadOnly}>
                          <Icon sprite={IconX} />
                          <span>
                            <Trans id="action.cancel">Cancel</Trans>
                          </span>
                        </Button>
                      </HStack>
                    )}
                    <div className="border-4 border-blue-500" />
                  </VStack>

                  <ResponseDetails status={this.props.post.status} response={this.props.post.response} />
                </VStack>
              </div>

              <div className="p-show-post__discussion_col">
                <DiscussionPanel post={this.props.post} comments={this.props.comments} highlightedComment={this.state.highlightedComment} />
                <Button
                  variant="secondary"
                  onClick={this.handleScrollToTop}
                  className="mt-4"
                >
                  <Icon sprite={IconChevronUp} />
                  <span>
                    <Trans id="returntop.button">Return to top</Trans>
                  </span>
                </Button>
              </div>
            </div>
            <div className="p-show-post__action-col">
              <VotesPanel post={this.props.post} votes={this.props.votes} />
            </div>
          </div>
        </div>
        <PostLockingModal
          post={this.props.post}
          isOpen={this.state.showLockModal || this.state.showUnlockModal}
          onClose={() => this.state.showLockModal ? this.setShowLockModal(false) : this.setShowUnlockModal(false)}
          mode={this.state.showLockModal ? "lock" : "unlock"}
        />
      </>
    )
  }
}
