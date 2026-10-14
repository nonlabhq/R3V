# src/shared

What the App shares with R3V Cloud's web app, so the two look and work the
same: the design tokens and the plain elements' look, colours and branch
lanes, project and program icons, kinds of file, the version graph
(VersionGraph: drawn from versions and branches; the App adds your
changes, parked ones and the card's buttons through its props and
snippets) and the small components built on them.

The App imports it as `$shared/…`. R3V Cloud copies this folder, pinned to
an R3V commit, and never changes its copy: what the web app needs here is
asked of Core and made here.

The rules (`contract.test.ts` checks the first):

- Nothing here imports the App: not `../lib`, the Go bindings or Wails.
  Components take data (`types.ts`) and give events; whoever uses them
  fetches the data (the App from its bindings, the web app from its API).
- `types.ts` is an interface between the two: add fields, never rename or
  remove one.
- Texts are English, the key to their translation: `t`/`tn` from `i18n.ts`
  translate them through what `setTranslator` was given (the App's own
  i18n; the web app's).
- Styles use the tokens (`tokens.css`), as everywhere in the App.
