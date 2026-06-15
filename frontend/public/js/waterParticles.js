class WaterParticleSystem {
    constructor(canvas) {
        this.canvas = canvas;
        this.ctx = canvas.getContext('2d');
        this.particles = [];
        this.particleCount = 200;
        this.flowSpeed = 1;
        this.flowDirection = 'right';
        this.isRunning = false;
        this.animationId = null;
        
        this.gatePosition = { x: 0.5, y: 0.5, width: 0.05, height: 0.6 };
        this.waterLevelUp = 0.7;
        this.waterLevelDown = 0.4;
        
        this.init();
    }

    init() {
        this.resize();
        window.addEventListener('resize', () => this.resize());
    }

    resize() {
        const rect = this.canvas.getBoundingClientRect();
        this.canvas.width = rect.width * window.devicePixelRatio;
        this.canvas.height = rect.height * window.devicePixelRatio;
        this.ctx.scale(window.devicePixelRatio, window.devicePixelRatio);
        this.width = rect.width;
        this.height = rect.height;
    }

    setGatePosition(x, y, width, height) {
        this.gatePosition = { x, y, width, height };
    }

    setWaterLevels(up, down) {
        this.waterLevelUp = up;
        this.waterLevelDown = down;
    }

    setFlowSpeed(speed) {
        this.flowSpeed = speed;
    }

    createParticle(side) {
        const gateX = this.width * this.gatePosition.x;
        const gateY = this.height * this.gatePosition.y;
        const gateH = this.height * this.gatePosition.height;
        
        if (side === 'upstream') {
            return {
                x: Math.random() * (gateX - this.width * 0.1),
                y: this.height * (1 - this.waterLevelUp) + Math.random() * (this.height * this.waterLevelUp * 0.9),
                vx: -0.5 + Math.random(),
                vy: (Math.random() - 0.5) * 0.5,
                size: 1 + Math.random() * 2,
                alpha: 0.3 + Math.random() * 0.4,
                life: 1,
                side: 'upstream'
            };
        } else {
            return {
                x: gateX + this.width * this.gatePosition.width + Math.random() * (this.width * 0.3),
                y: this.height * (1 - this.waterLevelDown) + Math.random() * (this.height * this.waterLevelDown * 0.9),
                vx: -0.5 + Math.random(),
                vy: (Math.random() - 0.5) * 0.5,
                size: 1 + Math.random() * 2,
                alpha: 0.3 + Math.random() * 0.4,
                life: 1,
                side: 'downstream'
            };
        }
    }

    updateParticle(p) {
        const gateX = this.width * this.gatePosition.x;
        const gateW = this.width * this.gatePosition.width;
        const gateTop = this.height * this.gatePosition.y - this.height * this.gatePosition.height / 2;
        const gateBottom = this.height * this.gatePosition.y + this.height * this.gatePosition.height / 2;
        
        const upWaterTop = this.height * (1 - this.waterLevelUp);
        const downWaterTop = this.height * (1 - this.waterLevelDown);
        
        const gateOpenTop = gateTop + (gateBottom - gateTop) * (1 - 0.8);
        const gateOpenBottom = gateBottom;
        
        if (p.side === 'upstream') {
            if (p.x > gateX - 10 && p.x < gateX + gateW + 10) {
                if (p.y > gateOpenTop && p.y < gateOpenBottom) {
                    p.vx = this.flowSpeed * 3;
                    p.vy += (Math.random() - 0.5) * 0.5;
                }
            }
            
            if (p.x > gateX + gateW) {
                p.side = 'downstream';
            }
        } else {
            p.vx = Math.max(p.vx * 0.98, this.flowSpeed * 0.5);
            
            if (p.x > this.width * 0.9) {
                p.life -= 0.02;
            }
        }
        
        p.x += p.vx;
        p.y += p.vy;
        
        const waterSurface = p.side === 'upstream' ? upWaterTop : downWaterTop;
        if (p.y < waterSurface) {
            p.y = waterSurface;
            p.vy = Math.abs(p.vy) * 0.3;
        }
        
        if (p.y > this.height * 0.95) {
            p.y = this.height * 0.95;
            p.vy = -Math.abs(p.vy) * 0.3;
        }
        
        p.vy += 0.02;
        p.vy *= 0.99;
        
        return p.life > 0 && p.x > -10 && p.x < this.width + 10;
    }

    drawParticle(p) {
        this.ctx.beginPath();
        this.ctx.arc(p.x, p.y, p.size, 0, Math.PI * 2);
        this.ctx.fillStyle = `rgba(100, 180, 255, ${p.alpha})`;
        this.ctx.fill();
    }

    drawWaterSurface() {
        const upWaterTop = this.height * (1 - this.waterLevelUp);
        const downWaterTop = this.height * (1 - this.waterLevelDown);
        const gateX = this.width * this.gatePosition.x;
        const gateW = this.width * this.gatePosition.width;
        
        const gradient = this.ctx.createLinearGradient(0, upWaterTop, 0, this.height);
        gradient.addColorStop(0, 'rgba(100, 180, 255, 0.3)');
        gradient.addColorStop(0.5, 'rgba(50, 120, 200, 0.4)');
        gradient.addColorStop(1, 'rgba(30, 80, 150, 0.5)');
        
        this.ctx.fillStyle = gradient;
        this.ctx.beginPath();
        this.ctx.moveTo(0, upWaterTop);
        for (let x = 0; x <= gateX; x += 5) {
            const wave = Math.sin((x + Date.now() * 0.002) * 0.05) * 2;
            this.ctx.lineTo(x, upWaterTop + wave);
        }
        this.ctx.lineTo(gateX, this.height);
        this.ctx.lineTo(0, this.height);
        this.ctx.closePath();
        this.ctx.fill();
        
        const gradient2 = this.ctx.createLinearGradient(0, downWaterTop, 0, this.height);
        gradient2.addColorStop(0, 'rgba(100, 180, 255, 0.3)');
        gradient2.addColorStop(0.5, 'rgba(50, 120, 200, 0.4)');
        gradient2.addColorStop(1, 'rgba(30, 80, 150, 0.5)');
        
        this.ctx.fillStyle = gradient2;
        this.ctx.beginPath();
        this.ctx.moveTo(gateX + gateW, downWaterTop);
        for (let x = gateX + gateW; x <= this.width; x += 5) {
            const wave = Math.sin((x + Date.now() * 0.002) * 0.05) * 2;
            this.ctx.lineTo(x, downWaterTop + wave);
        }
        this.ctx.lineTo(this.width, this.height);
        this.ctx.lineTo(gateX + gateW, this.height);
        this.ctx.closePath();
        this.ctx.fill();
        
        this.ctx.strokeStyle = 'rgba(150, 200, 255, 0.6)';
        this.ctx.lineWidth = 1.5;
        this.ctx.beginPath();
        for (let x = 0; x <= gateX; x += 5) {
            const wave = Math.sin((x + Date.now() * 0.002) * 0.05) * 2;
            if (x === 0) {
                this.ctx.moveTo(x, upWaterTop + wave);
            } else {
                this.ctx.lineTo(x, upWaterTop + wave);
            }
        }
        this.ctx.stroke();
        
        this.ctx.beginPath();
        for (let x = gateX + gateW; x <= this.width; x += 5) {
            const wave = Math.sin((x + Date.now() * 0.002) * 0.05) * 2;
            if (x === gateX + gateW) {
                this.ctx.moveTo(x, downWaterTop + wave);
            } else {
                this.ctx.lineTo(x, downWaterTop + wave);
            }
        }
        this.ctx.stroke();
    }

    drawGate() {
        const gateX = this.width * this.gatePosition.x;
        const gateY = this.height * this.gatePosition.y;
        const gateW = this.width * this.gatePosition.width;
        const gateH = this.height * this.gatePosition.height;
        const opening = 0.8;
        const openH = gateH * opening;
        
        const gradient = this.ctx.createLinearGradient(gateX, 0, gateX + gateW, 0);
        gradient.addColorStop(0, '#5a4a3a');
        gradient.addColorStop(0.5, '#8b7355');
        gradient.addColorStop(1, '#5a4a3a');
        
        this.ctx.fillStyle = gradient;
        this.ctx.fillRect(gateX, gateY - gateH / 2, gateW, gateH - openH);
        
        const gradient2 = this.ctx.createLinearGradient(gateX, 0, gateX + gateW, 0);
        gradient2.addColorStop(0, '#6a5a4a');
        gradient2.addColorStop(0.5, '#9b8365');
        gradient2.addColorStop(1, '#6a5a4a');
        
        this.ctx.fillStyle = gradient2;
        this.ctx.fillRect(gateX - 5, gateY - gateH / 2 - 10, gateW + 10, 15);
        
        this.ctx.strokeStyle = 'rgba(0, 0, 0, 0.3)';
        this.ctx.lineWidth = 1;
        for (let i = 1; i < 5; i++) {
            const y = gateY - gateH / 2 + (gateH - openH) * (i / 5);
            this.ctx.beginPath();
            this.ctx.moveTo(gateX, y);
            this.ctx.lineTo(gateX + gateW, y);
            this.ctx.stroke();
        }
    }

    animate() {
        if (!this.isRunning) return;
        
        this.ctx.clearRect(0, 0, this.width, this.height);
        
        this.drawWaterSurface();
        this.drawGate();
        
        while (this.particles.length < this.particleCount) {
            const side = Math.random() < 0.6 ? 'upstream' : 'downstream';
            this.particles.push(this.createParticle(side));
        }
        
        this.particles = this.particles.filter(p => this.updateParticle(p));
        
        this.particles.forEach(p => this.drawParticle(p));
        
        this.animationId = requestAnimationFrame(() => this.animate());
    }

    start() {
        if (!this.isRunning) {
            this.isRunning = true;
            this.particles = [];
            for (let i = 0; i < this.particleCount; i++) {
                this.particles.push(this.createParticle(Math.random() < 0.5 ? 'upstream' : 'downstream'));
            }
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
