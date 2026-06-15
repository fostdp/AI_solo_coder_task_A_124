class SPHParticle {
    constructor(x, y) {
        this.x = x;
        this.y = y;
        this.vx = (Math.random() - 0.5) * 0.5;
        this.vy = (Math.random() - 0.5) * 0.2;
        this.fx = 0;
        this.fy = 0;
        this.density = 0;
        this.pressure = 0;
        this.size = 2 + Math.random() * 1.5;
        this.colorAlpha = 0.5 + Math.random() * 0.3;
        this.side = 'free';
    }
}

class SPHParams {
    constructor() {
        this.restDensity = 1000;
        this.gasConstant = 2000;
        this.viscosity = 150;
        this.particleRadius = 8;
        this.smoothingRadius = 16;
        this.smoothingRadius2 = this.smoothingRadius * this.smoothingRadius;
        this.gravityX = 0;
        this.gravityY = 500;
        this.damping = 0.995;
        this.dt = 0.012;
        this.boundaryDamping = -0.3;
        this.poly6Coeff = 4 / (Math.PI * Math.pow(this.smoothingRadius, 8));
        this.spikyGradCoeff = -10 / (Math.PI * Math.pow(this.smoothingRadius, 5));
        this.viscosityCoeff = 40 / (Math.PI * Math.pow(this.smoothingRadius, 5));
    }
}

class WaterParticleSystem {
    constructor(canvas) {
        this.canvas = canvas;
        this.ctx = canvas.getContext('2d');
        this.params = new SPHParams();
        this.particles = [];
        this.maxParticles = 0;
        this.grid = {};
        this.cellSize = this.params.smoothingRadius;
        this.isRunning = false;
        this.animationId = null;
        this.flowDirection = 1;
        this.gatePosition = { x: 0.5, y: 0.5, width: 0.05, height: 0.6 };
        this.waterLevelUp = 0.7;
        this.waterLevelDown = 0.4;
        this.gateOpening = 0.8;
        this.width = 0;
        this.height = 0;
        this.dpr = window.devicePixelRatio || 1;
        this.flowStrength = 1;
        this.spawnRate = 4;
        this.upstreamBoundary = { x: 0, y: 0, w: 0, h: 0 };
        this.downstreamBoundary = { x: 0, y: 0, w: 0, h: 0 };
        this.chamberBoundary = { x: 0, y: 0, w: 0, h: 0 };
        this.time = 0;
        this._boundaryForce = 800;
        this._particleCountTarget = 250;
        this.init();
    }

    init() {
        this.resize();
        window.addEventListener('resize', () => this.resize());
    }

    resize() {
        const rect = this.canvas.getBoundingClientRect();
        this.dpr = window.devicePixelRatio || 1;
        this.canvas.width = rect.width * this.dpr;
        this.canvas.height = rect.height * this.dpr;
        this.ctx.setTransform(this.dpr, this.dpr, 0, 0, 1, 0, 0, 1, 0, 0);
        this.width = rect.width;
        this.height = rect.height;
        this.updateBoundaries();
    }

    updateBoundaries() {
        const gw = this.width * this.gatePosition.x;
        const gh = this.height * this.gatePosition.height;
        const gy = this.height * this.gatePosition.y;
        const gx = this.width * this.gatePosition.x;
        const gateW = this.width * this.gatePosition.width;
        const gateH = gh;
        const openH = gateH * this.gateOpening;
        const gateTop = gy - gh / 2;
        const gateOpenTop = gateTop + (gh - openH);
        const gateBottom = gy + gh / 2;
        this.gateRect = {
            x: gx,
            y: gateTop,
            w: gateW,
            h: gh,
            openTop: gateOpenTop,
            openBottom: gateBottom,
            openH: openH
        };
        const upWaterTop = this.height * (1 - this.waterLevelUp);
        const downWaterTop = this.height * (1 - this.waterLevelDown);
        const bottomY = this.height * 0.96;
        this.upstreamBoundary = {
            x: 0,
            y: upWaterTop,
            w: gx,
            h: bottomY - upWaterTop,
            waterTop: upWaterTop
        };
        this.downstreamBoundary = {
            x: gx + gateW,
            y: downWaterTop,
            w: this.width - gx - gateW,
            h: bottomY - downWaterTop,
            waterTop: downWaterTop
        };
        this.chamberBoundary = {
            x: gx,
            y: Math.min(upWaterTop, downWaterTop),
            w: gateW,
            h: bottomY - Math.min(upWaterTop, downWaterTop)
        };
        this.bottomBoundary = bottomY;
    }

    setGatePosition(x, y, width, height) {
        this.gatePosition = { x, y, width, height };
        this.updateBoundaries();
    }

    setWaterLevels(up, down) {
        this.waterLevelUp = Math.max(0.05, Math.min(0.95, up));
        this.waterLevelDown = Math.max(0.05, Math.min(0.95, down));
        this.updateBoundaries();
    }

    setGateOpening(opening) {
        this.gateOpening = Math.max(0, Math.min(1, opening));
        this.updateBoundaries();
    }

    setFlowSpeed(speed) {
        this.flowStrength = Math.max(0, Math.min(3, speed));
    }

    poly6(r2) {
        if (r2 > this.params.smoothingRadius2) return 0;
        const diff = this.params.smoothingRadius2 - r2;
        return this.params.poly6Coeff * diff * diff * diff;
    }

    spikyGradient(r, rx, ry) {
        if (r > this.params.smoothingRadius || r < 0.001) {
            return { x: 0, y: 0 };
        }
        const h = this.params.smoothingRadius;
        const diff = h - r;
        const coeff = this.params.spikyGradCoeff * diff * diff / r;
        return { x: coeff * rx, y: coeff * ry };
    }

    viscosityLaplacian(r) {
        if (r > this.params.smoothingRadius) return 0;
        return this.params.viscosityCoeff * (this.params.smoothingRadius - r);
    }

    getCellKey(cx, cy) {
        return cx + "_" + cy;
    }

    buildGrid() {
        this.grid = {};
        const cs = this.cellSize;
        for (let i = 0; i < this.particles.length; i++) {
            const p = this.particles[i];
            const cx = Math.floor(p.x / cs);
            const cy = Math.floor(p.y / cs);
            const key = this.getCellKey(cx, cy);
            if (!this.grid[key]) {
                this.grid[key] = [];
            }
            this.grid[key].push(i);
        }
    }

    findNeighbors(idx) {
        const neighbors = [];
        const cs = this.cellSize;
        const p = this.particles[idx];
        const cx = Math.floor(p.x / cs);
        const cy = Math.floor(p.y / cs);
        for (let dx = -1; dx <= 1; dx++) {
            for (let dy = -1; dy <= 1; dy++) {
                const key = this.getCellKey(cx + dx, cy + dy);
                if (this.grid[key]) {
                    for (let j = 0; j < this.grid[key].length; j++) {
                        const nIdx = this.grid[key][j];
                        if (nIdx !== idx) {
                            neighbors.push(nIdx);
                        }
                    }
                }
            }
        }
        return neighbors;
    }

    computeDensityPressure() {
        const h2 = this.params.smoothingRadius2;
        for (let i = 0; i < this.particles.length; i++) {
            const pi = this.particles[i];
            pi.density = 0;
            const neighbors = this.findNeighbors(i);
            for (let j = 0; j < neighbors.length; j++) {
                const pj = this.particles[neighbors[j]];
                const dx = pj.x - pi.x;
                const dy = pj.y - pi.y;
                const r2 = dx * dx + dy * dy;
                if (r2 < h2) {
                    pi.density += this.poly6(r2);
                }
            }
            pi.density = Math.max(pi.density, this.params.restDensity * 0.3);
            pi.pressure = this.params.gasConstant * (pi.density - this.params.restDensity);
        }
    }

    computeForces() {
        const h = this.params.smoothingRadius;
        const h2 = this.params.smoothingRadius2;
        for (let i = 0; i < this.particles.length; i++) {
            const pi = this.particles[i];
            let fpressX = 0, fpressY = 0;
            let fviscX = 0, fviscY = 0;
            const neighbors = this.findNeighbors(i);
            for (let j = 0; j < neighbors.length; j++) {
                const pj = this.particles[neighbors[j]];
                const dx = pj.x - pi.x;
                const dy = pj.y - pi.y;
                const r2 = dx * dx + dy * dy;
                const r = Math.sqrt(r2);
                if (r > 0.001 && r < h) {
                    const pressTerm = (pi.pressure + pj.pressure) / (2 * Math.max(pj.density, 0.001));
                    const grad = this.spikyGradient(r, dx, dy);
                    fpressX -= pressTerm * grad.x;
                    fpressY -= pressTerm * grad.y;
                    const viscTerm = this.viscosityLaplacian(r) / Math.max(pj.density, 0.001);
                    fviscX += viscTerm * (pj.vx - pi.vx);
                    fviscY += viscTerm * (pj.vy - pi.vy);
                }
            }
            pi.fx = fpressX + fviscX * this.params.viscosity;
            pi.fy = fpressY + fviscY * this.params.viscosity;
        }
    }

    integrate() {
        const dt = this.params.dt;
        const gx = this.params.gravityX;
        const gy = this.params.gravityY;
        const damping = this.params.damping;
        const gate = this.gateRect;
        const bottom = this.bottomBoundary;
        for (let i = 0; i < this.particles.length; i++) {
            const p = this.particles[i];
            const invDensity = 1 / Math.max(p.density, 0.001);
            p.vx += dt * (p.fx * invDensity + gx);
            p.vy += dt * (p.fy * invDensity + gy);
            p.vx *= damping;
            p.vy *= damping;
            const flowForceX = this.flowStrength * 150;
            if (p.x < this.width * 0.3) {
                p.vx += flowForceX * dt * 2;
            } else if (p.x > this.width * 0.7) {
                p.vx += flowForceX * dt * 0.5;
            }
            if (p.y > gate.openTop - 20 && p.y < gate.openBottom) {
                const nearGate = Math.abs(p.x - (gate.x + gate.w / 2));
                if (nearGate < gate.w * 3) {
                    p.vx += flowForceX * dt * 5;
                }
            }
            const maxV = 400;
            const v2 = p.vx * p.vx + p.vy * p.vy;
            if (v2 > maxV * maxV) {
                const scale = maxV / Math.sqrt(v2);
                p.vx *= scale;
                p.vy *= scale;
            }
            p.x += p.vx * dt;
            p.y += p.vy * dt;
            if (p.x < 5) {
                p.x = 5;
                p.vx *= this.params.boundaryDamping;
                p.vx = Math.abs(p.vx) + 50;
            }
            if (p.x > this.width - 5) {
                p.x = this.width - 5;
                p.vx *= this.params.boundaryDamping * 0.5;
            }
            if (p.y > bottom - 5) {
                p.y = bottom - 5;
                p.vy *= this.params.boundaryDamping * 0.5;
                p.vy = -Math.abs(p.vy) * 0.5;
            }
            if (p.y < 5) {
                p.y = 5;
                p.vy = Math.abs(p.vy) * 0.5;
            }
            if (p.x >= gate.x - 5 && p.x <= gate.x + gate.w + 5) {
                const aboveOpen = p.y < gate.openTop - 2;
                const belowOpen = p.y > gate.openBottom + 2;
                if (aboveOpen || belowOpen) {
                    const midX = gate.x + gate.w / 2;
                    if (p.x < midX) {
                        p.x = gate.x - 5;
                        p.vx = -Math.abs(p.vx) * 0.5 - 30;
                    } else {
                        p.x = gate.x + gate.w + 5;
                        p.vx = Math.abs(p.vx) * 0.5 + 30;
                    }
                }
            }
        }
    }

    spawnParticles() {
        if (this.particles.length < this._particleCountTarget) {
            const target = Math.min(this.spawnRate, this._particleCountTarget - this.particles.length);
            const spawnZoneW = this.upstreamBoundary.w * 0.3;
            const spawnZoneH = this.upstreamBoundary.h * 0.8;
            const spawnX = this.upstreamBoundary.x + 10;
            const spawnY = this.upstreamBoundary.waterTop + 10;
            for (let i = 0; i < target; i++) {
                const x = spawnX + Math.random() * spawnZoneW;
                const y = spawnY + Math.random() * spawnZoneH;
                if (this.isValidSpawn(x, y)) {
                    const p = new SPHParticle(x, y);
                    p.vx = 80 + Math.random() * 40;
                    p.side = 'upstream';
                    this.particles.push(p);
                }
            }
        }
    }

    isValidSpawn(x, y) {
        for (let i = 0; i < this.particles.length; i++) {
            const p = this.particles[i];
            const dx = p.x - x;
            const dy = p.y - y;
            if (dx * dx + dy * dy < this.params.smoothingRadius2 * 0.25) {
                return false;
            }
        }
        return true;
    }

    removeEscapedParticles() {
        const maxX = this.width + 50;
        this.particles = this.particles.filter(p => {
            if (p.x > maxX) return false;
            if (p.y > this.height + 50) return false;
            if (p.y < -50) return false;
            return true;
        });
    }

    drawWaterSurfaces() {
        const ctx = this.ctx;
        const up = this.upstreamBoundary;
        const down = this.downstreamBoundary;
        const gate = this.gateRect;
        const waveAmp = 2;
        const waveSpeed = this.time * 2;
        const upWaterGrad = ctx.createLinearGradient(0, up.y, 0, this.height);
        upWaterGrad.addColorStop(0, 'rgba(100, 180, 255, 0.35)');
        upWaterGrad.addColorStop(0.5, 'rgba(50, 120, 200, 0.45)');
        upWaterGrad.addColorStop(1, 'rgba(30, 80, 150, 0.55)');
        ctx.fillStyle = upWaterGrad;
        ctx.beginPath();
        ctx.moveTo(0, up.y);
        for (let x = 0; x <= up.w; x += 4) {
            const wave = Math.sin((x + waveSpeed) * 0.05) * waveAmp;
            ctx.lineTo(x, up.y + wave);
        }
        ctx.lineTo(up.w, this.height);
        ctx.lineTo(0, this.height);
        ctx.closePath();
        ctx.fill();
        const downWaterGrad = ctx.createLinearGradient(down.x, 0, down.x, this.height);
        downWaterGrad.addColorStop(0, 'rgba(100, 180, 255, 0.35)');
        downWaterGrad.addColorStop(0.5, 'rgba(50, 120, 200, 0.45)');
        downWaterGrad.addColorStop(1, 'rgba(30, 80, 150, 0.55)');
        ctx.fillStyle = downWaterGrad;
        ctx.beginPath();
        ctx.moveTo(down.x, down.y);
        for (let x = 0; x <= down.w; x += 4) {
            const absX = down.x + x;
            const wave = Math.sin((absX + waveSpeed) * 0.05) * waveAmp;
            ctx.lineTo(absX, down.y + wave);
        }
        ctx.lineTo(down.x + down.w, this.height);
        ctx.lineTo(down.x, this.height);
        ctx.closePath();
        ctx.fill();
        ctx.strokeStyle = 'rgba(150, 210, 255, 0.5)';
        ctx.lineWidth = 1.5;
        ctx.beginPath();
        for (let x = 0; x <= up.w; x += 4) {
            const wave = Math.sin((x + waveSpeed) * 0.05) * waveAmp;
            if (x === 0) { ctx.moveTo(x, up.y + wave); }
            else { ctx.lineTo(x, up.y + wave); }
        }
        ctx.stroke();
        ctx.beginPath();
        for (let x = 0; x <= down.w; x += 4) {
            const absX = down.x + x;
            const wave = Math.sin((absX + waveSpeed) * 0.05) * waveAmp;
            if (x === 0) { ctx.moveTo(absX, down.y + wave); }
            else { ctx.lineTo(absX, down.y + wave); }
        }
        ctx.stroke();
    }

    drawGate() {
        const ctx = this.ctx;
        const gate = this.gateRect;
        const openH = gate.openH;
        const closedH = gate.h - openH;
        const grad = ctx.createLinearGradient(gate.x, 0, gate.x + gate.w, 0);
        grad.addColorStop(0, '#5a4a3a');
        grad.addColorStop(0.5, '#8b7355');
        grad.addColorStop(1, '#5a4a3a');
        ctx.fillStyle = grad;
        ctx.fillRect(gate.x, gate.y, gate.w, closedH);
        const frameGrad = ctx.createLinearGradient(gate.x, 0, gate.x + gate.w, 0);
        frameGrad.addColorStop(0, '#6a5a4a');
        frameGrad.addColorStop(0.5, '#9b8365');
        frameGrad.addColorStop(1, '#6a5a4a');
        ctx.fillStyle = frameGrad;
        ctx.fillRect(gate.x - 6, gate.y - 10, gate.w + 12, 14);
        ctx.strokeStyle = 'rgba(0, 0, 0, 0.3)';
        ctx.lineWidth = 1;
        for (let i = 1; i < 4; i++) {
            const y = gate.y + (closedH / 5) * i;
            ctx.beginPath();
            ctx.moveTo(gate.x, y);
            ctx.lineTo(gate.x + gate.w, y);
            ctx.stroke();
        }
        if (openH > 5) {
            const flowGrad = ctx.createLinearGradient(gate.x, gate.openTop, gate.x + gate.w, gate.openBottom);
            flowGrad.addColorStop(0, 'rgba(150, 220, 255, 0.4)');
            flowGrad.addColorStop(0.5, 'rgba(100, 180, 255, 0.6)');
            flowGrad.addColorStop(1, 'rgba(80, 160, 255, 0.4)');
            ctx.fillStyle = flowGrad;
            ctx.fillRect(gate.x, gate.openTop, gate.w, openH);
            ctx.strokeStyle = 'rgba(200, 240, 255, 0.6)';
            ctx.lineWidth = 1;
            const lines = 3;
            for (let i = 0; i < lines; i++) {
                const offset = ((this.time * 60 + i * 20) % openH;
                ctx.beginPath();
                ctx.moveTo(gate.x + 2, gate.openTop + offset);
                ctx.lineTo(gate.x + gate.w - 2, gate.openTop + offset);
                ctx.stroke();
            }
        }
    }

    drawParticles() {
        const ctx = this.ctx;
        for (let i = 0; i < this.particles.length; i++) {
            const p = this.particles[i];
            const alpha = p.colorAlpha * Math.min(1, p.density / this.params.restDensity + 0.3);
            ctx.beginPath();
            ctx.arc(p.x, p.y, p.size, 0, Math.PI * 2);
            ctx.fillStyle = `rgba(120, 190, 255, ${alpha})`;
            ctx.fill();
            if (p.size > 2.5) {
                ctx.beginPath();
                ctx.arc(p.x - p.size * 0.25, p.y - p.size * 0.25, p.size * 0.35, 0, Math.PI * 2);
                ctx.fillStyle = `rgba(200, 230, 255, ${alpha * 0.7})`;
                ctx.fill();
            }
        }
    }

    drawVelocities() {
        if (this.particles.length < 1) return;
        const ctx = this.ctx;
        const step = Math.max(1, Math.floor(this.particles.length / 60));
        ctx.strokeStyle = 'rgba(255, 255, 100, 0.15)';
        ctx.lineWidth = 1;
        for (let i = 0; i < this.particles.length; i += step) {
            const p = this.particles[i];
            const scale = 0.05;
            ctx.beginPath();
            ctx.moveTo(p.x, p.y);
            ctx.lineTo(p.x + p.vx * scale, p.y + p.vy * scale);
            ctx.stroke();
        }
    }

    animate() {
        if (!this.isRunning) return;
        this.time += this.params.dt;
        this.spawnParticles();
        this.buildGrid();
        this.computeDensityPressure();
        this.computeForces();
        this.integrate();
        this.removeEscapedParticles();
        this.ctx.clearRect(0, 0, this.width, this.height);
        this.drawWaterSurfaces();
        this.drawGate();
        this.drawParticles();
        this.animationId = requestAnimationFrame(() => this.animate());
    }

    start() {
        if (!this.isRunning) {
            this.isRunning = true;
            this.particles = [];
            const upArea = this.upstreamBoundary;
            const initCount = 120;
            for (let i = 0; i < initCount; i++) {
                const x = 20 + Math.random() * (upArea.w * 0.8);
                const y = upArea.waterTop + 20 + Math.random() * (upArea.h * 0.7);
                const p = new SPHParticle(x, y);
                p.vx = 60 + Math.random() * 40;
                p.colorAlpha = 0.4 + Math.random() * 0.3;
                this.particles.push(p);
            }
            this.updateBoundaries();
            this.animate();
        }
    }

    stop() {
        this.isRunning = false;
        if (this.animationId) {
            cancelAnimationFrame(this.animationId);
            this.animationId = null;
        }
    }

    pause() {
        this.isRunning = false;
    }

    resume() {
        if (!this.isRunning) {
            this.isRunning = true;
            this.animate();
        }
    }
}
