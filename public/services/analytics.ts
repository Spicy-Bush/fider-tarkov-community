import { trackEvent } from "./google"

export const analytics = {
  event: (eventCategory: string, eventAction: string): void => {
    trackEvent(eventAction, { event_category: eventCategory })
  },
  error: (err?: Error): void => {
    trackEvent("exception", { description: err?.name || "Error", fatal: false })
  },
}
