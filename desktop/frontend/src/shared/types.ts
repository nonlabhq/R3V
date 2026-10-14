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
 * authorId: the member (looks are by it); actor: the AI agent that made it. */
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

/** A preset in use for a folder ("" is the project folder). */
export interface AppliedPreset {
  folder: string;
  preset: string;
}
