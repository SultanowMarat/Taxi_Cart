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
	"strconv"
	"strings"
	"time"
)

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
	log     *logger
	version string
	client  *http.Client
}

func main() {
	a := &app{log: newLogger(env("LOG_FILE", "/var/log/taxi/map-service.log")), version: "tm-2026.06-demo", client: &http.Client{Timeout: 6 * time.Second}}
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
	a.log.write("info", "get_version", nil)
	writeJSON(w, map[string]any{"region": "turkmenistan", "version": a.version, "updatedAt": time.Now().UTC()})
}

func (a *app) manifest(w http.ResponseWriter, r *http.Request) {
	region := r.URL.Query().Get("region")
	if region == "" {
		region = "turkmenistan"
	}
	tiles := []map[string]any{}
	for z := 5; z <= 12; z++ {
		tiles = append(tiles, map[string]any{
			"z": z,
			"checksum": checksum(fmt.Sprintf("%s:%s:%d", region, a.version, z)),
			"urlTemplate": "/tiles/{z}/{x}/{y}.png",
		})
	}
	a.log.write("info", "get_manifest", map[string]any{"region": region, "tile_groups": len(tiles)})
	writeJSON(w, map[string]any{
		"region": region,
		"version": a.version,
		"strategy": "delta",
		"nightlySyncHour": 3,
		"tiles": tiles,
	})
}

func (a *app) delta(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if to == "" {
		to = a.version
	}
	changed := []map[string]any{}
	if from != to {
		changed = append(changed, map[string]any{"z": 10, "x": 637, "y": 412, "checksum": checksum(to+":10:637:412")})
	}
	a.log.write("info", "delta_request", map[string]any{"from": from, "to": to, "changed": len(changed), "deleted": 0})
	writeJSON(w, map[string]any{"from": from, "to": to, "changed": changed, "deleted": []string{}})
}

func (a *app) downloadInfo(w http.ResponseWriter, r *http.Request) {
	path := env("MAP_OSM_PATH", "/data/osm/turkmenistan-latest.osm.pbf")
	stat, err := os.Stat(path)
	info := map[string]any{
		"source": "https://download.geofabrik.de/asia/turkmenistan-latest.osm.pbf",
		"path": path,
		"exists": false,
	}
	if err == nil {
		info["exists"] = true
		info["sizeBytes"] = stat.Size()
		info["modifiedAt"] = stat.ModTime().UTC()
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
	z, _ := strconv.Atoi(parts[0])
	x, _ := strconv.Atoi(parts[1])
	yRaw := strings.TrimSuffix(parts[2], ".png")
	y, _ := strconv.Atoi(yRaw)
	if a.proxyOSMTile(w, z, x, y) {
		a.log.write("info", "tile_request", map[string]any{"z": z, "x": x, "y": y, "status": "osm_proxy"})
		return
	}
	a.log.write("error", "tile_request", map[string]any{"z": z, "x": x, "y": y, "status": "osm_unavailable"})
	http.Error(w, "upstream tile unavailable", http.StatusBadGateway)
}

func (a *app) proxyOSMTile(w http.ResponseWriter, z, x, y int) bool {
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
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = io.Copy(w, resp.Body)
	return true
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
