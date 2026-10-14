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

/** A preset in use for a folder ("" is the project folder). */
export interface AppliedPreset {
  folder: string;
  preset: string;
}
