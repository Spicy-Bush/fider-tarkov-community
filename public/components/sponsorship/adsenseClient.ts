import { Fider } from "@fider/services"

/**
 * Publisher client id from env GOOGLE_ADSENSE (settings.googleAdSense) or window.__adsense_client.
 * Never hardcode publisher ids in components.
 */
export function getAdSenseClient(): string {
  if (typeof window !== "undefined") {
    const fromWindow = (window.__adsense_client || "").trim()
    if (fromWindow) return fromWindow
  }
  try {
    return (Fider.settings?.googleAdSense || "").trim()
  } catch {
    return ""
  }
}
