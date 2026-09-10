import React from "react"
import { render, cleanup } from "@testing-library/react"
import { AdSlot } from "./AdSlot"
import { PublicAd } from "@fider/models"
import { Fider } from "@fider/services"
import {
  __resetPlacementAdConfigCacheForTests,
  __setPlacementAdConfigCacheForTests,
} from "./usePlacementAdConfig"

const houseAd: PublicAd = {
  campaignId: 7,
  advertiser: "Acme Corp",
  placementId: "sidebar_top",
  creativeVersionId: 3,
  imageUrl: "https://example.com/ad.png",
  html: "",
  clickPath: "/ads/click/7?v=3",
}

function setClient(client: string) {
  Fider.initialize({
    settings: { environment: "test", oauth: [], googleAdSense: client },
    tenant: {},
    user: undefined,
  })
  if (client) {
    window.__adsense_client = client
  } else {
    delete window.__adsense_client
  }
}

describe("<AdSlot /> AdSense empty-placement fallback", () => {
  afterEach(() => {
    cleanup()
    __resetPlacementAdConfigCacheForTests()
    delete window.__adsense_client
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    delete (window as any).adsbygoogle
  })

  test("select returns campaign → AdSense not mounted for that instance", () => {
    setClient("ca-pub-test")
    __setPlacementAdConfigCacheForTests({
      sidebar_top: { adsenseSlotId: "111", adsenseFormat: "auto", emptyPolicy: "collapse" },
    })

    const { container } = render(
      <AdSlot instanceId="sidebar" placementId="sidebar_top" ad={houseAd} />
    )

    expect(container.querySelector('[data-ad-network="house"]')).not.toBeNull()
    expect(container.querySelector('[data-ad-network="adsense"]')).toBeNull()
    expect(container.querySelector("ins.adsbygoogle")).toBeNull()
    expect(container.querySelector("a")?.getAttribute("href")).toBe("/ads/click/7?v=3")
  })

  test("select null + client → AdSense mounted with placement slot/size", () => {
    setClient("ca-pub-test")
    __setPlacementAdConfigCacheForTests({
      sidebar_top: { adsenseSlotId: "999001", adsenseFormat: "rectangle", emptyPolicy: "collapse" },
    })

    const { container } = render(
      <AdSlot instanceId="sidebar" placementId="sidebar_top" ad={null} />
    )

    const ins = container.querySelector("ins.adsbygoogle")
    expect(ins).not.toBeNull()
    expect(ins?.getAttribute("data-ad-client")).toBe("ca-pub-test")
    expect(ins?.getAttribute("data-ad-slot")).toBe("999001")
    expect(ins?.getAttribute("data-ad-format")).toBe("rectangle")
    expect(container.querySelector('[data-ad-network="adsense"]')).not.toBeNull()
    expect(container.querySelector('[data-ad-network="house"]')).toBeNull()
  })

  test("select null + no client → nothing/reserved (collapse)", () => {
    setClient("")
    __setPlacementAdConfigCacheForTests({
      sidebar_top: { adsenseSlotId: "999001", emptyPolicy: "collapse" },
    })

    const { container } = render(
      <AdSlot instanceId="sidebar" placementId="sidebar_top" ad={null} />
    )

    expect(container.querySelector("ins.adsbygoogle")).toBeNull()
    expect(container.querySelector('[data-ad-network="adsense"]')).toBeNull()
    expect(container.querySelector('[data-ad-empty="reserve"]')).toBeNull()
    expect(container.innerHTML).toBe("")
  })

  test("select null + no client + reserve → reserved frame", () => {
    setClient("")
    __setPlacementAdConfigCacheForTests({
      sidebar_top: { adsenseSlotId: "", emptyPolicy: "reserve" },
    })

    const { container } = render(
      <AdSlot instanceId="sidebar" placementId="sidebar_top" ad={null} />
    )

    expect(container.querySelector('[data-ad-empty="reserve"]')).not.toBeNull()
    expect(container.querySelector("ins.adsbygoogle")).toBeNull()
  })

  test("two feed instances one win one miss → one house + one AdSense", () => {
    setClient("ca-pub-test")
    __setPlacementAdConfigCacheForTests({
      feed_native: { adsenseSlotId: "feed-slot", adsenseFormat: "fluid", emptyPolicy: "collapse" },
    })

    const feedHouse: PublicAd = { ...houseAd, placementId: "feed_native", clickPath: "/ads/click/7?v=3" }

    const { container } = render(
      <>
        <AdSlot instanceId="feed-0" placementId="feed_native" ad={feedHouse} />
        <AdSlot instanceId="feed-1" placementId="feed_native" ad={null} />
      </>
    )

    expect(container.querySelectorAll('[data-ad-network="house"]').length).toBe(1)
    expect(container.querySelectorAll('[data-ad-network="adsense"]').length).toBe(1)
    expect(container.querySelector("ins.adsbygoogle")?.getAttribute("data-ad-slot")).toBe("feed-slot")
  })

  test("selectFailed → never mounts AdSense even with null-looking absence", () => {
    setClient("ca-pub-test")
    __setPlacementAdConfigCacheForTests({
      sidebar_top: { adsenseSlotId: "999001", emptyPolicy: "collapse" },
    })

    const { container } = render(
      <AdSlot instanceId="sidebar" placementId="sidebar_top" ad={undefined} selectFailed />
    )

    expect(container.querySelector("ins.adsbygoogle")).toBeNull()
    expect(container.innerHTML).toBe("")
  })

  test("admin allowAdSense=false → no AdSense on null", () => {
    setClient("ca-pub-test")
    __setPlacementAdConfigCacheForTests({
      sidebar_top: { adsenseSlotId: "999001", emptyPolicy: "collapse" },
    })

    const { container } = render(
      <AdSlot instanceId="admin-preview" placementId="sidebar_top" ad={null} allowAdSense={false} />
    )

    expect(container.querySelector("ins.adsbygoogle")).toBeNull()
  })

  test("AdSense unit never renders bare Sponsored disclosure (#38/#39)", () => {
    setClient("ca-pub-test")
    __setPlacementAdConfigCacheForTests({
      sidebar_top: { adsenseSlotId: "999001", adsenseFormat: "auto", emptyPolicy: "collapse" },
    })

    const { container } = render(
      <AdSlot instanceId="sidebar" placementId="sidebar_top" ad={null} />
    )

    expect(container.querySelector('[data-ad-network="adsense"]')).not.toBeNull()
    expect(container.querySelector("ins.adsbygoogle")).not.toBeNull()
    // Must not claim TC Sponsored without an advertiser name (Google has its own labeling).
    expect(container.textContent || "").not.toMatch(/Sponsored/)
  })

})
