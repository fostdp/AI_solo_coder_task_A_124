-- ============================================
-- TimescaleDB 初始化脚本
-- 传感器数据 Hypertable、降采样、保留策略
-- ============================================

\set ON_ERROR_STOP 1

-- 1. 创建扩展（若不存在）
CREATE EXTENSION IF NOT EXISTS timescaledb;
CREATE EXTENSION IF NOT EXISTS postgis;

-- 2. 切换到库
\c lingqu;

-- ============================================
-- 3. 创建 陡门表
-- ============================================
CREATE TABLE IF NOT EXISTS dou_gates (
    id                    SERIAL PRIMARY KEY,
    name                  VARCHAR(100) NOT NULL,
    location              VARCHAR(255),
    gate_width            DOUBLE PRECISION DEFAULT 6.0,
    gate_height           DOUBLE PRECISION DEFAULT 8.0,
    max_water_level_up    DOUBLE PRECISION DEFAULT 7.5,
    min_water_level_up    DOUBLE PRECISION DEFAULT 6.5,
    max_water_level_down  DOUBLE PRECISION DEFAULT 4.0,
    min_water_level_down  DOUBLE PRECISION DEFAULT 3.0,
    chamber_length        DOUBLE PRECISION DEFAULT 18.0,
    chamber_width         DOUBLE PRECISION DEFAULT 6.0,
    discharge_coefficient DOUBLE PRECISION DEFAULT 0.62,
    status                VARCHAR(20) DEFAULT 'active',
    created_at            TIMESTAMPTZ DEFAULT NOW()
);

-- 插入36座标准陡门
INSERT INTO dou_gates (name, location, status)
SELECT
    '陡门' || i || '号',
    CASE
        WHEN i <= 12 THEN '北渠-上段'
        WHEN i <= 24 THEN '中渠-中段'
        ELSE            '南渠-下段'
    END,
    'active'
FROM generate_series(1, 36) i
ON CONFLICT DO NOTHING;

-- ============================================
-- 4. 传感器数据表 (Hypertable)
--    分区键: time (按天分区)
-- ============================================
CREATE TABLE IF NOT EXISTS sensor_data (
    time              TIMESTAMPTZ     NOT NULL,
    gate_id           INTEGER         NOT NULL REFERENCES dou_gates(id) ON DELETE CASCADE,
    water_level_up    DOUBLE PRECISION,
    water_level_down  DOUBLE PRECISION,
    gate_opening      DOUBLE PRECISION,
    flow_rate         DOUBLE PRECISION,
    passage_time      DOUBLE PRECISION,
    status            VARCHAR(20),
    PRIMARY KEY (time, gate_id)
);

-- 转为Hypertable (若尚未转换)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM _timescaledb_catalog.hypertable
        WHERE table_name = 'sensor_data' AND schema_name = 'public'
    ) THEN
        PERFORM create_hypertable('sensor_data', 'time',
            chunk_time_interval => INTERVAL '1 day');
    END IF;
END $$;

-- 推荐索引
CREATE INDEX IF NOT EXISTS idx_sensor_data_gate_time
    ON sensor_data (gate_id, time DESC);
CREATE INDEX IF NOT EXISTS idx_sensor_data_status
    ON sensor_data (status) WHERE status != 'normal';

-- ============================================
-- 5. 连续聚集（降采样）：1分钟 -> 1小时
-- ============================================
CREATE MATERIALIZED VIEW IF NOT EXISTS sensor_data_hourly
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', time) AS bucket,
    gate_id,
    avg(water_level_up)::DOUBLE PRECISION   AS water_level_up_avg,
    avg(water_level_down)::DOUBLE PRECISION AS water_level_down_avg,
    avg(gate_opening)::DOUBLE PRECISION     AS gate_opening_avg,
    avg(flow_rate)::DOUBLE PRECISION        AS flow_rate_avg,
    max(flow_rate)::DOUBLE PRECISION        AS flow_rate_max,
    min(flow_rate)::DOUBLE PRECISION        AS flow_rate_min,
    count(*)::BIGINT                        AS sample_count
FROM sensor_data
GROUP BY bucket, gate_id
WITH NO DATA;

-- 刷新策略：每15分钟刷新，计算2小时内数据
SELECT add_continuous_aggregate_policy('sensor_data_hourly',
    start_offset => INTERVAL '3 hours',
    end_offset   => INTERVAL '1 hour',
    schedule_interval => INTERVAL '15 minutes'
) ON CONFLICT DO NOTHING;

-- ============================================
-- 6. 连续聚集（降采样）：1小时 -> 1天
-- ============================================
CREATE MATERIALIZED VIEW IF NOT EXISTS sensor_data_daily
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 day', bucket) AS day_bucket,
    gate_id,
    avg(water_level_up_avg)::DOUBLE PRECISION   AS water_level_up_avg,
    avg(water_level_down_avg)::DOUBLE PRECISION AS water_level_down_avg,
    avg(gate_opening_avg)::DOUBLE PRECISION     AS gate_opening_avg,
    avg(flow_rate_avg)::DOUBLE PRECISION        AS flow_rate_avg,
    max(flow_rate_max)::DOUBLE PRECISION        AS flow_rate_max,
    sum(sample_count)::BIGINT                   AS sample_count
FROM sensor_data_hourly
GROUP BY day_bucket, gate_id
WITH NO DATA;

SELECT add_continuous_aggregate_policy('sensor_data_daily',
    start_offset => INTERVAL '3 days',
    end_offset   => INTERVAL '1 day',
    schedule_interval => INTERVAL '1 hour'
) ON CONFLICT DO NOTHING;

-- ============================================
-- 7. 数据保留策略
--    - 原始数据(1s级): 保留 30 天
--    - 小时聚合:       保留 1 年
--    - 天聚合:         保留 永久
-- ============================================
SELECT add_retention_policy('sensor_data',
    INTERVAL '30 days',
    schedule_interval => INTERVAL '1 day'
) ON CONFLICT DO NOTHING;

SELECT add_retention_policy('sensor_data_hourly',
    INTERVAL '1 year',
    schedule_interval => INTERVAL '1 day'
) ON CONFLICT DO NOTHING;

-- ============================================
-- 8. 告警表
-- ============================================
CREATE TABLE IF NOT EXISTS alerts (
    id            SERIAL PRIMARY KEY,
    time          TIMESTAMPTZ DEFAULT NOW(),
    gate_id       INTEGER     NOT NULL REFERENCES dou_gates(id) ON DELETE CASCADE,
    alert_type    VARCHAR(50) NOT NULL,
    severity      VARCHAR(20) NOT NULL,
    message       TEXT,
    resolved      BOOLEAN     DEFAULT FALSE,
    resolved_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alerts_gate_resolved ON alerts (gate_id, resolved);
CREATE INDEX IF NOT EXISTS idx_alerts_time ON alerts (time DESC);

-- ============================================
-- 9. 船舶记录表
-- ============================================
CREATE TABLE IF NOT EXISTS ships (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(100) NOT NULL,
    priority    INTEGER DEFAULT 1,
    length      DOUBLE PRECISION,
    width       DOUBLE PRECISION,
    draft       DOUBLE PRECISION,
    arrival_time TIMESTAMPTZ,
    direction   VARCHAR(20),
    status      VARCHAR(20) DEFAULT 'waiting'
);

CREATE INDEX IF NOT EXISTS idx_ships_status ON ships (status, arrival_time);

-- ============================================
-- 10. 调度计划表
-- ============================================
CREATE TABLE IF NOT EXISTS schedule_plans (
    id              SERIAL PRIMARY KEY,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    gate_id         INTEGER REFERENCES dou_gates(id),
    schedule        JSONB,
    total_wait_time DOUBLE PRECISION,
    generation      INTEGER,
    fitness         DOUBLE PRECISION
);

-- ============================================
-- 11. 通行记录 （Hypertable by time）
-- ============================================
CREATE TABLE IF NOT EXISTS passage_records (
    time        TIMESTAMPTZ NOT NULL,
    ship_id     INTEGER,
    gate_id     INTEGER NOT NULL REFERENCES dou_gates(id),
    entry_time  TIMESTAMPTZ,
    exit_time   TIMESTAMPTZ,
    fill_time   DOUBLE PRECISION,
    drain_time  DOUBLE PRECISION,
    total_time  DOUBLE PRECISION,
    wait_time   DOUBLE PRECISION,
    status      VARCHAR(20),
    PRIMARY KEY (time, gate_id)
);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM _timescaledb_catalog.hypertable
        WHERE table_name = 'passage_records' AND schema_name = 'public'
    ) THEN
        PERFORM create_hypertable('passage_records', 'time',
            chunk_time_interval => INTERVAL '1 month');
    END IF;
END $$;

-- 完成
\echo 'TimescaleDB schema initialized successfully'
\echo '36 dou gates seeded'
\echo 'Retention policies: raw=30d, hourly=1y, daily=forever'
