import { prepareDraftImages, restoreDraftImage } from "./draftImages"

jest.mock("./postSubmission", () => ({ newSubmissionID: () => "imported-image" }))

test("stored, removed and local files have one unambiguous wire representation", async () => {
  const file = new File([new Uint8Array([0, 255, 42])], "image.png", { type: "image/png" })
  const images = await prepareDraftImages([
    { kind: "stored", bkey: "kept" },
    { kind: "removed", bkey: "deleted" },
    { kind: "local", fileId: "local", file },
  ])

  expect(images).toEqual([
    { bkey: "kept", remove: false },
    { bkey: "deleted", remove: true },
    { remove: false, upload: { fileName: "image.png", contentType: "image/png", content: "AP8q" } },
  ])
})

test("unavailable files cannot become an empty upload", async () => {
  await expect(prepareDraftImages([{ kind: "missing", fileId: "lost", fileName: "lost.png" }]))
    .rejects.toThrow("Select lost.png again before submitting.")
})

test("importing a pending replacement preserves its original request bytes and key", async () => {
  const original = {
    bkey: "previous-image",
    remove: false,
    upload: { fileName: "replacement.png", contentType: "image/png", content: "AP8q" },
  }
  const image = restoreDraftImage(original)

  expect(image).toMatchObject({ kind: "local", fileId: "imported-image", replaces: "previous-image" })
  expect(await prepareDraftImages([image])).toEqual([original])
})

test("old file descriptors preserve bytes when present and expose missing bytes explicitly", () => {
  const file = new File(["saved"], "saved.png", { type: "image/png" })
  const descriptor = { fileId: "saved", fileName: file.name, contentType: file.type }

  expect(restoreDraftImage({ ...descriptor, file })).toEqual({ kind: "local", fileId: "saved", file })
  expect(restoreDraftImage(descriptor)).toEqual({ kind: "missing", fileId: "saved", fileName: "saved.png" })
})
