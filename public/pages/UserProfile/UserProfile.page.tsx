import React, { useState, useEffect, useRef } from "react"
import { Header, Avatar, UserName, Input, Button, Select, Modal, Form, Icon, Loader, SelectOption, Markdown } from "@fider/components"
import { Trans } from "@lingui/react/macro"
import { i18n } from "@lingui/core"
import { UserRole, UserStatus, UserAvatarType, UserSettings, ImageUpload } from "@fider/models"
import { actions, Fider, Failure, notify } from "@fider/services"
import IconSearch from "@fider/assets/images/heroicons-search.svg"
import IconX from "@fider/assets/images/heroicons-x.svg"
import IconDocument from "@fider/assets/images/heroicons-pencil-alt.svg"
import IconChat from "@fider/assets/images/heroicons-chat-alt-2.svg"
import IconCalendar from "@fider/assets/images/heroicons-calendar.svg"
import IconSort from "@fider/assets/images/heroicons-arrow-up-down.svg"
import IconBan from "@fider/assets/images/heroicons-block.svg"
import IconVolumeOff from "@fider/assets/images/heroicons-muted.svg"
import IconThumbsUp from "@fider/assets/images/heroicons-thumbsup.svg"
import { HStack, VStack } from "@fider/components/layout"
import "./UserProfile.scss"
import IconWarning from "@fider/assets/images/heroicons-exclamation.svg"
import { NotificationSettings } from "../MySettings/components/NotificationSettings"
import { APIKeyForm } from "../MySettings/components/APIKeyForm"
import { DangerZone } from "../MySettings/components/DangerZone"
import { ImageUploader } from "@fider/components"
import { ModerationModal } from "@fider/components/ModerationModal"

type ContentType = "all" | "posts" | "comments"
type SortField = "createdAt" | "title"
type SortOrder = "asc" | "desc"
type ProfileTab = "search" | "standing" | "settings"

interface UserProfilePageProps {
  user: {
    id: number
    name: string
    role: UserRole
    avatarURL: string
    status: UserStatus
    avatarType: UserAvatarType
  }
  userSettings?: UserSettings
}

interface Post {
  id: number
  title: string
  createdAt: string
}

interface Comment {
  id: number
  postId: number
  postTitle: string
  content: string
  createdAt: string
}

interface Warning {
  id: number
  reason: string
  createdAt: string
  expiresAt?: string
}

interface Mute {
  id: number
  reason: string
  createdAt: string
  expiresAt?: string
}

interface UserProfileStats {
  posts: number
  comments: number
  votes: number
}

interface UserProfileStanding {
  warnings: Warning[]
  mutes: Mute[]
}

interface UserProfileContent {
  posts: Post[]
  comments: Comment[]
}

interface UserProfilePageState {
  showModal: boolean
  name: string
  newEmail: string
  avatar?: ImageUpload
  avatarType: UserAvatarType
  changingEmail: boolean
  error?: Failure
  userSettings: UserSettings
  showAvatarModal: boolean
  isHoveringAvatar: boolean
  showNameEditModal: boolean
}

export default function UserProfilePage(props: UserProfilePageProps) {
    const [searchQuery, setSearchQuery] = useState("")
  const [contentType, setContentType] = useState<ContentType>("all")
  const [sortField, setSortField] = useState<SortField>("createdAt")
  const [sortOrder, setSortOrder] = useState<SortOrder>("desc")
  const [currentPage, setCurrentPage] = useState(1)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string>()
  const [activeTab, setActiveTab] = useState<ProfileTab>('search')
  const timerRef = useRef<number>()

    const [settingsState, setSettingsState] = useState<UserProfilePageState>({
    showModal: false,
    changingEmail: false,
    avatarType: Fider.session.user.avatarType,
    newEmail: "",
    name: props.user.name,
    userSettings: props.userSettings || {},
    showAvatarModal: false,
    isHoveringAvatar: false,
    showNameEditModal: false
  })

    const [searchResults, setSearchResults] = useState<UserProfileContent>({
    posts: [],
    comments: []
  })

    const [stats, setStats] = useState<UserProfileStats>({
    posts: 0,
    comments: 0,
    votes: 0
  })

    const [standing, setStanding] = useState<UserProfileStanding>({
    warnings: [],
    mutes: []
  })

    const [moderationModal, setModerationModal] = useState({
    isOpen: false,
    actionType: 'mute' as 'mute' | 'warning',
    error: undefined as Failure | undefined
  })

    const [avatarModalState, setAvatarModalState] = useState({
    isOpen: false,
    error: undefined as Failure | undefined,
    isHoveringAvatar: false
  })

  const ITEMS_PER_PAGE = 10

  useEffect(() => {
    loadStats(props.user.id)
    loadStanding(props.user.id)
  }, [props.user.id])

  useEffect(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current)
    }

    if (!searchQuery) {
      setSearchResults({ posts: [], comments: [] })
      return
    }

    setIsLoading(true)
    timerRef.current = window.setTimeout(() => {
      actions.searchUserContent(props.user.id, searchQuery).then((result) => {
        if (result.ok) {
          setSearchResults(result.data)
        }
        setIsLoading(false)
      })
    }, 500)

    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current)
      }
    }
  }, [searchQuery, props.user.id])

    useEffect(() => {
    const handleHashChange = () => {
      const hash = window.location.hash.replace('#', '');
      if (hash === 'search' || hash === 'standing' || hash === 'settings') {
        setActiveTab(hash as ProfileTab);
      }
    };

        handleHashChange();

        window.addEventListener('hashchange', handleHashChange);
    return () => window.removeEventListener('hashchange', handleHashChange);
  }, []);

    const handleTabChange = (tab: ProfileTab) => {
    setActiveTab(tab);
    window.location.hash = tab;
  };

  const loadStats = async (userID: number) => {
    const result = await actions.getUserProfileStats(userID)
    if (result.ok) {
      setStats(result.data)
    } else {
      setError("You are not authorized to view this profile")
    }
  }

  const loadStanding = async (userID: number) => {
    const result = await actions.getUserProfileStanding(userID)
    if (result.ok) {
      setStanding(result.data)
    } else {
      setError("You are not authorized to view this profile")
    }
  }

  const clearSearch = () => {
    setSearchQuery("")
    setSearchResults({ posts: [], comments: [] })
  }

  const handleContentTypeChange = (option?: { value: string }) => {
    if (option) {
      setContentType(option.value as ContentType)
      setCurrentPage(1)
    }
  }

  const handleSortChange = (field: SortField) => {
    setSortField(field)
    setSortOrder(prevOrder => 
      prevOrder === "desc" && field === sortField ? "asc" : "desc"
    )
  }

  const canModerateUser = (targetUser: { role: UserRole }) => {
    const currentUser = Fider.session.user
    if (!currentUser) return false

        if (currentUser.role === UserRole.Visitor) return false

        if (currentUser.id === props.user.id) return false

        if (targetUser.role === UserRole.Administrator) return false

        if (currentUser.role === UserRole.Moderator) {
      return targetUser.role === UserRole.Visitor
    }

        if (currentUser.role === UserRole.Collaborator) {
      return targetUser.role === UserRole.Visitor || targetUser.role === UserRole.Moderator
    }

        return currentUser.role === UserRole.Administrator
  }

  const canBlockUser = (targetUser: { role: UserRole }) => {
    const currentUser = Fider.session.user
    if (!currentUser) return false

        if (currentUser.role !== UserRole.Administrator && currentUser.role !== UserRole.Collaborator) return false

        if (currentUser.id === props.user.id) return false

        if (targetUser.role === UserRole.Administrator) return false

        if (currentUser.role === UserRole.Collaborator) {
      return targetUser.role === UserRole.Visitor
    }

        return true
  }

    const openMuteModal = () => {
    setModerationModal({
      isOpen: true,
      actionType: 'mute',
      error: undefined
    })
  }

    const openWarnModal = () => {
    setModerationModal({
      isOpen: true,
      actionType: 'warning',
      error: undefined
    })
  }

    const handleModeration = async (data: { reason: string; duration: string }) => {
    if (moderationModal.actionType === 'mute') {
      const result = await actions.muteUser(props.user.id, {
        reason: data.reason,
        duration: data.duration
      })

      if (result.ok) {
        setModerationModal(prev => ({ ...prev, isOpen: false }))
        await loadStanding(props.user.id)
      } else if (result.error) {
        setModerationModal(prev => ({ ...prev, error: result.error }))
      }
    } else {
      const result = await actions.warnUser(props.user.id, {
        reason: data.reason,
        duration: data.duration
      })

      if (result.ok) {
        setModerationModal(prev => ({ ...prev, isOpen: false }))
        await loadStanding(props.user.id)
      } else if (result.error) {
        setModerationModal(prev => ({ ...prev, error: result.error }))
      }
    }
  }

  const handleBlockUser = async () => {
    const result = await actions.blockUser(props.user.id)
    if (result.ok) {
            window.location.reload()
    }
  }

  const handleUnblockUser = async () => {
    const result = await actions.unblockUser(props.user.id)
    if (result.ok) {
            window.location.reload()
    }
  }

  const handleDeleteWarning = async (warningId: number) => {
    const result = await actions.deleteWarning(props.user.id, warningId)
    if (result.ok) {
      loadStanding(props.user.id)
      notify.success(i18n._("profile.warning.delete.success", { message: "Warning has been deleted successfully" }))
    }
  }

  const handleDeleteMute = async (muteId: number) => {
    const result = await actions.deleteMute(props.user.id, muteId)
    if (result.ok) {
      loadStanding(props.user.id)
      notify.success(i18n._("profile.mute.delete.success", { message: "Mute has been deleted successfully" }))
    }
  }

  const canDeleteModeration = (targetUser: { role: UserRole }) => {
    const currentUser = Fider.session.user
    if (!currentUser) return false

    if (currentUser.role !== UserRole.Administrator && 
        currentUser.role !== UserRole.Collaborator && 
        currentUser.role !== UserRole.Moderator) return false

    if (currentUser.id === props.user.id) return false

    if (targetUser.role === UserRole.Administrator) return false

    if (currentUser.role === UserRole.Moderator) {
      return targetUser.role === UserRole.Visitor
    }

    if (currentUser.role === UserRole.Collaborator) {
      return targetUser.role === UserRole.Visitor
    }

    return true
  }

  const getFilteredAndSortedResults = () => {
    let results: Array<any> = []

    if (contentType === "all" || contentType === "posts") {
      results = [...results, ...searchResults.posts.map(post => ({ ...post, type: "post" }))]
    }
    if (contentType === "all" || contentType === "comments") {
      results = [...results, ...searchResults.comments.map(comment => ({ ...comment, type: "comment" }))]
    }

    results.sort((a, b) => {
      const aValue = sortField === "createdAt" ? new Date(a.createdAt).getTime() : a.title || a.postTitle
      const bValue = sortField === "createdAt" ? new Date(b.createdAt).getTime() : b.title || b.postTitle
      
      if (sortOrder === "asc") {
        return aValue > bValue ? 1 : -1
      }
      return aValue < bValue ? 1 : -1
    })

    const startIndex = (currentPage - 1) * ITEMS_PER_PAGE
    const endIndex = startIndex + ITEMS_PER_PAGE
    return results.slice(startIndex, endIndex)
  }

  const getTotalPages = () => {
    let totalItems = 0
    if (contentType === "all" || contentType === "posts") {
      totalItems += searchResults.posts.length
    }
    if (contentType === "all" || contentType === "comments") {
      totalItems += searchResults.comments.length
    }
    return Math.ceil(totalItems / ITEMS_PER_PAGE)
  }

  const filteredResults = getFilteredAndSortedResults()
  const totalPages = getTotalPages()
  const canModerate = canModerateUser(props.user)
  const canBlock = canBlockUser(props.user)
  const isBlocked = props.user.status === UserStatus.Blocked

  const confirmSettings = async () => {
    const nameResult = await actions.updateUserName({
      name: settingsState.name,
    });
    if (!nameResult.ok) {
      setSettingsState(prev => ({ ...prev, error: nameResult.error }));
      return;
    }

    const avatarResult = await actions.updateUserAvatar({
      avatarType: settingsState.avatarType,
      avatar: settingsState.avatar,
    });
    if (!avatarResult.ok) {
      setSettingsState(prev => ({ ...prev, error: avatarResult.error }));
      return;
    }

    const settingsResult = await actions.updateUserSettings({
      settings: settingsState.userSettings,
    });
    if (settingsResult.ok) {
      location.reload();
    } else if (settingsResult.error) {
      setSettingsState(prev => ({ ...prev, error: settingsResult.error }));
    }
  }

  const submitNewEmail = async () => {
    const result = await actions.changeUserEmail(settingsState.newEmail)
    if (result.ok) {
      setSettingsState(prev => ({
        ...prev,
        error: undefined,
        changingEmail: false,
        showModal: true,
      }))
    } else if (result.error) {
      setSettingsState(prev => ({ ...prev, error: result.error }))
    }
  }

  const startChangeEmail = () => {
    setSettingsState(prev => ({ ...prev, changingEmail: true }))
  }

  const cancelChangeEmail = () => {
    setSettingsState(prev => ({
      ...prev,
      changingEmail: false,
      newEmail: "",
      error: undefined,
    }))
  }

  const setName = (name: string) => {
    setSettingsState(prev => ({ ...prev, name }))
  }

  const setNotificationSettings = (userSettings: UserSettings) => {
    setSettingsState(prev => ({ ...prev, userSettings }))
  }

  const setNewEmail = (newEmail: string) => {
    setSettingsState(prev => ({ ...prev, newEmail }))
  }

  const handleAvatarClick = () => {
    const canChangeAvatar = props.user.id === Fider.session.user?.id || 
      (Fider.session.user?.role === UserRole.Administrator || 
       Fider.session.user?.role === UserRole.Collaborator ||
       Fider.session.user?.role === UserRole.Moderator)
    
    if (canChangeAvatar) {
      setAvatarModalState(prev => ({ ...prev, isOpen: true }))
    }
  }

  const handleAvatarChange = async () => {
    const result = await actions.updateUserAvatar({
      avatarType: settingsState.avatarType,
      avatar: settingsState.avatar,
    }, props.user.id);

    if (result.ok) {
      setAvatarModalState(prev => ({ ...prev, isOpen: false }))
      location.reload()
    } else if (result.error) {
      setAvatarModalState(prev => ({ ...prev, error: result.error }))
    }
  }

  const handleAvatarTypeChange = (opt?: SelectOption) => {
    if (opt) {
      setSettingsState(prev => ({ ...prev, avatarType: opt.value as UserAvatarType }))
    }
  }

  const handleAvatarUpload = (avatar: ImageUpload): void => {
    setSettingsState(prev => ({ ...prev, avatar }))
  }

  const canEditName = () => {
    const currentUser = Fider.session.user
    if (!currentUser) return false

    if (currentUser.id === props.user.id) return true

    return currentUser.role === UserRole.Administrator || 
           currentUser.role === UserRole.Moderator || 
           currentUser.role === UserRole.Collaborator
  }

  const handleNameChange = async () => {
    const result = await actions.updateUserName({
      name: settingsState.name,
    }, props.user.id);

    if (result.ok) {
      setSettingsState(prev => ({ ...prev, showNameEditModal: false }))
      location.reload()
    } else if (result.error) {
      setSettingsState(prev => ({ ...prev, error: result.error }))
    }
  }

  if (error) {
    return (
      <>
        <Header />
        <div className="container">
          <div className="c-user-profile">
            <div className="c-user-profile__error">
              {error}
            </div>
          </div>
        </div>
      </>
    )
  }

  return (
    <>
      <Header />
      <div className="container">
        <div className="c-user-profile">
          <div className="c-user-profile__header">
            <div 
              className="c-user-profile__avatar"
              onMouseEnter={() => setAvatarModalState(prev => ({ ...prev, isHoveringAvatar: true }))}
              onMouseLeave={() => setAvatarModalState(prev => ({ ...prev, isHoveringAvatar: false }))}
              onClick={handleAvatarClick}
            >
              <Avatar user={props.user} size="normal" imageSize={120} />
              {avatarModalState.isHoveringAvatar && (
                <div className="c-user-profile__avatar-overlay">
                  <Trans id="profile.avatar.change">Change Avatar</Trans>
                </div>
              )}
            </div>
            <div className="c-user-profile__info">
              <div className="c-user-profile__name-container">
                <UserName user={props.user} />
                {canEditName() && (
                  <button 
                    className="c-user-profile__edit-name"
                    onClick={() => setSettingsState(prev => ({ ...prev, showNameEditModal: true }))}
                  >
                    <Icon sprite={IconDocument} className="h-4" />
                  </button>
                )}
              </div>
              {canBlock && (
                <div className="c-user-profile__moderation">
                  {!isBlocked ? (
                    <button className="danger" onClick={handleBlockUser}>
                      <Trans id="action.block">Block User</Trans>
                    </button>
                  ) : (
                    <button className="secondary" onClick={handleUnblockUser}>
                      <Trans id="action.unblock">Unblock User</Trans>
                    </button>
                  )}
                </div>
              )}
              {canModerate && (
                <div className="c-user-profile__moderation">
                  <button className="danger" onClick={openMuteModal}>
                    <Trans id="action.mute">Mute User</Trans>
                  </button>
                  <button className="secondary" onClick={openWarnModal}>
                    <Trans id="action.warn">Warn User</Trans>
                  </button>
                </div>
              )}
            </div>
            <div className="c-user-profile__stats-card">
              <div className="c-user-profile__stats">
                <div className="stat-item">
                  <Icon sprite={IconDocument} className="h-4" />
                  <span>{stats.posts}</span>
                </div>
                <div className="stat-item">
                  <Icon sprite={IconChat} className="h-4" />
                  <span>{stats.comments}</span>
                </div>
                <div className="stat-item">
                  <Icon sprite={IconThumbsUp} className="h-4" />
                  <span>{stats.votes}</span>
                </div>
              </div>
            </div>
          </div>

          {isBlocked && (
            <div className="c-user-profile__status-item blocked">
              <Icon sprite={IconBan} className="h-4" />
              <span><Trans id="profile.status.blocked">This user is blocked</Trans></span>
            </div>
          )}
          {standing.mutes.filter(mute => !mute.expiresAt || new Date(mute.expiresAt) > new Date()).map(mute => (
            <div key={mute.id} className="c-user-profile__status-item muted">
              <Icon sprite={IconVolumeOff} className="h-4" />
              <span>
                <Trans id="profile.status.muted">Muted until {new Date(mute.expiresAt!).toLocaleString()}</Trans>
                {mute.reason && (
                  <span className="c-user-profile__status-reason">
                    <Trans id="profile.status.reason">Reason: {mute.reason}</Trans>
                  </span>
                )}
              </span>
            </div>
          ))}

          <div className="c-user-profile__sidebar">
            <div className="c-user-profile__nav">
              <button 
                className={`c-user-profile__nav-button ${activeTab === 'search' ? 'active' : ''}`}
                onClick={() => handleTabChange('search')}
              >
                <span className="icon">
                  <Icon sprite={IconSearch} width="16" height="16" />
                </span>
                <Trans id="profile.tab.search">Search Posts</Trans>
              </button>
              <button 
                className={`c-user-profile__nav-button ${activeTab === 'standing' ? 'active' : ''}`}
                onClick={() => handleTabChange('standing')}
              >
                <span className="icon">
                  <Icon sprite={IconWarning} width="16" height="16" />
                </span>
                <Trans id="profile.tab.standing">Standing</Trans>
              </button>
              {props.user.id === Fider.session.user?.id && (
                <button 
                  className={`c-user-profile__nav-button ${activeTab === 'settings' ? 'active' : ''}`}
                  onClick={() => handleTabChange('settings')}
                >
                  <span className="icon">
                    <Icon sprite={IconDocument} width="16" height="16" />
                  </span>
                  <Trans id="profile.tab.settings">Settings</Trans>
                </button>
              )}
            </div>

            <div className={`c-user-profile__content ${activeTab === 'settings' ? 'active' : ''}`}>
              {props.user.id === Fider.session.user?.id && (
                <>
                  <Modal.Window isOpen={settingsState.showModal} onClose={() => setSettingsState(prev => ({ ...prev, showModal: false }))}>
                    <Modal.Header>
                      <Trans id="modal.changeemail.header">Confirm your new email</Trans>
                    </Modal.Header>
                    <Modal.Content>
                      <div>
                        <p>
                          <Trans id="modal.changeemail.text">
                            We have just sent a confirmation link to <b>{settingsState.newEmail}</b>. <br /> Click the link to update your email.
                          </Trans>
                        </p>
                        <p>
                          <a href="#" onClick={() => setSettingsState(prev => ({ ...prev, showModal: false }))}>
                            <Trans id="action.ok">OK</Trans>
                          </a>
                        </p>
                      </div>
                    </Modal.Content>
                  </Modal.Window>

                  <Form error={settingsState.error}>
                    <Input
                      label={i18n._("label.email", { message: "Email" })}
                      field="email"
                      value={settingsState.changingEmail ? settingsState.newEmail : Fider.session.user.email}
                      maxLength={200}
                      disabled={!settingsState.changingEmail}
                      afterLabel={settingsState.changingEmail ? undefined : (
                        <Button variant="tertiary" size="small" onClick={startChangeEmail}>
                          <Trans id="action.change">change</Trans>
                        </Button>
                      )}
                      onChange={setNewEmail}
                    >
                      <p className="text-muted">
                        {Fider.session.user.email || settingsState.changingEmail ? (
                          <Trans id="mysettings.message.privateemail">Your email is private and will never be publicly displayed.</Trans>
                        ) : (
                          <Trans id="mysettings.message.noemail">Your account doesn&apos;t have an email.</Trans>
                        )}
                      </p>
                      {settingsState.changingEmail && (
                        <>
                          <Button variant="primary" size="small" onClick={submitNewEmail}>
                            <Trans id="action.confirm">Confirm</Trans>
                          </Button>
                          <Button variant="tertiary" size="small" onClick={cancelChangeEmail}>
                            <Trans id="action.cancel">Cancel</Trans>
                          </Button>
                        </>
                      )}
                    </Input>

                    <NotificationSettings 
                      userSettings={props.userSettings || {}} 
                      settingsChanged={setNotificationSettings} 
                    />

                    <Button variant="primary" onClick={confirmSettings}>
                      <Trans id="action.save">Save</Trans>
                    </Button>
                  </Form>

                  <div className="mt-8">
                    {Fider.session.user.isCollaborator && <APIKeyForm />}
                  </div>
                  <div className="mt-8">
                    <DangerZone />
                  </div>
                </>
              )}
            </div>

            <div className={`c-user-profile__content ${activeTab === 'search' ? 'active' : ''}`}>
              <div className="c-user-profile__search-controls">
                <div className="c-user-profile__search-input">
                  <Input
                    field="search"
                    icon={searchQuery ? IconX : IconSearch}
                    onIconClick={searchQuery ? clearSearch : undefined}
                    value={searchQuery}
                    onChange={setSearchQuery}
                    placeholder={
                      props.user.id === Fider.session.user?.id ? (
                        i18n._("profile.search.placeholder.own", { message: "Search your posts and comments..." })
                      ) : (
                        i18n._("profile.search.placeholder.other", { message: `Search ${props.user.name}'s posts and comments...` })
                      )
                    }
                  />
                </div>
                <Select
                  field="contentType"
                  defaultValue={contentType}
                  onChange={handleContentTypeChange}
                  options={[
                    { value: "all", label: i18n._("profile.search.filter.all", { message: "All" }) },
                    { value: "posts", label: i18n._("profile.search.filter.posts", { message: "Posts" }) },
                    { value: "comments", label: i18n._("profile.search.filter.comments", { message: "Comments" }) }
                  ]}
                />
              </div>

              {isLoading ? (
                <div className="c-user-profile__loading">
                  <Loader />
                </div>
              ) : searchResults.posts.length > 0 || searchResults.comments.length > 0 ? (
                <div className="c-user-profile__search-results">
                  <div className="c-user-profile__search-header">
                    <div className="c-user-profile__search-sort">
                      <Button
                        variant="tertiary"
                        onClick={() => handleSortChange("createdAt")}
                        className={sortField === "createdAt" ? "active" : ""}
                      >
                        <HStack spacing={2}>
                          <Icon sprite={IconCalendar} className="h-4" />
                          <span><Trans id="profile.search.sort.date">Date</Trans></span>
                          {sortField === "createdAt" && (
                            <Icon sprite={IconSort} className={`h-4 transform ${sortOrder === "asc" ? "rotate-180" : ""}`} />
                          )}
                        </HStack>
                      </Button>
                      <Button
                        variant="tertiary"
                        onClick={() => handleSortChange("title")}
                        className={sortField === "title" ? "active" : ""}
                      >
                        <HStack spacing={2}>
                          <Icon sprite={IconDocument} className="h-4" />
                          <span><Trans id="profile.search.sort.title">Title</Trans></span>
                          {sortField === "title" && (
                            <Icon sprite={IconSort} className={`h-4 transform ${sortOrder === "asc" ? "rotate-180" : ""}`} />
                          )}
                        </HStack>
                      </Button>
                    </div>
                  </div>

                  <VStack spacing={4} divide>
                    {filteredResults.map(item => (
                      <div key={`${item.type}-${item.id}`} className="c-user-profile__search-item">
                        <div className="c-user-profile__search-content">
                          <a href={item.type === "post" ? `/posts/${item.id}` : `/posts/${item.postId}#comment-${item.id}`}>
                            {item.type === "post" ? item.title : item.postTitle}
                          </a>
                          {item.type === "comment" && (
                            <Markdown text={item.content} style="full" />
                          )}
                          <div className="c-user-profile__search-meta">
                            <span className="meta-item">
                              <Icon sprite={item.type === "post" ? IconDocument : IconChat} className="h-4" />
                              {item.type === "post" ? (
                                <Trans id="profile.search.type.post">Post</Trans>
                              ) : (
                                <Trans id="profile.search.type.comment">Comment</Trans>
                              )}
                            </span>
                            <span className="meta-item">
                              <Icon sprite={IconCalendar} className="h-4" />
                              {new Date(item.createdAt).toLocaleDateString()}
                            </span>
                          </div>
                        </div>
                      </div>
                    ))}
                  </VStack>

                  {totalPages > 1 && (
                    <div className="c-user-profile__pagination">
                      <Button
                        variant="tertiary"
                        disabled={currentPage === 1}
                        onClick={() => setCurrentPage(prev => prev - 1)}
                      >
                        <Trans id="action.previous">Previous</Trans>
                      </Button>
                      <span className="c-user-profile__page-info">
                        <Trans id="profile.search.page">Page {currentPage} of {totalPages}</Trans>
                      </span>
                      <Button
                        variant="tertiary"
                        disabled={currentPage === totalPages}
                        onClick={() => setCurrentPage(prev => prev + 1)}
                      >
                        <Trans id="action.next">Next</Trans>
                      </Button>
                    </div>
                  )}
                </div>
              ) : searchQuery ? (
                <div className="c-user-profile__no-results">
                  <Trans id="profile.search.noresults">No results found</Trans>
                </div>
              ) : null}
            </div>

            <div className={`c-user-profile__content ${activeTab === 'standing' ? 'active' : ''}`}>
              <div className="c-user-profile__standing">
                {standing.warnings.length === 0 && standing.mutes.length === 0 ? (
                  <div className="c-user-profile__good-standing">
                    <Icon sprite={IconThumbsUp} className="h-8" />
                    <h4 className="c-user-profile__good-standing-title">
                      {props.user.id === Fider.session.user?.id ? (
                        <Trans id="profile.standing.good.self">YOU ARE IN GOOD STANDING</Trans>
                      ) : (
                        <Trans id="profile.standing.good.other">
                          {props.user.name} IS IN GOOD STANDING
                        </Trans>
                      )}
                    </h4>
                    <p className="c-user-profile__good-standing-subtitle">
                      <Trans id="profile.standing.good.description">You have no warnings or mutes.</Trans>
                    </p>
                  </div>
                ) : (
                  <>
                    <div className="c-user-profile__standing-section">
                      <h3><Trans id="profile.standing.warnings">Warnings</Trans></h3>
                      {standing.warnings.length === 0 ? (
                        <p className="text-muted"><Trans id="profile.standing.nowarnings">No warnings</Trans></p>
                      ) : (
                        <VStack spacing={4} divide>
                          {standing.warnings.map(warning => {
                            const isExpired = warning.expiresAt && new Date(warning.expiresAt) < new Date();
                            const isActive = warning.expiresAt && new Date(warning.expiresAt) > new Date();
                            return (
                              <div 
                                key={warning.id} 
                                className="c-user-profile__standing-item"
                              >
                                <div className="c-user-profile__standing-content">
                                  <p>{warning.reason}</p>
                                  <div className="c-user-profile__standing-meta">
                                    <span className="meta-item">
                                      <Icon sprite={IconCalendar} className="h-4" />
                                      {new Date(warning.createdAt).toLocaleDateString()}
                                    </span>
                                    {warning.expiresAt && (
                                      <span className="meta-item">
                                        <Icon sprite={IconWarning} className="h-4" />
                                        <Trans id="profile.standing.expires">Expires: {new Date(warning.expiresAt).toLocaleDateString()}</Trans>
                                      </span>
                                    )}
                                  </div>
                                </div>
                                <div className="c-user-profile__standing-actions">
                                  {(isExpired || isActive) && (
                                    <span className={`c-user-profile__standing-status ${isExpired ? 'expired' : 'active'}`}>
                                      {isExpired ? 'Expired' : 'Active'}
                                    </span>
                                  )}
                                  {canDeleteModeration(props.user) && (
                                    <Button variant="danger" size="small" onClick={() => handleDeleteWarning(warning.id)}>
                                      <Trans id="action.delete">Delete</Trans>
                                    </Button>
                                  )}
                                </div>
                              </div>
                            );
                          })}
                        </VStack>
                      )}
                    </div>

                    <div className="c-user-profile__standing-section">
                      <h3>
                        <Icon sprite={IconVolumeOff} className="h-4" />
                        <Trans id="profile.standing.mutes">Mutes</Trans>
                      </h3>
                      {standing.mutes.length === 0 ? (
                        <p className="text-muted"><Trans id="profile.standing.nomutes">No mutes</Trans></p>
                      ) : (
                        <VStack spacing={4} divide>
                          {standing.mutes.map(mute => {
                            const isExpired = mute.expiresAt && new Date(mute.expiresAt) < new Date();
                            const isActive = mute.expiresAt && new Date(mute.expiresAt) > new Date();
                            return (
                              <div 
                                key={mute.id} 
                                className="c-user-profile__standing-item"
                              >
                                <div className="c-user-profile__standing-content">
                                  <p>{mute.reason}</p>
                                  <div className="c-user-profile__standing-meta">
                                    <span className="meta-item">
                                      <Icon sprite={IconCalendar} className="h-4" />
                                      {new Date(mute.createdAt).toLocaleDateString()}
                                    </span>
                                    {mute.expiresAt && (
                                      <span className="meta-item">
                                        <Icon sprite={IconVolumeOff} className="h-4" />
                                        <Trans id="profile.standing.expires">Expires: {new Date(mute.expiresAt).toLocaleDateString()}</Trans>
                                      </span>
                                    )}
                                  </div>
                                </div>
                                <div className="c-user-profile__standing-actions">
                                  {(isExpired || isActive) && (
                                    <span className={`c-user-profile__standing-status ${isExpired ? 'expired' : 'active'}`}>
                                      {isExpired ? 'Expired' : 'Active'}
                                    </span>
                                  )}
                                  {canDeleteModeration(props.user) && (
                                    <Button variant="danger" size="small" onClick={() => handleDeleteMute(mute.id)}>
                                      <Trans id="action.delete">Delete</Trans>
                                    </Button>
                                  )}
                                </div>
                              </div>
                            );
                          })}
                        </VStack>
                      )}
                    </div>
                  </>
                )}
              </div>
            </div>
          </div>
        </div>
      </div>

      <ModerationModal
        isOpen={moderationModal.isOpen}
        onClose={() => setModerationModal(prev => ({ ...prev, isOpen: false }))}
        actionType={moderationModal.actionType}
        onSubmit={handleModeration}
        error={moderationModal.error}
      />

      <Modal.Window 
        isOpen={avatarModalState.isOpen} 
        onClose={() => setAvatarModalState(prev => ({ ...prev, isOpen: false }))}
        center={false}
        size="large"
      >
        <Modal.Header>
          <Trans id="modal.avatar.header">Change Avatar</Trans>
        </Modal.Header>
        <Modal.Content>
          <Form error={avatarModalState.error}>
            <Select
              label={i18n._("label.avatar", { message: "Avatar Type" })}
              field="avatarType"
              defaultValue={settingsState.avatarType}
              options={[
                { label: i18n._("label.letter", { message: "Letter" }), value: UserAvatarType.Letter },
                { label: i18n._("label.gravatar", { message: "Gravatar" }), value: UserAvatarType.Gravatar },
                { label: i18n._("label.custom", { message: "Custom" }), value: UserAvatarType.Custom },
              ]}
              onChange={handleAvatarTypeChange}
            >
              {settingsState.avatarType === UserAvatarType.Gravatar && (
                <p className="text-muted mt-1">
                  <Trans id="mysettings.message.avatar.gravatar">
                    A{" "}
                    <a className="text-link" rel="noopener" href="https://en.gravatar.com" target="_blank">
                      Gravatar
                    </a>{" "}
                    will be used based on your email. If you don&apos;t have a Gravatar, a letter avatar based on your initials is generated for you.
                  </Trans>
                </p>
              )}
              {settingsState.avatarType === UserAvatarType.Letter && (
                <p className="text-muted">
                  <Trans id="mysettings.message.avatar.letter">A letter avatar based on your initials is generated for you.</Trans>
                </p>
              )}
              {settingsState.avatarType === UserAvatarType.Custom && (
                <ImageUploader 
                  field="avatar" 
                  onChange={handleAvatarUpload} 
                  bkey={props.user.avatarType === UserAvatarType.Custom ? props.user.avatarURL : undefined}
                >
                  <p className="text-muted">
                    <Trans id="mysettings.message.avatar.custom">
                      We accept JPG and PNG images, smaller than 5MB and with an aspect ratio of 1:1 with minimum dimensions of 50x50 pixels.
                    </Trans>
                  </p>
                </ImageUploader>
              )}
            </Select>
          </Form>
        </Modal.Content>
        <Modal.Footer>
          <Button variant="primary" onClick={handleAvatarChange}>
            <Trans id="action.save">Save</Trans>
          </Button>
          <Button variant="tertiary" onClick={() => setAvatarModalState(prev => ({ ...prev, isOpen: false }))}>
            <Trans id="action.cancel">Cancel</Trans>
          </Button>
        </Modal.Footer>
      </Modal.Window>

      <Modal.Window 
        isOpen={settingsState.showNameEditModal} 
        onClose={() => setSettingsState(prev => ({ ...prev, showNameEditModal: false }))}
        center={false}
        size="large"
      >
        <Modal.Header>
          <Trans id="modal.name.header">Change Name</Trans>
        </Modal.Header>
        <Modal.Content>
          <Form error={settingsState.error}>
            <Input 
              label={i18n._("label.name", { message: "Name" })} 
              field="name" 
              value={settingsState.name} 
              maxLength={100} 
              onChange={setName} 
            />
          </Form>
        </Modal.Content>
        <Modal.Footer>
          <Button variant="primary" onClick={handleNameChange}>
            <Trans id="action.save">Save</Trans>
          </Button>
          <Button variant="tertiary" onClick={() => setSettingsState(prev => ({ ...prev, showNameEditModal: false }))}>
            <Trans id="action.cancel">Cancel</Trans>
          </Button>
        </Modal.Footer>
      </Modal.Window>
    </>
  )
} 