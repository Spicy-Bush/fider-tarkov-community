import React from "react"
import { render, waitFor, cleanup } from "@testing-library/react"
import { useAdSelection } from "./useAdSelection"
import * as sponsorshipActions from "@fider/services/actions/sponsorship"
import { Fider } from "@fider/services/fider"
import { PublicAd } from "@fider/models"

jest.mock("@fider/services/actions/sponsorship", () => {
  const actual = jest.requireActual("@fider/services/actions/sponsorship")
  return {
    ...actual,
    selectAds: jest.fn(),
  }
})

const selectAds = sponsorshipActions.selectAds as jest.MockedFunction<typeof sponsorshipActions.selectAds>

function Probe({ onValue }: { onValue: (v: ReturnType<typeof useAdSelection>) => void }) {
  const value = useAdSelection([{ instanceId: "sidebar", placementId: "sidebar_top" }])
  React.useEffect(() => {
    onValue(value)
  }, [value, onValue])
  return <div data-loaded={String(value.loaded)} data-error={String(value.error)} />
}

describe("useAdSelection", () => {
  beforeEach(() => {
    Fider.initialize({
      settings: { environment: "test", oauth: [], locale: "en" },
      tenant: { locale: "en" },
      user: undefined,
    })
    selectAds.mockReset()
  })

  afterEach(() => {
    cleanup()
  })

  test("HTTP failure surfaces error and does not map instances to empty inventory", async () => {
    selectAds.mockResolvedValue({ ok: false, data: undefined as never })

    let latest: ReturnType<typeof useAdSelection> | null = null
    render(<Probe onValue={(v) => { latest = v }} />)

    await waitFor(() => expect(latest?.loaded).toBe(true))
    expect(latest?.error).toBe(true)
    // Must stay undefined (loading/failure), not null — null would wrongly unlock AdSense.
    expect(latest?.ads["sidebar"]).toBeUndefined()
  })

  test("successful null fill is explicit null (AdSense-eligible)", async () => {
    selectAds.mockResolvedValue({ ok: true, data: { sidebar: null } })

    let latest: ReturnType<typeof useAdSelection> | null = null
    render(<Probe onValue={(v) => { latest = v }} />)

    await waitFor(() => expect(latest?.loaded).toBe(true))
    expect(latest?.error).toBe(false)
    expect(latest?.ads["sidebar"]).toBeNull()
  })

  test("successful campaign fill returns PublicAd", async () => {
    const ad: PublicAd = {
      campaignId: 1,
      advertiser: "Brand",
      placementId: "sidebar_top",
      creativeVersionId: 2,
      imageUrl: "",
      html: "<b>hi</b>",
      clickPath: "/ads/click/1?v=2",
    }
    selectAds.mockResolvedValue({ ok: true, data: { sidebar: ad } })

    let latest: ReturnType<typeof useAdSelection> | null = null
    render(<Probe onValue={(v) => { latest = v }} />)

    await waitFor(() => expect(latest?.loaded).toBe(true))
    expect(latest?.error).toBe(false)
    expect(latest?.ads["sidebar"]?.campaignId).toBe(1)
  })
})
