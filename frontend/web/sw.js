importScripts('https://storage.googleapis.com/workbox-cdn/releases/4.3.1/workbox-sw.js');

workbox.core.skipWaiting();
workbox.core.clientsClaim();

workbox.routing.registerRoute(
    new RegExp('/assets/img/'),
    new workbox.strategies.CacheFirst({
        cacheName: 'IMAGE',
        plugins: [
            new workbox.expiration.Plugin({
                maxEntries: 100,
                maxAgeSeconds: 1 * 24 * 60 * 60,
            }),
            new workbox.cacheableResponse.Plugin({
                statuses: [200],
            })
        ],
    })
);

workbox.routing.registerRoute(
    new RegExp('/assets/css/'),
    new workbox.strategies.CacheFirst({
        cacheName: 'CSS',
        plugins: [
            new workbox.expiration.Plugin({
                maxEntries: 100,
                maxAgeSeconds: 1 * 24 * 60 * 60,
            }),
            new workbox.cacheableResponse.Plugin({
                statuses: [200],
            })
        ],
    })
);

workbox.routing.registerRoute(
    new RegExp('/assets/js/'),
    new workbox.strategies.CacheFirst({
        cacheName: 'JS',
        plugins: [
            new workbox.expiration.Plugin({
                maxEntries: 100,
                maxAgeSeconds: 1 * 24 * 60 * 60,
            }),
            new workbox.cacheableResponse.Plugin({
                statuses: [200],
            })
        ],
    })
);

workbox.precaching.precacheAndRoute([]);