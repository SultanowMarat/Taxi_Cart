# Map Service

Separate map microservice for Turkmenistan tiles, version manifests and delta updates.

## Standalone Docker Compose

Run only the map microservice:

```powershell
docker compose -f ../infra/docker-compose.map-service.yml up --build
```

From the repository root:

```powershell
docker compose -f infra/docker-compose.map-service.yml up --build
```

Available URLs:

- Service root and Swagger UI: `http://localhost:8090/`
- Swagger UI: `http://localhost:8090/swagger`
- OpenAPI spec: `http://localhost:8090/openapi.yaml`
- Healthcheck: `http://localhost:8090/health`

## Endpoints

### `GET /`

Redirects to `/swagger`. This is a convenience endpoint for opening the service documentation from the root URL.

### `GET /health`

Checks that the service is running and can respond to HTTP requests. Use it for Docker/Kubernetes healthchecks, monitoring and quick local verification.

### `GET /swagger`

Serves Swagger UI for manual API testing in a browser. It loads the local OpenAPI spec from `/openapi.yaml`.

### `GET /docs`

Alias for Swagger UI. It returns the same documentation page as `/swagger`.

### `GET /openapi.yaml`

Returns the OpenAPI 3.0 contract for this microservice. Use it for Swagger UI, Postman/Insomnia import and client generation.

### `GET /api/map/version`

Returns the current map version for the active region. Mobile apps use this before cache synchronization to decide whether they need to request the manifest and delta update.

Example:

```bash
curl http://localhost:8090/api/map/version
```

### `GET /api/map/manifest?region=turkmenistan`

Returns supported tile groups, checksum values and URL template for tile loading. Mobile apps use it to initialize or validate their local tile cache.

Query parameters:

- `region` - optional map region, defaults to `turkmenistan`.

Example:

```bash
curl "http://localhost:8090/api/map/manifest?region=turkmenistan"
```

### `GET /api/map/delta?from=<oldVersion>&to=<newVersion>`

Returns changed and deleted tiles between two map versions. Mobile apps use this to update only the changed tiles instead of downloading the whole map again.

Query parameters:

- `from` - local cached map version.
- `to` - target map version. If omitted, the service uses the current version.

Example:

```bash
curl "http://localhost:8090/api/map/delta?from=tm-2026.05-demo&to=tm-2026.06-demo"
```

### `GET /api/map/download-info`

Returns diagnostic information about the local OSM PBF source file: expected path, source URL, existence flag, file size and modification time.

Example:

```bash
curl http://localhost:8090/api/map/download-info
```

### `GET /tiles/{z}/{x}/{y}.png`

Returns a PNG raster tile for map rendering. Mapping clients use this endpoint through the URL template `/tiles/{z}/{x}/{y}.png`.

Path parameters:

- `z` - zoom level.
- `x` - tile column.
- `y` - tile row.

Example:

```text
http://localhost:8090/tiles/10/637/412.png
```

The generated MVP tiles are deterministic PNG placeholders so the apps can run immediately. The downloaded OSM PBF is stored under `../infra/osrm/data/turkmenistan-latest.osm.pbf`.
