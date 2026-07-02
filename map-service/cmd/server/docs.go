package main

const swaggerHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Taxi Map Service Swagger</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
  <style>
    body { margin: 0; background: #f7f7f7; }
    .swagger-ui .topbar { display: none; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function () {
      SwaggerUIBundle({
        url: "/openapi.yaml",
        dom_id: "#swagger-ui",
        presets: [SwaggerUIBundle.presets.apis],
        layout: "BaseLayout"
      });
    };
  </script>
</body>
</html>`

const openapiYAML = `openapi: 3.0.3
info:
  title: Taxi MVP Map Service API
  version: 1.0.0
  description: Map microservice for Turkmenistan tile delivery, map version manifests and delta updates.
servers:
  - url: http://localhost:8090
paths:
  /health:
    get:
      summary: Healthcheck
      responses:
        "200":
          description: Service health status.
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/HealthResponse"
  /api/map/version:
    get:
      summary: Current map version
      responses:
        "200":
          description: Current map version metadata.
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/MapVersionResponse"
  /api/map/manifest:
    get:
      summary: Tile manifest
      parameters:
        - name: region
          in: query
          required: false
          schema:
            type: string
            default: turkmenistan
      responses:
        "200":
          description: Tile groups and checksums for client cache synchronization.
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/MapManifestResponse"
  /api/map/delta:
    get:
      summary: Tile delta
      parameters:
        - name: from
          in: query
          required: false
          schema:
            type: string
          example: tm-2026.05-demo
        - name: to
          in: query
          required: false
          schema:
            type: string
          example: tm-2026.06-demo
      responses:
        "200":
          description: Changed and deleted tiles between versions.
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/MapDeltaResponse"
  /api/map/download-info:
    get:
      summary: OSM PBF download info
      responses:
        "200":
          description: Local OSM source file status.
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/DownloadInfoResponse"
  /tiles/{z}/{x}/{y}.png:
    get:
      summary: Raster tile
      parameters:
        - name: z
          in: path
          required: true
          schema:
            type: integer
          example: 10
        - name: x
          in: path
          required: true
          schema:
            type: integer
          example: 637
        - name: y
          in: path
          required: true
          schema:
            type: integer
          example: 412
      responses:
        "200":
          description: PNG map tile.
          content:
            image/png:
              schema:
                type: string
                format: binary
        "502":
          description: Upstream tile provider is unavailable.
components:
  schemas:
    HealthResponse:
      type: object
      properties:
        ok:
          type: boolean
          example: true
        service:
          type: string
          example: map-service
        time:
          type: string
          format: date-time
    MapVersionResponse:
      type: object
      properties:
        region:
          type: string
          example: turkmenistan
        version:
          type: string
          example: tm-2026.06-demo
        updatedAt:
          type: string
          format: date-time
    MapManifestResponse:
      type: object
      properties:
        region:
          type: string
          example: turkmenistan
        version:
          type: string
          example: tm-2026.06-demo
        strategy:
          type: string
          example: delta
        nightlySyncHour:
          type: integer
          example: 3
        tiles:
          type: array
          items:
            $ref: "#/components/schemas/TileGroup"
    TileGroup:
      type: object
      properties:
        z:
          type: integer
          example: 10
        checksum:
          type: string
        urlTemplate:
          type: string
          example: /tiles/{z}/{x}/{y}.png
    MapDeltaResponse:
      type: object
      properties:
        from:
          type: string
        to:
          type: string
        changed:
          type: array
          items:
            $ref: "#/components/schemas/ChangedTile"
        deleted:
          type: array
          items:
            type: string
    ChangedTile:
      type: object
      properties:
        z:
          type: integer
        x:
          type: integer
        y:
          type: integer
        checksum:
          type: string
    DownloadInfoResponse:
      type: object
      properties:
        source:
          type: string
          format: uri
        path:
          type: string
        exists:
          type: boolean
        sizeBytes:
          type: integer
          format: int64
        modifiedAt:
          type: string
          format: date-time
`
