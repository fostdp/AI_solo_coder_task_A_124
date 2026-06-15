let gate3D, particleSystem, shipAnimation, levelChart;
let currentGateId = 1;
let currentDirection = 'upstream';
let isSimulationRunning = false;
let simulationInterval = null;
let gateList = [];

document.addEventListener('DOMContentLoaded', init);

async function init() {
    initThreeJS();
    initParticleSystem();
    initChart();
    await loadGates();
    selectGate(1);
    startDataRefresh();
    
    document.getElementById('openingSlider').addEventListener('input', (e) => {
        const value = e.target.value;
        document.getElementById('openingValue').textContent = value;
        updateGateOpening(value / 100);
    });
}

function initThreeJS() {
    const canvas = document.getElementById('three-canvas');
    gate3D = new DouGate3D(canvas);
    shipAnimation = new ShipAnimation(gate3D.scene, gate3D.gateConfig);
}

function initParticleSystem() {
    const canvas = document.getElementById('particle-canvas');
    particleSystem = new WaterParticleSystem(canvas);
    particleSystem.setGatePosition(0.5, 0.5, 0.05, 0.6);
    particleSystem.start();
}

function initChart() {
    const canvas = document.getElementById('levelChart');
    levelChart = new DataChart(canvas);
    levelChart.setColorScheme('water');
}

async function loadGates() {
    const gates = await LingquAPI.getAllGates();
    gateList = gates;
    
    const gateListEl = document.getElementById('gateList');
    gateListEl.innerHTML = gates.map(gate => `
        <div class="gate-item ${gate.id === currentGateId ? 'active' : ''}" 
             onclick="selectGate(${gate.id})" data-gate-id="${gate.id}">
            <div class="gate-name">${gate.name}</div>
            <div class="gate-status">
                <span><span class="status-dot ${gate.status === 'active' ? 'normal' : 'warning'}"></span>${gate.status === 'active' ? '运行中' : '维护'}</span>
                <span>${gate.location}</span>
            </div>
        </div>
    `).join('');
}

async function selectGate(gateId) {
    currentGateId = gateId;
    
    document.querySelectorAll('.gate-item').forEach(item => {
        item.classList.remove('active');
        if (parseInt(item.dataset.gateId) === gateId) {
            item.classList.add('active');
        }
    });
    
    const gate = gateList.find(g => g.id === gateId);
    if (gate) {
        document.getElementById('currentGateTitle').textContent = `${gate.name} - 三维仿真视图`;
        
        if (gate3D) {
            gate3D.setGateConfig({
                gate_width: gate.gate_width,
                gate_height: gate.gate_height,
                chamber_length: gate.chamber_length,
                chamber_width: gate.chamber_width
            });
            
            gate3D.updateWaterLevels(gate.max_water_level_up - 1, gate.min_water_level_down + 1);
        }
    }
    
    await loadSensorData(gateId);
    await loadAlerts(gateId);
    await loadSchedule();
}

async function loadSensorData(gateId) {
    const data = await LingquAPI.getSensorData(gateId);
    
    document.getElementById('waterLevelUp').textContent = data.water_level_up.toFixed(2);
    document.getElementById('waterLevelDown').textContent = data.water_level_down.toFixed(2);
    document.getElementById('gateOpening').textContent = Math.round(data.gate_opening * 100);
    document.getElementById('flowRate').textContent = data.flow_rate.toFixed(1);
    
    if (gate3D) {
        gate3D.updateWaterLevels(data.water_level_up, data.water_level_down);
        gate3D.setGateOpening(data.gate_opening);
    }
    
    if (particleSystem) {
        const upRatio = data.water_level_up / 8.5;
        const downRatio = data.water_level_down / 5.0;
        particleSystem.setWaterLevels(upRatio, downRatio);
        particleSystem.setFlowSpeed(data.flow_rate / 20);
    }
    
    const history = await LingquAPI.getSensorHistory(gateId);
    if (history && history.length > 0) {
        const levelData = history.map(h => ({
            time: h.time,
            value: h.water_level_up
        }));
        levelChart.setData(levelData);
    }
}

async function loadAlerts(gateId) {
    const alerts = await LingquAPI.getAlerts(gateId);
    const alertListEl = document.getElementById('alertList');
    
    if (alerts && alerts.length > 0) {
        alertListEl.innerHTML = alerts.map(alert => `
            <div class="alert-item ${alert.severity}">
                <div class="alert-type">${alert.alert_type}</div>
                <div class="alert-msg">${alert.message}</div>
            </div>
        `).join('');
    } else {
        alertListEl.innerHTML = '<div style="color: #607d8b; font-size: 12px; text-align: center; padding: 20px;">暂无告警信息</div>';
    }
}

async function loadSchedule() {
    const ships = [];
    const now = new Date();
    for (let i = 1; i <= 6; i++) {
        ships.push({
            ship_id: i,
            ship_name: `船舶${i}号`,
            priority: (i % 3) + 1,
            arrival_time: new Date(now.getTime() + i * 15 * 60000).toISOString(),
            direction: i % 2 === 0 ? 'upstream' : 'downstream',
            wait_time: i * 8 * 60
        });
    }
    
    const scheduleListEl = document.getElementById('scheduleList');
    scheduleListEl.innerHTML = ships.map(ship => {
        const priorityClass = ship.priority >= 3 ? 'high' : ship.priority >= 2 ? 'medium' : 'low';
        const arrival = new Date(ship.arrival_time);
        const timeStr = arrival.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
        const dirStr = ship.direction === 'upstream' ? '上行' : '下行';
        
        return `
            <div class="schedule-item">
                <div class="priority-badge ${priorityClass}">${ship.priority}</div>
                <div class="ship-info">
                    <div class="ship-name">${ship.ship_name} · ${dirStr}</div>
                    <div class="ship-time">到达: ${timeStr}</div>
                </div>
            </div>
        `;
    }).join('');
}

async function runOptimization() {
    const ships = [];
    const now = new Date();
    for (let i = 1; i <= 10; i++) {
        ships.push({
            ship_id: i,
            ship_name: `船舶${i}`,
            priority: (i % 5) + 1,
            arrival_time: new Date(now.getTime() + i * 10 * 60000).toISOString(),
            direction: i % 2 === 0 ? 'upstream' : 'downstream'
        });
    }
    
    const result = await LingquAPI.optimizeSchedule({
        gate_ids: [currentGateId],
        ships: ships
    });
    
    if (result && result.schedule) {
        const scheduleListEl = document.getElementById('scheduleList');
        scheduleListEl.innerHTML = result.schedule.map(item => {
            const priorityClass = item.priority >= 4 ? 'high' : item.priority >= 2 ? 'medium' : 'low';
            const startTime = new Date(item.start_time);
            const timeStr = startTime.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
            const dirStr = item.direction === 'upstream' ? '上行' : '下行';
            const waitMin = Math.round(item.wait_time / 60);
            
            return `
                <div class="schedule-item">
                    <div class="priority-badge ${priorityClass}">${item.priority}</div>
                    <div class="ship-info">
                        <div class="ship-name">${item.ship_name} · ${dirStr}</div>
                        <div class="ship-time">开始: ${timeStr} · 等待 ${waitMin} 分钟</div>
                    </div>
                </div>
            `;
        }).join('');
    }
}

function setDirection(direction) {
    currentDirection = direction;
    
    document.getElementById('btnUpstream').classList.toggle('active', direction === 'upstream');
    document.getElementById('btnDownstream').classList.toggle('active', direction === 'downstream');
    
    if (shipAnimation) {
        shipAnimation.setDirection(direction);
    }
}

function updateGateOpening(opening) {
    if (gate3D) {
        gate3D.setGateOpening(opening);
    }
    if (particleSystem) {
        particleSystem.flowSpeed = opening * 2;
    }
}

function startSimulation() {
    if (isSimulationRunning) return;
    
    isSimulationRunning = true;
    
    const opening = parseInt(document.getElementById('openingSlider').value) / 100;
    const result = simulatePassageLocal(opening, currentDirection);
    
    document.getElementById('fillTime').textContent = result.fill_time.toFixed(0);
    document.getElementById('drainTime').textContent = result.drain_time.toFixed(0);
    document.getElementById('totalVolume').textContent = result.total_water_volume.toFixed(0);
    document.getElementById('maxFlowRate').textContent = result.max_flow_rate.toFixed(1);
    
    let currentIndex = 0;
    const curve = result.water_level_curve;
    
    simulationInterval = setInterval(() => {
        if (currentIndex >= curve.length) {
            clearInterval(simulationInterval);
            isSimulationRunning = false;
            return;
        }
        
        const point = curve[currentIndex];
        if (currentDirection === 'upstream') {
            gate3D.updateWaterLevels(gate3D.waterLevelUp, point.water_level);
            particleSystem.setWaterLevels(gate3D.waterLevelUp / 8.5, point.water_level / 5.0);
        } else {
            gate3D.updateWaterLevels(point.water_level, gate3D.waterLevelDown);
            particleSystem.setWaterLevels(point.water_level / 8.5, gate3D.waterLevelDown / 5.0);
        }
        
        currentIndex += 2;
    }, 50);
}

function simulatePassageLocal(opening, direction) {
    const gate = gateList.find(g => g.id === currentGateId);
    const levelUp = gate ? gate.max_water_level_up : 8.5;
    const levelDown = gate ? gate.min_water_level_down : 2.0;
    const chamberVol = (gate ? gate.chamber_length : 60) * (gate ? gate.chamber_width : 10);
    
    const levelCurve = [];
    const flowCurve = [];
    const duration = 200;
    const steps = 100;
    
    for (let i = 0; i <= steps; i++) {
        const t = (i / steps) * duration;
        const progress = i / steps;
        const eased = 1 - Math.pow(1 - progress, 2);
        
        let level;
        if (direction === 'upstream') {
            level = levelDown + (levelUp - levelDown) * eased;
        } else {
            level = levelUp - (levelUp - levelDown) * eased;
        }
        
        const flow = opening * 35 * (1 - Math.abs(progress - 0.5) * 1.5);
        
        levelCurve.push({ time: t, water_level: level });
        flowCurve.push({ time: t, flow_rate: Math.max(0, flow) });
    }
    
    return {
        fill_time: direction === 'upstream' ? duration : 0,
        drain_time: direction === 'downstream' ? duration : 0,
        water_level_curve: levelCurve,
        flow_rate_curve: flowCurve,
        max_flow_rate: opening * 35,
        total_water_volume: Math.abs(levelUp - levelDown) * chamberVol
    };
}

function startPassage() {
    if (shipAnimation && !shipAnimation.isAnimating) {
        shipAnimation.startPassage(currentDirection, () => {
            console.log('船舶通行完成');
        });
    }
}

function pauseSimulation() {
    if (shipAnimation) {
        if (shipAnimation.isAnimating) {
            shipAnimation.pause();
        } else {
            shipAnimation.resume();
        }
    }
    
    if (isSimulationRunning && simulationInterval) {
        clearInterval(simulationInterval);
        isSimulationRunning = false;
    }
}

function resetSimulation() {
    if (shipAnimation) {
        shipAnimation.reset();
    }
    
    if (simulationInterval) {
        clearInterval(simulationInterval);
    }
    isSimulationRunning = false;
    
    const gate = gateList.find(g => g.id === currentGateId);
    if (gate && gate3D) {
        gate3D.updateWaterLevels(gate.max_water_level_up - 1, gate.min_water_level_down + 1);
        particleSystem.setWaterLevels((gate.max_water_level_up - 1) / 8.5, (gate.min_water_level_down + 1) / 5.0);
    }
    
    document.getElementById('fillTime').textContent = '0';
    document.getElementById('drainTime').textContent = '0';
    document.getElementById('totalVolume').textContent = '0';
    document.getElementById('maxFlowRate').textContent = '0';
}

function resetView() {
    if (gate3D) {
        gate3D.resetView();
    }
}

function toggleAutoRotate() {
    if (gate3D) {
        const rotating = gate3D.toggleAutoRotate();
        console.log('Auto rotate:', rotating);
    }
}

function startDataRefresh() {
    setInterval(async () => {
        await loadSensorData(currentGateId);
    }, 30000);
    
    setInterval(async () => {
        const flowVal = 20 + Math.random() * 10;
        document.getElementById('flowRate').textContent = flowVal.toFixed(1);
        levelChart.addDataPoint(7 + Math.random() * 0.5);
    }, 5000);
}

window.addEventListener('resize', () => {
    if (levelChart) levelChart.resize();
    if (particleSystem) particleSystem.resize();
    if (gate3D) gate3D.onResize();
});
