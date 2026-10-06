// A refresh icon that turns while busy() is true, for at least one whole
// turn and stopping on a whole turn: a quick refresh still shows (instead
// of a twitch). turn matches the icon's animation (.8s).
export function spinner(busy: () => boolean, turn = 800) {
  let on = $state(false);
  let start = 0;
  let timer: ReturnType<typeof setTimeout> | undefined;
  $effect(() => {
    if (busy()) {
      clearTimeout(timer);
      if (!on) { on = true; start = performance.now(); }
    } else if (on) {
      const left = turn - ((performance.now() - start) % turn);
      clearTimeout(timer);
      timer = setTimeout(() => (on = false), left);
    }
  });
  $effect(() => () => clearTimeout(timer));
  return { get on() { return on; } };
}
