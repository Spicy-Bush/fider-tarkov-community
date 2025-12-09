import "./Home.page.scss"
import NoDataIllustration from "@fider/assets/images/undraw-no-data.svg"

import React, { useState, useEffect } from "react"
import { Post, Tag, PostStatus } from "@fider/models"
import { Markdown, Hint, Icon } from "@fider/components"
import { SimilarPosts } from "./components/SimilarPosts"
import { PostInput } from "./components/PostInput"
import { PostsContainer } from "./components/PostsContainer"
import { useFider } from "@fider/hooks"
import { VStack } from "@fider/components/layout"
import { isContentDismissed, dismissContentByValue, DismissableContentTypes } from "@fider/services/device"

import { i18n } from "@lingui/core"
import { Trans } from "@lingui/react/macro"

export interface HomePageProps {
  posts: Post[]
  tags: Tag[]
  countPerStatus: { [key: string]: number }
}

export interface HomePageState {
  title: string
}

const Lonely = () => {
  const fider = useFider()

  return (
    <div className="text-center">
      <Hint permanentCloseKey="at-least-3-posts" condition={fider.session.isAuthenticated && fider.session.user.isAdministrator}>
        <p>
          <Trans id="home.lonely.suggestion">
            It&apos;s recommended that you create <strong>at least 3</strong> suggestions here before sharing this site. The initial content is important to
            start engaging your audience.
          </Trans>
        </p>
      </Hint>
      <Icon sprite={NoDataIllustration} height="120" className="mt-6 mb-2" />
      <p className="text-muted">
        <Trans id="home.lonely.text">No posts have been created yet.</Trans>
      </p>
    </div>
  )
}

const HomePage = (props: HomePageProps) => {
  const fider = useFider()
  const [title, setTitle] = useState("")
  const [showMessageBanner, setShowMessageBanner] = useState(true)
  
  useEffect(() => {
    if (fider.session.tenant.messageBanner) {
      setShowMessageBanner(!isContentDismissed(
        DismissableContentTypes.MESSAGE_BANNER, 
        fider.session.tenant.messageBanner
      ))
    }
  }, [fider.session.tenant.messageBanner])

  const handleDismissMessage = () => {
    if (fider.session.tenant.messageBanner) {
      dismissContentByValue(
        DismissableContentTypes.MESSAGE_BANNER, 
        fider.session.tenant.messageBanner
      )
      setShowMessageBanner(false)
    }
  }

  const defaultWelcomeMessage = i18n._("home.form.defaultwelcomemessage", {
    message: `We'd love to hear what you're thinking about.

What can we do better? This is the place for you to vote, discuss and share ideas.`,
  })

  const defaultInvitation = i18n._("home.form.defaultinvitation", {
    message: "Enter your suggestion here...",
  })

  const isLonely = () => {
    const len = Object.keys(props.countPerStatus).length
    if (len === 0) {
      return true
    }

    if (len === 1 && PostStatus.Deleted.value in props.countPerStatus) {
      return true
    }

    return false
  }

  const renderMessageBanner = () => {
    if (fider.session.tenant.messageBanner !== "" && 
        fider.session.tenant.messageBanner !== undefined && 
        showMessageBanner) {
      return (
        <div className="p-home__message-banner">
          <div className="p-home__message-banner-content">
            <Markdown text={fider.session.tenant.messageBanner} style="full" />
          </div>
          <button 
            className="p-home__message-banner-dismiss" 
            onClick={handleDismissMessage}
            aria-label="Dismiss message"
          >
          </button>
        </div>
      )
    }
    return null
  }

  return (
    <div id="p-home" className="page container">
        <div className="p-home__welcome-col">
          <VStack spacing={2} className="p-4">
            <Markdown text={fider.session.tenant.welcomeMessage || defaultWelcomeMessage} style="full" />
            <PostInput placeholder={fider.session.tenant.invitation || defaultInvitation} onTitleChanged={setTitle} />
            <div style={{ textAlign: "center", marginTop: "1rem" }}>
              <a href="https://ko-fi.com/tarkovcommunity" target="_blank" rel="noreferrer" style={{ display: "inline-flex", alignItems: "center", gap: "0.5rem", textDecoration: "none", color: "inherit" }}>
                <img src="/misc/Ko-fi_HEART.gif" alt="" style={{ height: "1.5rem", width: "auto" }} />
                <span>Support us on Ko-fi</span>
              </a>
            </div>
          </VStack>
        </div>
        <div className="p-home__posts-col p-4">
          {isLonely() ? (
            <Lonely />
          ) : title ? (
            <SimilarPosts title={title} tags={props.tags} />
          ) : (
            <>
              {renderMessageBanner()}
              <PostsContainer posts={props.posts} tags={props.tags} countPerStatus={props.countPerStatus} />
            </>
          )}
        </div>
      </div>
  )
}

export default HomePage
