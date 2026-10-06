<script lang="ts">
  import { api } from "./api";
  import Modal from "./Modal.svelte";
  import { languages, chosenLanguage, setLanguage, t } from "./i18n.svelte";

  // R3V's own settings (not a project's or a team's): language, starting
  // with Windows, updates, where downloads go, and help.
  let { version, edition, autostart, autoUpdate, downloadDir, update, checking, onautostart, onautoupdate, oncheck,
    ondownloaddir, onupdate, onclose }: {
    version: string;
    edition: string;
    autostart: boolean;
    autoUpdate: boolean;
    downloadDir: string;
    update: { version: string } | null; // a newer release
    checking: boolean;
    onautostart: (on: boolean) => void;
    onautoupdate: (on: boolean) => void;
    oncheck: () => void;
    ondownloaddir: () => void;
    onupdate: (u: Awaited<ReturnType<typeof api.SetChannel>>) => void; // after a channel switch
    onclose: () => void;
  } = $props();

  let lang = $state(chosenLanguage());
  async function pickLanguage(code: string) {
    lang = code;
    await setLanguage(code);
  }

  // Where updates come from: Stable (tested releases), or Nightly (main as
  // it is, with what's still in testing: Unity, Unreal…).
  let channel = $state<{ build: string; chosen: string } | null>(null);
  let switching = $state(false);
  let channelError = $state("");
  $effect(() => { api.Channel().then((c) => (channel = c)).catch(() => {}); });
  async function pickChannel(ch: string) {
    if (!channel || ch === channel.chosen) return;
    switching = true;
    channelError = "";
    try {
      const u = await api.SetChannel(ch);
      channel = { ...channel, chosen: ch };
      onupdate(u);
    } catch (e) {
      channelError = String((e as Error)?.message ?? e);
    } finally {
      switching = false;
    }
  }

  const repo = "https://github.com/nonlabhq/R3V";
  // A new issue with what helps to look into it filled in (nothing about the
  // user or their projects).
  function reportIssue() {
    const body = [
      `**R3V** ${version}${edition ? ` ${edition}` : ""}`,
      `**System** ${navigator.userAgent.match(/Windows NT [\d.]+|Mac OS X [\d_]+/)?.[0] ?? navigator.platform}`,
      "",
      "**What happened**",
      "",
      "**What you expected**",
      "",
      "**Steps to see it again**",
      "1. ",
      "",
      "**Log** (Settings › Help › Log files: attach r3v.log; keys and connection codes are left out of it)",
    ].join("\n");
    api.OpenURL(`${repo}/issues/new?body=${encodeURIComponent(body)}`);
  }
</script>

<Modal title={t("R3V settings")} {onclose} width={560}>
  <section>
    <h3>{t("General")}</h3>
    <div class="line">
      <span class="label">{t("Language")}</span>
      <select value={lang} onchange={(e) => pickLanguage((e.currentTarget as HTMLSelectElement).value)}>
        <option value="">{t("Same as the system")}</option>
        {#each languages as l (l.code)}<option value={l.code}>{l.name}</option>{/each}
      </select>
    </div>
    <label class="check">
      <input type="checkbox" checked={autostart} onchange={(e) => onautostart(e.currentTarget.checked)} />
      <span>{t("Start with Windows")}
        <span class="hint">{t("Keeps R3V in the tray, so you hear about new versions from your team.")}</span></span>
    </label>
    <div class="line">
      <span class="label">{t("Downloads go to")}</span>
      <span class="mono path" title={downloadDir}>{downloadDir || t("a folder you choose")}</span>
      <button class="small" onclick={ondownloaddir}>{t("Change…")}</button>
    </div>
  </section>

  <section>
    <h3>{t("Updates")}</h3>
    <div class="line">
      <span class="label">{t("Version")}</span>
      <span>R3V{edition ? ` ${edition}` : ""} {version}</span>
      <span class="spacer"></span>
      {#if update}
        <span class="new">{t("{version} is available", { version: update.version })}</span>
      {/if}
      <button class="small" onclick={oncheck} disabled={checking}>{checking ? t("Checking…") : t("Check for updates")}</button>
    </div>
    {#if channel}
      <div class="line">
        <span class="label">{t("Channel")}</span>
        <div class="seg" role="radiogroup" aria-label={t("Channel")}>
          {#each ["stable", "nightly"] as ch}
            <button role="radio" aria-checked={channel.chosen === ch} class:on={channel.chosen === ch} disabled={switching}
              onclick={() => pickChannel(ch)}>{ch === "stable" ? t("Stable") : t("Nightly")}</button>
          {/each}
        </div>
      </div>
      <p class="hint">
        {#if channel.chosen === "stable"}
          {t("Tested releases, for Ableton Live projects.")}
          {#if channel.build === "nightly"}{t("This Nightly stays until a Stable release is newer than it.")}{/if}
        {:else}
          {t("The newest build, with what is still in testing: Unity, Unreal, code and design projects. Expect rough edges.")}
          {#if channel.build === "stable"}{switching ? t("Looking for the latest Nightly…") : t("The latest Nightly is offered as an update.")}{/if}
        {/if}
      </p>
      {#if channelError}<p class="hint err">{channelError}</p>{/if}
    {/if}
    <label class="check">
      <input type="checkbox" checked={autoUpdate} onchange={(e) => onautoupdate(e.currentTarget.checked)} />
      <span>{t("Install updates automatically")}
        <span class="hint">{t("When R3V is in the tray and idle, or when it quits.")}</span></span>
    </label>
  </section>

  <section>
    <h3>{t("Help")}</h3>
    <div class="links">
      <button onclick={() => api.OpenURL(repo)}>GitHub ↗</button>
      <button onclick={() => api.OpenURL(`${repo}#getting-started`)}>{t("Guide")} ↗</button>
      <button onclick={() => api.OpenURL(`${repo}/releases`)}>{t("What's new")} ↗</button>
      <button onclick={reportIssue}>{t("Report an issue")} ↗</button>
      <button onclick={() => api.ShowLogFolder()} title={t("What R3V did, to attach to a report. Keys and connection codes are left out.")}>{t("Log files")}</button>
    </div>
  </section>

  {#snippet footer()}
    <button class="primary" onclick={onclose}>{t("Done")}</button>
  {/snippet}
</Modal>

<style>
  section { padding: var(--sp-12) 0; border-bottom: var(--border-width) solid var(--line); }
  section:last-of-type { border-bottom: 0; }
  h3 { margin: 0 0 var(--sp-10); font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: .08em; color: var(--faint); }
  .line { display: flex; align-items: center; gap: var(--sp-10); margin: var(--sp-8) 0; min-height: 28px; }
  .label { width: 130px; flex: none; color: var(--muted); font-size: var(--fs-md); }
  .path { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .spacer { flex: 1; }
  .new { color: var(--accent); font-size: var(--fs-md); }
  select { background: var(--panel); color: var(--text); border: var(--border-width) solid var(--line); border-radius: var(--radius);
    padding: var(--sp-4) var(--sp-8); min-width: 200px; }
  label.check { display: flex; align-items: flex-start; gap: var(--sp-10); margin: var(--sp-10) 0; color: var(--text); font-size: var(--fs-base);
    cursor: pointer; }
  label.check input { margin-top: var(--sp-4); width: auto; flex: none; }
  label.check > span { flex: 1; }
  .hint { display: block; color: var(--faint); font-size: var(--fs-sm); margin-top: var(--sp-2); }
  .links { display: flex; flex-wrap: wrap; gap: var(--sp-8); }
  button.small { padding: var(--sp-4) var(--sp-10); font-size: var(--fs-md); }
  .seg { display: flex; }
  .seg button { padding: var(--sp-4) var(--sp-12); font-size: var(--fs-md); border-radius: 0; }
  .seg button:first-child { border-radius: var(--radius) 0 0 var(--radius); }
  .seg button:last-child { border-radius: 0 var(--radius) var(--radius) 0; margin-left: -1px; }
  .seg button.on { background: var(--accent); color: var(--accent-ink); border-color: var(--accent); }
  p.hint { margin: var(--sp-4) 0 var(--sp-10); }
  .err { color: var(--danger); }
</style>
