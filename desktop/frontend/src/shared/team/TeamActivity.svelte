<script lang="ts">
  import { t } from "../i18n";
  import { ago, errorText } from "../text";
  import { toolName } from "../tools";
  import type { ActivityEntry as ActivityItem, TeamSource } from "./source";

  // A team's Activity tab (docs/design/team-settings.md): what happened,
  // newest first, by day; filtered by project, by each member or each agent,
  // and how far back. A version's line opens it in its project's graph.
  // latest: only the newest few, unfiltered (a team's home).
  let { source, onopen, latest = 0 }: { source: TeamSource; onopen?: (project: string, version: string) => void; latest?: number } = $props();

  let days = $state(30);
  let items = $state<ActivityItem[] | null>(null);
  let error = $state("");
  $effect(() => {
    const src = source, d = days;
    items = null;
    error = "";
    src.activity(d).then((v) => { if (source === src && days === d) items = v ?? []; })
      .catch((e) => { if (source === src) error = errorText(e); });
  });

  // Who: everyone, people, agents, a member (m:<id>), an agent (a:<tool>).
  let who = $state("");
  let project = $state("");
  let projects = $derived([...new Map((items ?? []).filter((i) => i.project).map((i) => [i.project, i.projectName])).entries()]);
  let members = $derived([...new Map((items ?? []).filter((i) => i.by && !i.tool).map((i) => [i.by, i.byName || i.by])).entries()]);
  let agents = $derived([...new Set((items ?? []).map((i) => i.tool).filter(Boolean))]);
  let shown = $derived((items ?? []).filter((i) => (!project || i.project === project) && (
    !who || (who === "people" && !i.tool) || (who === "agents" && !!i.tool)
    || (who.startsWith("m:") && !i.tool && i.by === who.slice(2)) || (who.startsWith("a:") && i.tool === who.slice(2)))));

  // By day, as the computer's calendar has them.
  const dayOf = (iso: string) => new Date(iso).toLocaleDateString(undefined, { weekday: "short", month: "short", day: "numeric" });
  let byDay = $derived.by(() => {
    const out: { day: string; items: ActivityItem[] }[] = [];
    for (const it of shown) {
      const d = dayOf(it.time);
      if (out.at(-1)?.day !== d) out.push({ day: d, items: [] });
      out.at(-1)!.items.push(it);
    }
    return out;
  });

  function whoText(i: ActivityItem): string {
    const person = i.byName || t("Someone");
    return i.tool ? t("{agent} (for {who})", { agent: toolName(i.tool), who: person }) : person;
  }
  function text(i: ActivityItem): string {
    const who = whoText(i), version = i.title || t("(no description)");
    switch (i.kind) {
      case "share": return t("{who} shared “{version}”", { who, version });
      case "merge": return t("{who} merged “{version}”", { who, version });
      case "branch-made": return t("{who} made the branch {branch}", { who, branch: i.branch });
      case "branch-deleted": return t("{who} deleted the branch {branch}", { who, branch: i.branch });
      case "project-added": return t("{who} added the project {project}", { who, project: i.projectName });
      case "project-removed": return t("{who} removed the project {project}", { who, project: i.projectName });
      case "member-joined": return t("{who} joined the team", { who });
    }
    return i.kind;
  }
  // Where (the project, the branch) and when.
  function meta(i: ActivityItem): string {
    const where = i.projectName && !i.kind.startsWith("project") ? [i.projectName] : [];
    if (where.length && i.branch && (i.kind === "share" || i.kind === "merge")) where.push(i.branch);
    return [...where, ago(i.time)].join(" · ");
  }
  const opens = (i: ActivityItem) => !!onopen && !!i.version && (i.kind === "share" || i.kind === "merge" || i.kind === "branch-made");
</script>

{#snippet line(i: ActivityItem)}
  <li class={i.kind} class:agent={!!i.tool}>
    {#if opens(i)}
      <button class="line" onclick={() => onopen!(i.project, i.version)}>{text(i)}</button>
    {:else}
      <span class="line">{text(i)}</span>
    {/if}
    <span class="meta">{meta(i)}</span>
  </li>
{/snippet}

<section>
  {#if !latest}
  <div class="filters">
    <select aria-label={t("Project")} bind:value={project}>
      <option value="">{t("All projects")}</option>
      {#each projects as [id, name] (id)}<option value={id}>{name}</option>{/each}
    </select>
    <select aria-label={t("Who")} bind:value={who}>
      <option value="">{t("Everyone")}</option>
      <option value="people">{t("People")}</option>
      <option value="agents">{t("Agents")}</option>
      {#if members.length}<optgroup label={t("Members")}>
        {#each members as [id, name] (id)}<option value={"m:" + id}>{name}</option>{/each}
      </optgroup>{/if}
      {#if agents.length}<optgroup label={t("Agents")}>
        {#each agents as tool (tool)}<option value={"a:" + tool}>{toolName(tool)}</option>{/each}
      </optgroup>{/if}
    </select>
    <select aria-label={t("Date range")} bind:value={days}>
      <option value={7}>{t("Last 7 days")}</option>
      <option value={30}>{t("Last 30 days")}</option>
      <option value={90}>{t("Last 90 days")}</option>
    </select>
  </div>
  {/if}

  {#if error}
    <p class="error">{error}</p>
  {:else if items === null}
    <p class="faint">{t("Reading…")}</p>
  {:else if !shown.length}
    <p class="faint">{t("Nothing happened in this time.")}</p>
  {:else if latest}
    <ul>
      {#each shown.slice(0, latest) as i, n (n + i.time)}{@render line(i)}{/each}
    </ul>
  {:else}
    {#each byDay as d (d.day)}
      <h4>{d.day}</h4>
      <ul>
        {#each d.items as i, n (n + i.time)}{@render line(i)}{/each}
      </ul>
    {/each}
  {/if}
</section>

<style>
  .filters { display: flex; flex-wrap: wrap; gap: var(--sp-8); margin-bottom: var(--sp-12); }
  .filters select { min-width: 150px; }
  h4 { margin: var(--sp-14) 0 var(--sp-6); font-size: var(--fs-sm); color: var(--muted); font-weight: var(--fw-semibold); }
  ul { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: var(--sp-2); }
  li { display: flex; align-items: baseline; justify-content: space-between; gap: var(--sp-12); padding: var(--sp-4) var(--sp-6);
    border-radius: var(--radius); font-size: var(--fs-md); }
  li:hover { background: var(--hover-soft); }
  .line { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; text-align: left; }
  button.line { border: none; background: none; padding: 0; color: var(--text); font: inherit; cursor: pointer; }
  button.line:hover:not(:disabled) { background: none; color: var(--accent-text); }
  li.agent .line { color: var(--info-text); }
  li.merge .line { font-weight: var(--fw-medium); }
  .meta { flex: none; font-size: var(--fs-xs); color: var(--faint); }
  .error { color: var(--danger); }
</style>
