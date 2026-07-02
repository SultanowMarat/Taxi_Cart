package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultOSMSource = "https://download.geofabrik.de/asia/turkmenistan-latest.osm.pbf"

type logger struct {
	ch   chan map[string]any
	file *os.File
}

func newLogger(path string) *logger {
	if path == "" {
		path = "/var/log/taxi/map-service.log"
	}
	_ = os.MkdirAll("/var/log/taxi", 0755)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		f = os.Stdout
	}
	l := &logger{ch: make(chan map[string]any, 1024), file: f}
	go func() {
		enc := json.NewEncoder(l.file)
		for e := range l.ch {
			_ = enc.Encode(e)
		}
	}()
	return l
}

func (l *logger) write(level, event string, fields map[string]any) {
	if fields == nil {
		fields = map[string]any{}
	}
	fields["ts"] = time.Now().UTC().Format(time.RFC3339Nano)
	fields["level"] = level
	fields["service"] = "map-service"
	fields["event"] = event
	select {
	case l.ch <- fields:
	default:
	}
}

type app struct {
	mu              sync.RWMutex
	log             *logger
	version         string
	client          *http.Client
	osmSource       string
	osmPath         string
	osmMetaPath     string
	tileStoragePath string
	syncHourUTC     int
}

type osmMetadata struct {
	Source        string    `json:"source"`
	ETag          string    `json:"etag,omitempty"`
	LastModified  string    `json:"lastModified,omitempty"`
	ContentLength int64     `json:"contentLength,omitempty"`
	Version       string    `json:"version"`
	DownloadedAt  time.Time `json:"downloadedAt"`
}

func main() {
	a := newApp()
	a.syncOSM("startup")
	go a.runNightlySync()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", a.root)
	mux.HandleFunc("GET /health", a.health)
	mux.HandleFunc("GET /swagger", a.swaggerUI)
	mux.HandleFunc("GET /swagger/", a.swaggerUI)
	mux.HandleFunc("GET /docs", a.swaggerUI)
	mux.HandleFunc("GET /openapi.yaml", a.openapi)
	mux.HandleFunc("GET /api/map/version", a.versionHandler)
	mux.HandleFunc("GET /api/map/manifest", a.manifest)
	mux.HandleFunc("GET /api/map/delta", a.delta)
	mux.HandleFunc("GET /api/map/download-info", a.downloadInfo)
	mux.HandleFunc("GET /tiles/", a.tile)
	port := env("PORT", "8090")
	a.log.write("info", "server_start", map[string]any{"port": port})
	if err := http.ListenAndServe(":"+port, cors(mux)); err != nil {
		a.log.write("error", "server_failed", map[string]any{"error": err.Error()})
	}
}

func newApp() *app {
	osmPath := env("MAP_OSM_PATH", "/data/osm/turkmenistan-latest.osm.pbf")
	return &app{
		log:             newLogger(env("LOG_FILE", "/var/log/taxi/map-service.log")),
		version:         "tm-bootstrap",
		client:          &http.Client{Timeout: 12 * time.Second},
		osmSource:       env("MAP_OSM_SOURCE_URL", defaultOSMSource),
		osmPath:         osmPath,
		osmMetaPath:     osmPath + ".metadata.json",
		tileStoragePath: env("MAP_STORAGE_PATH", "/data/tiles"),
		syncHourUTC:     envInt("MAP_SYNC_HOUR_UTC", 3),
	}
}

func (a *app) root(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/swagger", http.StatusFound)
}

func (a *app) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"ok": true, "service": "map-service", "time": time.Now().UTC()})
}

func (a *app) swaggerUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, swaggerHTML)
}

func (a *app) openapi(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	_, _ = io.WriteString(w, openapiYAML)
}

func (a *app) versionHandler(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	version := a.version
	a.mu.RUnlock()
	a.log.write("info", "get_version", nil)
	writeJSON(w, map[string]any{"region": "turkmenistan", "version": version, "updatedAt": time.Now().UTC()})
}

func (a *app) manifest(w http.ResponseWriter, r *http.Request) {
	region := r.URL.Query().Get("region")
	if region == "" {
		region = "turkmenistan"
	}
	a.mu.RLock()
	version := a.version
	a.mu.RUnlock()
	tiles := []map[string]any{}
	for z := 5; z <= 12; z++ {
		tiles = append(tiles, map[string]any{
			"z": z,
			"checksum": checksum(fmt.Sprintf("%s:%s:%d", region, version, z)),
			"urlTemplate": "/tiles/{z}/{x}/{y}.png",
		})
	}
	a.log.write("info", "get_manifest", map[string]any{"region": region, "tile_groups": len(tiles)})
	writeJSON(w, map[string]any{
		"region": region,
		"version": version,
		"strategy": "delta",
		"nightlySyncHour": a.syncHourUTC,
		"tiles": tiles,
	})
}

func (a *app) delta(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	a.mu.RLock()
	version := a.version
	a.mu.RUnlock()
	if to == "" {
		to = version
	}
	changed := []map[string]any{}
	if from != to {
		changed = append(changed, map[string]any{"z": 10, "x": 637, "y": 412, "checksum": checksum(to+":10:637:412")})
	}
	a.log.write("info", "delta_request", map[string]any{"from": from, "to": to, "changed": len(changed), "deleted": 0})
	writeJSON(w, map[string]any{"from": from, "to": to, "changed": changed, "deleted": []string{}})
}

func (a *app) downloadInfo(w http.ResponseWriter, r *http.Request) {
	stat, err := os.Stat(a.osmPath)
	meta, _ := readOSMMetadata(a.osmMetaPath)
	a.mu.RLock()
	version := a.version
	a.mu.RUnlock()
	info := map[string]any{
		"source": a.osmSource,
		"path": a.osmPath,
		"metadataPath": a.osmMetaPath,
		"exists": false,
		"version": version,
		"syncHourUTC": a.syncHourUTC,
		"tileStoragePath": a.tileStoragePath,
	}
	if err == nil {
		info["exists"] = true
		info["sizeBytes"] = stat.Size()
		info["modifiedAt"] = stat.ModTime().UTC()
	}
	if meta != nil {
		info["etag"] = meta.ETag
		info["lastModified"] = meta.LastModified
		info["contentLength"] = meta.ContentLength
		info["downloadedAt"] = meta.DownloadedAt
	}
	a.log.write("info", "download_info", info)
	writeJSON(w, info)
}

func (a *app) tile(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/tiles/"), "/")
	if len(parts) != 3 {
		http.NotFound(w, r)
		return
	}
	yRaw := strings.TrimSuffix(parts[2], ".png")
	if !strings.HasSuffix(parts[2], ".png") {
		http.NotFound(w, r)
		return
	}
	z, zErr := strconv.Atoi(parts[0])
	x, xErr := strconv.Atoi(parts[1])
	y, yErr := strconv.Atoi(yRaw)
	if zErr != nil || xErr != nil || yErr != nil {
		http.NotFound(w, r)
		return
	}
	if a.serveCachedTile(w, z, x, y) {
		a.log.write("info", "tile_request", map[string]any{"z": z, "x": x, "y": y, "status": "cache_hit"})
		return
	}
	if a.fetchAndCacheOSMTile(w, z, x, y) {
		a.log.write("info", "tile_request", map[string]any{"z": z, "x": x, "y": y, "status": "cache_miss_downloaded"})
		return
	}
	a.log.write("error", "tile_request", map[string]any{"z": z, "x": x, "y": y, "status": "osm_unavailable"})
	http.Error(w, "upstream tile unavailable", http.StatusBadGateway)
}

func (a *app) serveCachedTile(w http.ResponseWriter, z, x, y int) bool {
	path := a.tilePath(z, x, y)
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=604800")
	_, _ = io.Copy(w, f)
	return true
}

func (a *app) fetchAndCacheOSMTile(w http.ResponseWriter, z, x, y int) bool {
	url := fmt.Sprintf("https://tile.openstreetmap.org/%d/%d/%d.png", z, x, y)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "TaxiTM-MVP/1.0 local map-service")
	resp, err := a.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}
	path := a.tilePath(z, x, y)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err == nil {
		tmp := path + ".tmp"
		if writeErr := os.WriteFile(tmp, body, 0644); writeErr == nil {
			_ = os.Rename(tmp, path)
		}
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=604800")
	_, _ = w.Write(body)
	return true
}

func (a *app) tilePath(z, x, y int) string {
	return filepath.Join(a.tileStoragePath, strconv.Itoa(z), strconv.Itoa(x), strconv.Itoa(y)+".png")
}

func (a *app) runNightlySync() {
	for {
		wait := durationUntilNextHourUTC(a.syncHourUTC)
		a.log.write("info", "nightly_sync_scheduled", map[string]any{"syncHourUTC": a.syncHourUTC, "waitSeconds": int(wait.Seconds())})
		time.Sleep(wait)
		a.syncOSM("nightly")
		time.Sleep(time.Minute)
	}
}

func (a *app) syncOSM(reason string) {
	remote, err := a.fetchRemoteOSMMetadata()
	if err != nil {
		a.log.write("error", "osm_remote_metadata_failed", map[string]any{"reason": reason, "error": err.Error()})
		if stat, statErr := os.Stat(a.osmPath); statErr == nil {
			if local, readErr := readOSMMetadata(a.osmMetaPath); readErr == nil && local != nil {
				a.setVersion(local.Version)
			} else {
				fallback := &osmMetadata{
					Source:        a.osmSource,
					LastModified:  stat.ModTime().UTC().Format(http.TimeFormat),
					ContentLength: stat.Size(),
					DownloadedAt:  stat.ModTime().UTC(),
				}
				fallback.Version = mapDataVersion(fallback)
				_ = writeOSMMetadata(a.osmMetaPath, fallback)
				a.setVersion(fallback.Version)
			}
			return
		}
		remote = &osmMetadata{Source: a.osmSource}
	}

	local, _ := readOSMMetadata(a.osmMetaPath)
	stat, statErr := os.Stat(a.osmPath)
	hasLocalFile := statErr == nil

	if hasLocalFile && local != nil && !osmChanged(local, remote) {
		a.setVersion(local.Version)
		a.log.write("info", "osm_sync_skipped", map[string]any{
			"reason": reason,
			"path": a.osmPath,
			"version": local.Version,
			"sizeBytes": stat.Size(),
		})
		return
	}

	if hasLocalFile && local == nil && remote.ContentLength > 0 && stat.Size() == remote.ContentLength {
		remote.Version = mapDataVersion(remote)
		remote.DownloadedAt = time.Now().UTC()
		if err := writeOSMMetadata(a.osmMetaPath, remote); err == nil {
			a.setVersion(remote.Version)
			a.log.write("info", "osm_metadata_recreated", map[string]any{"reason": reason, "version": remote.Version})
			return
		}
	}

	if err := a.downloadOSM(remote, reason); err != nil {
		a.log.write("error", "osm_download_failed", map[string]any{"reason": reason, "error": err.Error()})
		return
	}
}

func (a *app) fetchRemoteOSMMetadata() (*osmMetadata, error) {
	req, err := http.NewRequest(http.MethodHead, a.osmSource, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "TaxiTM-MapService/1.0")
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return metadataFromHeaders(a.osmSource, resp.Header, resp.ContentLength), nil
}

func (a *app) downloadOSM(meta *osmMetadata, reason string) error {
	req, err := http.NewRequest(http.MethodGet, a.osmSource, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "TaxiTM-MapService/1.0")
	downloadClient := &http.Client{}
	resp, err := downloadClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	if meta == nil {
		meta = metadataFromHeaders(a.osmSource, resp.Header, resp.ContentLength)
	} else {
		mergeMetadata(meta, metadataFromHeaders(a.osmSource, resp.Header, resp.ContentLength))
	}

	if err := os.MkdirAll(filepath.Dir(a.osmPath), 0755); err != nil {
		return err
	}
	tmp := a.osmPath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	size, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	if err := os.Rename(tmp, a.osmPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if meta.ContentLength <= 0 {
		meta.ContentLength = size
	}
	if parsed, err := http.ParseTime(meta.LastModified); err == nil {
		_ = os.Chtimes(a.osmPath, parsed, parsed)
	}
	meta.DownloadedAt = time.Now().UTC()
	meta.Version = mapDataVersion(meta)
	if err := writeOSMMetadata(a.osmMetaPath, meta); err != nil {
		return err
	}
	a.setVersion(meta.Version)
	a.log.write("info", "osm_downloaded", map[string]any{
		"reason": reason,
		"path": a.osmPath,
		"version": meta.Version,
		"sizeBytes": size,
	})
	return nil
}

func (a *app) setVersion(version string) {
	if version == "" {
		return
	}
	a.mu.Lock()
	a.version = version
	a.mu.Unlock()
}

func metadataFromHeaders(source string, header http.Header, contentLength int64) *osmMetadata {
	return &osmMetadata{
		Source:        source,
		ETag:          header.Get("ETag"),
		LastModified:  header.Get("Last-Modified"),
		ContentLength: contentLength,
	}
}

func mergeMetadata(dst, src *osmMetadata) {
	if dst.Source == "" {
		dst.Source = src.Source
	}
	if src.ETag != "" {
		dst.ETag = src.ETag
	}
	if src.LastModified != "" {
		dst.LastModified = src.LastModified
	}
	if src.ContentLength > 0 {
		dst.ContentLength = src.ContentLength
	}
}

func osmChanged(local, remote *osmMetadata) bool {
	if local == nil || remote == nil {
		return true
	}
	if local.ETag != "" && remote.ETag != "" && local.ETag != remote.ETag {
		return true
	}
	if local.LastModified != "" && remote.LastModified != "" && local.LastModified != remote.LastModified {
		return true
	}
	if local.ContentLength > 0 && remote.ContentLength > 0 && local.ContentLength != remote.ContentLength {
		return true
	}
	if local.ETag == "" && local.LastModified == "" && local.ContentLength <= 0 {
		return true
	}
	return false
}

func mapDataVersion(meta *osmMetadata) string {
	raw := fmt.Sprintf("%s:%s:%s:%d", meta.Source, meta.ETag, meta.LastModified, meta.ContentLength)
	if meta.ETag == "" && meta.LastModified == "" && meta.ContentLength <= 0 {
		raw = fmt.Sprintf("%s:%s", meta.Source, meta.DownloadedAt.Format(time.RFC3339Nano))
	}
	return "tm-" + checksum(raw)
}

func readOSMMetadata(path string) (*osmMetadata, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var meta osmMetadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func writeOSMMetadata(path string, meta *osmMetadata) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0644)
}

func durationUntilNextHourUTC(hour int) time.Duration {
	if hour < 0 || hour > 23 {
		hour = 3
	}
	now := time.Now().UTC()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, time.UTC)
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next.Sub(now)
}

func (a *app) fallbackTile(w http.ResponseWriter, z, x, y int) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=300")
	img := image.NewRGBA(image.Rect(0, 0, 256, 256))
	bg := color.RGBA{R: 214, G: 224, B: 213, A: 255}
	if (x+y+z)%2 == 0 {
		bg = color.RGBA{R: 207, G: 222, B: 229, A: 255}
	}
	for py := 0; py < 256; py++ {
		for px := 0; px < 256; px++ {
			img.Set(px, py, bg)
		}
	}
	block := color.RGBA{R: 188, G: 207, B: 181, A: 255}
	water := color.RGBA{R: 126, G: 176, B: 205, A: 255}
	major := color.RGBA{R: 226, G: 157, B: 47, A: 255}
	minor := color.RGBA{R: 248, G: 244, B: 226, A: 255}
	border := color.RGBA{R: 108, G: 126, B: 111, A: 255}
	for py := 28; py < 116; py++ {
		for px := 34; px < 134; px++ {
			if (px+py+x+y)%9 != 0 {
				img.Set(px, py, block)
			}
		}
	}
	for py := 154; py < 206; py++ {
		for px := 148; px < 234; px++ {
			if (px*3+py+y)%11 != 0 {
				img.Set(px, py, water)
			}
		}
	}
	drawWideLine(img, 0, (35+y*7)%256, 255, (205+x*5)%256, major, 5)
	drawWideLine(img, (18+x*11)%256, 255, (228+y*13)%256, 0, major, 4)
	drawWideLine(img, 0, (118+x*3)%256, 255, (92+y*4)%256, minor, 3)
	drawWideLine(img, (72+y*5)%256, 0, (138+x*9)%256, 255, minor, 3)
	for i := 0; i < 256; i += 32 {
		drawWideLine(img, i, 0, (i+48+x)%256, 255, border, 1)
		drawWideLine(img, 0, i, 255, (i+36+y)%256, border, 1)
	}
	_ = png.Encode(w, img)
}

func drawWideLine(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA, width int) {
	dx := abs(x1 - x0)
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	dy := -abs(y1 - y0)
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy
	for {
		for oy := -width; oy <= width; oy++ {
			for ox := -width; ox <= width; ox++ {
				px, py := x0+ox, y0+oy
				if px >= 0 && px < 256 && py >= 0 && py < 256 {
					img.Set(px, py, c)
				}
			}
		}
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func checksum(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

func envInt(k string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
