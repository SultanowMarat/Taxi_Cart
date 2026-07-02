# Map Service

Отдельный микросервис карт для Taxi MVP.

## Что делает сервис

- Отдает PNG-тайлы карты через `GET /tiles/{z}/{x}/{y}.png`.
- Отдает текущую версию карты через `GET /api/map/version`.
- Отдает manifest тайлов для кеширования через `GET /api/map/manifest`.
- Отдает delta-обновления между версиями через `GET /api/map/delta`.
- При запуске скачивает `turkmenistan-latest.osm.pbf`, если файла нет локально.
- Каждую ночь проверяет Geofabrik и скачивает новую карту, если remote-файл изменился.
- Кеширует PNG-тайлы в `/data/tiles` и отдает их клиентам/водителям.
- Показывает информацию о локальном OSM PBF-файле и metadata через `GET /api/map/download-info`.
- Отдает Swagger UI и OpenAPI спецификацию.

## Что сервис сейчас не делает

- Не получает GPS-координаты водителя или клиента с телефона.
- Не хранит live-локацию водителей.
- Не занимается заказами и realtime-статусами поездок.
- Не строит маршруты между точками.

GPS-координаты получает мобильное приложение через геолокацию устройства и отправляет их в основной backend. Map-service отвечает за отображение карты, тайлы, версии и кеширование карты.

## Тайлы

Тайлы относятся к этому сервису.

Сервис работает по cache-first схеме:

1. Сначала ищет тайл в `/data/tiles/{z}/{x}/{y}.png`.
2. Если файл есть, отдает локальную копию.
3. Если файла нет, скачивает PNG из OpenStreetMap.
4. Сохраняет тайл в `/data/tiles`.
5. Отдает тайл клиенту.

Upstream для первичной загрузки:

```text
https://tile.openstreetmap.org/{z}/{x}/{y}.png
```

В compose уже есть volume для локального хранения:

```text
infra/map-service/tiles -> /data/tiles
```

После первого запроса тайл хранится локально.

## Синхронизация OSM PBF

При запуске сервис проверяет:

```text
/data/osm/turkmenistan-latest.osm.pbf
```

Если файла нет, он скачивается из:

```text
https://download.geofabrik.de/asia/turkmenistan-latest.osm.pbf
```

Metadata хранится рядом:

```text
/data/osm/turkmenistan-latest.osm.pbf.metadata.json
```

Ночная синхронизация работает по `MAP_SYNC_HOUR_UTC`, по умолчанию `3`. Сервис делает `HEAD`-запрос к Geofabrik и сравнивает `ETag`, `Last-Modified`, `Content-Length`. Если карта изменилась, новая версия скачивается локально, metadata обновляется, а `/api/map/version` начинает отдавать новую версию.

## Отдельный Docker Compose

Запуск только микросервиса карт:

```powershell
docker compose -f infra/docker-compose.map-service.yml up --build
```

Доступные URL:

- Корень сервиса и Swagger UI: `http://localhost:8090/`
- Swagger UI: `http://localhost:8090/swagger`
- OpenAPI спецификация: `http://localhost:8090/openapi.yaml`
- Проверка состояния: `http://localhost:8090/health`

## Эндпоинты

### `GET /`

Делает redirect на `/swagger`. Нужен, чтобы при открытии корневого URL сразу попасть в документацию.

### `GET /health`

Проверяет, что сервис запущен и отвечает на HTTP-запросы. Используется для healthcheck, мониторинга и локальной проверки.

### `GET /swagger`

Открывает Swagger UI для ручного тестирования API в браузере.

### `GET /docs`

Алиас для Swagger UI. Возвращает ту же страницу, что и `/swagger`.

### `GET /openapi.yaml`

Возвращает OpenAPI 3.0 контракт микросервиса. Используется Swagger UI, Postman/Insomnia и генераторами API-клиентов.

### `GET /api/map/version`

Возвращает текущую версию карты. Мобильное приложение использует этот endpoint перед синхронизацией кеша.

Пример:

```bash
curl http://localhost:8090/api/map/version
```

### `GET /api/map/manifest?region=turkmenistan`

Возвращает список поддерживаемых групп тайлов, checksum и URL-шаблон для загрузки тайлов. Мобильное приложение использует manifest для первичной загрузки и проверки кеша.

Параметры:

- `region` - регион карты, по умолчанию `turkmenistan`.

Пример:

```bash
curl "http://localhost:8090/api/map/manifest?region=turkmenistan"
```

### `GET /api/map/delta?from=<oldVersion>&to=<newVersion>`

Возвращает измененные и удаленные тайлы между двумя версиями карты. Нужен, чтобы приложение обновляло только измененные тайлы, а не скачивало всю карту заново.

Параметры:

- `from` - версия карты, которая уже сохранена на устройстве.
- `to` - целевая версия карты. Если не передать, используется текущая версия сервиса.

Пример:

```bash
curl "http://localhost:8090/api/map/delta?from=tm-2026.05-demo&to=tm-2026.06-demo"
```

### `GET /api/map/download-info`

Возвращает диагностическую информацию об OSM PBF-файле: источник, путь внутри контейнера, наличие файла, размер и дату изменения.

Пример:

```bash
curl http://localhost:8090/api/map/download-info
```

### `GET /tiles/{z}/{x}/{y}.png`

Возвращает PNG-тайл для отображения карты.

Параметры:

- `z` - zoom level.
- `x` - колонка tile.
- `y` - строка tile.

Пример:

```text
http://localhost:8090/tiles/10/637/412.png
```
