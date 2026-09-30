import React, { useEffect, useRef, useState } from "react"
import { useFider } from "@fider/hooks/use-fider"
import { loadAdSense } from "@fider/services/google"

export interface AdSenseSlotProps {
  client: string
  slotId: string
  format?: string
  placementId: string
}

function AdSenseUnit({ client, slotId, format = "auto", placementId }: AdSenseSlotProps) {
  const element = useRef<HTMLModElement>(null)
  const requested = useRef(false)
  const [unavailable, setUnavailable] = useState(false)

  useEffect(() => {
    const unit = element.current!
    let active = true
    const observer = new IntersectionObserver(entries => {
      if (!entries.some(entry => entry.isIntersecting)) return
      observer.disconnect()

      void loadAdSense(client).then(loaded => {
        if (!active || !unit.isConnected || requested.current) return
        if (!loaded) {
          setUnavailable(true)
          return
        }

        requested.current = true
        try {
          window.adsbygoogle!.push({})
        } catch {
          setUnavailable(true)
        }
      })
    }, { rootMargin: "200px" })
    observer.observe(unit)

    const status = new MutationObserver(() => {
      if (unit.dataset.adStatus === "unfilled") setUnavailable(true)
    })
    status.observe(unit, { attributes: true, attributeFilter: ["data-ad-status"] })

    return () => {
      active = false
      observer.disconnect()
      status.disconnect()
    }
  }, [client])

  return (
    <div
      hidden={unavailable}
      className="my-3 w-full"
      data-ad-placement={placementId}
      data-ad-network="adsense"
    >
      <ins
        ref={element}
        className="adsbygoogle"
        style={{ display: "block", width: "100%" }}
        data-ad-client={client}
        data-ad-slot={slotId}
        data-ad-format={format}
        data-full-width-responsive="true"
      />
    </div>
  )
}

export function AdSenseSlot(props: AdSenseSlotProps) {
  const fider = useFider()
  if (fider.session.props.sponsorPreview || !props.client || !props.slotId) return null

  return <AdSenseUnit key={`${props.client}:${props.slotId}`} {...props} />
}
