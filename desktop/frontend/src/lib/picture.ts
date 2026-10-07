// A picture someone picks, made into what a team keeps: square (the middle
// of it), 128 pixels, as small as it looks good (PNG, else JPEG), under
// remote.MaxPictureSize.

export const SIDE = 128;
const MAX_BYTES = 60 * 1024;

const bytesOf = (url: string) => Math.ceil(((url.length - url.indexOf(",") - 1) * 3) / 4);

export async function squarePicture(file: Blob): Promise<string> {
  const img = await createImageBitmap(file);
  const side = Math.min(img.width, img.height);
  const canvas = document.createElement("canvas");
  canvas.width = canvas.height = SIDE;
  const g = canvas.getContext("2d")!;
  g.imageSmoothingQuality = "high";
  g.drawImage(img, (img.width - side) / 2, (img.height - side) / 2, side, side, 0, 0, SIDE, SIDE);
  img.close();
  let url = canvas.toDataURL("image/png");
  for (let q = 0.9; bytesOf(url) > MAX_BYTES && q > 0.3; q -= 0.15) url = canvas.toDataURL("image/jpeg", q);
  if (bytesOf(url) > MAX_BYTES) throw new Error("This picture can't be made small enough.");
  return url;
}
