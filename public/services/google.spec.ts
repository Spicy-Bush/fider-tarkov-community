import { Fider } from "./fider"
import { cookieConsent } from "./cookieConsent"
import { loadAdSense, startGoogle, trackEvent, trackPageView } from "./google"

test("analytics consent controls measurement independently of automatic AdSense delivery", async () => {
  Fider.initialize({ settings: { googleAnalytics: "G-TEST", googleAdSense: "ca-pub-test" }, props: {} })
  cookieConsent.save({ analytics: false })
  const stop = startGoogle()
  const commands = () => window.dataLayer!.map(entry => Array.from(entry))
  const pageViews = () => commands().filter(entry => entry[0] === "event" && entry[1] === "page_view")

  expect(document.querySelector("script[src*='google']")).toBeNull()
  const first = loadAdSense("ca-pub-test")
  const second = loadAdSense("ca-pub-test")
  expect(first).toBe(second)
  expect(window.adsbygoogle?.requestNonPersonalizedAds).toBe(1)
  expect(document.querySelector("#google-adsense")).not.toBeNull()

  trackEvent("upvote", { event_category: "post" })
  expect(commands().filter(entry => entry[0] === "event")).toEqual([])

  cookieConsent.save({ analytics: true })
  document.querySelector("#google-analytics")!.dispatchEvent(new Event("load"))
  await Promise.resolve()
  expect(pageViews()).toHaveLength(1)
  expect(commands().filter(entry => entry[0] === "consent").every(entry => {
    const signals = entry[2] as Record<string, string>
    return signals.ad_storage === "denied" && signals.ad_user_data === "denied" && signals.ad_personalization === "denied"
  })).toBe(true)

  history.pushState({}, "", "/posts/70/example?token=private#comment-2")
  trackPageView()
  history.replaceState({}, "", "/posts/70/example?sort=latest")
  trackPageView()
  expect(pageViews()).toHaveLength(2)
  expect(JSON.stringify(pageViews())).not.toContain("private")
  trackEvent("upvote", { event_category: "post" })
  expect(commands().filter(entry => entry[1] === "upvote")).toHaveLength(1)

  document.querySelector("#google-adsense")!.dispatchEvent(new Event("error"))
  expect(await first).toBe(false)
  expect(await loadAdSense("ca-pub-test")).toBe(false)
  expect(document.querySelectorAll("#google-adsense")).toHaveLength(1)

  document.cookie = "_ga=previous; path=/"
  document.cookie = "__gads=previous; path=/"
  cookieConsent.save({ analytics: false })
  trackPageView()
  trackEvent("upvote", { event_category: "post" })
  expect(window["ga-disable-G-TEST"]).toBe(true)
  expect(commands().filter(entry => entry[1] === "upvote")).toHaveLength(1)
  expect(document.cookie).not.toMatch(/_ga=/)
  expect(document.cookie).toContain("__gads=previous")

  Fider.initialize({ settings: { googleAnalytics: "G-TEST", googleAdSense: "ca-pub-test" }, props: { sponsorPreview: true } })
  cookieConsent.save({ analytics: true })
  expect(await loadAdSense("ca-pub-test")).toBe(false)
  trackEvent("upvote", { event_category: "post" })
  expect(commands().filter(entry => entry[1] === "upvote")).toHaveLength(1)
  expect(document.querySelectorAll("#google-adsense")).toHaveLength(1)
  expect(window["ga-disable-G-TEST"]).toBe(true)
  stop()
})
