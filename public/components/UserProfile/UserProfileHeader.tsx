// UserProfileHeader converted to Tailwind

import React, { useState } from "react"
import { Avatar, UserName, Icon, Modal, Form, Input, Button, Select, SelectOption, ImageUploader } from "@fider/components"
import { useUserProfile } from "./context"
import { Trans } from "@lingui/react/macro"
import { i18n } from "@lingui/core"
import { UserAvatarType, ImageUpload } from "@fider/models"
import { actions, Failure } from "@fider/services"
import { heroiconsPencilAlt as IconDocument, heroiconsChatAlt2 as IconChat, heroiconsThumbsup as IconThumbsUp } from "@fider/icons.generated"

interface UserProfileHeaderProps {
  compact?: boolean
}

export const UserProfileHeader: React.FC<UserProfileHeaderProps> = ({ compact: compactProp }) => {
  const { user, stats, compact: contextCompact, canEditName, canEditAvatar, updateUserName: contextUpdateName, updateUserAvatar: contextUpdateAvatar } = useUserProfile()
  const isCompact = compactProp ?? contextCompact

  const [avatarModalState, setAvatarModalState] = useState({
    isOpen: false,
    error: undefined as Failure | undefined,
    isHoveringAvatar: false,
    avatarType: (user?.avatarType as UserAvatarType) || UserAvatarType.Letter,
    avatar: undefined as ImageUpload | undefined,
  })

  const [nameModalState, setNameModalState] = useState({
    isOpen: false,
    error: undefined as Failure | undefined,
    name: user?.name || "",
  })

  if (!user) return null

  const userForComponents = {
    ...user,
    role: user.role as any,
    status: user.status as any,
  }

  const handleAvatarClick = () => {
    if (canEditAvatar) {
      setAvatarModalState(prev => ({ ...prev, isOpen: true }))
    }
  }

  const handleAvatarChange = async () => {
    const result = await actions.updateUserAvatar({
      avatarType: avatarModalState.avatarType,
      avatar: avatarModalState.avatar,
    }, user.id)

    if (result.ok) {
      setAvatarModalState(prev => ({ ...prev, isOpen: false }))
      if (avatarModalState.avatarType === UserAvatarType.Custom && avatarModalState.avatar?.bkey) {
        contextUpdateAvatar(`/avatars/custom/${user.id}/${avatarModalState.avatar.bkey}?t=${Date.now()}`)
      } else if (avatarModalState.avatarType === UserAvatarType.Gravatar) {
        contextUpdateAvatar(`/avatars/gravatar/${user.id}?t=${Date.now()}`)
      } else if (avatarModalState.avatarType === UserAvatarType.Letter) {
        contextUpdateAvatar(`/avatars/letter/${user.id}?t=${Date.now()}`)
      }
    } else if (result.error) {
      setAvatarModalState(prev => ({ ...prev, error: result.error }))
    }
  }

  const handleAvatarTypeChange = (opt?: SelectOption) => {
    if (opt) {
      setAvatarModalState(prev => ({ ...prev, avatarType: opt.value as UserAvatarType }))
    }
  }

  const handleAvatarUpload = (avatar: ImageUpload): void => {
    setAvatarModalState(prev => ({ ...prev, avatar }))
  }

  const handleNameChange = async () => {
    const result = await actions.updateUserName({
      name: nameModalState.name,
    }, user.id)

    if (result.ok) {
      setNameModalState(prev => ({ ...prev, isOpen: false }))
      contextUpdateName(nameModalState.name)
    } else if (result.error) {
      setNameModalState(prev => ({ ...prev, error: result.error }))
    }
  }

  return (
    <>
      <div className="col-span-full flex items-start gap-5 p-5 bg-surface-raised rounded-card shadow-sm max-md:flex-col max-md:p-3 max-md:gap-3 max-md:items-center">
        <div 
          className={`relative rounded-full overflow-hidden shrink-0 flex items-center justify-center ${isCompact ? 'w-20 h-20' : 'w-[120px] h-[120px]'} max-md:w-[100px] max-md:h-[100px]`}
          onMouseEnter={() => setAvatarModalState(prev => ({ ...prev, isHoveringAvatar: true }))}
          onMouseLeave={() => setAvatarModalState(prev => ({ ...prev, isHoveringAvatar: false }))}
          onClick={handleAvatarClick}
          style={canEditAvatar ? { cursor: "pointer" } : undefined}
        >
          <Avatar user={userForComponents} size="fill" imageSize={isCompact ? 80 : 120} clickable={false} />
          {canEditAvatar && avatarModalState.isHoveringAvatar && (
            <div className="absolute inset-0 bg-black/60 flex flex-col items-center justify-center text-white opacity-0 hover:opacity-100 transition-opacity text-sm">
              <Trans id="profile.avatar.change">Change Avatar</Trans>
            </div>
          )}
        </div>
        <div className="flex-grow flex flex-col gap-2 max-md:w-full max-md:items-center max-md:text-center">
          <div className="flex items-center gap-2">
            <UserName user={userForComponents} />
            {canEditName && (
              <button 
                className="bg-transparent border-none p-1 cursor-pointer text-muted rounded hover:text-foreground hover:bg-surface-alt transition-all"
                onClick={() => setNameModalState(prev => ({ ...prev, isOpen: true, name: user.name }))}
              >
                <Icon sprite={IconDocument} className="h-4" />
              </button>
            )}
          </div>
        </div>
        <div className="shrink-0 w-auto py-2 max-md:w-full max-md:mt-2">
          <div className="flex flex-col gap-2 max-md:flex-row max-md:justify-around">
            <div className="flex items-center gap-1 text-muted text-sm whitespace-nowrap">
              <Icon sprite={IconDocument} className="h-4 text-subtle" />
              <span className="font-medium">{stats.posts}</span>
            </div>
            <div className="flex items-center gap-1 text-muted text-sm whitespace-nowrap">
              <Icon sprite={IconChat} className="h-4 text-subtle" />
              <span className="font-medium">{stats.comments}</span>
            </div>
            <div className="flex items-center gap-1 text-muted text-sm whitespace-nowrap">
              <Icon sprite={IconThumbsUp} className="h-4 text-subtle" />
              <span className="font-medium">{stats.votes}</span>
            </div>
          </div>
        </div>
      </div>

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
              defaultValue={avatarModalState.avatarType}
              options={[
                { label: i18n._("label.letter", { message: "Letter" }), value: UserAvatarType.Letter },
                { label: i18n._("label.gravatar", { message: "Gravatar" }), value: UserAvatarType.Gravatar },
                { label: i18n._("label.custom", { message: "Custom" }), value: UserAvatarType.Custom },
              ]}
              onChange={handleAvatarTypeChange}
            >
              {avatarModalState.avatarType === UserAvatarType.Gravatar && (
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
              {avatarModalState.avatarType === UserAvatarType.Letter && (
                <p className="text-muted">
                  <Trans id="mysettings.message.avatar.letter">A letter avatar based on your initials is generated for you.</Trans>
                </p>
              )}
              {avatarModalState.avatarType === UserAvatarType.Custom && (
                <ImageUploader 
                  field="avatar" 
                  onChange={handleAvatarUpload} 
                  bkey={user.avatarType === UserAvatarType.Custom ? user.avatarURL : undefined}
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
        isOpen={nameModalState.isOpen} 
        onClose={() => setNameModalState(prev => ({ ...prev, isOpen: false }))}
        center={false}
        size="large"
      >
        <Modal.Header>
          <Trans id="modal.name.header">Change Name</Trans>
        </Modal.Header>
        <Modal.Content>
          <Form error={nameModalState.error}>
            <Input 
              label={i18n._("label.name", { message: "Name" })} 
              field="name" 
              value={nameModalState.name} 
              maxLength={100} 
              onChange={(name) => setNameModalState(prev => ({ ...prev, name }))} 
            />
          </Form>
        </Modal.Content>
        <Modal.Footer>
          <Button variant="primary" onClick={handleNameChange}>
            <Trans id="action.save">Save</Trans>
          </Button>
          <Button variant="tertiary" onClick={() => setNameModalState(prev => ({ ...prev, isOpen: false }))}>
            <Trans id="action.cancel">Cancel</Trans>
          </Button>
        </Modal.Footer>
      </Modal.Window>
    </>
  )
}
