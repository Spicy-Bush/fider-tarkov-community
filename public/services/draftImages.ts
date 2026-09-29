import { ImageUpload } from "@fider/models"
import { fileToBase64 } from "./utils"
import { newSubmissionID } from "./postSubmission"

export type DraftImage =
  | { kind: "stored"; bkey: string }
  | { kind: "local"; fileId: string; file: File; replaces?: string }
  | { kind: "missing"; fileId: string; fileName: string }
  | { kind: "removed"; bkey: string }

export type SavedDraftImage =
  | ImageUpload
  | {
      fileId: string
      fileName: string
      contentType: string
      file?: File
    }

export function restoreDraftImage(image: SavedDraftImage): DraftImage {
  if ("fileId" in image) {
    return image.file
      ? { kind: "local", fileId: image.fileId, file: image.file }
      : { kind: "missing", fileId: image.fileId, fileName: image.fileName }
  }

  if (image.bkey && (image.remove || !image.upload)) {
    return { kind: image.remove ? "removed" : "stored", bkey: image.bkey }
  }

  const upload = image.upload
  if (!upload?.content) {
    throw new Error("The saved image has no file data.")
  }

  const bytes = Uint8Array.from(atob(upload.content), character => character.charCodeAt(0))
  const file = new File([bytes], upload.fileName || "", { type: upload.contentType || "" })

  return { kind: "local", fileId: newSubmissionID(), file, ...(image.bkey && { replaces: image.bkey }) }
}

export async function prepareDraftImages(images: DraftImage[]): Promise<ImageUpload[]> {
  return Promise.all(images.map(async image => {
    switch (image.kind) {
      case "stored":
        return { bkey: image.bkey, remove: false }
      case "removed":
        return { bkey: image.bkey, remove: true }
      case "missing":
        throw new Error(`Select ${image.fileName} again before submitting.`)
      case "local":
        return {
          remove: false,
          ...(image.replaces && { bkey: image.replaces }),
          upload: { fileName: image.file.name, contentType: image.file.type, content: await fileToBase64(image.file) },
        }
    }
  }))
}
