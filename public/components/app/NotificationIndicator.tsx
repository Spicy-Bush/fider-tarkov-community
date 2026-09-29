import React, { useEffect, useState, useRef } from "react"
import { heroiconsTrash as IconTrash, undrawEmpty as NoDataIllustration, heroiconsBell as IconBell } from "@fider/icons.generated"
import { Fider } from "@fider/services/fider"
import { classSet } from "@fider/services/utils"
import { Avatar } from "@fider/components/common/Avatar"
import { Icon } from "@fider/components/common/Icon"
import { Markdown } from "@fider/components/common/Markdown"
import { Moment } from "@fider/components/common/Moment"
import { Button } from "@fider/components/common/Button"
import { DisplayError } from "@fider/components/common/form/DisplayError"
import { Tabs, TabPanels } from "../common/Tabs"
import { Dropdown, useDropdown } from "../common/Dropdown"
import { Notification } from "@fider/models"
import { HStack, VStack } from "@fider/components/layout/Stack"
import { useUnreadCounts } from "@fider/contexts/UnreadCountsContext"

import { Trans } from "@lingui/react/macro"

const NotificationSkeleton = () => {
  return (
    <HStack spacing={4} className="px-3 pr-5 py-4">
      <div className="skeleton h-10 w-10 rounded-full"></div>
      <div className="grow">
        <div className="skeleton h-4 w-3/4 mb-2"></div>
        <div className="skeleton h-3 w-1/4"></div>
      </div>
    </HStack>
  )
}

export const NotificationItem = ({ notification }: { notification: Notification }) => {
  const dropdown = useDropdown()
  const { inbox } = useUnreadCounts()

  const markRead = () => {
    if (!notification.read) {
      void inbox.markRead(notification.id)
    }
  }

  return (
    <a
      href={notification.link}
      className="block px-3 pr-5 py-4 text-foreground no-underline transition-colors hover:bg-surface-alt"
      onClick={() => {
        markRead()
        dropdown?.close()
      }}
      onAuxClick={(event) => event.button === 1 && markRead()}
    >
      <HStack spacing={4}>
        <Avatar user={{ name: notification.authorName, avatarURL: notification.avatarURL }} />
        <div className="flex-1 min-w-0 max-w-full">
          <Markdown
            className="wrap-break-word [word-break:break-word] [&_p]:wrap-break-word [&_p]:[word-break:break-word]"
            text={notification.title}
            style="full"
          />
          <span className="text-muted">
            <Moment locale={Fider.currentLocale} date={notification.createdAt} />
          </span>
        </div>
      </HStack>
    </a>
  )
}

const NotificationIcon = ({ unreadNotifications }: { unreadNotifications: number }) => {
  const isOverMaxCount = unreadNotifications > 99
  const displayCount = isOverMaxCount ? "99+" : unreadNotifications.toString()

  return (
    <span className="relative inline-flex items-center cursor-pointer group">
      <Icon sprite={IconBell} className="h-6 text-muted group-hover:text-foreground" />
      {unreadNotifications > 0 && (
        <div
          className={classSet({
            "absolute -top-1.5 flex justify-center items-center bg-danger text-white text-[10px] font-bold min-w-4 h-4 px-1 rounded-badge": true,
            "-right-2": !isOverMaxCount,
            "-right-4": isOverMaxCount,
          })}
        >
          {displayCount}
        </div>
      )}
    </span>
  )
}

const NOTIFICATION_TABS = ["unread", "read"] as const
type NotificationTab = (typeof NOTIFICATION_TABS)[number]

export const NotificationIndicator = () => {
  const { counts, inbox, snapshot } = useUnreadCounts()
  const [showingNotifications, setShowingNotifications] = useState(false)
  const [activeTab, setActiveTab] = useState<NotificationTab>("unread")
  const unreadContainerRef = useRef<HTMLDivElement>(null)
  const readContainerRef = useRef<HTMLDivElement>(null)
  const unread = snapshot.pages.unread
  const read = snapshot.pages.read
  const unreadTotal = snapshot.totals.unread
  const readTotal = snapshot.totals.read

  useEffect(() => {
    if (showingNotifications) {
      void inbox.refresh()
    }
  }, [showingNotifications, inbox])

  const handleScroll = (type: NotificationTab) => {
    const container = type === "unread" ? unreadContainerRef.current : readContainerRef.current
    if (container && container.scrollHeight > 0 && container.scrollHeight - container.scrollTop - container.clientHeight < 50) {
      void inbox.load(type)
    }
  }

  useEffect(() => {
    if (!showingNotifications) {
      return
    }
    const timer = setTimeout(() => handleScroll(activeTab), 100)
    return () => clearTimeout(timer)
  }, [showingNotifications, activeTab, unread.page, read.page])

  return (
    <Dropdown
      label="Notifications"
      wide={true}
      position="left"
      fullscreenSm={true}
      onToggled={(isOpen: boolean) => setShowingNotifications(isOpen)}
      renderHandle={<NotificationIcon unreadNotifications={counts.notifications} />}
    >
      <div className="max-h-[80vh] flex flex-col lg:min-w-[400px] lg:max-w-[500px]">
        {showingNotifications && (
          <>
            {snapshot.error && (
              <div className="px-4 pt-3">
                <DisplayError error={snapshot.error} />
                <Button size="small" variant="tertiary" onClick={() => inbox.refresh()}>Retry</Button>
              </div>
            )}
            <Tabs
              tabs={[
                { value: "unread", label: <Trans id="label.unread">Unread</Trans>, counter: unreadTotal },
                { value: "read", label: <Trans id="label.read">Read</Trans>, counter: readTotal },
              ]}
              activeTab={activeTab}
              onChange={setActiveTab}
              className="px-2 pt-2"
            />

            <TabPanels keys={NOTIFICATION_TABS} activeKey={activeTab}>
              {(tab) =>
                tab === "unread" ? (
                  <div ref={unreadContainerRef} onScroll={() => handleScroll("unread")} className="overflow-y-auto max-h-[400px] pb-2 scroll-smooth overscroll-contain">
                    {unread.loading && unread.page === 0 ? (
                      <VStack spacing={0} className="py-2" divide={false}>
                        {Array(5)
                          .fill(null)
                          .map((_, i) => (
                            <NotificationSkeleton key={i} />
                          ))}
                      </VStack>
                    ) : unreadTotal > 0 ? (
                      <>
                        <div className="flex justify-between items-center px-4 mt-4">
                          <p className="text-subtitle mb-0">
                            <Trans id="modal.notifications.unread">Unread notifications</Trans>
                          </p>
                          {unreadTotal > 1 && (
                            <Button size="small" variant="tertiary" disabled={snapshot.changing} onClick={() => inbox.markAllRead()}>
                              <Trans id="action.markallasread">Mark All as Read</Trans>
                            </Button>
                          )}
                        </div>
                        <VStack spacing={0} className="py-2" divide={false}>
                          {unread.items.map((n) => (
                            <NotificationItem key={n.id} notification={n} />
                          ))}
                        </VStack>
                        {unread.loading && unread.page > 0 && unread.items.length < unreadTotal && (
                          <VStack spacing={0} className="py-2" divide={false}>
                            {Array(3)
                              .fill(null)
                              .map((_, i) => (
                                <NotificationSkeleton key={i} />
                              ))}
                          </VStack>
                        )}
                      </>
                    ) : (
                      <div className="flex flex-col items-center py-6">
                        <p className="text-display text-center mt-6 px-4">
                          <Trans id="modal.notifications.nonew">No new notifications</Trans>
                        </p>
                        <Icon sprite={NoDataIllustration} height="120" className="mt-6 mb-2" />
                      </div>
                    )}
                  </div>
                ) : (
                  <div ref={readContainerRef} onScroll={() => handleScroll("read")} className="overflow-y-auto max-h-[400px] pb-2 scroll-smooth overscroll-contain">
                    {read.loading && read.page === 0 ? (
                      <VStack spacing={0} className="py-2" divide={false}>
                        {Array(5)
                          .fill(null)
                          .map((_, i) => (
                            <NotificationSkeleton key={i} />
                          ))}
                      </VStack>
                    ) : readTotal > 0 ? (
                      <>
                        <div className="flex justify-between items-center px-4 mt-4">
                          <p className="text-subtitle mb-0">
                            <Trans id="modal.notifications.read">Read notifications</Trans>
                          </p>
                          <Button size="small" variant="danger" disabled={snapshot.changing} onClick={() => inbox.purgeRead()}>
                            <Icon sprite={IconTrash} className="h-4 mr-1" />
                            <Trans id="action.purgeread">Purge All</Trans>
                          </Button>
                        </div>
                        <VStack spacing={0} className="py-2" divide={false}>
                          {read.items.map((n) => (
                            <NotificationItem key={n.id} notification={n} />
                          ))}
                        </VStack>
                        {read.loading && read.page > 0 && read.items.length < readTotal && (
                          <VStack spacing={0} className="py-2" divide={false}>
                            {Array(3)
                              .fill(null)
                              .map((_, i) => (
                                <NotificationSkeleton key={i} />
                              ))}
                          </VStack>
                        )}
                      </>
                    ) : (
                      <div className="flex flex-col items-center py-6">
                        <p className="text-display text-center mt-6 px-4">
                          <Trans id="modal.notifications.noread">No read notifications</Trans>
                        </p>
                        <Icon sprite={NoDataIllustration} height="120" className="mt-6 mb-2" />
                      </div>
                    )}
                  </div>
                )
              }
            </TabPanels>
          </>
        )}
      </div>
    </Dropdown>
  )
}
