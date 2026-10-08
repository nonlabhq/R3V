import { t } from "./i18n.svelte";

// Kinds of file by their extension, for finding them (the Files tab's
// filter), naming them (its Type column) and drawing them (FileIcon). A
// file in none of these is listed as it is, with no kind.
export type FileType = "project" | "audio" | "midi" | "image" | "video" | "model" | "code" | "data" | "doc" | "font" | "archive";

// The order they're offered in.
export const FILE_TYPES: FileType[] = ["project", "audio", "midi", "image", "video", "model", "code", "data", "doc", "font", "archive"];

const exts: Record<FileType, string> = {
  // a program's own project, set or scene
  project: "als alp flp rpp ptx cpr npr song reason bwproject logicx unity uproject umap godot tscn blend aep prproj drp",
  audio: "wav aif aiff flac mp3 ogg m4a aac opus wma",
  midi: "mid midi",
  image: "png jpg jpeg gif webp bmp tga tif tiff psd psb exr hdr kra xcf svg ico heic afphoto afdesign",
  video: "mp4 mov mkv avi webm m4v wmv mpg mpeg",
  model: "fbx obj gltf glb usd usda usdc usdz abc dae 3ds max ma mb c4d stl ply hip hipnc",
  code: "py js mjs cjs ts tsx jsx cs cpp cc c h hpp gd lua shader hlsl glsl cginc compute rs go java kt swift sh ps1 bat vex",
  data: "json yaml yml xml csv tsv ini toml cfg plist",
  doc: "txt md pdf doc docx rtf odt pages",
  font: "ttf otf woff woff2",
  archive: "zip rar 7z tar gz tgz",
};
const byExt = new Map<string, FileType>(FILE_TYPES.flatMap((k) => exts[k].split(" ").map((e) => [e, k] as const)));

/** A file's extension, lower case ("" for none). */
export function extOf(path: string): string {
  const name = path.slice(path.lastIndexOf("/") + 1);
  const dot = name.lastIndexOf(".");
  return dot > 0 ? name.slice(dot + 1).toLowerCase() : "";
}

/** The kind of a file by its extension ("" when it's none of them). */
export const typeOf = (path: string): FileType | "" => byExt.get(extOf(path)) ?? "";

/** A kind in words. */
export function typeTitle(k: FileType): string {
  return ({
    project: t("Project"), audio: t("Audio"), midi: "MIDI", image: t("Image"), video: t("Video"), model: t("3D"), code: t("Code"),
    data: t("Data & Config"), doc: t("Documents"), font: t("Fonts"), archive: t("Archives"),
  } as Record<FileType, string>)[k];
}
