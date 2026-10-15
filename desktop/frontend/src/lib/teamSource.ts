import { api } from "./api";
import type { TeamSource } from "$shared/team/source";

// The App's source for a team's shared settings pages ($shared/team): its
// Go bindings. countUsage: a team on its own storage (R3V counts it).
export function appTeamSource(teamId: string, hosted: boolean): TeamSource {
  const need = <T,>(v: T | null) => v!;
  return {
    people: () => api.CloudPeople(teamId).then(need),
    invite: (email, role, projects) => api.CloudInvite(teamId, email, role, projects).then(() => {}),
    withdraw: (id) => api.CloudWithdraw(teamId, id).then(() => {}),
    setRole: (user, role) => api.CloudSetRole(teamId, user, role).then(() => {}),
    setAccess: (project, user, access) => api.CloudSetAccess(teamId, project, user, access).then(() => {}),
    remove: (user) => api.CloudRemoveMember(teamId, user).then(() => {}),
    workflow: () => api.TeamWorkflow(teamId).then(need),
    setLocks: (on, kinds) => api.SetTeamLocks(teamId, on, kinds),
    setAgents: (w) => api.SetTeamAgents(teamId, w),
    clearOverrides: (pick) => api.ClearProjectWorkflow(teamId, pick),
    activity: (days) => api.TeamActivity(teamId, days).then((v) => v ?? []),
    usage: () => api.TeamUsage(teamId).then(need),
    countUsage: hosted ? undefined : () => api.CountTeamUsage(teamId).then(need),
  };
}
