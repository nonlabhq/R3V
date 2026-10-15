<script lang="ts">
  import { t } from "./i18n";

  // A project's say on one workflow setting (docs/design/team-settings.md):
  // as the team's (it shows which), on, or off for this project.
  let { label, hint = "", team, own, disabled = false, onchange }: {
    label: string;
    hint?: string;
    team: boolean;         // the team's value
    own: boolean | null;   // the project's (null: as the team)
    disabled?: boolean;
    onchange: (v: boolean | null) => void;
  } = $props();

  const value = $derived(own === null ? "team" : own ? "on" : "off");
</script>

<div class="choice">
  <div class="text">
    <strong>{label}</strong>
    {#if own !== null}<span class="own">{t("Set for this project")}</span>{/if}
    {#if hint}<p class="hint">{hint}</p>{/if}
  </div>
  <div class="pick">
    <select aria-label={label} {disabled} value={value}
      onchange={(e) => { const v = e.currentTarget.value; onchange(v === "team" ? null : v === "on"); }}>
      <option value="team">{team ? t("As the team: on") : t("As the team: off")}</option>
      <option value="on">{t("On for this project")}</option>
      <option value="off">{t("Off for this project")}</option>
    </select>
    {#if own !== null && !disabled}
      <button class="ghost reset" onclick={() => onchange(null)}>{t("Use the team's")}</button>
    {/if}
  </div>
</div>

<style>
  .choice { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--sp-12); padding: var(--sp-8) 0; }
  .text { min-width: 0; }
  .text strong { font-weight: var(--fw-medium); }
  .own { margin-left: var(--sp-6); font-size: var(--fs-xs); color: var(--accent-text); }
  .hint { margin: var(--sp-2) 0 0; font-size: var(--fs-sm); color: var(--faint); }
  .pick { flex: none; display: flex; flex-direction: column; align-items: flex-end; gap: var(--sp-2); }
  .pick select { min-width: 180px; }
  .reset { padding: 0 var(--sp-4); font-size: var(--fs-xs); }
</style>
