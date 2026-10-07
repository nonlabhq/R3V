<script lang="ts">
  import { t } from "./i18n.svelte";
  import { api, errorText, type TeamSummary, type Profile } from "./api";

  import Modal from "./Modal.svelte";
  import Avatar from "./Avatar.svelte";
  import Swatches from "./Swatches.svelte";
  import { colorOf, randomColor } from "./palette";
  import { squarePicture } from "./picture";
  import { toast } from "./notify.svelte";

  // The user: their name in the team, and how they show to their teams (a
  // colour for their initial, or a picture), on all their teams at once.
  let { team, onchanged, onclose }: {
    team?: TeamSummary; // the current team
    onchanged: (p: Profile) => void;
    onclose: () => void;
  } = $props();

  let profile = $state<Profile | null>(null);
  let error = $state("");
  let busy = $state(false);
  $effect(() => {
    api.Profile().then((p) => (profile = p)).catch((e) => (error = errorText(e)));
  });

  // svelte-ignore state_referenced_locally
  let name = $state(team?.memberName ?? "");
  let renaming = $state(false);
  async function rename() {
    if (!team) return;
    renaming = true;
    error = "";
    try {
      await api.SetIdentity(team.id, team.memberId, name.trim());
      toast(t("Renamed for everyone in {team}", { team: team.name }), "ok");
      if (profile) onchanged({ ...profile, name: name.trim() });
    } catch (e) {
      error = errorText(e);
    } finally {
      renaming = false;
    }
  }

  async function change(set: () => Promise<Profile>) {
    if (busy) return; // one at a time: the teams keep the last pick
    busy = true;
    error = "";
    try {
      profile = await set();
      onchanged(profile);
      if (profile.notShared.length)
        toast(t("Not shown in {teams} yet: R3V couldn't reach them. Change it again later to share it there.", { teams: profile.notShared.join(", ") }), "info", 8000);
    } catch (e) {
      error = errorText(e);
    } finally {
      busy = false;
    }
  }
  const pickColor = (c: string) => change(() => api.SetProfileColor(c));

  let fileInput = $state<HTMLInputElement>();
  // Bigger photos take long to read, for a 128-pixel picture.
  const MAX_FILE = 40 << 20;
  async function upload(e: Event) {
    const file = (e.currentTarget as HTMLInputElement).files?.[0];
    (e.currentTarget as HTMLInputElement).value = "";
    if (!file || busy) return;
    if (file.size > MAX_FILE) {
      error = t("This picture is too big: pick one under {size} MB.", { size: MAX_FILE >> 20 });
      return;
    }
    let url: string;
    busy = true;
    try {
      url = await squarePicture(file);
    } catch {
      error = t("This file can't be read as a picture.");
      return;
    } finally {
      busy = false;
    }
    change(() => api.SetProfilePicture(url));
  }

  let shown = $derived(profile ? colorOf(profile.color, profile.memberId || profile.name) : "");
</script>

<Modal title={t("User settings")} width={480} {onclose}>
  {#if profile}
    <div class="who">
      <Avatar name={name || profile.name} seed={profile.memberId} color={profile.color} picture={profile.picture} size={72} />
      <div class="pic">
        <button class="small" onclick={() => fileInput?.click()} disabled={busy}>{t("Upload picture…")}</button>
        {#if profile.picture}
          <button class="small ghost" onclick={() => change(() => api.SetProfilePicture(""))} disabled={busy}>{t("Remove picture")}</button>
        {/if}
        <span class="hint">{t("Square, made 128 pixels. Your teams see it on your versions.")}</span>
        <input bind:this={fileInput} type="file" accept="image/png,image/jpeg,image/webp,image/gif" hidden onchange={upload}
          data-testid="picture-file" />
      </div>
    </div>

    {#if team?.memberId}
      <section>
        <h3>{t("Name")}</h3>
        <div class="line">
          <input bind:value={name} maxlength="100" aria-label={t("Name")} onkeydown={(e) => e.key === "Enter" && rename()} />
          <button class="small" onclick={rename} disabled={renaming || !name.trim() || name.trim() === team.memberName}>
            {renaming ? t("Renaming…") : t("Rename")}</button>
        </div>
        <span class="hint">{t("Your name in {team}.", { team: team.name })}</span>
      </section>
    {/if}

    <section>
      <h3>{t("Colour")}</h3>
      <div class="line">
        <Swatches value={shown} label={t("Colour")} disabled={busy} onpick={pickColor} />
        <button class="small ghost" onclick={() => pickColor(randomColor(shown))} disabled={busy} title={t("Pick a colour at random")}>
          {t("Random")}</button>
      </div>
      <span class="hint">{profile.picture ? t("Shown when your picture can't be.") : t("Your initial shows on this colour.")}</span>
    </section>

    {#if team && !team.looks}
      <p class="hint">{t("{team} can't keep pictures and colours yet: your initial shows there.", { team: team.name })}</p>
    {/if}
  {/if}
  {#if error}<p class="hint err" role="alert">{error}</p>{/if}

  {#snippet footer()}
    <button class="primary" onclick={onclose}>{t("Done")}</button>
  {/snippet}
</Modal>

<style>
  .who { display: flex; align-items: center; gap: var(--sp-16); padding-bottom: var(--sp-12); border-bottom: var(--border-width) solid var(--line); }
  .pic { display: flex; flex-wrap: wrap; align-items: center; gap: var(--sp-8); }
  .pic .hint { flex-basis: 100%; }
  section { padding: var(--sp-12) 0; border-bottom: var(--border-width) solid var(--line); }
  section:last-of-type { border-bottom: 0; }
  h3 { margin: 0 0 var(--sp-10); font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: .08em; color: var(--faint); }
  .line { display: flex; align-items: center; gap: var(--sp-10); }
  .line input { flex: 1; }
  .hint { display: block; color: var(--faint); font-size: var(--fs-sm); margin-top: var(--sp-6); }
  p.hint { margin: var(--sp-8) 0 0; }
  .err { color: var(--danger); }
  button.small { padding: var(--sp-4) var(--sp-10); font-size: var(--fs-md); }
</style>
