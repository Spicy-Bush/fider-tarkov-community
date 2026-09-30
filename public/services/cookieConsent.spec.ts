import { cookieConsent } from "./cookieConsent"

test("stored choices expire, reject malformed values, and follow changes in another tab", () => {
  const stop = cookieConsent.listen()
  const reload = (value: string | null) => {
    if (value === null) localStorage.removeItem("fider-cookie-consent")
    else localStorage.setItem("fider-cookie-consent", value)

    window.dispatchEvent(new StorageEvent("storage", { key: "fider-cookie-consent" }))
  }

  const invalid = [
    null,
    "{",
    JSON.stringify({ analytics: "yes", expiresAt: Date.now() + 1000 }),
    JSON.stringify({ analytics: true, expiresAt: 1 }),
  ]

  for (const value of invalid) {
    reload(value)
    expect(cookieConsent.snapshot()).toBeNull()
  }

  reload(JSON.stringify({ analytics: true, expiresAt: Date.now() + 1000 }))
  expect(cookieConsent.snapshot()).toEqual({ analytics: true })
  reload(null)
  expect(cookieConsent.snapshot()).toBeNull()
  stop()
})

test("withdrawal clears an earlier grant when storage is full", () => {
  cookieConsent.save({ analytics: true })
  const write = jest.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
    throw new DOMException("Storage is full", "QuotaExceededError")
  })

  try {
    cookieConsent.save({ analytics: false })
    expect(cookieConsent.snapshot()).toEqual({ analytics: false })
    expect(localStorage.getItem("fider-cookie-consent")).toBeNull()
  } finally {
    write.mockRestore()
  }
})
