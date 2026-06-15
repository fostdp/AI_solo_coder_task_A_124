const LingquAPI = (function() {
    const API_BASE = 'http://localhost:8080/api';

    async function apiGet(url) {
        try {
            const response = await fetch(`${API_BASE}${url}`);
            const data = await response.json();
            return data.data;
        } catch (error) {
            console.warn('API request failed, using mock data:', error);
            return null;
        }
    }

    async function apiPost(url, body) {
        try {
            const response = await fetch(`${API_BASE}${url}`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body)
            });
            const data = await response.json();
            return data.data;
        } catch (error) {
            console.warn('API request failed, using mock data:', error);
            return null;
        }
    }

    async function getAllGates() {
        const data = await apiGet('/gates');
        if (data) return data;
        
        const gates = [];
        for (let i = 1; i <= 36; i++) {
            gates.push({
                id: i,
                name: `陡门${i}`,
                location: `灵渠第${i}座`,
                gate_width: 5.5 + Math.random() * 1.5,
                gate_height: 4.0 + Math.random() * 1.5,
                max_water_level_up: 8.0 + Math.random() * 1.5,
                min_water_level_up: 3.5 + Math.random() * 1.0,
                max_water_level_down: 4.5 + Math.random() * 1.5,
                min_water_level_down: 1.5 + Math.random() * 1.0,
                chamber_length: 50.0 + Math.random() * 30.0,
                chamber_width: 8.0 + Math.random() * 4.0,
                discharge_coefficient: 0.6 + Math.random() * 0.1,
                status: 'active'
            });
        }
        return gates;
    }

    async function getGate(id) {
        const data = await apiGet(`/gates/${id}`);
        if (data) return data;
        
        return {
            id: id,
            name: `陡门${id}`,
            location: `灵渠第${id}座`,
            gate_width: 6.0,
            gate_height: 4.5,
            max_water_level_up: 8.5,
            min_water_level_up: 4.0,
            max_water_level_down: 5.0,
            min_water_level_down: 2.0,
            chamber_length: 60.0,
            chamber_width: 10.0,
            discharge_coefficient: 0.63,
            status: 'active'
        };
    }

    async function getSensorData(gateId) {
        const data = await apiGet(`/sensors/${gateId}`);
        if (data) return data;
        
        return {
            time: new Date().toISOString(),
            gate_id: gateId,
            water_level_up: 7.5 + Math.random() * 0.2 - 0.1,
            water_level_down: 3.5 + Math.random() * 0.2 - 0.1,
            gate_opening: 0.5 + Math.random() * 0.2,
            flow_rate: 25.0 + Math.random() * 5.0,
            passage_time: 0,
            status: 'normal'
        };
    }

    async function getSensorHistory(gateId, startTime, endTime) {
        const data = await apiGet(`/sensors/${gateId}/history?start=${startTime}&end=${endTime}`);
        if (data) return data;
        
        const history = [];
        const now = new Date();
        for (let i = 100; i >= 0; i--) {
            const t = new Date(now.getTime() - i * 5 * 60 * 1000);
            history.push({
                time: t.toISOString(),
                gate_id: gateId,
                water_level_up: 7.0 + 0.5 * Math.sin(i * 0.1),
                water_level_down: 3.0 + 0.3 * Math.sin(i * 0.15),
                gate_opening: 0.4 + 0.2 * Math.sin(i * 0.08),
                flow_rate: 20.0 + 10.0 * Math.sin(i * 0.12),
                status: 'normal'
            });
        }
        return history;
    }

    async function simulatePassage(params) {
        const data = await apiPost('/simulate', params);
        if (data) return data;
        
        const { gate_id, water_level_up, water_level_down, gate_opening, direction } = params;
        const levelCurve = [];
        const flowCurve = [];
        const dt = 2;
        const duration = 200;
        
        let currentLevel = direction === 'upstream' ? water_level_down : water_level_up;
        const targetLevel = direction === 'upstream' ? water_level_up : water_level_down;
        
        for (let t = 0; t <= duration; t += dt) {
            const progress = t / duration;
            const eased = 1 - Math.pow(1 - progress, 2);
            
            if (direction === 'upstream') {
                currentLevel = water_level_down + (water_level_up - water_level_down) * eased;
            } else {
                currentLevel = water_level_up - (water_level_up - water_level_down) * eased;
            }
            
            const flowRate = gate_opening * 40 * (1 - Math.abs(progress - 0.5) * 1.5);
            
            levelCurve.push({ time: t, water_level: currentLevel });
            flowCurve.push({ time: t, flow_rate: Math.max(0, flowRate) });
        }
        
        return {
            fill_time: direction === 'upstream' ? duration : 0,
            drain_time: direction === 'downstream' ? duration : 0,
            water_level_curve: levelCurve,
            flow_rate_curve: flowCurve,
            max_flow_rate: gate_opening * 40,
            avg_flow_rate: gate_opening * 25,
            total_water_volume: Math.abs(water_level_up - water_level_down) * 60 * 10
        };
    }

    async function getSimulationData(gateId) {
        const data = await apiGet(`/simulation/${gateId}`);
        if (data) return data;
        
        const gate = await getGate(gateId);
        const sensor = await getSensorData(gateId);
        
        const levelCurve = [];
        const flowCurve = [];
        for (let t = 0; t <= 180; t += 5) {
            const progress = t / 180;
            const level = gate.min_water_level_down + (gate.max_water_level_up - gate.min_water_level_down) * (1 - Math.pow(1 - progress, 2));
            const flow = 30 * (1 - Math.abs(progress - 0.5));
            levelCurve.push({ time: t, water_level: level });
            flowCurve.push({ time: t, flow_rate: flow });
        }
        
        const ships = [];
        const now = new Date();
        for (let i = 1; i <= 8; i++) {
            ships.push({
                ship_id: i,
                ship_name: `船舶${i}`,
                priority: (i % 3) + 1,
                arrival_time: new Date(now.getTime() + i * 20 * 60000).toISOString(),
                direction: i % 2 === 0 ? 'upstream' : 'downstream'
            });
        }
        
        return {
            gate: gate,
            sensor_data: sensor,
            simulation: {
                fill_time: 180,
                drain_time: 160,
                water_level_curve: levelCurve,
                flow_rate_curve: flowCurve,
                max_flow_rate: 35,
                avg_flow_rate: 22,
                total_water_volume: 300
            },
            schedule: ships.map((s, i) => ({
                ...s,
                start_time: new Date(now.getTime() + (i * 15 + 5) * 60000).toISOString(),
                end_time: new Date(now.getTime() + (i * 15 + 15) * 60000).toISOString(),
                wait_time: i * 8 * 60
            })),
            alerts: [
                { id: 1, time: new Date().toISOString(), gate_id: gateId, alert_type: 'water_level_diff_high', severity: 'warning', message: '上下游水位差过大' }
            ]
        };
    }

    async function optimizeSchedule(params) {
        const data = await apiPost('/optimize', params);
        if (data) return data;
        
        const { ships, gate_ids } = params;
        const schedule = [];
        const numGates = gate_ids ? gate_ids.length : 3;
        
        const sortedShips = [...ships].sort((a, b) => {
            if (b.priority !== a.priority) return b.priority - a.priority;
            return new Date(a.arrival_time) - new Date(b.arrival_time);
        });
        
        const gateTimes = new Array(numGates).fill(Date.now());
        
        sortedShips.forEach((ship, idx) => {
            const gateIdx = idx % numGates;
            const startTime = Math.max(gateTimes[gateIdx], new Date(ship.arrival_time).getTime());
            const endTime = startTime + 600000;
            
            schedule.push({
                ship_id: ship.ship_id,
                ship_name: ship.ship_name,
                start_time: new Date(startTime).toISOString(),
                end_time: new Date(endTime).toISOString(),
                wait_time: (startTime - new Date(ship.arrival_time).getTime()) / 1000,
                priority: ship.priority,
                direction: ship.direction
            });
            
            gateTimes[gateIdx] = endTime;
        });
        
        const totalWait = schedule.reduce((sum, s) => sum + s.wait_time, 0);
        
        return {
            schedule: schedule,
            total_wait_time: totalWait,
            fitness: 10000 / (totalWait + 1000),
            generations: 150
        };
    }

    async function getAlerts(gateId) {
        const data = await apiGet(`/alerts${gateId ? `?gate_id=${gateId}` : ''}`);
        if (data) return data;
        
        return [
            { id: 1, time: new Date(Date.now() - 300000).toISOString(), gate_id: 1, alert_type: 'water_level_diff_high', severity: 'warning', message: '上下游水位差过大', resolved: false },
            { id: 2, time: new Date(Date.now() - 600000).toISOString(), gate_id: 1, alert_type: 'flow_rate_high', severity: 'info', message: '流量波动较大', resolved: false }
        ];
    }

    async function testAlert(alertData) {
        return await apiPost('/alerts/test', alertData);
    }

    return {
        getAllGates,
        getGate,
        getSensorData,
        getSensorHistory,
        simulatePassage,
        getSimulationData,
        optimizeSchedule,
        getAlerts,
        testAlert
    };
})();
