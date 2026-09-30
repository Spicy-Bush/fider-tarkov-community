import React from "react"
import { act, render, screen } from "@testing-library/react"
import { Fider } from "@fider/services/fider"
import { cookieConsent } from "@fider/services/cookieConsent"
import { loadAdSense } from "@fider/services/google"
import { AdSenseSlot } from "./AdSenseSlot"

jest.mock("@fider/services/google", () => ({ loadAdSense: jest.fn() }))

let intersect: () => void

beforeEach(() => {
  Fider.initialize({ settings: {}, tenant: {}, props: {} })
  cookieConsent.save({ analytics: false })
  window.adsbygoogle = []
  jest.mocked(loadAdSense).mockReset().mockResolvedValue(true)

  window.IntersectionObserver = jest.fn((callback: IntersectionObserverCallback) => {
    const observer = { observe: jest.fn(), disconnect: jest.fn() }
    intersect = () => callback(
      [{ isIntersecting: true } as IntersectionObserverEntry],
      observer as unknown as IntersectionObserver,
    )

    return observer
  }) as unknown as typeof IntersectionObserver
})

test("a visible unit requests once and survives analytics preference changes", async () => {
  const view = render(<AdSenseSlot client="ca-pub-test" slotId="123456" placementId="home_desktop" />)
  const unit = view.container.querySelector("ins.adsbygoogle")

  expect(unit).toHaveAttribute("data-ad-client", "ca-pub-test")
  expect(unit).toHaveAttribute("data-ad-slot", "123456")
  expect(loadAdSense).not.toHaveBeenCalled()

  await act(async () => { intersect() })
  expect(loadAdSense).toHaveBeenCalledTimes(1)
  expect(window.adsbygoogle).toHaveLength(1)

  cookieConsent.save({ analytics: true })
  view.rerender(<AdSenseSlot client="ca-pub-test" slotId="123456" placementId="home_desktop" />)

  expect(view.container.querySelector("ins.adsbygoogle")).toBe(unit)
  expect(loadAdSense).toHaveBeenCalledTimes(1)
  expect(window.adsbygoogle).toHaveLength(1)
})

test("a blocked script collapses its unit without retrying or showing an error", async () => {
  jest.mocked(loadAdSense).mockResolvedValue(false)
  const view = render(<AdSenseSlot client="ca-pub-test" slotId="123456" placementId="home_desktop" />)

  await act(async () => { intersect() })

  expect(view.container.querySelector('[data-ad-network="adsense"]')).not.toBeVisible()
  expect(screen.queryByRole("alert")).not.toBeInTheDocument()
  expect(window.adsbygoogle).toHaveLength(0)

  view.rerender(<AdSenseSlot client="ca-pub-test" slotId="123456" placementId="home_desktop" />)
  expect(loadAdSense).toHaveBeenCalledTimes(1)
})

test("unconfigured units and the admin preview do not request advertisements", () => {
  const view = render(<AdSenseSlot client="" slotId="123456" placementId="home_desktop" />)
  expect(view.container).toBeEmptyDOMElement()

  view.rerender(<AdSenseSlot client="ca-pub-test" slotId="" placementId="home_desktop" />)
  expect(view.container).toBeEmptyDOMElement()

  Fider.initialize({ settings: {}, tenant: {}, props: { sponsorPreview: true } })
  view.rerender(<AdSenseSlot client="ca-pub-test" slotId="123456" placementId="home_desktop" />)
  expect(view.container).toBeEmptyDOMElement()
  expect(loadAdSense).not.toHaveBeenCalled()
})
