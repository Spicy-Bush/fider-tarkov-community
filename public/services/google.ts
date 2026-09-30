import { Fider } from "./fider"
import { cookieConsent } from "./cookieConsent"

let analyticsID = ""
let analyticsReady = false
let analyticsLoading = false
let adSense: Promise<boolean> | undefined
let lastPage = ""
let consentInitialised = false

function consentSignals(): void {
  window.dataLayer ??= []
  window.gtag ??= function () { window.dataLayer!.push(arguments) }

  const choices = cookieConsent.snapshot()
  window.gtag("consent", consentInitialised ? "update" : "default", {
    analytics_storage: choices?.analytics ? "granted" : "denied",
    ad_storage: "denied",
    ad_user_data: "denied",
    ad_personalization: "denied",
  })
  consentInitialised = true
}

function loadScript(id: string, src: string): Promise<boolean> {
  return new Promise(resolve => {
    const script = document.createElement("script")
    script.id = id
    script.async = true
    script.src = src
    script.nonce = document.querySelector<HTMLScriptElement>("script[nonce]")?.nonce || ""
    script.crossOrigin = "anonymous"
    script.onload = () => resolve(true)
    script.onerror = () => resolve(false)
    document.head.appendChild(script)
  })
}

function clearAnalyticsCookies(): void {
  if (cookieConsent.snapshot()?.analytics) return

  const names = document.cookie.split(";").map(cookie => cookie.trim().split("=")[0])
  const host = location.hostname.split(".")
  const domains = ["", ...host.map((_, index) => `; domain=${host.slice(index).join(".")}`)]

  for (const name of names) {
    if (!/^(_ga($|_)|_gid$|_gat($|_))/.test(name)) continue

    for (const domain of domains) {
      document.cookie = `${name}=; Max-Age=0; path=/${domain}`
    }
  }
}

export function trackPageView(): void {
  if (!analyticsReady || !cookieConsent.snapshot()?.analytics || Fider.session.props.sponsorPreview) return

  const page = location.origin + location.pathname
  if (page === lastPage) return

  const pageDetails = {
    page_location: page,
    page_title: document.title,
    page_referrer: lastPage || document.referrer.split(/[?#]/)[0],
  }
  window.gtag!("set", pageDetails)
  window.gtag!("event", "page_view", pageDetails)
  lastPage = page
}

export function trackEvent(name: string, parameters: Record<string, string | boolean>): void {
  if (analyticsReady && cookieConsent.snapshot()?.analytics && !Fider.session.props.sponsorPreview) {
    window.gtag!("event", name, parameters)
  }
}

export function startGoogle(): () => void {
  analyticsID = (Fider.settings.googleAnalytics || "").trim()

  const update = () => {
    const choices = cookieConsent.snapshot()
    const preview = Fider.session.props.sponsorPreview === true
    if (analyticsID) window[`ga-disable-${analyticsID}`] = !choices?.analytics || preview

    clearAnalyticsCookies()
    if (preview) return

    consentSignals()
    if (!choices?.analytics || !analyticsID) {
      lastPage = ""
      return
    }

    if (analyticsReady) {
      trackPageView()
      return
    }
    if (analyticsLoading) return

    analyticsLoading = true
    void loadScript("google-analytics", `https://www.googletagmanager.com/gtag/js?id=${encodeURIComponent(analyticsID)}`).then(loaded => {
      if (!loaded) return

      window.gtag!("js", new Date())
      // Router commits own page views; disable history measurement in the GA stream.
      window.gtag!("config", analyticsID, {
        send_page_view: false,
        allow_google_signals: false,
        allow_ad_personalization_signals: false,
        page_location: location.origin + location.pathname,
      })
      analyticsReady = true
      trackPageView()
    })
  }

  update()
  const unsubscribe = cookieConsent.subscribe(update)
  const stopStorage = cookieConsent.listen()
  return () => {
    unsubscribe()
    stopStorage()
  }
}

export function loadAdSense(client: string): Promise<boolean> {
  if (Fider.session.props.sponsorPreview) return Promise.resolve(false)

  if (!adSense) {
    window.adsbygoogle ??= [] as Record<string, unknown>[]
    // Keep ads non-personalised where Google's automatic Limited Ads mode does not apply.
    window.adsbygoogle.requestNonPersonalizedAds = 1
    adSense = loadScript("google-adsense", `https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client=${encodeURIComponent(client)}`)
  }

  return adSense
}
