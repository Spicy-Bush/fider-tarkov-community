import { useEffect, useState } from "react"
import { PlacementAdConfig, SPONSORSHIP_SLOT_SPECS } from "@fider/models"
import { actions } from "@fider/services"

let cache: Record<string, PlacementAdConfig> | null = null
let inflight: Promise<Record<string, PlacementAdConfig>> | null = null

async function loadPlacementAdConfig(): Promise<Record<string, PlacementAdConfig>> {
  if (cache) return cache
  if (inflight) return inflight
  inflight = (async () => {
    const result = await actions.getAdPlacementConfig()
    const map: Record<string, PlacementAdConfig> = {}
    if (result.ok && result.data) {
      for (const [id, cfg] of Object.entries(result.data)) {
        map[id] = cfg
      }
    }
    cache = map
    inflight = null
    return map
  })()
  return inflight
}

/** Test helper — reset module cache between specs. */
export function __resetPlacementAdConfigCacheForTests(): void {
  cache = null
  inflight = null
}

/** Test / story helper — seed cache without HTTP. */
export function __setPlacementAdConfigCacheForTests(map: Record<string, PlacementAdConfig> | null): void {
  cache = map
  inflight = null
}

export function resolvePlacementAdConfig(
  placementId: string,
  fromApi?: Record<string, PlacementAdConfig> | null
): PlacementAdConfig {
  const api = fromApi?.[placementId]
  const spec = SPONSORSHIP_SLOT_SPECS[placementId]
  return {
    adsenseSlotId: (api?.adsenseSlotId || "").trim(),
    adsenseFormat: (api?.adsenseFormat || "").trim() || "auto",
    emptyPolicy: api?.emptyPolicy || spec?.emptyPolicy || "collapse",
    maxWidth: api?.maxWidth,
    maxHeight: api?.maxHeight,
    kind: api?.kind || spec?.kind,
  }
}

/**
 * Loads public placement AdSense/empty metadata once (catalog-owned slot ids).
 * When enabled=false (admin preview), skips fetch.
 */
export function usePlacementAdConfig(enabled: boolean): {
  config: Record<string, PlacementAdConfig>
  loaded: boolean
} {
  const [config, setConfig] = useState<Record<string, PlacementAdConfig>>(cache || {})
  const [loaded, setLoaded] = useState(!enabled || cache !== null)

  useEffect(() => {
    if (!enabled) {
      setLoaded(true)
      return
    }
    let cancelled = false
    ;(async () => {
      const map = await loadPlacementAdConfig()
      if (cancelled) return
      setConfig(map)
      setLoaded(true)
    })()
    return () => {
      cancelled = true
    }
  }, [enabled])

  return { config, loaded }
}
