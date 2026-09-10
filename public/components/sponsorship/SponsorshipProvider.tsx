import React, { createContext, useContext, useEffect, useMemo, useState } from "react"
import { PublicSponsorshipCampaign } from "@fider/models"
import { actions, Fider } from "@fider/services"

type AdMap = Record<string, PublicSponsorshipCampaign | null>

interface SponsorshipContextValue {
  ads: AdMap
  loaded: boolean
}

const SponsorshipContext = createContext<SponsorshipContextValue>({ ads: {}, loaded: false })

function siteLocale(): string {
  const loc = (Fider.currentLocale || "en").toLowerCase()
  return loc.startsWith("ru") ? "ru" : "en"
}

interface SponsorshipProviderProps {
  slots: string[]
  children: React.ReactNode
}

/** One network request for all requested slots on the page. */
export const SponsorshipProvider: React.FC<SponsorshipProviderProps> = ({ slots, children }) => {
  const key = useMemo(() => [...new Set(slots.filter(Boolean))].sort().join(","), [slots])
  const slotList = useMemo(() => (key ? key.split(",") : []), [key])
  const [ads, setAds] = useState<AdMap>({})
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    let cancelled = false
    if (slotList.length === 0) {
      setAds({})
      setLoaded(true)
      return
    }
    setLoaded(false)
    ;(async () => {
      const result = await actions.getActiveSponsorshipMap(slotList, siteLocale())
      if (cancelled) return
      if (result.ok && result.data) {
        const map: AdMap = {}
        for (const s of slotList) {
          const v = result.data[s]
          map[s] = v && typeof v === "object" && "id" in v && (v as PublicSponsorshipCampaign).id ? (v as PublicSponsorshipCampaign) : null
        }
        setAds(map)
      } else {
        const map: AdMap = {}
        for (const s of slotList) map[s] = null
        setAds(map)
      }
      setLoaded(true)
    })()
    return () => {
      cancelled = true
    }
  }, [key]) // eslint-disable-line react-hooks/exhaustive-deps

  return <SponsorshipContext.Provider value={{ ads, loaded }}>{children}</SponsorshipContext.Provider>
}

export function useSponsorshipAd(slot: string): PublicSponsorshipCampaign | null {
  const ctx = useContext(SponsorshipContext)
  return ctx.ads[slot] ?? null
}

export function useSponsorshipLoaded(): boolean {
  return useContext(SponsorshipContext).loaded
}
