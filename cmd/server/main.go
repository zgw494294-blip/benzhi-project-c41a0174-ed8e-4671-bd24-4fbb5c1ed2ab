package main

import (
	"bytes"
	"cityflood/internal/application"
	"cityflood/internal/httpapi"
	"cityflood/internal/storage"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func main() {
	addr := flag.String("addr", "", "监听地址")
	self := flag.Bool("selfcheck", false, "执行有界自检")
	data := flag.String("data", ".benzhi/data", "数据目录")
	flag.Parse()
	actual := *addr
	if actual == "" {
		if p := os.Getenv("PORT"); p != "" {
			actual = "127.0.0.1:" + p
		} else {
			actual = "127.0.0.1:19081"
		}
	}
	st, err := storage.New(*data)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	_ = st.Load()
	if *self {
		// 自检使用隔离内存状态，保证重复执行始终有界且幂等。
		st, _ = storage.New("")
	}
	app := application.New(st)
	api := httpapi.New(app)
	srv := &http.Server{Addr: actual, Handler: api.Handler(), ReadHeaderTimeout: 3 * time.Second}
	if *self {
		go srv.ListenAndServe()
		time.Sleep(80 * time.Millisecond)
		if err := runSelfcheck(actual); err != nil {
			fmt.Println("自检失败:", err)
			srv.Close()
			os.Exit(1)
		}
		srv.Close()
		fmt.Println("自检通过")
		return
	}
	fmt.Println("服务监听", actual)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Println(err)
		os.Exit(1)
	}
}
func runSelfcheck(addr string) error {
	base := "http://" + addr
	post := func(path string, v any) (map[string]any, error) {
		raw, _ := json.Marshal(v)
		resp, err := http.Post(base+path, "application/json", bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("%s %s", path, string(b))
		}
		var out map[string]any
		_ = json.Unmarshal(b, &out)
		return out, nil
	}
	if _, err := post("/api/facilities", map[string]any{"facilityID": "self-facility", "name": "自检调蓄池", "district": "测试区", "facilityType": "lake", "designCapacity": 1000, "normalWaterLevel": 10, "gateCount": 2, "pumpCount": 1, "emergencyRoute": "clear"}); err != nil {
		return err
	}
	if _, err := post("/api/facilities/self-facility/inspection-batches", map[string]any{"batchID": "self-batch", "inspectionWindow": "2026汛前", "windowStart": "2026-01-01T00:00:00+08:00", "windowEnd": "2026-12-31T00:00:00+08:00", "inspectorID": "inspector"}); err != nil {
		return err
	}
	if _, err := post("/api/inspection-batches/self-batch/items", map[string]any{"actor": "inspector", "items": []any{map[string]any{"itemID": "w", "metric": "water_level", "value": 8, "unit": "m", "observation": "正常", "evidenceRefs": []string{"self-w"}, "capturedBy": "inspector"}, map[string]any{"itemID": "g", "metric": "gate", "value": 2, "unit": "扇", "observation": "正常", "evidenceRefs": []string{"self-g"}, "capturedBy": "inspector"}, map[string]any{"itemID": "p", "metric": "pump", "value": 1, "unit": "台", "observation": "正常", "evidenceRefs": []string{"self-p"}, "capturedBy": "inspector"}, map[string]any{"itemID": "e", "metric": "emergency_route", "value": 1, "unit": "状态", "observation": "畅通", "evidenceRefs": []string{"self-e"}, "capturedBy": "inspector"}}}); err != nil {
		return err
	}
	if _, err := post("/api/inspection-batches/self-batch/review", map[string]any{"pass": true, "actor": "reviewer"}); err != nil {
		return err
	}
	if _, err := post("/api/inspection-batches/self-batch/permit", map[string]any{"issuedBy": "reviewer"}); err != nil {
		return err
	}
	return nil
}
