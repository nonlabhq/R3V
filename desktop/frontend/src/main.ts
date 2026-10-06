import { mount } from 'svelte'
// The fonts, bundled: Geist for the interface, Space Grotesk for tool badges.
import '@fontsource-variable/geist/wght.css'
import '@fontsource-variable/geist/wght-italic.css'
import '@fontsource-variable/space-grotesk/wght.css'
import App from './App.svelte'
import { startLanguage } from './lib/i18n.svelte'

// The language first: texts are shown in it from the start.
startLanguage().finally(() => {
  mount(App, { target: document.getElementById('app')! })
  // The app shows its own splash until it has read the teams.
  document.getElementById('splash')?.remove()
})
