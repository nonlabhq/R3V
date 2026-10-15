<script lang="ts">
  import { t } from "./i18n.svelte";
  import { api, type Member, type TeamSummary } from "./api";
  import Avatar from "$shared/Avatar.svelte";

  // The people of a team on its own storage: who has shared in it (the
  // members storage keeps), with their looks. Nothing to manage: anyone with
  // the connection code is in.
  let { team }: { team: TeamSummary } = $props();

  let members = $state<Member[] | null>(null);
  $effect(() => {
    const id = team.id;
    members = null;
    api.TeamMembers(id).then((ms) => { if (team.id === id) members = ms ?? []; }).catch(() => { if (team.id === id) members = []; });
  });
</script>

<section>
  <h3>{t("Members")}</h3>
  {#if members === null}
    <p class="faint small">{t("Reading…")}</p>
  {:else if members.length === 0}
    <p class="faint small">{t("No one has shared in this team yet.")}</p>
  {:else}
    <ul>
      {#each members as m (m.id)}
        <li>
          <Avatar name={m.name} seed={m.id} color={team.looks ? m.color : ""} picture={team.looks ? m.picture : ""} size={24} />
          <span class="name">{m.name}</span>
          {#if m.id === team.memberId}<span class="you">{t("you")}</span>{/if}
        </li>
      {/each}
    </ul>
  {/if}
  <p class="faint small">{t("Anyone with the team's connection code is in it: there are no roles to manage on a team's own storage.")}</p>
</section>

<style>
  section { margin-bottom: var(--sp-18); }
  h3 { font-size: var(--fs-sm); text-transform: uppercase; letter-spacing: .06em; color: var(--muted); margin: 0 0 var(--sp-8); }
  ul { list-style: none; margin: 0 0 var(--sp-8); padding: 0; display: flex; flex-direction: column; gap: var(--sp-6); }
  li { display: flex; align-items: center; gap: var(--sp-8); font-size: var(--fs-base); }
  .name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .you { font-size: var(--fs-xs); color: var(--faint); }
  .small { font-size: var(--fs-md); margin: 0; }
</style>
