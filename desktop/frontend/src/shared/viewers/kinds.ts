// Which shared viewer shows a file, by its name (the App decides from what
// R3V knows of the file; this is for a page with only its path).
export type ViewerKind = "text" | "markdown" | "image" | "video" | "model";

export function viewerKind(path: string): ViewerKind {
  const p = path.toLowerCase();
  if (/\.(md|markdown)$/.test(p)) return "markdown";
  if (/\.(png|jpe?g|gif|webp|avif|bmp)$/.test(p)) return "image";
  if (/\.(mp4|webm|mov|m4v)$/.test(p)) return "video";
  if (/\.(glb|gltf|fbx|obj|stl|ply)$/.test(p)) return "model";
  return "text";
}
