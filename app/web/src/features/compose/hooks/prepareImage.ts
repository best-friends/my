// SPDX-License-Identifier: AGPL-3.0-only
// Prepares a picked image for upload in the browser: it applies the EXIF orientation, scales the image
// down to the largest size the server keeps, and re-encodes it, which leaves every metadata block (EXIF
// with the location, XMP, comments) behind. The server re-encodes again (the uploadMedia operation), so
// this is about not sending the location at all and about sending fewer bytes.
//
// It uses createImageBitmap with imageOrientation "from-image" and OffscreenCanvas.convertToBlob
// (MDN; every browser the client supports has both).

/** The largest area the server keeps (MediaVariants.full in api/openapi.yaml): 3840 × 2160 pixels. */
const maxPixels = 3840 * 2160;

/** The JPEG and WebP quality of the re-encoded image. */
const quality = 0.9;

/** The size an image of `width` × `height` is scaled down to, keeping its aspect ratio. */
export function fitWithin(width: number, height: number, pixels = maxPixels): { width: number; height: number } {
  const scale = Math.min(1, Math.sqrt(pixels / (width * height)));
  return { width: Math.max(1, Math.floor(width * scale)), height: Math.max(1, Math.floor(height * scale)) };
}

/** The file name with the extension of `type`. */
function renamed(name: string, type: string): string {
  const extension = type === "image/png" ? "png" : type === "image/webp" ? "webp" : "jpg";
  const dot = name.lastIndexOf(".");
  return `${dot > 0 ? name.slice(0, dot) : name}.${extension}`;
}

/**
 * Returns the image to upload. A GIF is sent as it is, so that it keeps its animation (GIF has no EXIF;
 * the server removes the rest). PNG and WebP stay in their format (transparency); everything else
 * becomes JPEG. A browser that cannot encode the format returns PNG, which the server accepts too.
 */
export async function prepareImage(file: File): Promise<File> {
  if (file.type === "image/gif") return file;
  const bitmap = await createImageBitmap(file, { imageOrientation: "from-image" });
  try {
    const size = fitWithin(bitmap.width, bitmap.height);
    const canvas = new OffscreenCanvas(size.width, size.height);
    const context = canvas.getContext("2d");
    if (context === null) throw new Error("no 2D context for an OffscreenCanvas");
    context.drawImage(bitmap, 0, 0, size.width, size.height);
    const type = file.type === "image/png" || file.type === "image/webp" ? file.type : "image/jpeg";
    const blob = await canvas.convertToBlob({ type, quality });
    return new File([blob], renamed(file.name, blob.type), { type: blob.type });
  } finally {
    bitmap.close();
  }
}
