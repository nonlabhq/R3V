<script lang="ts">
  import { t } from "./i18n.svelte";
  import { api, errorText, type TeamSummary, type CloudPeople, type CloudMember } from "./api";
  import { toast } from "./notify.svelte";
  import ConfirmDialog from "./ConfirmDialog.svelte";

  // A hosted team's people (R3V-Cloud): who is in it and with what role,
  // inviting someone by email, pending invitations, and which projects a
  // collaborator reaches. Owners and admins change things; everyone sees.
  let { team }: { team: TeamSummary } = $props();

  let people = $state<CloudPeople | null>(null);
  let loadError = $state("");
  let busy = $state(false);

  async function load() {
    try {
      people = await api.CloudPeople(team.id);
      loadError = "";
    } catch (e) {
      loadError = errorText(e);
    }
  }
  $effect(() => {
    team.id;
    load();
  });

  let canManage = $derived(people?.myRole === "owner" || people?.myRole === "admin");
  const roleName = (r: string) =>
    ({ owner: t("Owner"), admin: t("Admin"), member: t("Member"), collaborator: t("Collaborator") } as Record<string, string>)[r] ?? r;
  // Admins manage people up to admin; owners anyone.
  const roles = $derived(people?.myRole === "owner" ? ["owner", "admin", "member", "collaborator"] : ["admin", "member", "collaborator"]);

  async function act(f: () => Promise<unknown>, done = "") {
    busy = true;
    try {
      await f();
      await load();
      if (done) toast(done, "ok");
    } catch (e) {
      toast(errorText(e), "error", 9000);
      await load();
    } finally {
      busy = false;
    }
  }

  // Inviting.
  let email = $state("");
  let role = $state("member");
  let access = $state<Record<string, string>>({}); // project id -> "", "read", "write"
  let inviteError = $state("");
  async function invite() {
    inviteError = "";
    const projects = Object.fromEntries(Object.entries(access).filter(([, a]) => a));
    busy = true;
    try {
      await api.CloudInvite(team.id, email.trim(), role, projects);
      toast(t("Invitation sent to {email}", { email: email.trim() }), "ok");
      email = "";
      access = {};
      await load();
    } catch (e) {
      inviteError = errorText(e);
    } finally {
      busy = false;
    }
  }

  let removing = $state<CloudMember | null>(null);
  const me = (m: CloudMember) => m.userId === people?.myUserId;
  const mayChange = (m: CloudMember) => canManage && !me(m) && (people?.myRole === "owner" || m.role !== "owner");
</script>

<section>
  <h3>{t("People")}</h3>
  {#if loadError}
    <p class="error small">{loadError}</p>
  {:else if !people}
    <p class="faint small">{t("Loading…")}</p>
  {:else}
    <ul class="people">
      {#each people.members as m (m.userId)}
        <li>
          <div class="row who">
            <span class="name">{m.name || m.email}{#if me(m)} <span class="faint">({t("you")})</span>{/if}</span>
            {#if m.name}<span class="faint small mail">{m.email}</span>{/if}
            <span class="spacer"></span>
            {#if mayChange(m)}
              <select aria-label={t("Role")} value={m.role} disabled={busy}
                onchange={(e) => act(() => api.CloudSetRole(team.id, m.userId, e.currentTarget.value))}>
                {#each roles as r}<option value={r}>{roleName(r)}</option>{/each}
              </select>
              <button class="ghost small danger-text" disabled={busy} onclick={() => (removing = m)}>{t("Remove")}</button>
            {:else}
              <span class="role">{roleName(m.role)}</span>
            {/if}
          </div>
          {#if m.role === "collaborator"}
            <div class="projects">
              {#each people.projects as p (p.id)}
                {@const a = m.projects?.[p.id] ?? ""}
                {#if canManage}
                  <label class="row proj">
                    <span>{p.name}</span><span class="spacer"></span>
                    <select value={a} disabled={busy} aria-label={t("Access to {project}", { project: p.name })}
                      onchange={(e) => act(() => api.CloudSetAccess(team.id, p.id, m.userId, e.currentTarget.value))}>
                      <option value="">{t("No access")}</option>
                      <option value="read">{t("Can view")}</option>
                      <option value="write">{t("Can edit")}</option>
                    </select>
                  </label>
                {:else if a}
                  <span class="faint small chip">{p.name} · {a === "write" ? t("Can edit") : t("Can view")}</span>
                {/if}
              {/each}
            </div>
          {/if}
        </li>
      {/each}
    </ul>

    {#if canManage}
      {#if people.invitations.length}
        <h4>{t("Invited")}</h4>
        <ul class="people">
          {#each people.invitations as i (i.id)}
            <li class="row who">
              <span>{i.email}</span><span class="faint small">{roleName(i.role)}</span><span class="spacer"></span>
              <button class="ghost small" disabled={busy}
                onclick={() => act(() => api.CloudWithdraw(team.id, i.id), t("Invitation withdrawn"))}>{t("Withdraw")}</button>
            </li>
          {/each}
        </ul>
      {/if}

      <h4>{t("Invite someone")}</h4>
      <form class="invite" onsubmit={(e) => { e.preventDefault(); invite(); }}>
        <div class="row">
          <input type="email" bind:value={email} placeholder={t("Their email")} aria-label={t("Their email")} />
          <select bind:value={role} aria-label={t("Role")}>
            {#each roles.filter((r) => r !== "owner") as r}<option value={r}>{roleName(r)}</option>{/each}
          </select>
        </div>
        {#if role === "collaborator"}
          <p class="faint small">{t("A collaborator works only on the projects you choose.")}</p>
          {#each people.projects as p (p.id)}
            <label class="row proj">
              <span>{p.name}</span><span class="spacer"></span>
              <select bind:value={access[p.id]} aria-label={t("Access to {project}", { project: p.name })}>
                <option value="">{t("No access")}</option>
                <option value="read">{t("Can view")}</option>
                <option value="write">{t("Can edit")}</option>
              </select>
            </label>
          {:else}
            <p class="faint small">{t("The team has no projects yet: share one first.")}</p>
          {/each}
        {/if}
        {#if inviteError}<p class="error small">{inviteError}</p>{/if}
        <div class="row btns">
          <span class="spacer"></span>
          <button type="submit" class="primary" disabled={busy || !email.trim().includes("@")}>{t("Send invitation")}</button>
        </div>
      </form>
    {/if}
  {/if}
</section>

{#if removing}
  {@const m = removing}
  <ConfirmDialog title={t("Remove {name}?", { name: m.name || m.email })}
    text={t("They can't reach the team or its projects any more. What they downloaded stays on their computer.")}
    confirm={t("Remove")} danger
    onconfirm={() => { removing = null; act(() => api.CloudRemoveMember(team.id, m.userId), t("Removed {name}", { name: m.name || m.email })); }}
    onclose={() => (removing = null)} />
{/if}

<style>
  section { margin-bottom: var(--sp-18); }
  h3 { font-size: var(--fs-sm); text-transform: uppercase; letter-spacing: .06em; color: var(--muted); margin: 0 0 var(--sp-8); }
  h4 { font-size: var(--fs-md); font-weight: var(--fw-semibold); margin: var(--sp-14) 0 var(--sp-6); }
  .people { list-style: none; margin: 0; padding: 0; }
  .people li { padding: var(--sp-6) 0; border-bottom: 1px solid var(--line-soft); }
  .who { gap: var(--sp-8); align-items: center; }
  .name { font-weight: var(--fw-medium); }
  .mail { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .role { color: var(--muted); font-size: var(--fs-md); }
  select { width: auto; font-size: var(--fs-md); }
  .projects { margin: var(--sp-4) 0 0 var(--sp-16); display: flex; flex-direction: column; gap: var(--sp-4); }
  .proj { gap: var(--sp-8); align-items: center; margin: 0; color: var(--text); font-size: var(--fs-md); }
  .chip { display: inline-block; }
  .invite .row { gap: var(--sp-8); }
  .invite input { flex: 1; min-width: 0; }
  .btns { margin-top: var(--sp-8); }
  .small { font-size: var(--fs-md); }
  .error { color: var(--danger); user-select: text; }
  .danger-text { color: var(--danger); }
</style>
