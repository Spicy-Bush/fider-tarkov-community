import React from "react"
import { Button, Icon } from "@fider/components"
import { classSet, Fider } from "@fider/services"
import {
  heroiconsChevronUp as IconChevronUp,
  heroiconsArrowLeft as IconArrowLeft,
} from "@fider/icons.generated"
import { Report, Post, Comment, User } from "@fider/models"
import { UserProfile } from "@fider/components/UserProfile"
import { ContentPreview } from "./ContentPreview"
import { ViewingUserType } from "../hooks/useReportsState"

interface ReportsPreviewProps {
  selectedReport: Report | null
  viewingUser: ViewingUserType | null
  profileKey: number
  previewPost: Post | null
  previewComment: Comment | null
  isLoadingPreview: boolean
  onDeselectReport: () => void
  onCloseUserProfile: () => void
  onAssign: () => Promise<void>
  onUnassign: () => Promise<void>
  onResolveClick: (status: "resolved" | "dismissed", shiftKey: boolean) => Promise<void>
  onViewUser: (user: ViewingUserType) => void
  onUserUpdate: (updates: Partial<ViewingUserType>) => void
}

export const ReportsPreview: React.FC<ReportsPreviewProps> = ({
  selectedReport,
  viewingUser,
  profileKey,
  previewPost,
  previewComment,
  isLoadingPreview,
  onDeselectReport,
  onCloseUserProfile,
  onAssign,
  onUnassign,
  onResolveClick,
  onViewUser,
  onUserUpdate,
}) => {
  return (
    <div
      className={classSet({
        "flex-1 min-w-0 h-full min-h-full overflow-y-auto bg-tertiary rounded-panel border border-surface-alt relative lg:max-h-[90vh]": true,
        "max-lg:hidden": selectedReport === null && viewingUser === null,
        "max-lg:fixed max-lg:inset-0 max-lg:z-modal max-lg:overflow-y-auto max-lg:p-4": selectedReport !== null || viewingUser !== null,
      })}
    >
      {viewingUser ? (
        <>
          <Button
            variant="tertiary"
            size="small"
            className="flex mb-3"
            onClick={onCloseUserProfile}
          >
            <Icon sprite={IconArrowLeft} className="h-4" />
            <span>Back to report</span>
          </Button>
          <UserProfile
            key={profileKey}
            userId={viewingUser.id}
            user={viewingUser as User}
            embedded
            compact
            onUserUpdate={onUserUpdate}
          >
            <UserProfile.Header compact />
            <UserProfile.Actions />
            <UserProfile.Status />
            <UserProfile.Tabs>
              <UserProfile.Search />
              <UserProfile.Standing />
            </UserProfile.Tabs>
          </UserProfile>
        </>
      ) : (
        <>
          {selectedReport && (
            <Button
              variant="tertiary"
              size="small"
              className="hidden max-lg:flex mb-3"
              onClick={onDeselectReport}
            >
              <Icon sprite={IconChevronUp} className="-rotate-90 w-4 h-4" />
              <span>Back to list</span>
            </Button>
          )}
          <ContentPreview
            report={selectedReport}
            post={previewPost}
            comment={previewComment}
            isLoading={isLoadingPreview}
            onAssign={onAssign}
            onUnassign={onUnassign}
            onResolve={onResolveClick}
            currentUserId={Fider.session.user.id}
            onUserClick={onViewUser}
          />
        </>
      )}
    </div>
  )
}
