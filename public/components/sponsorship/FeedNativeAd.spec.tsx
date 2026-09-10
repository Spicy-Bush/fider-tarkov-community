import React from "react"
import { render, cleanup } from "@testing-library/react"
import { FeedNativeAd } from "./FeedNativeAd"
import { PublicAd } from "@fider/models"

const houseAd: PublicAd = {
  campaignId: 7,
  advertiser: "Acme Corp",
  placementId: "feed_native",
  creativeVersionId: 3,
  imageUrl: "https://example.com/ad.png",
  html: "",
  clickPath: "/ads/click/7?v=3",
}

describe("<FeedNativeAd />", () => {
  afterEach(() => cleanup())

  test("html creative is not wrapped in a parent anchor", () => {
    const ad: PublicAd = { ...houseAd, html: "<p>native</p>" }
    const { container } = render(<FeedNativeAd ad={ad} />)
    expect(container.querySelector("iframe")).not.toBeNull()
    expect(container.querySelector("iframe")?.closest("a")).toBeNull()
    expect(container.querySelector('[data-ad-kind="native"]')).not.toBeNull()
  })

  test("image-only may be a link", () => {
    const { container } = render(<FeedNativeAd ad={houseAd} />)
    expect(container.querySelector("a")?.getAttribute("href")).toBe("/ads/click/7?v=3")
  })
})
