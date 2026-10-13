<script lang="ts">
  import type { TeamSummary } from "./api";
  import ProjectIcon from "./ProjectIcon.svelte";

  // A team's mark, the way a project's icon shows (ProjectIcon): its own
  // picture, else its icon, else its initial, on its colour (the team's
  // pick, else one picked from its id).
  let { team, size = 32 }: { team: Pick<TeamSummary, "id" | "name"> & Partial<Pick<TeamSummary, "icon" | "color" | "picture">>; size?: number } = $props();
</script>

<ProjectIcon p={{ id: team.id, name: team.name, status: "downloaded", icon: team.icon, color: team.color, picture: team.picture }} {size} />
