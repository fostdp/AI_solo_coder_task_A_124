CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE IF NOT EXISTS dou_gates (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    location VARCHAR(200),
    gate_width DOUBLE PRECISION NOT NULL DEFAULT 6.0,
    gate_height DOUBLE PRECISION NOT NULL DEFAULT 4.5,
    max_water_level_up DOUBLE PRECISION NOT NULL DEFAULT 8.5,
    min_water_level_up DOUBLE PRECISION NOT NULL DEFAULT 4.0,
    max_water_level_down DOUBLE PRECISION NOT NULL DEFAULT 5.0,
    min_water_level_down DOUBLE PRECISION NOT NULL DEFAULT 2.0,
    chamber_length DOUBLE PRECISION NOT NULL DEFAULT 60.0,
    chamber_width DOUBLE PRECISION NOT NULL DEFAULT 10.0,
    discharge_coefficient DOUBLE PRECISION NOT NULL DEFAULT 0.63,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sensor_data (
    time TIMESTAMPTZ NOT NULL,
    gate_id INTEGER NOT NULL REFERENCES dou_gates(id),
    water_level_up DOUBLE PRECISION NOT NULL,
    water_level_down DOUBLE PRECISION NOT NULL,
    gate_opening DOUBLE PRECISION NOT NULL,
    flow_rate DOUBLE PRECISION NOT NULL,
    passage_time DOUBLE PRECISION,
    status VARCHAR(20) NOT NULL DEFAULT 'normal'
);

SELECT create_hypertable('sensor_data', 'time', if_not_exists => TRUE);

CREATE INDEX IF NOT EXISTS idx_sensor_data_gate_id ON sensor_data (gate_id, time DESC);

CREATE TABLE IF NOT EXISTS ships (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    priority INTEGER NOT NULL DEFAULT 5,
    length DOUBLE PRECISION NOT NULL DEFAULT 20.0,
    width DOUBLE PRECISION NOT NULL DEFAULT 5.0,
    draft DOUBLE PRECISION NOT NULL DEFAULT 2.0,
    arrival_time TIMESTAMPTZ NOT NULL,
    direction VARCHAR(10) NOT NULL CHECK (direction IN ('upstream', 'downstream')),
    status VARCHAR(20) NOT NULL DEFAULT 'waiting'
);

CREATE TABLE IF NOT EXISTS passage_records (
    id SERIAL PRIMARY KEY,
    ship_id INTEGER NOT NULL REFERENCES ships(id),
    gate_id INTEGER NOT NULL REFERENCES dou_gates(id),
    entry_time TIMESTAMPTZ,
    exit_time TIMESTAMPTZ,
    fill_time DOUBLE PRECISION,
    drain_time DOUBLE PRECISION,
    total_time DOUBLE PRECISION,
    wait_time DOUBLE PRECISION,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
);

CREATE TABLE IF NOT EXISTS alerts (
    id SERIAL PRIMARY KEY,
    time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    gate_id INTEGER REFERENCES dou_gates(id),
    alert_type VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL CHECK (severity IN ('info', 'warning', 'critical')),
    message TEXT NOT NULL,
    resolved BOOLEAN NOT NULL DEFAULT FALSE,
    resolved_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_alerts_gate_id ON alerts (gate_id, time DESC);
CREATE INDEX IF NOT EXISTS idx_alerts_resolved ON alerts (resolved, time DESC);

CREATE TABLE IF NOT EXISTS schedule_plans (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    gate_id INTEGER NOT NULL REFERENCES dou_gates(id),
    schedule JSONB NOT NULL,
    total_wait_time DOUBLE PRECISION NOT NULL,
    generation INTEGER NOT NULL,
    fitness DOUBLE PRECISION NOT NULL
);

INSERT INTO dou_gates (name, location, gate_width, gate_height, max_water_level_up, min_water_level_up,
    max_water_level_down, min_water_level_down, chamber_length, chamber_width, discharge_coefficient)
SELECT 
    '陡门' || i,
    '灵渠第' || i || '座',
    5.5 + random() * 1.5,
    4.0 + random() * 1.5,
    8.0 + random() * 1.5,
    3.5 + random() * 1.0,
    4.5 + random() * 1.5,
    1.5 + random() * 1.0,
    50.0 + random() * 30.0,
    8.0 + random() * 4.0,
    0.6 + random() * 0.1
FROM generate_series(1, 36) i
WHERE NOT EXISTS (SELECT 1 FROM dou_gates WHERE id = i);
