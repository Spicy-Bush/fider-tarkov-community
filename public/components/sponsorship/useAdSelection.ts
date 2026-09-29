import { useEffect, useState } from "react"
import { PublicAd } from "@fider/models"
import { Fider } from "@fider/services/fider"
import { RequestError } from "@fider/services/http"
import { selectAds, type AdSelectRequestSlot } from "@fider/services/actions/sponsorship"

function siteLocale(): string {
  const loc = (Fider.currentLocale || "en").toLowerCase()
  return loc.startsWith("ru") ? "ru" : "en"
}

export type { AdSelectRequestSlot }

interface AdSelection {
  scope: string
  ads: Record<string, PublicAd | null>
}

export function useAdSelection(slots: AdSelectRequestSlot[]): {
  ads: Record<string, PublicAd | null | undefined>
  loaded: boolean
  error: boolean
  retry: () => void
} {
  const locale = siteLocale()
  const scope = `${Fider.session.tenant.id}:${locale}`
  const key = JSON.stringify(slots)
  const requestKey = `${scope}:${key}`
  // Scrolling a slot out of view must not change its ad.
  const [selection, setSelection] = useState<AdSelection>({ scope, ads: {} })
  const [failure, setFailure] = useState<string>()
  const [attempt, setAttempt] = useState(0)
  const ads = selection.scope === scope ? selection.ads : {}
  const complete = slots.every((slot) => ads[slot.instanceId] !== undefined)
  const error = !complete && failure === requestKey

  useEffect(() => {
    const request = new AbortController()
    const missing = slots.filter((slot) => ads[slot.instanceId] === undefined)
    setFailure(undefined)

    const load = async () => {
      try {
        for (let offset = 0; offset < missing.length; offset += 32) {
          const batch = missing.slice(offset, offset + 32)
          const result = await selectAds(batch, locale, request.signal)

          if (request.signal.aborted) {
            return
          }

          if (!result.ok) {
            setFailure(requestKey)
            return
          }

          const received: Record<string, PublicAd | null> = {}
          for (const slot of batch) {
            const ad = result.data[slot.instanceId]

            if (ad === undefined) {
              throw new RequestError("POST", "/api/ads/select", "response", new Error(`Ad selection omitted ${slot.instanceId}`))
            }

            received[slot.instanceId] = ad
          }

          setSelection((previous) => ({
            scope,
            ads: { ...(previous.scope === scope ? previous.ads : {}), ...received },
          }))
        }
      } catch (cause) {
        if (request.signal.aborted) {
          return
        }

        if (!(cause instanceof RequestError)) {
          throw cause
        }

        setFailure(requestKey)
      }
    }

    void load()
    return () => request.abort()
  }, [scope, key, attempt])

  return {
    ads,
    loaded: complete || error,
    error,
    retry: () => setAttempt((previous) => previous + 1),
  }
}
