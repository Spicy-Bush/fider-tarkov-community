// UserProfileHeader converted to Tailwind

import React, { useState } from "react"
import { Avatar, UserName, Icon, Modal, Form, Input, Button, SelectOption, ImageUploader } from "@fider/components"
import { getInitials, getAvatarColor } from "@fider/components/common/Avatar"
import { useUserProfile } from "./context"
import { Trans } from "@lingui/react/macro"
import { i18n } from "@lingui/core"
import { UserAvatarType, ImageUpload } from "@fider/models"
import { actions, Failure, classSet } from "@fider/services"
import { heroiconsPencilAlt as IconDocument, heroiconsChatAlt2 as IconChat, heroiconsThumbsup as IconThumbsUp, heroiconsPhotograph as IconPhotograph } from "@fider/icons.generated"

interface InitialsAvatarProps {
  name: string
  size: "sm" | "md" | "lg"
  className?: string
}

const InitialsAvatar: React.FC<InitialsAvatarProps> = ({ name, size, className = "" }) => {
  const initials = getInitials(name)
  const color = getAvatarColor(name)
  const sizeClasses = {
    sm: "w-12 h-12",
    md: "w-14 h-14",
    lg: "w-full h-full",
  }
  
  return (
    <div 
      className={`rounded-full flex items-center justify-center shrink-0 overflow-hidden ${sizeClasses[size]} ${className}`}
      style={{ background: color.bg }}
    >
      <svg viewBox="0 0 100 100" className="w-full h-full">
        <text
          x="50"
          y="50"
          dominantBaseline="central"
          textAnchor="middle"
          fill={color.text}
          fontSize="42"
          fontWeight="700"
          fontFamily="inherit"
        >
          {initials}
        </text>
      </svg>
    </div>
  )
}

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

    if (result.ok && result.data) {
      setAvatarModalState(prev => ({ ...prev, isOpen: false }))
      contextUpdateAvatar(result.data.avatarURL)
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
      <div className="col-span-full flex items-start gap-5 p-5 bg-surface-alt rounded-card shadow-sm max-md:flex-col max-md:p-3 max-md:gap-3 max-md:items-center">
        <div 
          className={`relative rounded-full overflow-hidden shrink-0 flex items-center justify-center ${isCompact ? 'w-20 h-20' : 'w-[120px] h-[120px]'} max-md:w-[100px] max-md:h-[100px]`}
          onMouseEnter={() => setAvatarModalState(prev => ({ ...prev, isHoveringAvatar: true }))}
          onMouseLeave={() => setAvatarModalState(prev => ({ ...prev, isHoveringAvatar: false }))}
          onClick={handleAvatarClick}
          style={canEditAvatar ? { cursor: "pointer" } : undefined}
        >
          <Avatar user={{ ...userForComponents, avatarType: user.avatarType as UserAvatarType }} size="fill" imageSize={isCompact ? 80 : 120} clickable={false} />
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
            <div className="mb-6">
              <label className="block text-sm font-medium mb-3">
                <Trans id="label.avatar.type">Avatar Type</Trans>
              </label>
              <div className="grid grid-cols-3 gap-3">
                <button
                  type="button"
                  onClick={() => handleAvatarTypeChange({ value: UserAvatarType.Letter, label: i18n._("label.letter", { message: "Letter" }) })}
                  className={classSet({
                    "p-4 rounded-card border-2 transition-all duration-150 text-left": true,
                    "border-primary bg-accent-light shadow-sm": avatarModalState.avatarType === UserAvatarType.Letter,
                    "border-border bg-transparent hover:border-border-strong hover:bg-surface-alt hover:shadow-sm": avatarModalState.avatarType !== UserAvatarType.Letter,
                  })}
                >
                  <div className="flex flex-col items-center gap-2">
                    <InitialsAvatar name={user.name} size="sm" />
                    <div className="text-sm font-medium">
                      <Trans id="label.letter">Letter</Trans>
                    </div>
                  </div>
                </button>
                <button
                  type="button"
                  onClick={() => handleAvatarTypeChange({ value: UserAvatarType.Gravatar, label: i18n._("label.gravatar", { message: "Gravatar" }) })}
                  className={classSet({
                    "p-4 rounded-card border-2 transition-all duration-150 text-left": true,
                    "border-primary bg-accent-light shadow-sm": avatarModalState.avatarType === UserAvatarType.Gravatar,
                    "border-border bg-transparent hover:border-border-strong hover:bg-surface-alt hover:shadow-sm": avatarModalState.avatarType !== UserAvatarType.Gravatar,
                  })}
                >
                  <div className="flex flex-col items-center gap-2">
                    <InitialsAvatar name={user.name} size="sm" />
                    <div className="text-sm font-medium">
                      <Trans id="label.gravatar">Gravatar</Trans>
                    </div>
                  </div>
                </button>
                <button
                  type="button"
                  onClick={() => handleAvatarTypeChange({ value: UserAvatarType.Custom, label: i18n._("label.custom", { message: "Custom" }) })}
                  className={classSet({
                    "p-4 rounded-card border-2 transition-all duration-150 text-left": true,
                    "border-primary bg-accent-light shadow-sm": avatarModalState.avatarType === UserAvatarType.Custom,
                    "border-border bg-transparent hover:border-border-strong hover:bg-surface-alt hover:shadow-sm": avatarModalState.avatarType !== UserAvatarType.Custom,
                  })}
                >
                  <div className="flex flex-col items-center gap-2">
                    <div className="w-12 h-12 rounded-full bg-surface-alt flex items-center justify-center shrink-0">
                      <Icon sprite={IconPhotograph} className="h-6 w-6 text-muted" />
                    </div>
                    <div className="text-sm font-medium">
                      <Trans id="label.custom">Custom</Trans>
                    </div>
                  </div>
                </button>
              </div>
            </div>

            <div className="h-[120px] mb-6 flex items-start">
              {avatarModalState.avatarType === UserAvatarType.Gravatar && (
                <div className="p-4 bg-surface-alt rounded-card w-full">
                  <p className="text-sm text-muted">
                    <Trans id="mysettings.message.avatar.gravatar">
                      A{" "}
                      <a className="text-link" rel="noopener" href="https://en.gravatar.com" target="_blank">
                        Gravatar
                      </a>{" "}
                      will be used based on your email. If you don&apos;t have a Gravatar, a letter avatar based on your initials is generated for you.
                    </Trans>
                  </p>
                </div>
              )}

              {avatarModalState.avatarType === UserAvatarType.Letter && (
                <div className="p-4 bg-surface-alt rounded-card w-full">
                  <p className="text-sm text-muted">
                    <Trans id="mysettings.message.avatar.letter">A letter avatar based on your initials is generated for you.</Trans>
                  </p>
                </div>
              )}

              {avatarModalState.avatarType === UserAvatarType.Custom && (
                <div className="w-full">
                  <ImageUploader 
                    field="avatar" 
                    onChange={handleAvatarUpload} 
                    bkey={user.avatarType === UserAvatarType.Custom ? user.avatarURL : undefined}
                  >
                    <p className="text-sm text-muted mt-2">
                      <Trans id="mysettings.message.avatar.custom">
                        We accept JPG and PNG images, smaller than 5MB and with an aspect ratio of 1:1 with minimum dimensions of 50x50 pixels.
                      </Trans>
                    </p>
                  </ImageUploader>
                </div>
              )}
            </div>

            {(avatarModalState.avatar?.upload || avatarModalState.avatarType !== user?.avatarType) && (
              <div className="pt-4 border-t border-border">
                <div className="flex items-center justify-center gap-4">
                  <div className="flex flex-col items-center">
                    <div className="text-xs font-medium mb-1.5 text-muted">
                      <Trans id="modal.avatar.preview.current">Current</Trans>
                    </div>
                    <div className="w-14 h-14 rounded-full overflow-hidden border-2 border-border shrink-0">
                      <Avatar user={{ ...userForComponents, avatarType: user.avatarType as UserAvatarType }} size="fill" imageSize={56} clickable={false} />
                    </div>
                  </div>
                  <div className="text-muted text-xl">→</div>
                  <div className="flex flex-col items-center">
                    <div className="text-xs font-medium mb-1.5 text-muted">
                      <Trans id="modal.avatar.preview.new">Preview</Trans>
                    </div>
                    <div className="w-14 h-14 rounded-full overflow-hidden border-2 border-primary shrink-0">
                      {avatarModalState.avatarType === UserAvatarType.Custom && avatarModalState.avatar?.upload ? (
                        <img 
                          src={`data:${avatarModalState.avatar.upload.contentType};base64,${avatarModalState.avatar.upload.content}`}
                          alt="Preview"
                          className="w-full h-full object-cover"
                        />
                      ) : (
                        <InitialsAvatar name={user.name} size="lg" />
                      )}
                    </div>
                  </div>
                </div>
              </div>
            )}
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
