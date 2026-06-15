# 灵渠陡门船舶调度系统 v1.0

基于 Actor 模式的全栈容器化工程化版本。包含：
- Go 后端（4 模块 Actor + Channel 通信总线
- TimescaleDB 时序数据库 + 降采样 + 保留策略
- EMQX MQTT Broker
- 陡门传感器模拟器（36 座陡门 + 船舶注入）
- Three.js 三维前端（SPH 水流粒子 + 船舶动画）
- Prometheus 指标 + pprof 性能剖析
- Grafana 可视化（可选 profile）

## 一、系统架构

```
                          ┌──────────────────────────────────────────────────────────────┐
                          │                        Docker Compose                │
                          │                                                    │
   ┌──────────────┐     │  ┌──────────────┐                                   │
   │   外部用户     │────▶│  │   Nginx FE    │────┐ Gzip 压缩                     │
   └──────────────┘     │  │ (端口 80)        │  │                                 │
                     │  └──────────────┘  │                                  │
                     │                    │  ▼                                  │
                     │  ┌──────────────────────────────────────────────────────────┐         │
                     │  │   Go Backend (Gin  :8080                   │         │
                     │  │                                             │         │
                     │  │  ┌─────────────┬─────────────┬──────────┬──────────┐│         │
                     │  │  │  DTU    │ Hydraulic  │  GA    │ Alarm  ││         │
                     │  │  │Receiver │    Sim    │Scheduler│  MQTT  ││         │
                     │  │  └────┬────┴──────┬─────┴────┬─────┴────┬─────┘│         │
                     │  │       │         │          │         │      │         │
                     │  │       │         │          │         │      │         │
                     │  └───────┼─────────┼──────────┼─────────┼──────┘         │
                     │          │         │          │         │                            │
                     │          ▼         ▼          ▼         ▼                            │
                     │  ┌──────────────────────────────────────────────────┐         │
                     │  │     ① Sensor Simulator 36 DouGates     │         │
                     │  │     ② TimescaleDB :5432             │         │
                     │  │     ③ EMQX MQTT :1883                │         │
                     │  └──────────────────────────────────────────────────┘         │
                     │                              │                                    │
                     │  ┌──────────────────────┐   │  ┌──────────────┐                │
                     │  │   Prometheus      │◀──┤  │  Admin Port    │                │
                     │  │   :9091          │   │  │   :6060       │                │
                     │  └────────┬─────────────┘   │  │  (pprof/metrics │                │
                     │       │                 │  └──────────────┘                │
                     │       ▼                 │                                    │
                     │  ┌──────────────┐         │                                    │
                     │  │    Grafana   │ (可选) │                                    │
                     │  │    :3000      │         │                                    │
                     │  └──────────────┘         │                                    │
                     └──────────────────────────────────────────────────────────────┘

              │
              │ 传感器 MQTT / HTTP
              ▼
   ┌─────────────────────────────────────────────────┐
   │      Sensor Simulator :9090                     │
   │  - 36 DouGates + 水位正弦波动
   │  - 船舶事件（随机生成
   │  - HTTP API 故障注入
   │  - Prometheus 指标
   └─────────────────────────────────────────────────┘
```

## 二、模块说明

### 2.1 Go 后端模块

| 模块          | 职责                                                | Channel 方向 |
|---------------|-------------------------------------------------|-----------|
| **DTU Receiver | 传感器数据校验 → 入库 → 广播 validated 到下游                         | 输入 (dataIn → validatedOutChan
| Hydraulic Sim | 闸孔 4 流态判别、充放水仿真                     | 请求-回复 (requestChan + replyChan)
| GA Scheduler  | 遗传算法船舶调度优化（锦标赛/自适应/2-opt/并行评估）| 请求-回复 (requestChan + replyChan)
| Alarm MQTT    | 告警评估 + MQTT 推送                                   | 订阅 DTU validated 数据

所有模块 数据

### 2.2 TimescaleDB 数据策略

| 对象               | 粒度      | 保留期 |
|-------------------|-----------|--------|
| sensor_data       | 原始采样 | 30 天  |
| sensor_data_hourly   | 1 小时 聚合 | 1 年   |
| sensor_data_daily    | 1 天 聚合  | 永久   |
| passage_records | 通行记录（按月分区） | 永久   |

自动建表脚本见 [deployment/timescaledb/init.sql。

### 2.3 管理端口一览

| 服务         | 端口     | 说明 |
|--------------|----------|------|
| 前端 Nginx     | 80       | 静态资源 (Gzip) + API 代理
| Go API        | 8080     | REST API
| Go Admin     | 6060     | /metrics /debug/pprof /healthz /build
| EMQX MQTT     | 1883     | MQTT 协议
| EMQX Dashboard | 18083    | EMQX Web 管理台 (admin/admin123!)
| TimescaleDB | 5432     | PostgreSQL
| Prometheus    | 9091     | Prometheus Web UI
| Grafana     | 3000     | 可选（profile: monitoring）
| 模拟器 Admin | 9090     | /health /metrics /inject-fault /gen-ship

## 三、快速部署

### 3.1 环境要求

- Docker 24.0+ / Docker Compose v2+
- 至少 4 CPU / 8 GB RAM

### 3.2 启动最小化（核心服务（生产模式）

```bash
# 1. 构建镜像并启动（所有服务
$ docker compose up -d --build

# 2. 查看日志
$ docker compose logs -f backend

# 3. 状态
$ docker compose ps

# 4. 可选启用 Grafana（监控仪表盘 profile）
$ docker compose --profile monitoring up -d grafana
```

### 3.3 环境变量（可选）

在项目根目录创建 `.env` 覆盖默认密码：

```bash
# .env
DB_PASSWORD=your_strong_password
EMQX_PASSWORD=your_mqtt_admin
GRAFANA_PASSWORD=your_grafana
SIM_GATE_COUNT=36
SIM_INTERVAL_MS=1000
```

### 3.4 验证服务

```bash
# 前端
$ curl http://localhost/
# API 健康检查
$ curl http://localhost:8080/api/gates
# Prometheus 指标
$ curl http://localhost:6060/metrics
# 模拟器状态
$ curl http://localhost:9090/health
```

## 四、陡门传感器模拟器

### 4.1 功能
- **36 座陡门（可配置数量 + 自定义每座陡门参数
- **正弦水位 + 高斯测量噪声
- **随机船舶事件生成（可调速率
- **传感器故障注入（卡死 / 尖峰 / 丢包 3 种模式
- **MQTT 主题: `lingqu/sensors/{gate_id}` 广播到 EMQX
- **HTTP POST 到 `/api/sensors`

### 4.2 配置

可通过 `deployment/sensor-simulator/config.json 或环境变量：

| 变量                        | 默认值                    | 说明 |
|-----------------------------|--------------------------|------|
| SIM_GATE_COUNT           | 36                     | 模拟陡门数 |
| SIM_INTERVAL_MS              | 1000                   | 采样间隔毫秒 |
| SIM_MQTT_BROKER         | tcp://emqx:1883        | MQTT Broker 地址 |
| SIM_MQTT_TOPIC         | lingqu/sensors            | MQTT 主题前缀 |
| SIM_HTTP_ENDPOINT       | http://backend:8080/api/sensors | HTTP 上报地址 |
| SIM_WATER_LEVEL_NOISE   | 0.1                     | 水位噪声标准差 |
| SIM_FAULT_RATE         | 0.001                   | 每 tick 故障概率 |
| SIM_SHIP_GENERATION_RATE | 0.05                     | 每分钟平均生成 |

### 4.3 使用 HTTP API 手动操作

```bash
# 查看状态
curl http://localhost:9090/health

# 注入故障（gate=1, mode=spike, 持续 5 分钟
curl -X POST "http://localhost:9090/inject-fault?gate=1&mode=spike&minutes=5"
# 支持的 mode: stuck（卡死闸门 / spike（水位尖峰 / dropout（丢包）

# 手动生成船舶
curl -X POST "http://localhost:9090/gen-ship?gate=3&direction=upstream&priority=5"

# 查看指标
curl http://localhost:9090/metrics
```

## 五、REST API

### 5.1 陡门

| 方法 | 路径                 | 说明 |
|------|----------------------|------|
| GET    | `/api/gates          | 获取全部 36 座陡门 |
| GET    | `/api/gates/:id        | 单座陡门 |

### 5.2 传感器

| 方法 | 路径                 | 说明 |
|------|----------------------|------|
| GET | `/api/sensors/:gateId`       | 某陡门最新数据 |
| GET    | `/api/sensors/:gateId/history?hours=24 | 历史数据 |
| POST   | `/api/sensors`       | 上报传感器数据 (202 Accepted)

POST 示例：

```bash
curl -X POST http://localhost:8080/api/sensors \
  -H 'Content-Type: application/json' \
  -d '{"gate_id":1,"water_level_up":7.2,"water_level_down":3.5,"gate_opening":0.5,"flow_rate":12.5}'
```

### 5.3 水力学仿真

| 方法 | 路径                 | 说明 |
|------|----------------------|------|
| POST   | `/api/simulate       | 单次通行仿真 |
| GET    | `/api/simulation/:gateId | 默认仿真 |

POST 示例：

```json
{
  "gate_id": 1,
  "water_level_up": 7.0,
  "water_level_down": 3.0,
  "gate_opening": 1.0,
  "direction": "upstream"
}
```

### 5.4 调度优化

| 方法 | 路径                 | 说明 |
|------|----------------------|------|
| POST   | `/api/optimize`       | GA 优化调度 |

请求体：

```json
{
  "gate_ids": [1,2,3],
  "ships": [
    {"ship_id": 101, "ship_name": "灵运号", "priority": 3,
     "arrival_time": "2026-06-16T10:00:00Z", "direction": "upstream"}
  ]
}
```

### 5.5 告警

| 方法 | 路径                 | 说明 |
|------|----------------------|------|
| GET    | `/api/alerts?gate_id=1 | 未解决的告警 |
| POST   | `/api/alerts/:id/resolve | 解决告警 |
| POST   | `/api/alerts/test       | 测试告警 |

## 六、指标与性能剖析

### 6.1 Prometheus 指标 (:6060/metrics

| 指标前缀          | 类型    | 说明 |
|-------------------|---------|------|
| `http_requests_total{endpoint="..."| counter | 每端点请求总数 |
| `http_errors_total`    | counter | 错误数 |
| `http_request_duration_seconds` | summary | 时延 p50/p95/p99 |
| `lingqu_sensor_data_received` | counter | 传感器接收总数 |
| `lingqu_alerts_generated{severity=...} | counter | 告警数 |
| `lingqu_simulations_run`   | counter | 仿真次数 |
| `lingqu_optimizations_run` | counter | 优化次数 |
| `lingqu_avg_wait_time_seconds` | gauge   | 船舶平均等待时间 |

### 6.2 pprof 使用

```bash
# 采集 30s CPU profile
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# 内存 profile
go tool pprof http://localhost:6060/debug/pprof/heap

# goroutine 泄露检查
curl http://localhost:6060/debug/pprof/goroutine?debug=1
```

### 6.3 构建信息

```bash
curl http://localhost:6060/build
# {"version":"1.0.0","git_hash":"abc1234","build_time":"2026-06-16T01:02:03Z"}
```

## 七、配置文件

```
.
├── Dockerfile                     # Go 后端多阶段构建
├── Dockerfile.frontend          # Nginx 前端
├── Dockerfile.simulator         # 传感器模拟器
├── docker-compose.yml
├── backend/
│   ├── config/
│   │   ├── hydraulic_params.json   # 水力学参数（16项）
│   │   └── ga_params.json        # GA 参数（24项）
│   ├── cmd/main.go
│   └── internal/
│       ├── middleware/metrics.go   # Prometheus 指标
│       ├── modules/
│       │   ├── dtu_receiver/
│       │   ├── hydraulic_sim/
│       │   ├── scheduler_ga/
│       │   └── alarm_mqtt/
│       └── handlers/handlers.go
│       └── services/
├── frontend/public/
│   └── js/
│   │   ├── doumen_3d.js          # 3D 场景 + SPH 粒子
│   │   ├── scheduling_panel.js  # 业务面板 + 面板 + API
│   │   └── main.js
│   └── index.html
└── deployment/
    ├── nginx.conf
    ├── nginx-gzip.conf
    ├── prometheus/prometheus.yml
    ├── timescaledb/
    │   ├── init.sql
    │   └── postgresql.conf
    └── sensor-simulator/
        ├── main.go
        ├── go.mod
        └── config.json
```

## 八、故障排查

```bash
# 后端panic 日志
docker compose logs backend --tail=200
# 模拟器卡慢查询 Timescale
docker compose exec timescaledb psql -U postgres lingqu
# 查看
MQTT EMQX 连接
docker compose exec emqx ./bin/emqx ctl clients list
```

---
