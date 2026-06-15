package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math"
	mrand "math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// ================== 配置 ==================

type GateConfig struct {
	ID           uint    `json:"id"`
	Name         string  `json:"name"`
	BaseUp       float64 `json:"base_up"`       // 基础上游水位
	BaseDown     float64 `json:"base_down"`     // 基础下游水位
	Amplitude    float64 `json:"amplitude"`     // 水位波动振幅
	PeriodSec    int     `json:"period_sec"`    // 水位波动周期
	NoiseStddev  float64 `json:"noise_stddev"`  // 测量噪声
	MaxWidth     float64 `json:"max_width"`
	MaxHeight    float64 `json:"max_height"`
	FaultRate    float64 `json:"fault_rate"`    // 传感器故障率
}

type SimulatorConfig struct {
	GateCount       int      `json:"gate_count"`
	IntervalMs      int      `json:"interval_ms"`
	MqttBroker      string   `json:"mqtt_broker"`
	MqttTopic       string   `json:"mqtt_topic"`
	HttpEndpoint    string   `json:"http_endpoint"`
	ShipGenRate     float64  `json:"ship_generation_rate"`
	WaterNoise      float64  `json:"water_level_noise"`
	CustomGates     []GateConfig `json:"gates,omitempty"`
	StartTime       time.Time
}

// ================== 内部状态 ==================

type GateState struct {
	sync.RWMutex
	cfg          GateConfig
	phase        float64       // 当前波形相位
	opening      float64       // 闸门开度 0-1
	openingState string        // open/opening/closing/closed
	flowRate     float64
	lastShipTime time.Time
	faultMode    string        // "none" / "stuck" / "spike" / "dropout"
	faultUntil   time.Time
}

type ShipEvent struct {
	GateID      uint      `json:"gate_id"`
	ShipName    string    `json:"ship_name"`
	Direction   string    `json:"direction"`
	Priority    int       `json:"priority"`
	Time        time.Time `json:"time"`
	Operate     string    `json:"operate"`   // arrival / start_fill / enter / exit
	Length      float64   `json:"length"`
	Width       float64   `json:"width"`
	Draft       float64   `json:"draft"`
}

type SensorPayload struct {
	Time           time.Time `json:"time"`
	GateID         uint      `json:"gate_id"`
	WaterLevelUp   float64   `json:"water_level_up"`
	WaterLevelDown float64   `json:"water_level_down"`
	GateOpening    float64   `json:"gate_opening"`
	FlowRate       float64   `json:"flow_rate"`
	PassageTime    float64   `json:"passage_time"`
	Status         string    `json:"status"`
}

// ================== 全局 ==================

var (
	cfg     SimulatorConfig
	gates   map[uint]*GateState
	gatesMu sync.RWMutex

	sentSensorCount uint64
	sentShipCount   uint64
	mqttClient      mqtt.Client
	sentMqttCount   uint64
	sentHttpCount   uint64
)

// ================== 初始化 ==================

func loadConfig() {
	if data, err := os.ReadFile("config.json"); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}

	// 环境变量覆盖
	if v := os.Getenv("SIM_GATE_COUNT"); v != "" { cfg.GateCount, _ = strconv.Atoi(v) }
	if v := os.Getenv("SIM_INTERVAL_MS"); v != "" { cfg.IntervalMs, _ = strconv.Atoi(v) }
	if v := os.Getenv("SIM_MQTT_BROKER"); v != "" { cfg.MqttBroker = v }
	if v := os.Getenv("SIM_MQTT_TOPIC"); v != "" { cfg.MqttTopic = v }
	if v := os.Getenv("SIM_HTTP_ENDPOINT"); v != "" { cfg.HttpEndpoint = v }
	if v := os.Getenv("SIM_WATER_LEVEL_NOISE"); v != "" { cfg.WaterNoise, _ = strconv.ParseFloat(v, 64) }
	if v := os.Getenv("SIM_FAULT_RATE"); v != "" {
		fr, _ := strconv.ParseFloat(v, 64)
		for i := range cfg.CustomGates { cfg.CustomGates[i].FaultRate = fr }
	}
	if v := os.Getenv("SIM_SHIP_GENERATION_RATE"); v != "" { cfg.ShipGenRate, _ = strconv.ParseFloat(v, 64) }

	// 默认值
	if cfg.GateCount == 0 { cfg.GateCount = 36 }
	if cfg.IntervalMs == 0 { cfg.IntervalMs = 1000 }
	if cfg.MqttBroker == "" { cfg.MqttBroker = "tcp://localhost:1883" }
	if cfg.MqttTopic == "" { cfg.MqttTopic = "lingqu/sensors" }
	if cfg.WaterNoise == 0 { cfg.WaterNoise = 0.1 }
	if cfg.ShipGenRate == 0 { cfg.ShipGenRate = 0.05 }

	cfg.StartTime = time.Now()

	log.Printf("Simulator config: gates=%d, interval=%dms, mqtt=%s, http=%s, ship_rate=%.3f",
		cfg.GateCount, cfg.IntervalMs, cfg.MqttBroker, cfg.HttpEndpoint, cfg.ShipGenRate)
}

func buildGates() {
	gates = make(map[uint]*GateState)
	gatesMu.Lock()
	defer gatesMu.Unlock()

	// 使用自定义配置或生成36座标准陡门
	customGates := cfg.CustomGates
	if len(customGates) == 0 {
		for i := 1; i <= cfg.GateCount; i++ {
			// 上游水位: 7.0 ± 1.0 渐变；下游水位: 3.0 ± 1.0 渐变
			baseUp := 6.5 + (6.5 * math.Sin(float64(i)/8.0)) * 0.15
			baseDown := 3.5 + (3.5 * math.Cos(float64(i)/6.0)) * 0.2
			customGates = append(customGates, GateConfig{
				ID:          uint(i),
				Name:        fmt.Sprintf("陡门%d号", i),
				BaseUp:      round2(baseUp),
				BaseDown:    round2(baseDown),
				Amplitude:   0.3 + mrand.Float64()*0.4,
				PeriodSec:   300 + mrand.Intn(300),
				NoiseStddev: cfg.WaterNoise,
				MaxWidth:    6.0,
				MaxHeight:   8.0,
				FaultRate:   0.001,
			})
		}
	}

	for _, gc := range customGates {
		gates[gc.ID] = &GateState{
			cfg:          gc,
			phase:        mrand.Float64() * 2 * math.Pi,
			opening:      0.0,
			openingState: "closed",
			lastShipTime: cfg.StartTime.Add(-time.Duration(mrand.Intn(600)) * time.Second),
			faultMode:    "none",
		}
	}
	log.Printf("Built %d gates", len(gates))
}

func initMQTT() {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.MqttBroker)
	clientID := fmt.Sprintf("sensor_sim_%x", randomHex(6))
	opts.SetClientID(clientID)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(10 * time.Second)
	opts.SetCleanSession(true)

	opts.OnConnect = func(c mqtt.Client) {
		log.Printf("MQTT connected to %s as %s", cfg.MqttBroker, clientID)
	}
	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		log.Printf("MQTT connection lost: %v", err)
	}

	mqttClient = mqtt.NewClient(opts)
	go func() {
		for i := 0; i < 60; i++ {
			if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
				log.Printf("MQTT connect attempt %d failed: %v (retrying in 5s)", i+1, token.Error())
				time.Sleep(5 * time.Second)
			} else {
				return
			}
		}
		log.Println("MQTT gave up connecting; running in HTTP-only mode")
	}()
}

// ================== 运行循环 ==================

func runTick(t time.Time) {
	elapsed := t.Sub(cfg.StartTime).Seconds()

	gatesMu.RLock()
	gateIDs := make([]uint, 0, len(gates))
	for id := range gates { gateIDs = append(gateIDs, id) }
	gatesMu.RUnlock()

	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)

	for _, id := range gateIDs {
		id := id
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			gatesMu.RLock()
			gs := gates[id]
			gatesMu.RUnlock()
			if gs == nil { return }

			// 计算传感器值
			payload := computeSensor(gs, elapsed, t)

			// 发送
			publishSensor(payload)

			// 处理船舶事件
			maybeGenShip(gs, t)

			// 故障注入
			maybeFault(gs, t)
		}()
	}
	wg.Wait()
}

func computeSensor(gs *GateState, elapsed float64, t time.Time) SensorPayload {
	gs.Lock()
	defer gs.Unlock()

	c := gs.cfg

	// 正弦水位 + 噪声
	sinPart := math.Sin(gs.phase + 2*math.Pi*elapsed/float64(c.PeriodSec))
	up := c.BaseUp + c.Amplitude*sinPart + gaussian(c.NoiseStddev)
	down := c.BaseDown - c.Amplitude*0.7*sinPart + gaussian(c.NoiseStddev)

	// 故障模式
	switch gs.faultMode {
	case "spike":
		up += 2.0 * (2*mrand.Float64() - 1)
		down += 2.0 * (2*mrand.Float64() - 1)
	case "dropout":
		if t.After(gs.faultUntil) {
			gs.faultMode = "none"
		} else {
			up = -1
			down = -1
		}
	}

	// 闸门开度: 船舶到达后先开闸/再关闸
	if gs.openingState == "opening" {
		gs.opening += 0.02
		if gs.opening >= 1.0 { gs.opening = 1.0; gs.openingState = "open" }
	} else if gs.openingState == "closing" {
		gs.opening -= 0.015
		if gs.opening <= 0 { gs.opening = 0; gs.openingState = "closed" }
	}

	// 简单流量估算: Q=Cc*b*e*sqrt(2gΔH)
	flow := 0.0
	if up > down && gs.opening > 0 {
		dh := math.Max(up - down, 0.01)
		flow = 0.62 * c.MaxWidth * gs.opening * c.MaxHeight * math.Sqrt(2*9.81*dh)
	}
	gs.flowRate = flow

	status := "normal"
	if gs.faultMode != "none" && gs.faultMode != "stuck" { status = "fault" }
	if up > c.BaseUp+0.8 || down > c.BaseDown+0.8 { status = "warning" }

	return SensorPayload{
		Time:           t,
		GateID:         c.ID,
		WaterLevelUp:   round2(up),
		WaterLevelDown: round2(down),
		GateOpening:    round2(gs.opening),
		FlowRate:       round2(flow),
		PassageTime:    0,
		Status:         status,
	}
}

func maybeFault(gs *GateState, t time.Time) {
	if gs.cfg.FaultRate <= 0 { return }
	if mrand.Float64() < gs.cfg.FaultRate {
		gs.Lock()
		defer gs.Unlock()
		modes := []string{"stuck", "spike", "dropout"}
		mode := modes[mrand.Intn(len(modes))]
		gs.faultMode = mode
		gs.faultUntil = t.Add(time.Duration(60+mrand.Intn(300)) * time.Second)
		log.Printf("[GATE %d] Fault injected: %s until %s", gs.cfg.ID, mode, gs.faultUntil.Format("15:04:05"))
	} else if gs.faultMode != "none" {
		gs.Lock()
		if t.After(gs.faultUntil) {
			log.Printf("[GATE %d] Fault %s recovered", gs.cfg.ID, gs.faultMode)
			gs.faultMode = "none"
		}
		gs.Unlock()
	}
}

func maybeGenShip(gs *GateState, t time.Time) {
	gs.RLock()
	tooSoon := t.Sub(gs.lastShipTime) < 10*time.Minute
	gs.RUnlock()
	if tooSoon { return }

	if mrand.Float64() < cfg.ShipGenRate {
		gs.Lock()
		gs.lastShipTime = t
		gs.openingState = "opening"
		gs.Unlock()

		direction := "upstream"
		if mrand.Intn(2) == 0 { direction = "downstream" }
		priority := 1 + mrand.Intn(5)

		ship := ShipEvent{
			GateID:    gs.cfg.ID,
			ShipName:  fmt.Sprintf("%s-%c%d", "灵运号灵运渡灵船商运", 'A'+mrand.Intn(26), mrand.Intn(9000)+1000),
			Direction: direction,
			Priority:  priority,
			Time:      t,
			Operate:   "arrival",
			Length:    10 + mrand.Float64()*40,
			Width:     3 + mrand.Float64()*2.5,
			Draft:     1.2 + mrand.Float64()*1.8,
		}
		publishShip(ship)

		// 5分钟后模拟关闸
		go func(id uint, delay time.Duration) {
			time.Sleep(delay)
			gatesMu.RLock()
			gs := gates[id]
			gatesMu.RUnlock()
			if gs != nil {
				gs.Lock()
				gs.openingState = "closing"
				gs.Unlock()
			}
		}(gs.cfg.ID, 4*time.Minute+time.Duration(mrand.Intn(120))*time.Second)
	}
}

// ================== 发送 ==================

func publishSensor(p SensorPayload) {
	atomic.AddUint64(&sentSensorCount, 1)
	data, err := json.Marshal(p)
	if err != nil { return }

	// MQTT
	if mqttClient != nil && mqttClient.IsConnected() {
		topic := fmt.Sprintf("%s/%d", cfg.MqttTopic, p.GateID)
		if token := mqttClient.Publish(topic, 0, false, data); token.WaitTimeout(2*time.Second) && token.Error() == nil {
			atomic.AddUint64(&sentMqttCount, 1)
		}
	}

	// HTTP
	if cfg.HttpEndpoint != "" {
		go func() {
			client := &http.Client{Timeout: 3 * time.Second}
			if r, err := client.Post(cfg.HttpEndpoint, "application/json", bytes.NewReader(data)); err == nil {
				if r.StatusCode < 300 { atomic.AddUint64(&sentHttpCount, 1) }
				r.Body.Close()
			}
		}()
	}
}

func publishShip(s ShipEvent) {
	atomic.AddUint64(&sentShipCount, 1)
	// MQTT广播船舶事件
	if mqttClient != nil && mqttClient.IsConnected() {
		data, _ := json.Marshal(s)
		topic := fmt.Sprintf("lingqu/ships/%d", s.GateID)
		mqttClient.Publish(topic, 0, false, data)
	}
	log.Printf("[SHIP] %s gate=%d dir=%s pri=%d", s.ShipName, s.GateID, s.Direction, s.Priority)
}

// ================== HTTP 管理接口 ==================

func startAdminHTTP() {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":         "ok",
			"uptime_sec":     time.Since(cfg.StartTime).Seconds(),
			"sensors_sent":   atomic.LoadUint64(&sentSensorCount),
			"ships_sent":     atomic.LoadUint64(&sentShipCount),
			"mqtt_sent":      atomic.LoadUint64(&sentMqttCount),
			"http_sent":      atomic.LoadUint64(&sentHttpCount),
			"mqtt_connected": mqttClient != nil && mqttClient.IsConnected(),
			"gate_count":     len(gates),
		})
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "# HELP simulator_sensor_sent_total 已发送传感器数据总数\n")
		fmt.Fprintf(w, "# TYPE simulator_sensor_sent_total counter\n")
		fmt.Fprintf(w, "simulator_sensor_sent_total %d\n", atomic.LoadUint64(&sentSensorCount))
		fmt.Fprintf(w, "# HELP simulator_ships_generated_total 已生成船舶事件总数\n")
		fmt.Fprintf(w, "# TYPE simulator_ships_generated_total counter\n")
		fmt.Fprintf(w, "simulator_ships_generated_total %d\n", atomic.LoadUint64(&sentShipCount))
		fmt.Fprintf(w, "# HELP simulator_mqtt_sent_total 已发送MQTT消息数\n")
		fmt.Fprintf(w, "# TYPE simulator_mqtt_sent_total counter\n")
		fmt.Fprintf(w, "simulator_mqtt_sent_total %d\n", atomic.LoadUint64(&sentMqttCount))
		fmt.Fprintf(w, "# HELP simulator_http_sent_total 已发送HTTP POST数\n")
		fmt.Fprintf(w, "# TYPE simulator_http_sent_total counter\n")
		fmt.Fprintf(w, "simulator_http_sent_total %d\n", atomic.LoadUint64(&sentHttpCount))
		fmt.Fprintf(w, "# HELP simulator_gate_count 模拟的陡门数量\n")
		fmt.Fprintf(w, "# TYPE simulator_gate_count gauge\n")
		fmt.Fprintf(w, "simulator_gate_count %d\n", len(gates))
	})

	// 注入故障: POST /inject-fault?gate=1&mode=spike&minutes=5
	mux.HandleFunc("/inject-fault", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { http.Error(w, "POST only", 405); return }
		q := r.URL.Query()
		gid, _ := strconv.Atoi(q.Get("gate"))
		mode := q.Get("mode")
		mins, _ := strconv.Atoi(q.Get("minutes"))
		if mode == "" { mode = "spike" }
		if mins <= 0 { mins = 5 }

		gatesMu.RLock()
		gs := gates[uint(gid)]
		gatesMu.RUnlock()
		if gs == nil {
			http.Error(w, "gate not found", 404)
			return
		}
		gs.Lock()
		gs.faultMode = mode
		gs.faultUntil = time.Now().Add(time.Duration(mins) * time.Minute)
		gs.Unlock()
		log.Printf("Injected %s on gate %d for %d min", mode, gid, mins)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// 生成船舶: POST /gen-ship?gate=1&direction=upstream&priority=3
	mux.HandleFunc("/gen-ship", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { http.Error(w, "POST only", 405); return }
		q := r.URL.Query()
		gid, _ := strconv.Atoi(q.Get("gate"))
		if gid == 0 { gid = 1 + mrand.Intn(cfg.GateCount) }
		direction := q.Get("direction")
		if direction == "" { direction = []string{"upstream","downstream"}[mrand.Intn(2)] }
		priority, _ := strconv.Atoi(q.Get("priority"))
		if priority == 0 { priority = 1 + mrand.Intn(5) }

		gatesMu.RLock()
		gs := gates[uint(gid)]
		gatesMu.RUnlock()
		if gs == nil { http.Error(w, "gate not found", 404); return }
		gs.Lock()
		gs.lastShipTime = time.Now()
		gs.openingState = "opening"
		gs.Unlock()
		ship := ShipEvent{
			GateID: uint(gid), ShipName: fmt.Sprintf("注入船-%04d", atomic.LoadUint64(&sentShipCount)+1),
			Direction: direction, Priority: priority, Time: time.Now(), Operate: "arrival",
			Length: 15 + mrand.Float64()*30, Width: 3 + mrand.Float64()*2, Draft: 1.5 + mrand.Float64(),
		}
		publishShip(ship)
		w.Write([]byte(`{"status":"ok","ship":"` + ship.ShipName + `"}`))
	})

	mux.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg)
	})

	addr := ":9090"
	log.Printf("Admin HTTP on %s [/health, /metrics, /inject-fault, /gen-ship, /config]", addr)
	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Fatalf("Admin HTTP error: %v", err)
		}
	}()
}

// ================== 工具函数 ==================

func round2(v float64) float64 { return math.Round(v*100) / 100 }
func gaussian(stddev float64) float64 {
	// Box-Muller
	u1 := mrand.Float64()
	u2 := mrand.Float64()
	return stddev * math.Sqrt(-2*math.Log(u1+1e-9)) * math.Cos(2*math.Pi*u2)
}
func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// ================== main ==================

func main() {
	mrand.New(mrand.NewSource(time.Now().UnixNano()))
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	loadConfig()
	buildGates()
	initMQTT()
	startAdminHTTP()

	ticker := time.NewTicker(time.Duration(cfg.IntervalMs) * time.Millisecond)
	defer ticker.Stop()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	statsTicker := time.NewTicker(30 * time.Second)
	defer statsTicker.Stop()

	log.Println("Simulator started. Press Ctrl+C to stop.")

	for {
		select {
		case t := <-ticker.C:
			runTick(t)
		case <-statsTicker.C:
			log.Printf("STATS | sensors=%d ships=%d mqtt=%d http=%d gates=%d",
				atomic.LoadUint64(&sentSensorCount),
				atomic.LoadUint64(&sentShipCount),
				atomic.LoadUint64(&sentMqttCount),
				atomic.LoadUint64(&sentHttpCount),
				len(gates))
		case <-quit:
			log.Println("Simulator shutting down...")
			if mqttClient != nil && mqttClient.IsConnected() { mqttClient.Disconnect(1000) }
			return
		}
	}
}
