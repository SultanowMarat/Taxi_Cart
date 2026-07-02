# Taxi Cart Map Service

Отдельный репозиторий для микросервиса карт Taxi MVP.

Сервис отвечает за:

- выдачу raster tiles для отображения карты в мобильных приложениях;
- версионирование карты;
- manifest для кеширования тайлов на клиенте;
- delta updates между версиями карты;
- информацию о локальном OSM PBF-файле;
- Swagger UI для ручного тестирования API.

## Структура

```text
map-service/                         Go microservice
infra/docker-compose.map-service.yml Standalone Docker Compose
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

## Endpoint'ы

### `GET /`

Служебный root endpoint.

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

Alias для Swagger UI.

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

Возвращает текущую версию карты для региона.

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

Показывает статус локального OSM PBF-файла.

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
  "exists": true,
  "sizeBytes": 987654321,
  "modifiedAt": "2026-07-02T10:15:30Z"
}
```

Поля:

- `source` — источник OSM данных;
- `path` — путь внутри контейнера;
- `exists` — найден ли файл;
- `sizeBytes` — размер файла, если он найден;
- `modifiedAt` — время последнего изменения файла.

### `GET /tiles/{z}/{x}/{y}.png`

Возвращает PNG tile для отображения карты.

Этот endpoint напрямую используется картографической библиотекой в мобильном приложении. Например Leaflet/MapLibre/другой клиент подставляет `z`, `x`, `y` в URL шаблон и загружает нужные PNG-тайлы при перемещении или масштабировании карты.

Параметры path:

- `z` — zoom level;
- `x` — tile column;
- `y` — tile row.

Пример:

```text
http://localhost:8090/tiles/10/637/412.png
```

Для Android emulator на Mac:

```text
http://10.0.2.2:8090/tiles/10/637/412.png
```

Успешный ответ:

```text
Content-Type: image/png
```

Если upstream tile provider недоступен, сервис вернет:

```text
502 Bad Gateway
```

## Подключение Android emulator на Mac

Для Android emulator нельзя использовать `localhost` внутри мобильного приложения, потому что `localhost` будет указывать на сам эмулятор.

Используйте `10.0.2.2`, это адрес host-машины из Android emulator:

```env
VITE_MAP_SERVICE_URL=http://10.0.2.2:8090
```

Если приложение также ходит в backend:

```env
VITE_API_URL=http://10.0.2.2:8080
VITE_WS_URL=ws://10.0.2.2:8080
VITE_MAP_SERVICE_URL=http://10.0.2.2:8090
```

Для iOS Simulator обычно можно использовать:

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

## Production notes

Тайлы сейчас проксируются с `https://tile.openstreetmap.org`. Для production нужно заменить это на собственный tile provider/cache и закрыть публичный доступ правилами rate limit/API gateway.
