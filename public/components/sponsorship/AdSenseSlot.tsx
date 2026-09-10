import React, { useEffect, useRef } from "react"
import { SPONSORSHIP_SLOT_SPECS } from "@fider/models"

export interface AdSenseSlotProps {
  client: string
  slotId: string
  format?: string
  placementId: string
  instanceId?: string
  className?: string
}

/**
 * Presentational Google AdSense unit. Clicks stay with Google (no /ads/click).
 * Script tag is boot-injected from GOOGLE_ADSENSE; this only pushes adsbygoogle once per mount.
 * No TC "Sponsored" disclosure — Google provides its own labeling; bare Sponsored is #38/#39.
 */
export const AdSenseSlot: React.FC<AdSenseSlotProps> = ({
  client,
  slotId,
  format = "auto",
  placementId,
  instanceId,
  className,
}) => {
  const pushed = useRef(false)
  const spec = SPONSORSHIP_SLOT_SPECS[placementId] || SPONSORSHIP_SLOT_SPECS.sidebar_top
  const frameClassName = spec.frameClassName

  useEffect(() => {
    if (pushed.current) return
    if (!client || !slotId) return
    try {
      const w = window as Window
      w.adsbygoogle = w.adsbygoogle || []
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      ;(w.adsbygoogle as any[]).push({})
      pushed.current = true
    } catch {
      // Ad blockers / missing script — leave reserved frame empty.
    }
  }, [client, slotId])

  if (!client || !slotId) {
    return null
  }

  return (
    <div
      className={className || "block my-3"}
      data-ad-instance={instanceId}
      data-ad-placement={placementId}
      data-ad-network="adsense"
    >
      <div className={frameClassName}>
        <ins
          className="adsbygoogle"
          style={{ display: "block", width: "100%", height: "100%" }}
          data-ad-client={client}
          data-ad-slot={slotId}
          data-ad-format={format || "auto"}
          data-full-width-responsive="true"
        />
      </div>
    </div>
  )
}
