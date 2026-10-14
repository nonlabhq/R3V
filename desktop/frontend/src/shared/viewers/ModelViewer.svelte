<script lang="ts">
  import { untrack } from "svelte";
  import { t } from "../i18n";
  import ModelCompare from "./ModelCompare.svelte";
  import { opened, type Opened } from "./opened.svelte";
  import { dirOf, type FileSource, type OpenedFile, type Side, type ViewerProps } from "./source";

  // A 3D model in a viewer; comparing, the other beside it, cameras together.
  let { source, file, a, b, compare }: ViewerProps = $props();

  const extOf = (p: string) => p.slice(p.lastIndexOf(".")).toLowerCase();
  const oa = opened(() => source, () => a);
  const ob = opened(() => source, () => b);

  // The files a model names (a glTF's .bin and textures), from the same
  // version: at once from a source with URLs (the App's); else a glTF's are
  // opened before it loads (a .glb has its own inside).
  type Ready = { label: string; src: string; resolve: (rel: string) => string };
  function prepare(get: () => Side | null, o: Opened) {
    const st = $state<{ take: Ready | null }>({ take: null });
    $effect(() => {
      const f = o.file, src: FileSource = source, s = untrack(get); // (opened again when the side really changes)
      st.take = null;
      if (!s || !f) return;
      const dir = dirOf(s.path);
      if (src.urlOf || extOf(s.path) !== ".gltf") {
        st.take = { label: s.label, src: f.url, resolve: (rel) => src.urlOf?.(dir + rel, s.version) ?? "" };
        return;
      }
      let gone = false;
      const got: OpenedFile[] = [];
      const urls = new Map<string, string>();
      fetch(f.url).then((r) => r.json()).then(async (gltf) => {
        const refs = [...(gltf.buffers ?? []), ...(gltf.images ?? [])].map((x: { uri?: string }) => x.uri)
          .filter((u): u is string => !!u && !/^[a-z][a-z0-9+.-]*:/i.test(u));
        await Promise.all([...new Set(refs)].map(async (u) => {
          let rel = u;
          try { rel = decodeURIComponent(u); } catch { /* as written */ }
          const opened = await src.open(dir + rel, s.version).catch(() => null);
          if (!opened) return;
          if (gone) { opened.close(); return; }
          got.push(opened);
          urls.set(rel, opened.url);
        }));
      }).catch(() => { /* (the loader says what it can't read) */ }).finally(() => {
        if (!gone) st.take = { label: s.label, src: f.url, resolve: (rel) => urls.get(rel) ?? "" };
      });
      return () => { gone = true; for (const x of got) x.close(); };
    });
    return st;
  }
  const pa = prepare(() => a, oa);
  const pb = prepare(() => b, ob);
  let error = $derived(oa.error || ob.error);
  let ready = $derived((!a || !!pa.take) && (!b || !!pb.take));
  const key = $derived(`${a?.path}@${a?.version}|${b?.path}@${b?.version}`);
</script>

{#if error === "tooBig"}
  <p class="muted small">{t("Too big to show here.")}</p>
{:else if error}
  <p class="muted small">{t("Couldn't read it:")} {error}</p>
{:else if !ready}
  <p class="muted small">{t("Reading…")}</p>
{:else}
  {#key key}
    <ModelCompare ext={extOf((a ?? b ?? file).path)} a={pa.take} b={pb.take} {compare} />
  {/key}
{/if}

<style>
  .small { font-size: var(--fs-sm); }
</style>
