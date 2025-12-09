import "./NotificationIndicator.scss"
import React, { useEffect, useState, useRef, useCallback } from "react"
import IconTrash from "@fider/assets/images/heroicons-trash.svg"
import NoDataIllustration from "@fider/assets/images/undraw-empty.svg"
import IconBell from "@fider/assets/images/heroicons-bell.svg"
import { actions, Fider } from "@fider/services"
import { Avatar, Icon, Markdown, Moment, Button, ButtonClickEvent } from "./common"
import { Tabs } from "./common/Tabs"
import { Dropdown } from "./common/Dropdown"
import { Notification } from "@fider/models"
import { HStack, VStack } from "./layout"
import { useUnreadCounts } from "@fider/contexts/UnreadCountsContext"

import { Trans } from "@lingui/react/macro"

const NotificationSkeleton = () => {
  return (
    <HStack spacing={4} className="px-3 pr-5 py-4">
      <div className="skeleton h-10 w-10 rounded-full"></div>
      <div className="flex-grow">
        <div className="skeleton h-4 w-3/4 mb-2"></div>
        <div className="skeleton h-3 w-1/4"></div>
      </div>
    </HStack>
  )
}

export const NotificationItem = ({ notification }: { notification: Notification }) => {
  const openNotification = () => {
    window.location.href = `/notifications/${notification.id}`
  }

  return (
    <HStack spacing={4} className="px-3 pr-5 clickable hover py-4" onClick={openNotification}>
      <Avatar user={{ name: notification.authorName, avatarURL: notification.avatarURL }} />
      <div>
        <Markdown className="c-notification-indicator-text" text={notification.title} style="full" />
        <span className="text-muted">
          <Moment locale={Fider.currentLocale} date={notification.createdAt} />
        </span>
      </div>
    </HStack>
  )
}

const NotificationIcon = ({ unreadNotifications }: { unreadNotifications: number }) => {
  const isOverMaxCount = unreadNotifications > 99
  const displayCount = isOverMaxCount ? "99+" : unreadNotifications.toString()
  
  return (
    <span className="c-notification-indicator mr-3">
      <Icon sprite={IconBell} className="h-6 text-gray-500" />
      {unreadNotifications > 0 && (
        <div className={`c-notification-indicator-unread-counter ${isOverMaxCount ? "is-max-count" : ""}`}>
          {displayCount}
        </div>
      )}
    </span>
  )
}

export const NotificationIndicator = () => {
  const { counts, setNotificationCount, refreshCounts } = useUnreadCounts()
  const [showingNotifications, setShowingNotifications] = useState(false)
  const [activeTab, setActiveTab] = useState("unread")
  const [loading, setLoading] = useState(false)
  const [unread, setUnread] = useState<Notification[]>([])
  const [read, setRead] = useState<Notification[]>([])
  const [unreadPage, setUnreadPage] = useState(1)
  const [readPage, setReadPage] = useState(1)
  const [unreadTotal, setUnreadTotal] = useState(0)
  const [readTotal, setReadTotal] = useState(0)
  const [purging, setPurging] = useState(false)
  const [initialLoad, setInitialLoad] = useState(true)
  
  const unreadContainerRef = useRef<HTMLDivElement>(null)
  const readContainerRef = useRef<HTMLDivElement>(null)

  const unreadNotifications = counts.notifications

  useEffect(() => {
    if (showingNotifications) {
      setInitialLoad(true)
      loadNotifications("unread", 1, true)
      loadNotifications("read", 1, true)
    }
  }, [showingNotifications])

  const loadNotifications = async (type: string, page: number, reset: boolean = false) => {
    if (loading) return
    
    setLoading(true)
    
    const result = await actions.getNotifications(page, 10, type)
    
    if (result.ok && result.data) {
      const { notifications, total } = result.data

      if (type === "unread") {
        setUnreadTotal(total)
        setUnread(prev => reset ? notifications : [...(prev || []), ...notifications])
        setUnreadPage(page)
        setNotificationCount(total)
      } else {
        setReadTotal(total)
        setRead(prev => reset ? notifications : [...(prev || []), ...notifications])
        setReadPage(page)
      }
    }
    
    setLoading(false)
    setInitialLoad(false)
  }

  const stateRef = useRef({
    unreadPage,
    readPage,
    unread,
    read,
    unreadTotal,
    readTotal,
    loading,
  })
  
  useEffect(() => {
    stateRef.current = { unreadPage, readPage, unread, read, unreadTotal, readTotal, loading }
  }, [unreadPage, readPage, unread, read, unreadTotal, readTotal, loading])

  const handleScroll = useCallback((type: "unread" | "read") => {
    const container = type === "unread" ? unreadContainerRef.current : readContainerRef.current
    if (!container) return
    
    const { scrollTop, scrollHeight, clientHeight } = container
    if (scrollHeight === 0) return
    
    const scrolledToBottom = scrollHeight - scrollTop - clientHeight < 50
    const state = stateRef.current
    
    if (scrolledToBottom && !state.loading) {
      if (type === "unread" && state.unread.length < state.unreadTotal) {
        loadNotifications("unread", state.unreadPage + 1)
      } else if (type === "read" && state.read.length < state.readTotal) {
        loadNotifications("read", state.readPage + 1)
      }
    }
  }, [])

  useEffect(() => {
    const unreadContainer = unreadContainerRef.current
    const readContainer = readContainerRef.current
    
    const handleUnreadScroll = () => handleScroll("unread")
    const handleReadScroll = () => handleScroll("read")
    
    unreadContainer?.addEventListener("scroll", handleUnreadScroll)
    readContainer?.addEventListener("scroll", handleReadScroll)
    
    return () => {
      unreadContainer?.removeEventListener("scroll", handleUnreadScroll)
      readContainer?.removeEventListener("scroll", handleReadScroll)
    }
  }, [handleScroll])

  const markAllAsRead = async (event: ButtonClickEvent) => {
    const response = await actions.markAllAsRead()
    if (response.ok) {
      loadNotifications("unread", 1, true)
      loadNotifications("read", 1, true)
    }
  }

  const purgeReadNotifications = async (event: ButtonClickEvent) => {
    if (purging) return
    
    setPurging(true)
    const response = await actions.purgeReadNotifications()
    
    if (response.ok) {
      setRead([])
      setReadTotal(0)
      refreshCounts()
    }
    
    setPurging(false)
  }

  const handleTabChange = (tab: string) => {
    setActiveTab(tab)
    setTimeout(() => {
      if (tab === "unread" && unreadContainerRef.current) {
        handleScroll("unread")
      } else if (tab === "read" && readContainerRef.current) {
        handleScroll("read")
      }
    }, 100)
  }

  return (
    <Dropdown
      wide={true}
      position="left"
      fullscreenSm={true}
      onToggled={(isOpen: boolean) => setShowingNotifications(isOpen)}
      renderHandle={<NotificationIcon unreadNotifications={unreadNotifications} />}
    >
      <div className="c-notifications-container">
        {showingNotifications && (
          <>
            <Tabs
              tabs={[
                { value: "unread", label: <Trans id="label.unread">Unread</Trans>, counter: unreadTotal },
                { value: "read", label: <Trans id="label.read">Read</Trans>, counter: readTotal },
              ]}
              activeTab={activeTab}
              onChange={handleTabChange}
              className="px-2 pt-2"
            />
            
            {activeTab === "unread" && (
              <div 
                ref={unreadContainerRef} 
                className="c-notifications-scroll-container"
              >
                {(loading && initialLoad) ? (
                  <VStack spacing={0} className="py-2" divide={false}>
                    {Array(5).fill(null).map((_, i) => (
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
                        <Button 
                          size="small" 
                          variant="tertiary" 
                          onClick={markAllAsRead}
                        >
                          <Trans id="action.markallasread">Mark All as Read</Trans>
                        </Button>
                      )}
                    </div>
                    <VStack spacing={0} className="py-2" divide={false}>
                      {unread.map((n) => (
                        <NotificationItem key={n.id} notification={n} />
                      ))}
                    </VStack>
                    {loading && !initialLoad && unread.length < unreadTotal && (
                      <VStack spacing={0} className="py-2" divide={false}>
                        {Array(3).fill(null).map((_, i) => (
                          <NotificationSkeleton key={i} />
                        ))}
                      </VStack>
                    )}
                  </>
                ) : (
                  <div className="text-center py-6">
                    <p className="text-display text-center mt-6 px-4">
                      <Trans id="modal.notifications.nonew">No new notifications</Trans>
                    </p>
                    <Icon sprite={NoDataIllustration} height="120" className="mt-6 mb-2" />
                  </div>
                )}
              </div>
            )}
            
            {activeTab === "read" && (
              <div 
                ref={readContainerRef} 
                className="c-notifications-scroll-container"
              >
                {(loading && initialLoad) ? (
                  <VStack spacing={0} className="py-2" divide={false}>
                    {Array(5).fill(null).map((_, i) => (
                      <NotificationSkeleton key={i} />
                    ))}
                  </VStack>
                ) : readTotal > 0 ? (
                  <>
                    <div className="flex justify-between items-center px-4 mt-4">
                      <p className="text-subtitle mb-0">
                        <Trans id="modal.notifications.read">Read notifications</Trans>
                      </p>
                      <Button
                        size="small"
                        variant="danger"
                        disabled={purging}
                        onClick={purgeReadNotifications}
                      >
                        <Icon sprite={IconTrash} className="h-4 mr-1" />
                        <Trans id="action.purgeread">Purge All</Trans>
                      </Button>
                    </div>
                    <VStack spacing={0} className="py-2" divide={false}>
                      {read.map((n) => (
                        <NotificationItem key={n.id} notification={n} />
                      ))}
                    </VStack>
                    {loading && !initialLoad && read.length < readTotal && (
                      <VStack spacing={0} className="py-2" divide={false}>
                        {Array(3).fill(null).map((_, i) => (
                          <NotificationSkeleton key={i} />
                        ))}
                      </VStack>
                    )}
                  </>
                ) : (
                  <div className="text-center py-6">
                    <p className="text-display text-center mt-6 px-4">
                      <Trans id="modal.notifications.noread">No read notifications</Trans>
                    </p>
                    <Icon sprite={NoDataIllustration} height="120" className="mt-6 mb-2" />
                  </div>
                )}
              </div>
            )}
          </>
        )}
      </div>
    </Dropdown>
  )
}
