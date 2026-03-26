/**
 * Entry point for the YinMoNote frontend application.
 * Initializes the Vue app and mounts it to the DOM.
 * Registers the PWA service worker for offline static asset caching.
 */
import { createApp } from 'vue'
import './style.css'
import App from './App.vue'
import { loadConfig } from './config'

loadConfig().then(() => {
  createApp(App).mount('#app')
})

// Register service worker for PWA installability and offline static caching.
// The ?v= parameter carries the build stamp so the SW derives a unique cache
// name per deployment, ensuring stale JS bundles are evicted on each update.
declare const __SW_VERSION__: string
if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    navigator.serviceWorker.register(`/sw.js?v=${__SW_VERSION__}`).catch(() => {
      // SW registration failure is non-fatal — app still works without it
    })
  })
}
