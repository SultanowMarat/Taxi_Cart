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

- `GET /health`
- `GET /swagger`
- `GET /openapi.yaml`
- `GET /api/map/version`
- `GET /api/map/manifest?region=turkmenistan`
- `GET /api/map/delta?from=v1&to=v1`
- `GET /tiles/{z}/{x}/{y}.png`
- `GET /api/map/download-info`

The generated MVP tiles are deterministic PNG placeholders so the apps can run immediately. The downloaded OSM PBF is stored under `../infra/osrm/data/turkmenistan-latest.osm.pbf`.
