import { createPageLoader } from "./pageLoader"

jest.mock("./constants", () => ({
  RETRY: { MAX_PAGE_LOAD_RETRIES: 1, PAGE_LOAD_RETRY_INTERVAL_MS: 1 },
}))

test("a failed module load can recover while concurrent callers share each attempt", async () => {
  jest.useFakeTimers()
  const module = { default: () => null }
  const importPage = jest.fn()
    .mockRejectedValueOnce(new Error("Disconnected"))
    .mockResolvedValueOnce(module)
  const loader = createPageLoader({ "./Page.tsx": importPage }, { pathPrefix: "./" })

  try {
    const failed = loader.load("Page")
    expect(loader.load("Page")).toBe(failed)
    const rejected = expect(failed).rejects.toThrow("Disconnected")

    await Promise.resolve()
    await Promise.resolve()
    jest.runOnlyPendingTimers()
    await rejected

    const recovered = loader.load("Page")
    expect(loader.load("Page")).toBe(recovered)
    await expect(recovered).resolves.toBe(module)
    expect(loader.getCached("Page")).toBe(module)
    expect(importPage).toHaveBeenCalledTimes(2)
  } finally {
    jest.useRealTimers()
  }
})
