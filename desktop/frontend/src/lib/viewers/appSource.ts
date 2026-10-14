import { api, fileURL, previewURL } from "../api";
import type { FileSource } from "$shared/viewers/source";

// The App's source for the shared viewers (src/shared/viewers): the
// project's files through the local file server, text read and compared in
// Go. fresh changes when the folder may have changed: now's files are read
// again (versions never change).
export function appSource(root: string, fresh: number | string): FileSource {
  const now = (url: string, version: string) => (version ? url : `${url}&t=${fresh}`);
  const opened = (url: string) => Promise.resolve({ url, size: 0, type: "", close() {} });
  return {
    open: (path, version) => opened(now(fileURL(root, path, version), version)),
    picture: (path, version) => opened(now(previewURL(root, path, version), version)),
    urlOf: (path, version) => now(fileURL(root, path, version), version),
    text: (path, version) => api.TextFile(root, path, version).then((c) => c!),
    diff: (file, from, to, whole, fromFile) => api.TextDiff(root, file, from, to, whole, fromFile).then((d) => d!),
  };
}
