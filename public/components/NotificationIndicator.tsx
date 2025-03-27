import "./NotificationIndicator.scss"
import React, { useEffect, useState, useRef } from "react"
import IconTrash from "@fider/assets/images/heroicons-trash.svg"
import NoDataIllustration from "@fider/assets/images/undraw-empty.svg"
import IconBell from "@fider/assets/images/heroicons-bell.svg"
import { useFider } from "@fider/hooks"
import { actions, Fider } from "@fider/services"
import { Avatar, Icon, Markdown, Moment, Button, ButtonClickEvent } from "./common"
import { Tabs } from "./common/Tabs"
import { Dropdown } from "./common/Dropdown"
import { Notification } from "@fider/models"
import { HStack, VStack } from "./layout"

import { Trans } from "@lingui/react/macro"

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
  const isOverMaxCount = unreadNotifications > 99;
  const displayCount = isOverMaxCount ? "99+" : unreadNotifications.toString();
  
  return (
    <>
      <span className="c-notification-indicator mr-3">
        <Icon sprite={IconBell} className="h-6 text-gray-500" />
        {unreadNotifications > 0 && (
          <div className={`c-notification-indicator-unread-counter ${isOverMaxCount ? 'is-max-count' : ''}`}>
            {displayCount}
          </div>
        )}
      </span>
    </>
  )
}

export const NotificationIndicator = () => {
  const fider = useFider()
  const [unreadNotifications, setUnreadNotifications] = useState(0)
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
  
  const unreadContainerRef = useRef<HTMLDivElement>(null)
  const readContainerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (fider.session.isAuthenticated) {
      actions.getTotalUnreadNotifications().then((result) => {
        if (result.ok && result.data > 0) {
          setUnreadNotifications(result.data)
        }
      })
    }
  }, [fider.session.isAuthenticated])

  useEffect(() => {
    if (showingNotifications) {
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
        setUnreadNotifications(total)
      } else {
        setReadTotal(total)
        setRead(prev => reset ? notifications : [...(prev || []), ...notifications])
        setReadPage(page)
      }
    }
    
    setLoading(false)
  }

  const handleScroll = (type: string) => {
    const container = type === "unread" ? unreadContainerRef.current : readContainerRef.current
    if (!container) return
    
    const { scrollTop, scrollHeight, clientHeight } = container
    const scrolledToBottom = scrollHeight - scrollTop - clientHeight < 50
    
    if (scrolledToBottom) {
      if (type === "unread" && (unread || []).length < unreadTotal && !loading) {
        loadNotifications("unread", unreadPage + 1)
      } else if (type === "read" && (read || []).length < readTotal && !loading) {
        loadNotifications("read", readPage + 1)
      }
    }
  }

  const handleUnreadScroll = () => handleScroll("unread")
  const handleReadScroll = () => handleScroll("read")

  useEffect(() => {
    const unreadContainer = unreadContainerRef.current
    if (unreadContainer && activeTab === "unread") {
      unreadContainer.addEventListener("scroll", handleUnreadScroll)
      return () => {
        unreadContainer.removeEventListener("scroll", handleUnreadScroll)
      }
    }
  }, [activeTab, unreadPage, unread, unreadTotal, loading])

  useEffect(() => {
    const readContainer = readContainerRef.current
    if (readContainer && activeTab === "read") {
      readContainer.addEventListener("scroll", handleReadScroll)
      return () => {
        readContainer.removeEventListener("scroll", handleReadScroll)
      }
    }
  }, [activeTab, readPage, read, readTotal, loading])

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
      
      actions.getTotalUnreadNotifications().then((result) => {
        if (result.ok) {
          setUnreadNotifications(result.data || 0)
        }
      })
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
      fullsceenSm={true}
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
                className="overflow-y-auto"
                style={{ maxHeight: "400px" }}
              >
                {unreadTotal > 0 ? (
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
                    {loading && unread.length < unreadTotal && (
                      <div className="text-center py-2">
                        <span className="text-muted">Loading more...</span>
                      </div>
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
                className="overflow-y-auto"
                style={{ maxHeight: "400px" }}
              >
                {readTotal > 0 ? (
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
                    {loading && read.length < readTotal && (
                      <div className="text-center py-2">
                        <span className="text-muted">Loading more...</span>
                      </div>
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
