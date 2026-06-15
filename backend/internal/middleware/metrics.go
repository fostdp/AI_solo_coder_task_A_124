package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lingqu-dou-gate/internal/models"
)

type MetricsCollector struct {
	mu sync.RWMutex

	requestCount   map[string]*uint64
	requestLatency map[string][]float64
	errorCount     map[string]*uint64

	// 业务指标
	sensorDataReceived uint64
	alertsGenerated    uint64
	alertsCritical     uint64
	alertsWarning      uint64
	simulationsRun     uint64
	optimizationsRun   uint64
	optimizationGenSum uint64
	avgWaitTimeSum     uint64
	avgWaitTimeCount   uint64
}

var collectorInstance *MetricsCollector
var collectorOnce sync.Once

func GetMetricsCollector() *MetricsCollector {
	collectorOnce.Do(func() {
		collectorInstance = &MetricsCollector{
			requestCount:   make(map[string]*uint64),
			requestLatency: make(map[string][]float64),
			errorCount:     make(map[string]*uint64),
		}
	})
	return collectorInstance
}

// ------ HTTP中间件 ------

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (m *MetricsCollector) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: 200}

		next.ServeHTTP(rw, r)

		duration := time.Since(start).Seconds()
		endpoint := normalizeEndpoint(r.URL.Path)
		method := r.Method

		m.recordRequest(method, endpoint, duration, rw.statusCode)
	})
}

func normalizeEndpoint(path string) string {
	if strings.HasPrefix(path, "/api/gates/") && len(path) > len("/api/gates/") {
		return "/api/gates/:id"
	}
	if strings.HasPrefix(path, "/api/sensors/") {
		remain := path[len("/api/sensors/"):]
		if strings.HasSuffix(remain, "/history") {
			return "/api/sensors/:gateId/history"
		}
		return "/api/sensors/:gateId"
	}
	if strings.HasPrefix(path, "/api/simulation/") {
		return "/api/simulation/:gateId"
	}
	if strings.HasPrefix(path, "/api/alerts/") {
		return "/api/alerts/:id/resolve"
	}
	return path
}

func (m *MetricsCollector) recordRequest(method, endpoint string, duration float64, statusCode int) {
	key := method + " " + endpoint

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.requestCount[key] == nil {
		var v uint64
		m.requestCount[key] = &v
		m.errorCount[key] = new(uint64)
	}

	atomic.AddUint64(m.requestCount[key], 1)
	m.requestLatency[key] = append(m.requestLatency[key], duration)
	if len(m.requestLatency[key]) > 1000 {
		m.requestLatency[key] = m.requestLatency[key][len(m.requestLatency[key])-1000:]
	}

	if statusCode >= 400 {
		atomic.AddUint64(m.errorCount[key], 1)
	}
}

// ------ 业务指标 ------

func (m *MetricsCollector) IncSensorDataReceived() {
	atomic.AddUint64(&m.sensorDataReceived, 1)
}

func (m *MetricsCollector) IncAlert(severity string) {
	atomic.AddUint64(&m.alertsGenerated, 1)
	switch severity {
	case "critical":
		atomic.AddUint64(&m.alertsCritical, 1)
	case "warning":
		atomic.AddUint64(&m.alertsWarning, 1)
	}
}

func (m *MetricsCollector) IncSimulation() {
	atomic.AddUint64(&m.simulationsRun, 1)
}

func (m *MetricsCollector) IncOptimization(generations int, avgWaitTime float64) {
	atomic.AddUint64(&m.optimizationsRun, 1)
	atomic.AddUint64(&m.optimizationGenSum, uint64(generations))
	atomic.AddUint64(&m.avgWaitTimeSum, uint64(avgWaitTime))
	atomic.AddUint64(&m.avgWaitTimeCount, 1)
}

// ------ Prometheus格式导出 ------

func (m *MetricsCollector) ExportPrometheus() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var b strings.Builder

	// 头部
	b.WriteString("# HELP http_requests_total HTTP请求总数\n")
	b.WriteString("# TYPE http_requests_total counter\n")
	for key, p := range m.requestCount {
		b.WriteString(`http_requests_total{endpoint="`)
		b.WriteString(key)
		b.WriteString(`"} `)
		b.WriteString(strconv.FormatUint(atomic.LoadUint64(p), 10))
		b.WriteString("\n")
	}

	b.WriteString("# HELP http_errors_total HTTP错误请求数\n")
	b.WriteString("# TYPE http_errors_total counter\n")
	for key, p := range m.errorCount {
		v := atomic.LoadUint64(p)
		if v == 0 {
			continue
		}
		b.WriteString(`http_errors_total{endpoint="`)
		b.WriteString(key)
		b.WriteString(`"} `)
		b.WriteString(strconv.FormatUint(v, 10))
		b.WriteString("\n")
	}

	b.WriteString("# HELP http_request_duration_seconds HTTP请求时延(秒)\n")
	b.WriteString("# TYPE http_request_duration_seconds summary\n")
	for key, latencies := range m.requestLatency {
		if len(latencies) == 0 {
			continue
		}
		var sum float64
		for _, d := range latencies {
			sum += d
		}
		avg := sum / float64(len(latencies))
		p50 := percentile(latencies, 0.5)
		p95 := percentile(latencies, 0.95)
		p99 := percentile(latencies, 0.99)

		b.WriteString(`http_request_duration_seconds_sum{endpoint="`)
		b.WriteString(key)
		b.WriteString(`"} `)
		b.WriteString(strconv.FormatFloat(sum, 'f', -1, 64))
		b.WriteString("\n")
		b.WriteString(`http_request_duration_seconds_count{endpoint="`)
		b.WriteString(key)
		b.WriteString(`"} `)
		b.WriteString(strconv.Itoa(len(latencies)))
		b.WriteString("\n")
		b.WriteString(`http_request_duration_seconds{quantile="0.5",endpoint="`)
		b.WriteString(key)
		b.WriteString(`"} `)
		b.WriteString(strconv.FormatFloat(p50, 'f', -1, 64))
		b.WriteString("\n")
		b.WriteString(`http_request_duration_seconds{quantile="0.95",endpoint="`)
		b.WriteString(key)
		b.WriteString(`"} `)
		b.WriteString(strconv.FormatFloat(p95, 'f', -1, 64))
		b.WriteString("\n")
		b.WriteString(`http_request_duration_seconds{quantile="0.99",endpoint="`)
		b.WriteString(key)
		b.WriteString(`"} `)
		b.WriteString(strconv.FormatFloat(p99, 'f', -1, 64))
		b.WriteString("\n")
	}

	// === 业务指标 ===
	b.WriteString("# HELP lingqu_sensor_data_received 收到的传感器数据总数\n")
	b.WriteString("# TYPE lingqu_sensor_data_received counter\n")
	b.WriteString("lingqu_sensor_data_received ")
	b.WriteString(strconv.FormatUint(atomic.LoadUint64(&m.sensorDataReceived), 10))
	b.WriteString("\n")

	b.WriteString("# HELP lingqu_alerts_generated 告警生成总数\n")
	b.WriteString("# TYPE lingqu_alerts_generated counter\n")
	b.WriteString(`lingqu_alerts_generated{severity="all"} `)
	b.WriteString(strconv.FormatUint(atomic.LoadUint64(&m.alertsGenerated), 10))
	b.WriteString("\n")
	b.WriteString(`lingqu_alerts_generated{severity="critical"} `)
	b.WriteString(strconv.FormatUint(atomic.LoadUint64(&m.alertsCritical), 10))
	b.WriteString("\n")
	b.WriteString(`lingqu_alerts_generated{severity="warning"} `)
	b.WriteString(strconv.FormatUint(atomic.LoadUint64(&m.alertsWarning), 10))
	b.WriteString("\n")

	b.WriteString("# HELP lingqu_simulations_run 水力学仿真运行次数\n")
	b.WriteString("# TYPE lingqu_simulations_run counter\n")
	b.WriteString("lingqu_simulations_run ")
	b.WriteString(strconv.FormatUint(atomic.LoadUint64(&m.simulationsRun), 10))
	b.WriteString("\n")

	b.WriteString("# HELP lingqu_optimizations_run GA调度优化次数\n")
	b.WriteString("# TYPE lingqu_optimizations_run counter\n")
	b.WriteString("lingqu_optimizations_run ")
	b.WriteString(strconv.FormatUint(atomic.LoadUint64(&m.optimizationsRun), 10))
	b.WriteString("\n")

	optCnt := atomic.LoadUint64(&m.optimizationsRun)
	if optCnt > 0 {
		b.WriteString("# HELP lingqu_optimization_avg_generations 每次优化的平均进化代数\n")
		b.WriteString("# TYPE lingqu_optimization_avg_generations gauge\n")
		b.WriteString("lingqu_optimization_avg_generations ")
		avgGen := float64(atomic.LoadUint64(&m.optimizationGenSum)) / float64(optCnt)
		b.WriteString(strconv.FormatFloat(avgGen, 'f', -1, 64))
		b.WriteString("\n")
	}

	waitCnt := atomic.LoadUint64(&m.avgWaitTimeCount)
	if waitCnt > 0 {
		b.WriteString("# HELP lingqu_avg_wait_time_seconds 船舶平均等待时间(秒)\n")
		b.WriteString("# TYPE lingqu_avg_wait_time_seconds gauge\n")
		b.WriteString("lingqu_avg_wait_time_seconds ")
		avgWait := float64(atomic.LoadUint64(&m.avgWaitTimeSum)) / float64(waitCnt)
		b.WriteString(strconv.FormatFloat(avgWait, 'f', -1, 64))
		b.WriteString("\n")
	}

	// Go运行时的指标
	b.WriteString("# HELP go_info Go运行时信息\n")
	b.WriteString("# TYPE go_info gauge\n")
	b.WriteString("go_info 1\n")

	return b.String()
}

func percentile(sortedCandidate []float64, p float64) float64 {
	n := len(sortedCandidate)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sortedCandidate[0]
	}
	tmp := make([]float64, n)
	copy(tmp, sortedCandidate)
	for i := 0; i < n-1; i++ {
		for j := i + 1; j < n; j++ {
			if tmp[j] < tmp[i] {
				tmp[i], tmp[j] = tmp[j], tmp[i]
			}
		}
	}
	idx := int(float64(n-1) * p)
	return tmp[idx]
}

// ------ 兼容旧AlertManager接口 ------

func (m *MetricsCollector) ProcessAlertsBridge(alerts []models.Alert) {
	for _, a := range alerts {
		m.IncAlert(a.Severity)
	}
}
