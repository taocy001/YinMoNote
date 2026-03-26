/**
 * YinMoNote Service Worker - Cache-first strategy for static assets.
 * Caches the app shell for offline viewing. API requests bypass the cache
 * so note data is always fetched from the server when online.
 */
// Cache name is derived from the ?v= query parameter injected at registration
// time by main.ts. Each build produces a unique stamp, so old caches from
// previous deployments are always evicted when this SW activates.
const _v = new URL(self.location.href).searchParams.get('v') || 'v1'
const CACHE_NAME = `yinmo-${_v}`
const STATIC_EXTENSIONS = ['.js', '.css', '.html', '.svg', '.png', '.ico', '.woff2']

self.addEventListener('install', (event) => {
  // Skip waiting so the new SW activates immediately
  self.skipWaiting()
})

self.addEventListener('activate', (event) => {
  // Clean up old caches on activation; claim clients so the new SW controls
  // all open tabs immediately without requiring a page reload.
  event.waitUntil(
    caches.keys().then(keys =>
      Promise.all(keys.filter(k => k !== CACHE_NAME).map(k => caches.delete(k)))
    ).then(() => self.clients.claim())
  )
})

self.addEventListener('fetch', (event) => {
  const url = new URL(event.request.url)

  // Never cache API requests — always go to network
  if (url.pathname.startsWith('/api/') || url.pathname.startsWith('/uploads/')) {
    return
  }

  // Only cache GET requests for static file extensions.
  // index.html (pathname '/') is intentionally excluded: the server sends
  // Cache-Control: no-cache for it so that updated JS bundles reach users
  // after each deployment. Caching it here would bypass that header.
  const isStatic = STATIC_EXTENSIONS.some(ext => url.pathname.endsWith(ext))
  if (event.request.method !== 'GET' || !isStatic) return

  event.respondWith(
    caches.open(CACHE_NAME).then(async cache => {
      const cached = await cache.match(event.request)
      if (cached) return cached
      try {
        const response = await fetch(event.request)
        if (response.ok) cache.put(event.request, response.clone())
        return response
      } catch {
        // Network failed — serve stale cache if available, else 503.
        // (cached is always null here because we already returned it above
        // when it was truthy; this branch is the true offline case.)
        return cached || new Response('Offline', { status: 503 })
      }
    })
  )
})
