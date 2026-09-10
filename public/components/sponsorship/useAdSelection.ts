import { useEffect, useMemo, useState } from "react"
import { PublicAd } from "@fider/models"
import { actions, Fider } from "@fider/services"
import type { AdSelectRequestSlot } from "@fider/services/actions/sponsorship"

function siteLocale(): string {
  const loc = (Fider.currentLocale || "en").toLowerCase()
  return loc.startsWith("ru") ? "ru" : "en"
}

export type { AdSelectRequestSlot }

/**
 * Page-owned ad selection. POST /api/v1/ads/select once per distinct slot set.
 * Values: undefined while loading; null = no fill; PublicAd = fill.
 */
export function useAdSelection(slots: AdSelectRequestSlot[]): {
  ads: Record<string, PublicAd | null | undefined>
  loaded: boolean
} {
  const key = useMemo(() => {
    const norm = slots
      .filter((s) => s.instanceId && s.placementId)
      .map((s) => `${s.instanceId}\0${s.placementId}`)
      .sort()
    return norm.join("|")
  }, [slots])

  const slotList = useMemo((): AdSelectRequestSlot[] => {
    if (!key) return []
    return key.split("|").map((pair) => {
      const [instanceId, placementId] = pair.split("\0")
      return { instanceId, placementId }
    })
  }, [key])

  const [ads, setAds] = useState<Record<string, PublicAd | null | undefined>>({})
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    let cancelled = false
    if (slotList.length === 0) {
      setAds({})
      setLoaded(true)
      return
    }
    setLoaded(false)
    const pending: Record<string, PublicAd | null | undefined> = {}
    for (const s of slotList) pending[s.instanceId] = undefined
    setAds(pending)
    ;(async () => {
      const result = await actions.selectAds(slotList, siteLocale())
      if (cancelled) return
      const map: Record<string, PublicAd | null> = {}
      for (const s of slotList) {
        if (!result.ok || !result.data) {
          map[s.instanceId] = null
          continue
        }
        const v = result.data[s.instanceId]
        if (v && typeof v === "object" && typeof v.campaignId === "number") {
          const advertiser = (v.advertiser || "").trim()
          map[s.instanceId] = advertiser ? v : null // #39
        } else {
          map[s.instanceId] = null
        }
      }
      setAds(map)
      setLoaded(true)
    })()
    return () => {
      cancelled = true
    }
  }, [key]) // eslint-disable-line react-hooks/exhaustive-deps

  return { ads, loaded }
}
