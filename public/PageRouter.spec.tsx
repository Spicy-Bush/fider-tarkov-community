import React from "react"
import { act, render, screen } from "@testing-library/react"
import { Fider } from "@fider/services/fider"
import { pageLoader } from "./AsyncPages"
import { PageRouter } from "./PageRouter"

jest.mock("@fider/components/common/Loader", () => ({ Loader: () => <span>Loading</span> }))
jest.mock("@fider/components/layouts/LayoutResolver", () => ({
  LayoutResolver: ({ pageName, pageProps }: { pageName: string; pageProps: { title: string } }) => (
    <main>{pageName}: {pageProps.title}</main>
  ),
}))
jest.mock("./AsyncPages", () => ({
  pageLoader: {
    getCached: jest.fn(() => ({ default: () => null })),
    load: jest.fn(async () => ({ default: () => null })),
  },
}))
jest.mock("@fider/services/fider", () => {
  const session = { page: "Home/Home.page", props: { title: "Initial" } }
  return {
    Fider: {
      session,
      settings: { version: "test", assetsURL: "/assets/", locale: "en" },
      currentLocale: "en",
      refresh: jest.fn((data) => Object.assign(session, data)),
    },
  }
})

type Interception = Parameters<NavigateEvent["intercept"]>[0]

const startNavigation = (path: string, options: Partial<NavigateEvent> = {}) => {
  const controller = new AbortController()
  Object.assign(controller.signal, {
    throwIfAborted: () => {
      if (controller.signal.aborted) {
        throw new DOMException("Navigation cancelled", "AbortError")
      }
    },
  })
  const event = new Event("navigate", { cancelable: true })
  const intercept = jest.fn<void, [Interception]>()

  Object.assign(event, {
    canIntercept: true,
    navigationType: "push",
    destination: { key: "entry", url: new URL(path, location.href).href, sameDocument: false, getState: () => undefined },
    hashChange: false,
    downloadRequest: null,
    formData: null,
    sourceElement: null,
    signal: controller.signal,
    intercept,
    scroll: jest.fn(),
    ...options,
  })

  act(() => {
    window.navigation!.dispatchEvent(event)
  })

  return { controller, interception: intercept.mock.calls[0]?.[0] }
}

const respond = (path: string, page = "ShowPost/ShowPost.page", status = 200) => {
  jest.mocked(fetch).mockResolvedValueOnce({
    ok: status < 400,
    status,
    url: new URL(path, location.href).href,
    headers: new Headers({ "Content-Type": "application/json" }),
    json: async () => ({ page, title: path, props: { title: path }, settings: Fider.settings }),
  } as Response)
}

const finishNavigation = async (interception: Interception) => {
  await act(async () => {
    await interception.precommitHandler?.({ redirect: jest.fn() })
    await interception.handler()
  })
}

beforeEach(() => {
  jest.clearAllMocks()
  window.history.replaceState(null, "", "/")
  Object.assign(Fider.session, { page: "Home/Home.page", props: { title: "Initial" } })
  Object.assign(window, {
    navigation: Object.assign(new EventTarget(), {
      currentEntry: { id: "next", key: "entry", url: location.href, getState: () => undefined },
      updateCurrentEntry: jest.fn(),
    }),
    NavigationPrecommitController: { prototype: { redirect: jest.fn() } },
    matchMedia: () => ({ matches: true }),
    fetch: jest.fn(),
    scrollTo: jest.fn(),
  })
  document.startViewTransition = undefined as unknown as typeof document.startViewTransition
})

test.each([401, 403, 404])("publishes the server snapshot and authoritative %i page together", async (status) => {
  render(<PageRouter initialPageName={Fider.session.page} />)
  const page = `Error/Error${status}.page`
  respond("/admin", page, status)

  const { interception } = startNavigation("/admin")
  expect(screen.getByRole("main")).toHaveTextContent("Initial")
  expect(Fider.refresh).not.toHaveBeenCalled()

  await finishNavigation(interception)

  expect(screen.getByRole("main")).toHaveTextContent(`${page}: /admin`)
  expect(Fider.refresh).toHaveBeenCalledTimes(1)
  expect(fetch).toHaveBeenCalledWith(expect.any(String), expect.objectContaining({
    headers: { Accept: "application/vnd.fider.page+json" },
    signal: expect.any(AbortSignal),
  }))
})

test.each([
  { status: 502, contentType: "text/html" },
  { status: 500, contentType: "application/json" },
  { status: 503, contentType: "application/json" },
])("a $status $contentType failure preserves the current page and offers a working retry", async ({ status, contentType }) => {
  render(<PageRouter initialPageName={Fider.session.page} />)
  jest.mocked(fetch).mockResolvedValueOnce({
    ok: false,
    status,
    url: new URL("/posts/1", location.href).href,
    headers: new Headers({ "Content-Type": contentType }),
    json: async () => ({ page: "Error/Error500.page", props: {}, settings: Fider.settings }),
  } as Response)

  const failed = startNavigation("/posts/1")
  await act(async () => {
    await expect(failed.interception.precommitHandler!({ redirect: jest.fn() })).rejects.toHaveProperty("status", status)
  })

  expect(screen.getByRole("main")).toHaveTextContent("Initial")
  expect(screen.getByRole("alert")).toHaveTextContent("Try again")
  expect(Fider.refresh).not.toHaveBeenCalled()

  respond("/posts/1")
  await finishNavigation(startNavigation("/posts/1").interception)

  expect(screen.queryByRole("alert")).toBeNull()
  expect(screen.getByRole("main")).toHaveTextContent("/posts/1")
})

test("loads a traversal when only the query changes", async () => {
  window.history.replaceState(null, "", "/?view=newest")
  render(<PageRouter initialPageName={Fider.session.page} />)
  respond("/?view=trending", "Home/Home.page")

  const { interception } = startNavigation("/?view=trending", { navigationType: "traverse" })
  await finishNavigation(interception)

  expect(screen.getByRole("main")).toHaveTextContent("/?view=trending")
})

test("identical URLs retain separate page entries while modal entries share their page", async () => {
  render(<PageRouter initialPageName={Fider.session.page} />)
  const destination = { key: "modal", url: location.href, sameDocument: true, getState: () => ({ pageKey: "entry" }) }

  expect(startNavigation("/", { navigationType: "traverse", destination }).interception).toBeUndefined()
  expect(fetch).not.toHaveBeenCalled()

  respond("/", "Home/Home.page")
  const previous = startNavigation("/", {
    navigationType: "traverse",
    destination: { ...destination, key: "previous", getState: () => ({ pageKey: "previous" }) },
  })
  await finishNavigation(previous.interception)

  expect(Fider.refresh).toHaveBeenCalledTimes(1)
})

test("cancellation between the response and module load prevents publication", async () => {
  let resolveModule: (module: Awaited<ReturnType<typeof pageLoader.load>>) => void
  jest.mocked(pageLoader.load).mockReturnValueOnce(new Promise((resolve) => {
    resolveModule = resolve
  }))
  render(<PageRouter initialPageName={Fider.session.page} />)
  respond("/posts/1")

  const { controller, interception } = startNavigation("/posts/1")
  const loaded = interception.precommitHandler!({ redirect: jest.fn() })
  const rejected = expect(loaded).rejects.toHaveProperty("name", "AbortError")

  await act(async () => {
    await Promise.resolve()
    controller.abort()
    resolveModule!({ default: () => null })
    await rejected
  })

  expect(Fider.refresh).not.toHaveBeenCalled()
  expect(screen.getByRole("main")).toHaveTextContent("Initial")
})

test("a cancelled view transition cannot publish or block the next navigation", async () => {
  window.matchMedia = (() => ({ matches: false })) as typeof window.matchMedia
  render(<PageRouter initialPageName={Fider.session.page} />)
  respond("/posts/1")

  const trigger = document.createElement("a")
  trigger.dataset.morph = "post-1"
  trigger.getBoundingClientRect = () => ({ top: 20, bottom: 40 }) as DOMRect

  const { controller, interception } = startNavigation("/posts/1", { sourceElement: trigger })
  document.startViewTransition = jest.fn((update: () => void) => {
    const done = Promise.resolve().then(() => {
      controller.abort()
      update()
    })

    return { updateCallbackDone: done, finished: done, ready: Promise.resolve(), skipTransition: jest.fn() }
  }) as typeof document.startViewTransition

  await act(async () => {
    await interception.precommitHandler!({ redirect: jest.fn() })
    await expect(interception.handler()).rejects.toHaveProperty("name", "AbortError")
  })

  expect(Fider.refresh).not.toHaveBeenCalled()
  expect(screen.getByRole("main")).toHaveTextContent("Initial")

  document.startViewTransition = undefined as unknown as typeof document.startViewTransition
  respond("/posts/2")
  const next = startNavigation("/posts/2")
  await finishNavigation(next.interception)

  expect(screen.getByRole("main")).toHaveTextContent("/posts/2")
  expect(Fider.refresh).toHaveBeenCalledTimes(1)
})

test.each([false, true])("page animation respects reduced motion (%s) without a shared title", async (reduced) => {
  window.matchMedia = (() => ({ matches: reduced })) as typeof window.matchMedia
  document.startViewTransition = jest.fn()
  const { unmount } = render(
    <div id="root">
      <PageRouter initialPageName={Fider.session.page} />
    </div>
  )
  const content = screen.getByRole("main")
  const cancel = jest.fn()
  const animatedContent: string[] = []
  content.animate = jest.fn(() => {
    animatedContent.push(content.textContent!)
    return { cancel } as unknown as Animation
  })

  respond("/pages", "Page/ListPages.page")
  const { interception } = startNavigation("/pages")
  await finishNavigation(interception)

  expect(screen.getByRole("main")).toHaveTextContent("Page/ListPages.page: /pages")
  expect(document.startViewTransition).not.toHaveBeenCalled()
  expect(animatedContent).toEqual(reduced ? [] : ["Page/ListPages.page: /pages"])

  respond("/posts/2")
  const next = startNavigation("/posts/2")
  expect(cancel).toHaveBeenCalledTimes(reduced ? 0 : 1)
  await finishNavigation(next.interception)

  unmount()
  expect(cancel).toHaveBeenCalledTimes(reduced ? 0 : 2)
})

test("page-owned filters and modal history do not fetch or remount the page", () => {
  render(<PageRouter initialPageName={Fider.session.page} />)
  const url = new URL("/?view=newest", location.href).href

  expect(startNavigation(url, {
    navigationType: "replace",
    destination: { url, sameDocument: true } as NavigationDestination,
  }).interception).toBeUndefined()
  expect(startNavigation(url, { navigationType: "traverse" }).interception).toBeUndefined()
  expect(fetch).not.toHaveBeenCalled()
})

test("unsupported browsers, downloads and side-effecting routes retain document navigation", () => {
  const { unmount } = render(<PageRouter initialPageName={Fider.session.page} />)
  expect(startNavigation("/admin/export/backup.zip").interception).toBeUndefined()
  expect(startNavigation("/signout").interception).toBeUndefined()
  expect(startNavigation("/posts/1", { downloadRequest: "post.html" }).interception).toBeUndefined()
  expect(startNavigation("/#comment-1", {
    hashChange: true,
    destination: { url: new URL("/#comment-1", location.href).href, sameDocument: true } as NavigationDestination,
  }).interception).toBeUndefined()

  unmount()
  delete window.NavigationPrecommitController
  render(<PageRouter initialPageName={Fider.session.page} />)

  expect(startNavigation("/posts/1").interception).toBeUndefined()
  expect(fetch).not.toHaveBeenCalled()
})
