# Taxi Cart Map Service

Отдельный репозиторий для микросервиса карт Taxi MVP.

Сервис отвечает за:

- выдачу raster tiles: `GET /tiles/{z}/{x}/{y}.png`;
- текущую версию карты: `GET /api/map/version`;
- manifest для кеширования тайлов: `GET /api/map/manifest`;
- delta updates между версиями: `GET /api/map/delta`;
- информацию о локальном OSM PBF-файле: `GET /api/map/download-info`;
- Swagger UI для тестирования API.

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

## Основные endpoint'ы

```text
GET /health
GET /swagger
GET /openapi.yaml
GET /api/map/version
GET /api/map/manifest?region=turkmenistan
GET /api/map/delta?from=<oldVersion>&to=<newVersion>
GET /api/map/download-info
GET /tiles/{z}/{x}/{y}.png
```

## Notes

Swagger UI загружает frontend assets с CDN `unpkg.com`, а сама OpenAPI спецификация отдается локально через `/openapi.yaml`.

Тайлы сейчас проксируются с `https://tile.openstreetmap.org`. Для production нужно заменить это на собственный tile provider/cache и закрыть публичный доступ правилами rate limit/API gateway.
