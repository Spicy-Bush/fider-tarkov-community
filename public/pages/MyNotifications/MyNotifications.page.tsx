import React from "react"

import { Notification } from "@fider/models"
import { Header, Markdown, Moment, PageTitle, Button, ButtonClickEvent, Icon } from "@fider/components"
import { actions, Fider } from "@fider/services"
import { HStack, VStack } from "@fider/components/layout"
import { i18n } from "@lingui/core"
import { Trans } from "@lingui/react/macro"
import IconTrash from "@fider/assets/images/heroicons-trash.svg"
import { Tabs } from "@fider/components/common/Tabs"

interface MyNotificationsPageProps {
  notifications: Notification[]
  total: number
  page: number
  perPage: number
}

interface MyNotificationsPageState {
  activeTab: string
  unread: Notification[]
  read: Notification[]
  unreadPage: number
  readPage: number
  unreadTotal: number
  readTotal: number
  loading: boolean
  purging: boolean
}

export default class MyNotificationsPage extends React.Component<MyNotificationsPageProps, MyNotificationsPageState> {
  private unreadContainerRef: React.RefObject<HTMLDivElement>
  private readContainerRef: React.RefObject<HTMLDivElement>

  constructor(props: MyNotificationsPageProps) {
    super(props)

    const [unread, read] = (this.props.notifications || []).reduce(
      (result, item) => {
        result[item.read ? 1 : 0].push(item)
        return result
      },
      [[] as Notification[], [] as Notification[]]
    )

    this.state = {
      activeTab: "unread",
      unread,
      read,
      unreadPage: 1,
      readPage: 1,
      unreadTotal: 0,
      readTotal: 0,
      loading: false,
      purging: false
    }

    this.unreadContainerRef = React.createRef()
    this.readContainerRef = React.createRef()
  }

  componentDidMount() {
    this.loadNotifications("unread", 1, true)
    this.loadNotifications("read", 1, true)
    this.setupScrollListeners()
  }

  componentDidUpdate(prevProps: MyNotificationsPageProps, prevState: MyNotificationsPageState) {
    if (prevState.activeTab !== this.state.activeTab) {
      this.removeScrollListeners()
      this.setupScrollListeners()
      
      setTimeout(() => {
        if (this.state.activeTab === "unread" && this.unreadContainerRef.current) {
          this.handleScroll("unread")
        } else if (this.state.activeTab === "read" && this.readContainerRef.current) {
          this.handleScroll("read")
        }
      }, 100)
    }
  }

  componentWillUnmount() {
    this.removeScrollListeners()
  }

  setupScrollListeners = () => {
    if (this.state.activeTab === "unread" && this.unreadContainerRef.current) {
      this.unreadContainerRef.current.addEventListener("scroll", this.handleUnreadScroll)
    }

    if (this.state.activeTab === "read" && this.readContainerRef.current) {
      this.readContainerRef.current.addEventListener("scroll", this.handleReadScroll)
    }
  }

  removeScrollListeners = () => {
    if (this.unreadContainerRef.current) {
      this.unreadContainerRef.current.removeEventListener("scroll", this.handleUnreadScroll)
    }

    if (this.readContainerRef.current) {
      this.readContainerRef.current.removeEventListener("scroll", this.handleReadScroll)
    }
  }

  handleUnreadScroll = () => {
    this.handleScroll("unread")
  }

  handleReadScroll = () => {
    this.handleScroll("read")
  }

  handleScroll = (type: string) => {
    const container = type === "unread" ? this.unreadContainerRef.current : this.readContainerRef.current
    if (!container) return
    
    const { scrollTop, scrollHeight, clientHeight } = container
    const scrolledToBottom = scrollHeight - scrollTop - clientHeight < 50
    
    if (scrolledToBottom) {
      if (type === "unread" && (this.state.unread || []).length < this.state.unreadTotal && !this.state.loading) {
        this.loadNotifications("unread", this.state.unreadPage + 1)
      } else if (type === "read" && (this.state.read || []).length < this.state.readTotal && !this.state.loading) {
        this.loadNotifications("read", this.state.readPage + 1)
      }
    }
  }

  loadNotifications = async (type: string, page: number, reset: boolean = false) => {
    if (this.state.loading) return
    
    this.setState({ loading: true })
    
    const result = await actions.getNotifications(page, 10, type)
    
    if (result.ok && result.data) {
      const { notifications, total } = result.data

      if (type === "unread") {
        this.setState(prev => {
          const currentList = reset ? [] : [...(prev.unread || [])]
          return {
            unreadTotal: total,
            unread: [...currentList, ...notifications],
            unreadPage: page,
            loading: false
          }
        })
      } else {
        this.setState(prev => {
          const currentList = reset ? [] : [...(prev.read || [])]
          return {
            readTotal: total,
            read: [...currentList, ...notifications],
            readPage: page,
            loading: false
          }
        })
      }
    } else {
      this.setState({ loading: false })
    }
  }

  private renderNotificationItems(notifications: Notification[]): JSX.Element[] {
    return notifications.map((n) => {
      return (
        <div key={n.id}>
          <a className="text-link block" href={`/notifications/${n.id}`}>
            <Markdown text={n.title} style="full" />
          </a>
          <span className="text-muted">
            <Moment locale={Fider.currentLocale} date={n.createdAt} />
          </span>
        </div>
      )
    })
  }

  private markAllAsRead = async (event: ButtonClickEvent) => {
    const response = await actions.markAllAsRead()
    if (response.ok) {
      this.loadNotifications("unread", 1, true)
      this.loadNotifications("read", 1, true)
    }
  }

  private purgeReadNotifications = async (event: ButtonClickEvent) => {
    if (this.state.purging) return
    
    this.setState({ purging: true })
    
    const response = await actions.purgeReadNotifications()
    
    if (response.ok) {
      this.setState({
        read: [],
        readTotal: 0,
        purging: false
      })
    } else {
      this.setState({ purging: false })
    }
  }

  private handleTabChange = (tab: string) => {
    this.setState({ activeTab: tab })
  }

  public render() {
    const { activeTab, unread, read, unreadTotal, readTotal, loading, purging } = this.state
    
    return (
      <>
        <Header />
        <div id="p-my-notifications" className="page container">
          <PageTitle
            title={i18n._("mynotifications.page.title", { message: "Notifications" })}
            subtitle={i18n._("mynotifications.page.subtitle", { message: "Stay up to date with what's happening" })}
          />

          <Tabs
            tabs={[
              { value: "unread", label: <Trans id="label.unread">Unread</Trans>, counter: unreadTotal },
              { value: "read", label: <Trans id="label.read">Read</Trans>, counter: readTotal },
            ]}
            activeTab={activeTab}
            onChange={this.handleTabChange}
            className="mt-6"
          />

          {activeTab === "unread" && (
            <div 
              ref={this.unreadContainerRef}
              className="mt-4 overflow-y-auto"
              style={{ maxHeight: "600px" }}
            >
              <HStack spacing={4} className="mb-2">
                <h4 className="text-title">
                  <Trans id="label.unread">Unread</Trans>
                </h4>
                {unreadTotal > 0 && (
                  <Button 
                    size="small" 
                    onClick={this.markAllAsRead}
                  >
                    <Trans id="action.markallasread">Mark All as Read</Trans>
                  </Button>
                )}
              </HStack>

              <VStack spacing={2} className="border-t pt-2">
                {unreadTotal > 0 ? (
                  this.renderNotificationItems(unread)
                ) : (
                  <span className="text-muted py-4">
                    <Trans id="mynotifications.message.nounread">No unread notifications.</Trans>
                  </span>
                )}
                
                {loading && unread.length < unreadTotal && (
                  <div className="text-center py-4">
                    <span className="text-muted">Loading more...</span>
                  </div>
                )}
              </VStack>
            </div>
          )}

          {activeTab === "read" && (
            <div 
              ref={this.readContainerRef}
              className="mt-4 overflow-y-auto"
              style={{ maxHeight: "600px" }}
            >
              <div className="flex justify-between items-center mb-2">
                <h4 className="text-title">
                  <Trans id="label.read">Read</Trans>
                </h4>
                {readTotal > 0 && (
                  <Button
                    size="small"
                    variant="danger"
                    disabled={purging}
                    onClick={this.purgeReadNotifications}
                  >
                    <Icon sprite={IconTrash} className="h-4 mr-1" />
                    <Trans id="action.purgeread">Purge All</Trans>
                  </Button>
                )}
              </div>

              <VStack spacing={2} className="border-t pt-2">
                {readTotal > 0 ? (
                  this.renderNotificationItems(read)
                ) : (
                  <span className="text-muted py-4">
                    <Trans id="mynotifications.message.noread">No read notifications.</Trans>
                  </span>
                )}
                
                {loading && read.length < readTotal && (
                  <div className="text-center py-4">
                    <span className="text-muted">Loading more...</span>
                  </div>
                )}
              </VStack>
            </div>
          )}
        </div>
      </>
    )
  }
}
