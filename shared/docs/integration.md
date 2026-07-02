# Интеграция микросервиса карт

## Роль map-service

`map-service` отвечает за картографическую часть Taxi MVP:

- выдачу PNG-тайлов карты;
- manifest тайлов для клиентского кеша;
- delta-обновления карты;
- версию карты;
- startup-загрузку OSM PBF-файла Туркменистана;
- ночную синхронизацию OSM PBF-файла;
- локальное кеширование PNG-тайлов;
- OpenAPI/Swagger документацию.

## Координаты

GPS-координаты водителя или клиента получает не map-service, а мобильное приложение через геолокацию устройства.

Дальше мобильное приложение отправляет координаты в основной `backend`, где обрабатываются:

- live-локация водителя;
- движение маркера на карте;
- статусы поездки;
- WebSocket-обновления;
- логика заказов.

`map-service` может получать координаты в будущих endpoint'ах, например для reverse geocoding или построения маршрута, но в текущей версии такие endpoint'ы не реализованы.

## Тайлы

Тайлы относятся к `map-service`.

Сейчас endpoint:

```text
GET /tiles/{z}/{x}/{y}.png
```

отдает PNG-тайлы по cache-first схеме:

1. Сначала проверяет локальный файл `/data/tiles/{z}/{x}/{y}.png`.
2. Если файл есть, отдает его без обращения во внешний сервис.
3. Если файла нет, скачивает тайл из OpenStreetMap.
4. Сохраняет скачанный тайл в `/data/tiles`.
5. Возвращает тайл клиенту.

В Docker Compose подготовлен volume:

```text
infra/map-service/tiles -> /data/tiles
```

После первого запроса конкретный тайл хранится локально.

## Синхронизация OSM PBF

При запуске `map-service` проверяет локальный файл:

```text
/data/osm/turkmenistan-latest.osm.pbf
```

Если файла нет, сервис скачивает карту Туркменистана из Geofabrik:

```text
https://download.geofabrik.de/asia/turkmenistan-latest.osm.pbf
```

Рядом сохраняется metadata:

```text
/data/osm/turkmenistan-latest.osm.pbf.metadata.json
```

Каждую ночь сервис делает `HEAD`-проверку remote-файла. Если изменился `ETag`, `Last-Modified` или `Content-Length`, сервис скачивает новую версию PBF, обновляет metadata и меняет версию карты, которую отдает `/api/map/version`.

По умолчанию синхронизация запускается в `03:00 UTC`:

```env
MAP_SYNC_HOUR_UTC=3
```

## Контракт кеша карты

Мобильные приложения читают:

1. `GET /api/map/version`
2. `GET /api/map/manifest?region=turkmenistan`
3. `GET /api/map/delta?from=<cached>&to=<current>`

Логика:

1. Приложение получает текущую версию карты.
2. Сравнивает ее с версией, сохраненной на устройстве.
3. Если версия изменилась, запрашивает manifest и delta.
4. Скачивает только измененные тайлы.
5. Удаляет тайлы, которые пришли в `deleted`.

## URL map-service для мобильных приложений

Клиентское и водительское приложения читают базовый URL карты из:

```env
VITE_MAP_SERVICE_URL
```

Локальная разработка в браузере/PWA на той же машине:

```env
VITE_MAP_SERVICE_URL=http://localhost:8090
```

Физический телефон в той же Wi-Fi сети:

```env
VITE_MAP_SERVICE_URL=http://<host-lan-ip>:8090
```

Production-сборки должны указывать на публичный HTTPS URL развернутого `map-service`.
