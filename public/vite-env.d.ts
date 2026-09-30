/// <reference types="vite/client" />

interface Window {
  adsbygoogle?: {
    push: (entry: Record<string, unknown>) => unknown
    requestNonPersonalizedAds?: number
  }
  dataLayer?: IArguments[]
  gtag?: (...args: unknown[]) => void
  [key: `ga-disable-${string}`]: boolean
}
