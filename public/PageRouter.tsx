import React, { useEffect, useState } from "react"
import { flushSync } from "react-dom"
import { Loader } from "@fider/components/common/Loader"
import { LayoutResolver } from "@fider/components/layouts/LayoutResolver"
import { Fider } from "@fider/services/fider"
import { ServerData } from "@fider/services/fider"
import { RequestError } from "@fider/services/http"
import { captureReadingPosition, finishReadingPosition, prepareReadingPosition, ReadingPosition } from "@fider/services/readingPosition"
import { pageLoader, PageModule } from "./AsyncPages"
import { trackPageView } from "@fider/services/google"

// Page interception must not bypass downloads or writes.
const isPagePath = (path: string): boolean =>
  path === "/" ||
  path === "/pages" ||
  path === "/terms" ||
  path === "/privacy" ||
  path === "/advertise" ||
  path === "/notifications" ||
  path === "/profile" ||
  path === "/admin" ||
  path.startsWith("/posts/") ||
  path.startsWith("/pages/") ||
  path.startsWith("/profile/") ||
  (path.startsWith("/admin/") && !path.startsWith("/admin/export/"))

interface Destination {
  url: URL
  data: ServerData
  module: PageModule
}

interface PageEntry {
  pageKey?: string
  readingPosition?: ReadingPosition
}

const loadDestination = async (url: string, signal: AbortSignal): Promise<Destination | null> => {
  const response = await fetch(url, {
    headers: { Accept: "application/vnd.fider.page+json" },
    credentials: "same-origin",
    signal,
  })

  const isPageData = response.headers.get("Content-Type")?.includes("application/json")
  if (response.status >= 500 || !isPageData) {
    await response.body?.cancel()

    if (!response.ok) {
      throw new RequestError("GET", new URL(url).pathname, "response", new Error("Page request failed"), response.status)
    }

    return null
  }

  const data = await response.json() as ServerData
  signal.throwIfAborted()

  const locale = data.tenant?.locale || data.settings.locale
  if (
    locale !== Fider.currentLocale ||
    data.settings.version !== Fider.settings.version ||
    data.settings.assetsURL !== Fider.settings.assetsURL
  ) {
    return null
  }

  const module = await pageLoader.load(data.page)
  signal.throwIfAborted()

  const finalURL = new URL(response.url)
  if (data.canonicalURL) {
    const canonicalURL = new URL(data.canonicalURL, finalURL)
    if (canonicalURL.origin === finalURL.origin) {
      finalURL.pathname = canonicalURL.pathname
    }
  }

  finalURL.hash = new URL(url).hash

  return { url: finalURL, data, module }
}

const findHero = (trigger: Element | null): HTMLElement | null => {
  const selected = trigger?.closest<HTMLElement>("[data-morph]") ?? trigger?.querySelector<HTMLElement>("[data-morph]")
  if (selected) {
    return selected
  }

  const candidates = document.querySelectorAll<HTMLElement>("#root [data-morph]")
  return candidates.length === 1 ? candidates[0] : null
}

const isInViewport = (element: HTMLElement): boolean => {
  const rect = element.getBoundingClientRect()
  return rect.bottom > 0 && rect.top < window.innerHeight
}

interface PageRouterProps {
  initialPageName: string
}

export const PageRouter: React.FC<PageRouterProps> = ({ initialPageName }) => {
  const [page, setPage] = useState(() => {
    const module = pageLoader.getCached(initialPageName)
    return module ? { key: "initial", module } : null
  })
  const [loadError, setLoadError] = useState<Error | null>(null)
  const [isPending, setIsPending] = useState(false)
  const [failedURL, setFailedURL] = useState<string | null>(null)

  useEffect(() => {
    if (page) {
      return
    }

    let mounted = true
    pageLoader.load(initialPageName).then((module) => {
      if (mounted) {
        setPage({ key: "initial", module })
      }
    }).catch((error) => {
      if (mounted) {
        setLoadError(error)
      }
    })

    return () => {
      mounted = false
    }
  }, [initialPageName])

  useEffect(() => {
    const navigation = window.navigation
    const precommit = window.NavigationPrecommitController
    if (!navigation || typeof precommit?.prototype.redirect !== "function") {
      return
    }

    let renderedURL = new URL(location.href)
    let renderedEntryKey = navigation.currentEntry.key
    let renderedPageKey = (navigation.currentEntry.getState() as PageEntry | undefined)?.pageKey || renderedEntryKey
    let transition: ViewTransition | undefined
    let pageAnimation: Animation | undefined
    const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)")

    const onNavigate = (rawEvent: Event) => {
      const event = rawEvent as NavigateEvent
      const url = new URL(event.destination.url)

      if (!event.canIntercept || event.downloadRequest !== null || event.formData || event.navigationType === "reload") {
        return
      }

      // History updates from filters and modals must not reload the page.
      if (event.destination.sameDocument && event.navigationType !== "traverse") {
        renderedURL = url
        const pageKey = renderedPageKey
        navigation.addEventListener("currententrychange", () => {
          navigation.updateCurrentEntry({ state: { ...navigation.currentEntry.getState(), pageKey } })
        }, { once: true, signal: event.signal })
        return
      }

      const saved = event.destination.getState() as PageEntry | undefined
      const destinationPageKey = saved?.pageKey || event.destination.key
      if (
        event.navigationType === "traverse" &&
        destinationPageKey === renderedPageKey &&
        url.pathname === renderedURL.pathname &&
        url.search === renderedURL.search
      ) {
        return
      }

      if (!isPagePath(url.pathname)) {
        return
      }

      const currentURL = new URL(navigation.currentEntry.url!)
      const currentState = navigation.currentEntry.getState() as PageEntry | undefined
      const currentPageKey = currentState?.pageKey || navigation.currentEntry.key
      if (
        currentPageKey === renderedPageKey &&
        currentURL.pathname === renderedURL.pathname &&
        currentURL.search === renderedURL.search
      ) {
        renderedEntryKey = navigation.currentEntry.key
        const readingPosition = captureReadingPosition()
        navigation.updateCurrentEntry({ state: { ...currentState, pageKey: renderedPageKey, readingPosition } })
      }

      transition?.skipTransition()
      pageAnimation?.cancel()
      setFailedURL(null)
      setIsPending(true)
      event.signal.addEventListener("abort", () => setIsPending(false), { once: true })

      const destination = loadDestination(url.href, event.signal).then((loaded) => {
        if (!loaded) {
          navigation.removeEventListener("navigate", onNavigate)

          if (event.navigationType === "push") {
            location.assign(url.href)
          } else {
            location.replace(url.href)
          }

          throw new DOMException("Document navigation required", "AbortError")
        }

        return loaded
      }, async (error) => {
        if (!event.signal.aborted) {
          setFailedURL(url.href)
          setIsPending(false)

          if (event.navigationType === "traverse") {
            await navigation.traverseTo(renderedEntryKey).finished
          }
        }

        throw error
      })

      const publish = async () => {
        const loaded = await destination
        event.signal.throwIfAborted()

        await transition?.finished.catch(() => {})
        event.signal.throwIfAborted()

        const commit = () => {
          event.signal.throwIfAborted()
          renderedURL = loaded.url
          renderedEntryKey = navigation.currentEntry.key
          renderedPageKey = saved?.pageKey || renderedEntryKey
          navigation.updateCurrentEntry({ state: { ...saved, pageKey: renderedPageKey } })
          prepareReadingPosition(saved?.readingPosition)

          flushSync(() => {
            document.title = loaded.data.title
            document.querySelector('meta[name="description"]')?.setAttribute("content", loaded.data.description || "")
            Fider.refresh(loaded.data)
            setPage({ key: navigation.currentEntry.id, module: loaded.module })
            setIsPending(false)
          })

          if (event.navigationType === "traverse" && location.href !== loaded.url.href) {
            history.replaceState(history.state, "", loaded.url.href)
          }

          trackPageView()
        }

        const restoreScroll = () => {
          event.scroll()

          if (event.navigationType !== "traverse" && !loaded.url.hash) {
            window.scrollTo(0, 0)
          }
        }

        const animatePage = () => {
          if (event.signal.aborted || reducedMotion.matches) {
            return
          }

          const content = document.querySelector<HTMLElement>("#root main, #root .page")
          pageAnimation = content?.animate(
            [
              { transform: "translateY(6px)" },
              { transform: "translateY(0)" },
            ],
            { duration: 150, easing: "cubic-bezier(0.2, 0, 0, 1)" }
          )
        }

        if (reducedMotion.matches || !document.startViewTransition) {
          commit()
          restoreScroll()
          await finishReadingPosition(event.signal)
          return
        }

        const outgoingHero = findHero(event.sourceElement)
        const heroKey = outgoingHero?.dataset.morph

        if (!outgoingHero || !isInViewport(outgoingHero)) {
          commit()
          restoreScroll()
          await finishReadingPosition(event.signal)
          animatePage()
          return
        }

        outgoingHero.style.viewTransitionName = "page-hero"
        let incomingHero: HTMLElement | null = null
        transition = document.startViewTransition(() => {
          commit()
          restoreScroll()

          if (heroKey && outgoingHero.style.viewTransitionName) {
            incomingHero = document.querySelector<HTMLElement>(`#root [data-morph="${CSS.escape(heroKey)}"]`)
            if (incomingHero && isInViewport(incomingHero)) {
              incomingHero.style.viewTransitionName = "page-hero"
            }
          }
        })

        const clearHeroNames = () => {
          outgoingHero.style.viewTransitionName = ""

          if (incomingHero) {
            incomingHero.style.viewTransitionName = ""
          }
        }

        void transition.ready.then(animatePage, () => {})
        void transition.finished.then(clearHeroNames, clearHeroNames)
        await transition.updateCallbackDone
        await finishReadingPosition(event.signal)
      }

      event.intercept({
        precommitHandler: event.navigationType === "traverse" ? undefined : async (controller) => {
          const loaded = await destination
          event.signal.throwIfAborted()

          if (loaded.url.href !== url.href) {
            controller.redirect(loaded.url.href)
          }
        },
        handler: publish,
        scroll: "manual",
      })
    }

    navigation.addEventListener("navigate", onNavigate)

    return () => {
      navigation.removeEventListener("navigate", onNavigate)
      transition?.skipTransition()
      pageAnimation?.cancel()
    }
  }, [])

  if (loadError) {
    return (
      <div className="page">
        <p>Failed to load page: {loadError.message}</p>
      </div>
    )
  }

  if (!page) {
    return (
      <div className="page">
        <Loader />
      </div>
    )
  }

  return (
    <>
      {isPending && <div className="nav-progress" aria-hidden="true" />}
      {failedURL && (
        <div role="alert" className="border-b border-danger/40 bg-danger/10 px-4 py-3 text-center text-sm">
          Unable to load this page. <a className="cursor-pointer underline" href={failedURL}>Try again</a>
        </div>
      )}
      <LayoutResolver
        pageName={Fider.session.page}
        pageKey={page.key}
        pageComponent={page.module.default}
        pageProps={Fider.session.props}
        pageConfig={page.module.pageConfig}
      />
    </>
  )
}
