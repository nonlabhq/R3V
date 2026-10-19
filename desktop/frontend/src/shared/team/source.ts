// Where a team's settings pages get their data and do their changes: a
// source per team (as a viewer's file source), the App's through its Go
// bindings, R3V Cloud's web app's through its API
// (docs/design/team-settings.md).

/** A person in an R3V Cloud team. projects: a collaborator's access, by project id ("read", "write"). */
export interface TeamPerson {
  userId: string;
  memberId: string;
  name: string;
  email: string;
  role: string; // owner, admin, member, collaborator
  projects?: Record<string, string | undefined>;
}

export interface TeamInvitation {
  id: string;
  email: string;
  role: string;
  projects?: Record<string, string | undefined>;
  expiresAt: number;
}

/** An R3V Cloud team's people, as the one asking sees them. */
export interface TeamPeopleData {
  myRole: string;
  myUserId: string;
  members: TeamPerson[];
  invitations: TeamInvitation[];
  projects: { id: string; name: string }[];
}

/** What the team says about agents (R3V's defaults: all on). */
export interface AgentDefaults {
  steps: boolean;
  prompts: boolean;
  agentLocks: boolean;
}

/** A team's Workflow tab. locks: file locking can be had (R3V Cloud);
 * locksOn: projects lock unless they say otherwise; kinds: those that lock
 * by themselves; all: the kinds to pick from. overrides: the projects
 * setting some of their own. */
export interface TeamWorkflowData {
  canEdit: boolean;
  /** The agents' defaults mean something here (false: this App has no agents yet; the web leaves it out). */
  agentsHere?: boolean;
  locks: boolean;
  locksOn: boolean;
  kinds: string[];
  all: { id: string; patterns: string[] }[];
  agents: AgentDefaults;
  overrides: { projectId: string; name: string; locks: boolean; agents: boolean }[];
}

/** One thing that happened in the team. kind: share, merge, branch-made,
 * branch-deleted, project-added, project-removed, project-archived,
 * project-unarchived, member-joined. tool and
 * model: the AI agent that made the version, for its member (by). */
export interface ActivityEntry {
  time: string;
  kind: string;
  project: string;
  projectName: string;
  branch: string;
  version: string;
  title: string;
  by: string;
  byName: string;
  tool: string;
  model: string;
}

/** A team's usage: on its own storage counted (counted: when; "" never),
 * on R3V Cloud the service's against the plan (quota 0: none). */
export interface TeamUsageData {
  hosted: boolean;
  counted: string;
  used: number;
  stored: number;
  versions: number;
  quota: number;
  plan: string;
  projects: { id: string; name: string; bytes: number; removed: boolean }[];
}

export type TeamSource = {
  people(): Promise<TeamPeopleData>;
  invite(email: string, role: string, projects: Record<string, string>): Promise<void>;
  withdraw(invitationId: string): Promise<void>;
  setRole(userId: string, role: string): Promise<void>;
  setAccess(projectId: string, userId: string, access: string): Promise<void>;
  remove(userId: string): Promise<void>;

  workflow(): Promise<TeamWorkflowData>;
  setLocks(on: boolean, kinds: string[]): Promise<void>;
  setAgents(w: AgentDefaults): Promise<void>;
  clearOverrides(pick: "locks" | "agents"): Promise<number>;

  activity(days: number): Promise<ActivityEntry[]>;
  usage(): Promise<TeamUsageData>;
  /** Counts again (a team on its own storage); none: the usage is the service's. */
  countUsage?(): Promise<TeamUsageData>;
};
