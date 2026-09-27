import React from "react"
import { act, render, renderHook, waitFor, cleanup } from "@testing-library/react"
import { useAdSelection } from "./useAdSelection"
import * as sponsorshipActions from "@fider/services/actions/sponsorship"
import { Fider } from "@fider/services/fider"
import { PublicAd } from "@fider/models"
import { RequestError } from "@fider/services/http"

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

  test("selects only new slots and keeps confirmed ads when the viewport changes", async () => {
    selectAds.mockImplementation(async (slots) => ({
      ok: true,
      data: Object.fromEntries(slots.map((slot) => [slot.instanceId, null])),
    }))
    const first = { instanceId: "feed-0", placementId: "feed_native" }
    const second = { instanceId: "feed-1", placementId: "feed_native" }
    const { result, rerender } = renderHook(({ slots }) => useAdSelection(slots), {
      initialProps: { slots: [first] },
    })

    await waitFor(() => expect(result.current.ads[first.instanceId]).toBeNull())
    rerender({ slots: [first, second] })
    expect(result.current.ads[first.instanceId]).toBeNull()
    await waitFor(() => expect(result.current.ads[second.instanceId]).toBeNull())

    rerender({ slots: [] })
    rerender({ slots: [first] })

    expect(selectAds.mock.calls.map(([slots]) => slots)).toEqual([[first], [second]])
    expect(result.current.ads[first.instanceId]).toBeNull()
    expect(result.current.loaded).toBe(true)
  })

  test("bounds each request for a large slot set", async () => {
    selectAds.mockImplementation(async (slots) => ({
      ok: true,
      data: Object.fromEntries(slots.map((slot) => [slot.instanceId, null])),
    }))
    const slots = Array.from({ length: 100 }, (_, index) => ({
      instanceId: `feed-${index}`,
      placementId: "feed_native",
    }))
    const { result } = renderHook(() => useAdSelection(slots))

    await waitFor(() => expect(result.current.loaded).toBe(true))

    expect(selectAds.mock.calls.map(([batch]) => batch.length)).toEqual([32, 32, 32, 4])
    expect(selectAds.mock.calls.flatMap(([batch]) => batch)).toEqual(slots)
    expect(Object.keys(result.current.ads)).toHaveLength(100)
  })

  test.each(["transport", "HTTP", "missing slot"])("recovers %s failures without treating them as empty inventory", async (failure) => {
    if (failure === "transport") {
      selectAds.mockRejectedValueOnce(new RequestError("POST", "/api/ads/select", "transport", new Error("offline")))
    } else if (failure === "HTTP") {
      selectAds.mockResolvedValueOnce({ ok: false, status: 503, error: {} })
    } else {
      selectAds.mockResolvedValueOnce({ ok: true, data: {} })
    }

    const slot = { instanceId: "feed-0", placementId: "feed_native" }
    const { result } = renderHook(() => useAdSelection([slot]))

    await waitFor(() => expect(result.current.error).toBe(true))
    expect(result.current.ads[slot.instanceId]).toBeUndefined()

    selectAds.mockResolvedValueOnce({ ok: true, data: { [slot.instanceId]: null } })
    act(() => result.current.retry())

    await waitFor(() => expect(result.current.ads[slot.instanceId]).toBeNull())
    expect(result.current.error).toBe(false)
    expect(selectAds).toHaveBeenCalledTimes(2)
  })

  test("retry preserves successful batches and requests only the failed slots", async () => {
    const slots = Array.from({ length: 33 }, (_, index) => ({
      instanceId: `feed-${index}`,
      placementId: "feed_native",
    }))
    selectAds.mockResolvedValueOnce({
      ok: true,
      data: Object.fromEntries(slots.slice(0, 32).map((slot) => [slot.instanceId, null])),
    })
    selectAds.mockResolvedValueOnce({ ok: false, status: 503, error: {} })
    const { result } = renderHook(() => useAdSelection(slots))

    await waitFor(() => expect(result.current.error).toBe(true))
    expect(Object.keys(result.current.ads)).toHaveLength(32)
    expect(result.current.ads["feed-32"]).toBeUndefined()

    selectAds.mockResolvedValueOnce({ ok: true, data: { "feed-32": null } })
    act(() => result.current.retry())

    await waitFor(() => expect(result.current.ads["feed-32"]).toBeNull())
    expect(selectAds.mock.calls[2][0]).toEqual([slots[32]])
    expect(Object.keys(result.current.ads)).toHaveLength(33)
    expect(result.current.error).toBe(false)
  })

  test("aborts superseded selection and ignores its late response", async () => {
    let finish!: (result: Awaited<ReturnType<typeof selectAds>>) => void
    selectAds.mockReturnValueOnce(new Promise((resolve) => { finish = resolve }))
    selectAds.mockResolvedValueOnce({ ok: true, data: { second: null } })
    const { result, rerender } = renderHook(({ instanceId }) => useAdSelection([{ instanceId, placementId: "feed_native" }]), {
      initialProps: { instanceId: "first" },
    })
    const signal = selectAds.mock.calls[0][2]!

    rerender({ instanceId: "second" })
    await waitFor(() => expect(result.current.ads.second).toBeNull())
    expect(signal.aborted).toBe(true)

    await act(async () => finish({ ok: true, data: { first: null } }))

    expect(result.current.ads.first).toBeUndefined()
    expect(result.current.ads.second).toBeNull()
  })

  test("a new tenant cannot reuse the previous tenant's selection", async () => {
    const slot = { instanceId: "sidebar", placementId: "sidebar_top" }
    selectAds.mockResolvedValue({ ok: true, data: { sidebar: null } })
    const { result, rerender } = renderHook(() => useAdSelection([slot]))

    await waitFor(() => expect(result.current.loaded).toBe(true))

    Fider.initialize({
      settings: { environment: "test", oauth: [], locale: "en" },
      tenant: { id: 2, locale: "en" },
      user: undefined,
    })
    rerender()

    expect(result.current.ads.sidebar).toBeUndefined()
    await waitFor(() => expect(result.current.ads.sidebar).toBeNull())
    expect(selectAds).toHaveBeenCalledTimes(2)
  })
})
