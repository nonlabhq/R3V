// What the shared components are given: plain shapes, filled from the
// App's bindings or from R3V Cloud's HTTP API. An interface between the
// two: add fields, never rename or remove one.

/** A project's look, as its icon shows it. status: "remote" (on the team
 * only), "missing" (its folder gone), anything else shows no mark. */
export interface ProjectLook {
  id?: string;
  name: string;
  status: string;
  icon?: string;
  color?: string;
  picture?: string;
}

/** A team's look, as its mark shows it. */
export interface TeamLook {
  id: string;
  name: string;
  icon?: string;
  color?: string;
  picture?: string;
}

/** How a member shows: a palette name, a picture (a data: or https: URL). */
export interface MemberLook {
  color: string;
  picture: string;
}

/** A version as the graph draws it. branch: the branch it was made on;
 * authorId: the member (looks are by it); actor: the AI agent that made it
 * (none: a person did). What the graph's segments use, where the data has
 * it: files: how many files it changed; check: its checks' result ("pass",
 * "fail"; none: not checked; a checked version is never in a segment);
 * step: the agent's step it was made in (a segment never spans two). */
export interface GraphVersion {
  id: string;
  short: string;
  author: string;
  authorId: string;
  time: string; // RFC 3339
  message: string;
  parents: string[];
  branch: string;
  actor?: { tool: string; model: string } | null;
  files?: number;
  check?: string;
  step?: string;
}

/** A branch as the graph draws it. latest: its newest version's id ("" with
 * none yet); label, color: the team's; parent: the branch it came from;
 * kind: "agent" for an AI agent's. */
export interface GraphBranch {
  name: string;
  latest: string;
  label?: string;
  color?: string;
  parent?: string;
  kind?: string;
}

/** A version the team named (a milestone). */
export interface GraphMilestone {
  version: string;
  name: string;
}

/** A file a version changed. status: added, modified, deleted, renamed
 * (from: where it was), untracked; kind: what R3V knows it as ("set",
 * "audio"…; "" when nothing special). */
export interface ChangedFile {
  path: string;
  status: string;
  from?: string;
  kind?: string;
}

/** A file as the Files explorer lists it. status: unchanged, added,
 * modified, deleted, renamed, ignored…; kind: what R3V knows it as;
 * preview: it can be shown as a picture; modified: when it last changed
 * (a thumbnail's address changes with it). */
export interface BrowserFile {
  path: string;
  status: string;
  size: number;
  kind?: string;
  preview?: boolean;
  modified?: string;
}

/** The version that last changed a file. */
export interface LastChange {
  message: string;
  author: string;
  authorId: string;
  time: string;
}

/** A preset in use for a folder ("" is the project folder). */
export interface AppliedPreset {
  folder: string;
  preset: string;
}
