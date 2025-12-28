// import "./Home.page.scss"
import { undrawNoData as NoDataIllustration, heroiconsX as IconX } from "@fider/icons.generated"

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
        <div className="flex items-start gap-2 p-3 mb-4 bg-accent-light border border-accent rounded-card">
          <div className="flex-1">
            <Markdown text={fider.session.tenant.messageBanner} style="full" />
          </div>
          <button 
            className="shrink-0 flex items-center justify-center w-6 h-6 p-0 bg-transparent border-none rounded cursor-pointer text-muted hover:text-foreground hover:bg-accent-light transition-colors" 
            onClick={handleDismissMessage}
            aria-label="Dismiss message"
          >
            <Icon sprite={IconX} className="w-4 h-4" />
          </button>
        </div>
      )
    }
    return null
  }

  return (
    <div id="p-home" className="page container grid grid-cols-1 gap-y-6 lg:grid-cols-3 lg:gap-x-6 lg:gap-y-0 grow">
      {/* Welcome column - 1 column on lg */}
      <div className="p-home__welcome-col bg-border tag-clipped p-px self-start">
        <VStack spacing={2} className="p-4 bg-elevated tag-clipped-inner">
          <Markdown text={fider.session.tenant.welcomeMessage || defaultWelcomeMessage} style="full" />
          <PostInput placeholder={fider.session.tenant.invitation || defaultInvitation} onTitleChanged={setTitle} />
          <div className="text-center mt-4">
            <a 
              href="https://ko-fi.com/tarkovcommunity" 
              target="_blank" 
              rel="noreferrer" 
              className="kofi-button inline-flex items-center gap-2.5 px-4 py-2 rounded-button bg-[#ff5e5b]/10 border border-[#ff5e5b]/30 text-foreground no-underline transition-all duration-200 hover:bg-[#ff5e5b]/20 hover:border-[#ff5e5b]/50 hover:scale-[1.02]"
            >
              <img src="/misc/Ko-fi_HEART.webp" alt="" width="28" height="28" className="h-7 w-7" />
              <span className="font-medium text-sm">Support us on Ko-fi</span>
            </a>
          </div>
        </VStack>
      </div>
      {/* Posts column - 2 columns on lg */}
      <div className="p-home__posts-col col-span-1 lg:col-span-2 bg-elevated rounded-panel border border-border p-4 self-start">
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
