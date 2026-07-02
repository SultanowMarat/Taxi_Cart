# Integration Notes

The MVP is split into independent projects:

- `backend`: REST API and binary realtime WebSocket.
- `map-service`: tile manifest, delta API and raster tile delivery.
- `client-app`: mobile-first customer PWA.
- `driver-app`: mobile-first driver PWA.
- `admin`: desktop operations panel.

## Realtime Frame

The shared schema is `shared/proto/ws.proto`. The runnable MVP uses a compact binary frame where the first byte is `MessageType` and the remaining bytes are a UTF-8 payload matching the proto message shape. This gives all apps a binary WebSocket contract and keeps generation tooling optional for the first runnable phase.

## Map Cache Contract

Frontends read:

1. `GET /api/map/version`
2. `GET /api/map/manifest?region=turkmenistan`
3. `GET /api/map/delta?from=<cached>&to=<current>`

Changed tile checksums are stored in IndexedDB/local cache by the apps.

## Map Service URL For Mobile Apps

The customer and driver apps read the map base URL from `VITE_MAP_SERVICE_URL`.

Local browser/PWA development on the same machine:

```env
VITE_MAP_SERVICE_URL=http://localhost:8090
```

Android emulator against the host machine:

```env
VITE_MAP_SERVICE_URL=http://10.0.2.2:8090
```

Physical phones on the same Wi-Fi network:

```env
VITE_MAP_SERVICE_URL=http://<host-lan-ip>:8090
```

Production builds should point to the public HTTPS URL of the deployed map service.
