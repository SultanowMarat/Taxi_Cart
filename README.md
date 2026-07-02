# Taxi Cart Map Service

Отдельный репозиторий для микросервиса карт Taxi MVP.

Сервис отвечает за:

- выдачу raster tiles для отображения карты в мобильных приложениях;
- версионирование карты;
- manifest для кеширования тайлов на клиенте;
- delta updates между версиями карты;
- скачивание и ночную синхронизацию OSM PBF-файла Туркменистана;
- локальное хранение и выдачу PNG-тайлов;
- Swagger UI для ручного тестирования API.

## Зона ответственности

Этот микросервис отвечает именно за картографическую часть:

- отдает тайлы карты по URL `/tiles/{z}/{x}/{y}.png`;
- сообщает текущую версию карты;
- отдает manifest тайлов для кеширования на клиенте;
- отдает delta-обновления между версиями карты;
- при запуске проверяет и скачивает `turkmenistan-latest.osm.pbf`, если файла нет локально;
- каждую ночь сверяет remote metadata Geofabrik и скачивает новую версию, если карта изменилась;
- хранит metadata скачанной карты рядом с PBF-файлом;
- кеширует PNG-тайлы в локальном volume и отдает их клиентам/водителям;
- показывает диагностическую информацию по локальному OSM PBF-файлу;
- предоставляет Swagger/OpenAPI документацию.

Что сервис сейчас не делает:

- не получает GPS-координаты водителя или клиента с телефона;
- не хранит live-локацию водителей;
- не занимается заказами, статусами поездок и WebSocket-трекингом;
- не строит маршруты между двумя точками.

GPS-координаты получает мобильное приложение через геолокацию устройства. Затем приложение отправляет эти координаты в основной backend, где живет realtime-логика такси: водитель онлайн, движение маркера, заказ, статусы поездки.

Map-service может принимать координаты как параметры будущих картографических endpoint'ов, например для reverse geocoding, geocoding или построения маршрута, но в текущей версии таких endpoint'ов еще нет.

## Тайлы

Тайлы тоже относятся к этому микросервису.

Endpoint `/tiles/{z}/{x}/{y}.png` работает по принципу cache-first:

1. Сервис ищет PNG-тайл в локальном хранилище `/data/tiles`.
2. Если тайл найден, сразу отдает его клиенту.
3. Если тайла нет, скачивает его из OpenStreetMap.
4. Сохраняет скачанный тайл в `/data/tiles/{z}/{x}/{y}.png`.
5. Отдает тайл клиенту.

Upstream-источник для первичной загрузки тайлов:

```text
https://tile.openstreetmap.org/{z}/{x}/{y}.png
```

В Docker Compose уже подготовлен volume для локального хранения тайлов:

```text
infra/map-service/tiles -> /data/tiles
```

После первого запроса конкретный тайл хранится локально и повторно отдается уже без обращения к OpenStreetMap.

## Синхронизация карты Туркменистана

При запуске сервис проверяет локальный файл:

```text
/data/osm/turkmenistan-latest.osm.pbf
```

Если файла нет, сервис скачивает его из Geofabrik:

```text
https://download.geofabrik.de/asia/turkmenistan-latest.osm.pbf
```

Рядом сохраняется metadata-файл:

```text
/data/osm/turkmenistan-latest.osm.pbf.metadata.json
```

В metadata хранятся:

- `etag`;
- `lastModified`;
- `contentLength`;
- рассчитанная `version`;
- время скачивания.

Каждую ночь, по умолчанию в `03:00 UTC`, сервис делает `HEAD`-проверку remote-файла. Если `ETag`, `Last-Modified` или `Content-Length` изменились, сервис скачивает новый PBF, обновляет metadata и меняет версию карты.

Настройки:

```env
MAP_OSM_SOURCE_URL=https://download.geofabrik.de/asia/turkmenistan-latest.osm.pbf
MAP_OSM_PATH=/data/osm/turkmenistan-latest.osm.pbf
MAP_SYNC_HOUR_UTC=3
MAP_STORAGE_PATH=/data/tiles
```

## Структура

```text
map-service/                         Go-микросервис карты
infra/docker-compose.map-service.yml Отдельный Docker Compose
shared/openapi/map-service.yaml      OpenAPI спецификация
shared/docs/integration.md           Интеграционные заметки
```

## Запуск на Mac

Из корня репозитория:

```bash
docker compose -f infra/docker-compose.map-service.yml up --build
```

Проверка:

```bash
curl http://localhost:8090/health
```

Swagger:

```text
http://localhost:8090/swagger
```

OpenAPI YAML:

```text
http://localhost:8090/openapi.yaml
```

## Эндпоинты

### `GET /`

Служебный корневой endpoint.

Он не возвращает бизнес-данные, а делает redirect на Swagger UI: `/swagger`. Нужен для удобства: можно открыть `http://localhost:8090/` и сразу попасть в документацию сервиса.

Пример:

```text
http://localhost:8090/
```

### `GET /health`

Проверяет, что микросервис запущен и отвечает.

Используется для:

- Docker/Kubernetes healthcheck;
- мониторинга;
- быстрой проверки после запуска;
- проверки доступности сервиса перед подключением мобильного приложения.

Пример:

```bash
curl http://localhost:8090/health
```

Пример ответа:

```json
{
  "ok": true,
  "service": "map-service",
  "time": "2026-07-02T10:15:30Z"
}
```

### `GET /swagger`

Открывает Swagger UI для ручного тестирования API в браузере.

Используется для:

- проверки endpoint'ов без Postman;
- демонстрации API backend/mobile разработчикам;
- просмотра параметров, схем и примеров ответов.

Пример:

```text
http://localhost:8090/swagger
```

Важно: Swagger UI загружает frontend assets с CDN `unpkg.com`, поэтому браузеру нужен доступ к интернету. Сама спецификация отдается локально через `/openapi.yaml`.

### `GET /docs`

Алиас для Swagger UI.

Работает так же, как `/swagger`, и нужен для удобного короткого URL документации.

Пример:

```text
http://localhost:8090/docs
```

### `GET /openapi.yaml`

Отдает OpenAPI 3.0 спецификацию микросервиса.

Используется для:

- генерации API-клиентов;
- импорта в Postman/Insomnia;
- отображения документации в Swagger UI;
- синхронизации контракта между backend, mobile и QA.

Пример:

```bash
curl http://localhost:8090/openapi.yaml
```

### `GET /api/map/version`

Возвращает текущую версию карты для региона. Версия рассчитывается из metadata локального OSM PBF-файла и меняется после успешного скачивания новой версии карты.

Мобильное приложение вызывает этот endpoint при старте или перед обновлением кеша. Если версия на сервере отличается от версии, сохраненной на устройстве, приложение запрашивает manifest и delta.

Пример:

```bash
curl http://localhost:8090/api/map/version
```

Пример ответа:

```json
{
  "region": "turkmenistan",
  "version": "tm-2026.06-demo",
  "updatedAt": "2026-07-02T10:15:30Z"
}
```

Поля:

- `region` — регион карты;
- `version` — текущая версия набора тайлов/данных;
- `updatedAt` — время формирования ответа.

### `GET /api/map/manifest?region=turkmenistan`

Возвращает manifest тайлов для выбранного региона.

Manifest нужен мобильному приложению, чтобы понять:

- какие zoom levels поддерживаются;
- какой URL шаблон использовать для загрузки тайлов;
- какие checksum значения использовать для проверки кеша;
- какую стратегию обновления применять.

Параметры:

- `region` — опционально, регион карты. По умолчанию `turkmenistan`.

Пример:

```bash
curl "http://localhost:8090/api/map/manifest?region=turkmenistan"
```

Пример ответа:

```json
{
  "region": "turkmenistan",
  "version": "tm-2026.06-demo",
  "strategy": "delta",
  "nightlySyncHour": 3,
  "tiles": [
    {
      "z": 10,
      "checksum": "a1b2c3d4e5f6a7b8",
      "urlTemplate": "/tiles/{z}/{x}/{y}.png"
    }
  ]
}
```

Поля:

- `strategy` — стратегия обновления, сейчас используется `delta`;
- `nightlySyncHour` — рекомендуемый час фоновой синхронизации;
- `tiles[].z` — zoom level;
- `tiles[].checksum` — checksum группы тайлов;
- `tiles[].urlTemplate` — шаблон URL для загрузки PNG тайлов.

### `GET /api/map/delta?from=<oldVersion>&to=<newVersion>`

Возвращает список измененных и удаленных тайлов между двумя версиями карты.

Мобильное приложение использует этот endpoint, чтобы не скачивать всю карту заново. Оно передает старую локальную версию `from` и новую серверную версию `to`, затем обновляет только измененные тайлы.

Параметры:

- `from` — старая версия карты на устройстве;
- `to` — новая версия карты. Если не передать, сервис использует текущую версию.

Пример:

```bash
curl "http://localhost:8090/api/map/delta?from=tm-2026.05-demo&to=tm-2026.06-demo"
```

Пример ответа:

```json
{
  "from": "tm-2026.05-demo",
  "to": "tm-2026.06-demo",
  "changed": [
    {
      "z": 10,
      "x": 637,
      "y": 412,
      "checksum": "a1b2c3d4e5f6a7b8"
    }
  ],
  "deleted": []
}
```

Поля:

- `changed` — тайлы, которые нужно скачать заново;
- `deleted` — тайлы, которые нужно удалить из локального кеша;
- `z`, `x`, `y` — координаты tile в стандартной web map tile scheme.

### `GET /api/map/download-info`

Показывает статус локального OSM PBF-файла и metadata последней синхронизации.

Endpoint нужен для диагностики инфраструктуры: можно быстро понять, скачан ли исходный OSM-файл, где он лежит в контейнере и какой у него размер.

Пример:

```bash
curl http://localhost:8090/api/map/download-info
```

Пример ответа:

```json
{
  "source": "https://download.geofabrik.de/asia/turkmenistan-latest.osm.pbf",
  "path": "/data/osm/turkmenistan-latest.osm.pbf",
  "metadataPath": "/data/osm/turkmenistan-latest.osm.pbf.metadata.json",
  "exists": true,
  "version": "tm-a1b2c3d4e5f6a7b8",
  "sizeBytes": 987654321,
  "modifiedAt": "2026-07-02T10:15:30Z",
  "etag": "\"abc123\"",
  "lastModified": "Thu, 02 Jul 2026 02:10:00 GMT",
  "contentLength": 987654321,
  "downloadedAt": "2026-07-02T03:00:02Z",
  "syncHourUTC": 3,
  "tileStoragePath": "/data/tiles"
}
```

Поля:

- `source` — источник OSM данных;
- `path` — путь внутри контейнера;
- `metadataPath` — путь к metadata-файлу;
- `exists` — найден ли файл;
- `version` — текущая версия карты;
- `sizeBytes` — размер файла, если он найден;
- `modifiedAt` — время последнего изменения файла;
- `etag`, `lastModified`, `contentLength` — remote metadata, по которым сервис определяет изменения;
- `downloadedAt` — когда PBF был скачан;
- `syncHourUTC` — час ночной синхронизации;
- `tileStoragePath` — локальное хранилище PNG-тайлов.

### `GET /tiles/{z}/{x}/{y}.png`

Возвращает PNG tile для отображения карты.

Этот endpoint напрямую используется картографической библиотекой в мобильном приложении. Например Leaflet/MapLibre/другой клиент подставляет `z`, `x`, `y` в URL шаблон и загружает нужные PNG-тайлы при перемещении или масштабировании карты.

Сервис сначала ищет тайл локально в `/data/tiles`. Если тайл уже скачан, он отдается из локального хранилища. Если тайла нет, сервис скачивает его из OpenStreetMap, сохраняет локально и возвращает клиенту.

Параметры path:

- `z` — zoom level;
- `x` — tile column;
- `y` — tile row.

Пример:

```text
http://localhost:8090/tiles/10/637/412.png
```

Успешный ответ:

```text
Content-Type: image/png
```

Если upstream tile provider недоступен, сервис вернет:

```text
502 Bad Gateway
```

## Подключение локального приложения

Если приложение запущено на той же машине, что и `map-service`, можно использовать:

```env
VITE_MAP_SERVICE_URL=http://localhost:8090
```

## Подключение физического телефона

Телефон и Mac должны быть в одной Wi-Fi сети. Вместо `localhost` укажите LAN IP Mac:

```env
VITE_MAP_SERVICE_URL=http://<mac-lan-ip>:8090
```

Например:

```env
VITE_MAP_SERVICE_URL=http://192.168.1.50:8090
```

## Заметки для production

Текущая реализация уже хранит PBF-файл и кеширует PNG-тайлы локально. Для production следующий шаг - заменить upstream `https://tile.openstreetmap.org` на собственный tile server/render pipeline, который будет строить тайлы из локального `turkmenistan-latest.osm.pbf`.
