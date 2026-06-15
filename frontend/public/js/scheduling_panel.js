class SchedulingPanel {
    constructor(options = {}) {
        this.gateId = options.gateId || 1;
        this.refreshInterval = options.refreshInterval || 30000;
        this.apiBase = options.apiBase || '/api';
        this.charts = {};
        this.data = {
            gate: null,
            sensorData: null,
            simulation: null,
            schedule: [],
            alerts: []
        };
        this.timers = [];
        this.onDataUpdate = options.onDataUpdate || null;
        this.gate3D = options.gate3D || null;
        this.shipManager = options.shipManager || null;
    }

    async init() {
        try {
            await Promise.all([
                this.loadGateData(),
                this.loadSensorData(),
                this.loadSimulationData(),
                this.loadAlerts()
            ]);
            this.startAutoRefresh();
            return true;
        } catch (err) {
            console.error('Failed to initialize scheduling panel:', err);
            return false;
        }
    }

    startAutoRefresh() {
        this.stopAutoRefresh();
        const sensorTimer = setInterval(() => {
            this.loadSensorData().catch(console.error);
        }, this.refreshInterval);
        const alertTimer = setInterval(() => {
            this.loadAlerts().catch(console.error);
        }, this.refreshInterval * 2);
        this.timers.push(sensorTimer, alertTimer);
    }

    stopAutoRefresh() {
        this.timers.forEach(t => clearInterval(t));
        this.timers = [];
    }

    async loadGateData() {
        const resp = await API.getGate(this.gateId);
        if (resp && resp.data) {
            this.data.gate = resp.data;
            this.updateGateUI();
            if (this.gate3D) {
                this.gate3D.setGateOpening(0.8);
                if (resp.data.MaxWaterLevelUp && resp.data.MinWaterLevelDown) {
                    const range = resp.data.MaxWaterLevelUp - resp.data.MinWaterLevelDown;
                    const up = (resp.data.MaxWaterLevelUp - resp.data.MinWaterLevelDown) / range;
                    this.gate3D.setWaterLevels(0.8, 0.3);
                }
            }
        }
        return resp;
    }

    async loadSensorData() {
        const resp = await API.getSensorData(this.gateId);
        if (resp && resp.data) {
            this.data.sensorData = resp.data;
            this.updateSensorUI();
            if (this.gate3D && resp.data.GateOpening !== undefined) {
                this.gate3D.setGateOpening(resp.data.GateOpening);
            }
            if (this.onDataUpdate) {
                this.onDataUpdate('sensor', resp.data);
            }
        }
        return resp;
    }

    async loadSensorHistory(hours = 24) {
        const end = new Date();
        const start = new Date(end.getTime() - hours * 3600 * 1000);
        const resp = await API.getSensorHistory(this.gateId, start, end);
        if (resp && resp.data) {
            this.data.sensorHistory = resp.data;
            this.updateCharts(resp.data);
        }
        return resp;
    }

    async loadSimulationData() {
        const resp = await API.getSimulationData(this.gateId);
        if (resp && resp.data) {
            this.data.simulation = resp.data.simulation;
            this.data.schedule = resp.data.schedule || [];
            this.data.alerts = resp.data.alerts || [];
            if (resp.data.gate) {
                this.data.gate = resp.data.gate;
            }
            this.updateSimulationUI();
            this.updateScheduleUI();
            this.updateAlertsUI();
            this.syncShipsFromSchedule();
        }
        return resp;
    }

    async simulatePassage(params) {
        const resp = await API.simulatePassage({
            gate_id: params.gateId || this.gateId,
            water_level_up: params.waterLevelUp,
            water_level_down: params.waterLevelDown,
            gate_opening: params.gateOpening,
            direction: params.direction || 'upstream'
        });
        if (resp && resp.data) {
            this.data.simulation = resp.data;
            this.updateSimulationUI();
        }
        return resp;
    }

    async optimizeSchedule(ships, gateIds) {
        const resp = await API.optimizeSchedule({
            gate_ids: gateIds || [this.gateId],
            ships: ships || []
        });
        if (resp && resp.data) {
            this.data.schedule = resp.data.schedule || [];
            this.data.optimization = {
                totalWaitTime: resp.data.total_wait_time,
                fitness: resp.data.fitness,
                generations: resp.data.generations,
                historyCount: resp.data.history_count
            };
            this.updateScheduleUI();
            this.syncShipsFromSchedule();
        }
        return resp;
    }

    async loadAlerts() {
        const resp = await API.getAlerts(this.gateId);
        if (resp && resp.data) {
            this.data.alerts = resp.data;
            this.updateAlertsUI();
        }
        return resp;
    }

    async resolveAlert(alertId) {
        const resp = await API.resolveAlert(alertId);
        if (resp) {
            this.loadAlerts();
        }
        return resp;
    }

    async testAlert(alertData) {
        return await API.testAlert(alertData);
    }

    updateGateUI() {
        const gate = this.data.gate;
        if (!gate) return;
        const el = document.getElementById('gate-info');
        if (el) {
            el.innerHTML = `
                <h3>${gate.Name || '陡门' + gate.ID}</h3>
                <div class="gate-specs">
                    <span>闸室长: ${gate.ChamberLength?.toFixed?.(1) || 'N/A'}m</span>
                    <span>闸室宽: ${gate.ChamberWidth?.toFixed?.(1) || 'N/A'}m</span>
                    <span>上游水位: ${gate.MaxWaterLevelUp?.toFixed?.(2) || 'N/A'}m</span>
                    <span>下游水位: ${gate.MinWaterLevelDown?.toFixed?.(2) || 'N/A'}m</span>
                </div>
            `;
        }
    }

    updateSensorUI() {
        const data = this.data.sensorData;
        if (!data) return;

        const el = document.getElementById('sensor-display');
        if (el) {
            const time = data.Time ? new Date(data.Time).toLocaleString() : '--';
            el.innerHTML = `
                <div class="sensor-item">
                    <label>上游水位</label>
                    <span class="value">${data.WaterLevelUp?.toFixed(2)} m</span>
                </div>
                <div class="sensor-item">
                    <label>下游水位</label>
                    <span class="value">${data.WaterLevelDown?.toFixed(2)} m</span>
                </div>
                <div class="sensor-item">
                    <label>闸门开度</label>
                    <span class="value">${(data.GateOpening * 100)?.toFixed(1)} %</span>
                </div>
                <div class="sensor-item">
                    <label>流量</label>
                    <span class="value">${data.FlowRate?.toFixed(2)} m³/s</span>
                </div>
                <div class="sensor-item">
                    <label>状态</label>
                    <span class="status status-${data.Status || 'normal'}">${data.Status || '正常'}</span>
                </div>
                <div class="sensor-item sensor-time">
                    <label>更新时间</label>
                    <span>${time}</span>
                </div>
            `;
        }
    }

    updateSimulationUI() {
        const sim = this.data.simulation;
        if (!sim) return;

        const el = document.getElementById('simulation-result');
        if (el) {
            const fillTime = sim.FillTime ? (sim.FillTime / 60).toFixed(1) : '--';
            const drainTime = sim.DrainTime ? (sim.DrainTime / 60).toFixed(1) : '--';
            const maxFlow = sim.MaxFlowRate?.toFixed(2) || '--';
            const avgFlow = sim.AvgFlowRate?.toFixed(2) || '--';
            const volume = sim.TotalWaterVolume?.toFixed(1) || '--';

            el.innerHTML = `
                <div class="sim-row">
                    <div class="sim-item">
                        <label>充水时间</label>
                        <span class="value">${fillTime} min</span>
                    </div>
                    <div class="sim-item">
                        <label>放水时间</label>
                        <span class="value">${drainTime} min</span>
                    </div>
                </div>
                <div class="sim-row">
                    <div class="sim-item">
                        <label>最大流量</label>
                        <span class="value">${maxFlow} m³/s</span>
                    </div>
                    <div class="sim-item">
                        <label>平均流量</label>
                        <span class="value">${avgFlow} m³/s</span>
                    </div>
                </div>
                <div class="sim-item full-width">
                    <label>总水量</label>
                    <span class="value">${volume} m³</span>
                </div>
            `;
        }

        if (sim.FlowRateCurve && sim.FlowRateCurve.length > 0) {
            this.renderFlowChart(sim.FlowRateCurve);
        }
        if (sim.WaterLevelCurve && sim.WaterLevelCurve.length > 0) {
            this.renderWaterLevelChart(sim.WaterLevelCurve);
        }
    }

    updateScheduleUI() {
        const schedule = this.data.schedule;
        const el = document.getElementById('schedule-list');
        if (!el) return;

        if (!schedule || schedule.length === 0) {
            el.innerHTML = '<div class="empty-state">暂无调度计划</div>';
            return;
        }

        const items = schedule.map((item, idx) => {
            const start = item.StartTime ? new Date(item.StartTime).toLocaleTimeString() : '--';
            const end = item.EndTime ? new Date(item.EndTime).toLocaleTimeString() : '--';
            const wait = item.WaitTime ? (item.WaitTime / 60).toFixed(1) : '0';
            const prioColors = { 1: 'priority-low', 2: 'priority-low', 3: 'priority-medium', 4: 'priority-high', 5: 'priority-critical' };
            const prioClass = prioColors[item.Priority] || 'priority-low';

            return `
                <div class="schedule-item">
                    <div class="schedule-rank">${idx + 1}</div>
                    <div class="schedule-info">
                        <div class="ship-name">${item.ShipName || '船舶' + item.ShipID}</div>
                        <div class="ship-time">${start} - ${end}</div>
                        <div class="ship-wait">等待: ${wait} 分钟</div>
                    </div>
                    <div class="schedule-priority ${prioClass}">
                        P${item.Priority || 1}
                    </div>
                </div>
            `;
        }).join('');

        el.innerHTML = items;

        const optInfo = document.getElementById('optimization-info');
        if (optInfo && this.data.optimization) {
            const opt = this.data.optimization;
            optInfo.innerHTML = `
                <div class="opt-stat">
                    <label>总等待时间</label>
                    <span>${(opt.totalWaitTime / 60).toFixed(1)} min</span>
                </div>
                <div class="opt-stat">
                    <label>进化代数</label>
                    <span>${opt.generations}</span>
                </div>
                <div class="opt-stat">
                    <label>适应度</label>
                    <span>${opt.fitness?.toFixed(2)}</span>
                </div>
            `;
        }
    }

    updateAlertsUI() {
        const alerts = this.data.alerts;
        const el = document.getElementById('alert-list');
        if (!el) return;

        if (!alerts || alerts.length === 0) {
            el.innerHTML = '<div class="empty-state">暂无告警</div>';
            return;
        }

        const items = alerts.slice(0, 10).map(alert => {
            const time = alert.Time ? new Date(alert.Time).toLocaleString() : '--';
            const sevColors = {
                'critical': 'alert-critical',
                'warning': 'alert-warning',
                'info': 'alert-info'
            };
            const sevClass = sevColors[alert.Severity] || 'alert-info';

            return `
                <div class="alert-item ${sevClass}" data-id="${alert.ID}">
                    <div class="alert-header">
                        <span class="alert-type">${alert.AlertType || '告警'}</span>
                        <span class="alert-severity">${alert.Severity || 'info'}</span>
                    </div>
                    <div class="alert-message">${alert.Message || ''}</div>
                    <div class="alert-footer">
                        <span class="alert-time">${time}</span>
                        ${!alert.Resolved ? `<button class="resolve-btn" data-id="${alert.ID}">标记处理</button>` : '<span class="resolved">已处理</span>'}
                    </div>
                </div>
            `;
        }).join('');

        el.innerHTML = items;

        el.querySelectorAll('.resolve-btn').forEach(btn => {
            btn.addEventListener('click', (e) => {
                const id = e.target.getAttribute('data-id');
                this.resolveAlert(id);
            });
        });
    }

    updateCharts(historyData) {
        if (!historyData || historyData.length === 0) return;
        this.renderHistoryChart(historyData);
    }

    renderFlowChart(flowCurve) {
        const canvas = document.getElementById('flow-chart');
        if (!canvas) return;
        if (!this.charts.flow) {
            this.charts.flow = new SimpleChart(canvas, {
                title: '流量变化曲线',
                color: '#4da6ff',
                yLabel: 'm³/s'
            });
        }
        this.charts.flow.setData(
            flowCurve.map(p => p.Time),
            flowCurve.map(p => p.FlowRate)
        );
    }

    renderWaterLevelChart(levelCurve) {
        const canvas = document.getElementById('waterlevel-chart');
        if (!canvas) return;
        if (!this.charts.waterLevel) {
            this.charts.waterLevel = new SimpleChart(canvas, {
                title: '水位变化曲线',
                color: '#2ecc71',
                yLabel: 'm'
            });
        }
        this.charts.waterLevel.setData(
            levelCurve.map(p => p.Time),
            levelCurve.map(p => p.WaterLevel)
        );
    }

    renderHistoryChart(historyData) {
        const canvas = document.getElementById('history-chart');
        if (!canvas || !historyData || historyData.length === 0) return;

        const times = historyData.map(d => new Date(d.Time).getTime());
        const levels = historyData.map(d => d.WaterLevelUp);

        if (!this.charts.history) {
            this.charts.history = new SimpleChart(canvas, {
                title: '历史水位',
                color: '#3498db',
                yLabel: 'm'
            });
        }
        this.charts.history.setData(times, levels);
    }

    syncShipsFromSchedule() {
        if (!this.shipManager) return;
        this.shipManager.clear();
        const schedule = this.data.schedule || [];
        schedule.slice(0, 8).forEach((item, idx) => {
            this.shipManager.addShip({
                id: item.ShipID || idx,
                name: item.ShipName,
                color: idx % 2 === 0 ? 0x8b4513 : 0x654321,
                targetY: (item.Priority || 1) * 0.2,
                startY: 0.5
            });
        });
    }

    setGate3D(gate3D) {
        this.gate3D = gate3D;
    }

    setShipManager(shipManager) {
        this.shipManager = shipManager;
    }

    destroy() {
        this.stopAutoRefresh();
        Object.values(this.charts).forEach(chart => {
            if (chart.destroy) chart.destroy();
        });
        this.charts = {};
    }
}

class SimpleChart {
    constructor(canvas, options = {}) {
        this.canvas = canvas;
        this.ctx = canvas.getContext('2d');
        this.title = options.title || '';
        this.color = options.color || '#4da6ff';
        this.yLabel = options.yLabel || '';
        this.xData = [];
        this.yData = [];
        this.width = canvas.width;
        this.height = canvas.height;
        this.padding = { top: 30, right: 20, bottom: 30, left: 50 };
    }

    setData(xData, yData) {
        this.xData = xData || [];
        this.yData = yData || [];
        this.draw();
    }

    draw() {
        const ctx = this.ctx;
        const w = this.canvas.width;
        const h = this.canvas.height;
        const pad = this.padding;

        ctx.clearRect(0, 0, w, h);

        const plotW = w - pad.left - pad.right;
        const plotH = h - pad.top - pad.bottom;

        if (this.yData.length === 0 || plotW <= 0 || plotH <= 0) return;

        const yMin = Math.min(...this.yData) * 0.95;
        const yMax = Math.max(...this.yData) * 1.05;
        const yRange = yMax - yMin || 1;

        if (this.title) {
            ctx.fillStyle = '#333';
            ctx.font = '14px sans-serif';
            ctx.textAlign = 'center';
            ctx.fillText(this.title, w / 2, 18);
        }

        ctx.strokeStyle = '#eee';
        ctx.lineWidth = 1;
        for (let i = 0; i <= 4; i++) {
            const y = pad.top + (plotH / 4) * i;
            ctx.beginPath();
            ctx.moveTo(pad.left, y);
            ctx.lineTo(w - pad.right, y);
            ctx.stroke();

            const value = yMax - (yRange / 4) * i;
            ctx.fillStyle = '#666';
            ctx.font = '11px sans-serif';
            ctx.textAlign = 'right';
            ctx.fillText(value.toFixed(1), pad.left - 5, y + 4);
        }

        ctx.strokeStyle = this.color;
        ctx.lineWidth = 2;
        ctx.beginPath();

        this.yData.forEach((val, i) => {
            const x = pad.left + (i / (this.yData.length - 1 || 1)) * plotW;
            const y = pad.top + ((yMax - val) / yRange) * plotH;
            if (i === 0) {
                ctx.moveTo(x, y);
            } else {
                ctx.lineTo(x, y);
            }
        });
        ctx.stroke();

        const fillGrad = ctx.createLinearGradient(0, pad.top, 0, pad.top + plotH);
        fillGrad.addColorStop(0, this.color + '40');
        fillGrad.addColorStop(1, this.color + '05');
        ctx.fillStyle = fillGrad;
        ctx.lineTo(pad.left + plotW, pad.top + plotH);
        ctx.lineTo(pad.left, pad.top + plotH);
        ctx.closePath();
        ctx.fill();
    }

    destroy() {
        this.xData = [];
        this.yData = [];
    }
}

const API = {
    baseUrl: '/api',

    async request(path, options = {}) {
        const url = this.baseUrl + path;
        try {
            const resp = await fetch(url, {
                headers: { 'Content-Type': 'application/json' },
                ...options
            });
            if (!resp.ok) {
                throw new Error(`HTTP ${resp.status}`);
            }
            return await resp.json();
        } catch (err) {
            console.warn(`API Error [${path}]:`, err);
            return null;
        }
    },

    async getGates() {
        return this.request('/gates');
    },

    async getGate(id) {
        return this.request(`/gates/${id}`);
    },

    async getSensorData(gateId) {
        return this.request(`/sensors/${gateId}`);
    },

    async getSensorHistory(gateId, start, end) {
        const params = new URLSearchParams();
        if (start) params.set('start', start.toISOString());
        if (end) params.set('end', end.toISOString());
        return this.request(`/sensors/${gateId}/history?${params.toString()}`);
    },

    async postSensorData(data) {
        return this.request('/sensors', {
            method: 'POST',
            body: JSON.stringify(data)
        });
    },

    async simulatePassage(params) {
        return this.request('/simulate', {
            method: 'POST',
            body: JSON.stringify(params)
        });
    },

    async getSimulationData(gateId) {
        return this.request(`/simulation/${gateId}`);
    },

    async optimizeSchedule(params) {
        return this.request('/optimize', {
            method: 'POST',
            body: JSON.stringify(params)
        });
    },

    async getAlerts(gateId) {
        const params = new URLSearchParams();
        if (gateId) params.set('gate_id', gateId);
        return this.request(`/alerts?${params.toString()}`);
    },

    async resolveAlert(alertId) {
        return this.request(`/alerts/${alertId}/resolve`, {
            method: 'POST'
        });
    },

    async testAlert(alertData) {
        return this.request('/alerts/test', {
            method: 'POST',
            body: JSON.stringify(alertData || {})
        });
    }
};
