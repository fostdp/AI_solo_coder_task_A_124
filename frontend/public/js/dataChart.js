class DataChart {
    constructor(canvas) {
        this.canvas = canvas;
        this.ctx = canvas.getContext('2d');
        this.data = [];
        this.maxDataPoints = 50;
        this.colors = {
            line1: '#4fc3f7',
            line2: '#81d4fa',
            fill: 'rgba(79, 195, 247, 0.2)',
            grid: 'rgba(79, 195, 247, 0.2)',
            text: '#90caf9'
        };
        
        this.resize();
    }

    resize() {
        const rect = this.canvas.getBoundingClientRect();
        this.canvas.width = rect.width * window.devicePixelRatio;
        this.canvas.height = rect.height * window.devicePixelRatio;
        this.ctx.scale(window.devicePixelRatio, window.devicePixelRatio);
        this.width = rect.width;
        this.height = rect.height;
    }

    setData(data, valueKey = 'water_level') {
        this.data = data.map(d => ({
            time: new Date(d.time || d.timestamp || d.x),
            value: d[valueKey] !== undefined ? d[valueKey] : d.value
        }));
        
        if (this.data.length > this.maxDataPoints) {
            this.data = this.data.slice(-this.maxDataPoints);
        }
        
        this.draw();
    }

    addDataPoint(value, time) {
        this.data.push({
            time: time || new Date(),
            value: value
        });
        
        if (this.data.length > this.maxDataPoints) {
            this.data.shift();
        }
        
        this.draw();
    }

    draw() {
        this.ctx.clearRect(0, 0, this.width, this.height);
        
        if (this.data.length < 2) {
            this.drawPlaceholder();
            return;
        }

        const padding = { top: 20, right: 40, bottom: 25, left: 10 };
        const chartWidth = this.width - padding.left - padding.right;
        const chartHeight = this.height - padding.top - padding.bottom;

        const values = this.data.map(d => d.value);
        const minVal = Math.min(...values) * 0.95;
        const maxVal = Math.max(...values) * 1.05;
        const valueRange = maxVal - minVal || 1;

        this.drawGrid(padding, chartWidth, chartHeight, minVal, maxVal, valueRange);
        this.drawLine(padding, chartWidth, chartHeight, minVal, valueRange);
        this.drawLabels(padding, chartWidth, chartHeight, minVal, maxVal);
    }

    drawGrid(padding, chartWidth, chartHeight, minVal, maxVal, valueRange) {
        this.ctx.strokeStyle = this.colors.grid;
        this.ctx.lineWidth = 0.5;
        
        for (let i = 0; i <= 4; i++) {
            const y = padding.top + (i / 4) * chartHeight;
            this.ctx.beginPath();
            this.ctx.moveTo(padding.left, y);
            this.ctx.lineTo(padding.left + chartWidth, y);
            this.ctx.stroke();
        }
    }

    drawLine(padding, chartWidth, chartHeight, minVal, valueRange) {
        const gradient = this.ctx.createLinearGradient(0, padding.top, 0, padding.top + chartHeight);
        gradient.addColorStop(0, 'rgba(79, 195, 247, 0.4)');
        gradient.addColorStop(1, 'rgba(79, 195, 247, 0.05)');

        this.ctx.beginPath();
        this.ctx.moveTo(padding.left, padding.top + chartHeight);
        
        this.data.forEach((d, i) => {
            const x = padding.left + (i / (this.data.length - 1)) * chartWidth;
            const y = padding.top + chartHeight - ((d.value - minVal) / valueRange) * chartHeight;
            
            if (i === 0) {
                this.ctx.lineTo(x, y);
            } else {
                const prevX = padding.left + ((i - 1) / (this.data.length - 1)) * chartWidth;
                const prevY = padding.top + chartHeight - ((this.data[i-1].value - minVal) / valueRange) * chartHeight;
                const cpX = (prevX + x) / 2;
                this.ctx.quadraticCurveTo(prevX, prevY, cpX, (prevY + y) / 2);
                this.ctx.quadraticCurveTo(cpX, (prevY + y) / 2, x, y);
            }
        });
        
        this.ctx.lineTo(padding.left + chartWidth, padding.top + chartHeight);
        this.ctx.closePath();
        this.ctx.fillStyle = gradient;
        this.ctx.fill();

        this.ctx.beginPath();
        this.data.forEach((d, i) => {
            const x = padding.left + (i / (this.data.length - 1)) * chartWidth;
            const y = padding.top + chartHeight - ((d.value - minVal) / valueRange) * chartHeight;
            
            if (i === 0) {
                this.ctx.moveTo(x, y);
            } else {
                const prevX = padding.left + ((i - 1) / (this.data.length - 1)) * chartWidth;
                const prevY = padding.top + chartHeight - ((this.data[i-1].value - minVal) / valueRange) * chartHeight;
                const cpX = (prevX + x) / 2;
                this.ctx.quadraticCurveTo(prevX, prevY, cpX, (prevY + y) / 2);
                this.ctx.quadraticCurveTo(cpX, (prevY + y) / 2, x, y);
            }
        });
        
        this.ctx.strokeStyle = this.colors.line1;
        this.ctx.lineWidth = 2;
        this.ctx.stroke();

        const lastPoint = this.data[this.data.length - 1];
        const lastX = padding.left + chartWidth;
        const lastY = padding.top + chartHeight - ((lastPoint.value - minVal) / valueRange) * chartHeight;
        
        this.ctx.beginPath();
        this.ctx.arc(lastX, lastY, 4, 0, Math.PI * 2);
        this.ctx.fillStyle = this.colors.line1;
        this.ctx.fill();
        this.ctx.strokeStyle = '#fff';
        this.ctx.lineWidth = 2;
        this.ctx.stroke();
    }

    drawLabels(padding, chartWidth, chartHeight, minVal, maxVal) {
        this.ctx.fillStyle = this.colors.text;
        this.ctx.font = '10px sans-serif';
        this.ctx.textAlign = 'left';
        
        this.ctx.fillText(maxVal.toFixed(1), padding.left + chartWidth + 5, padding.top + 4);
        this.ctx.fillText(minVal.toFixed(1), padding.left + chartWidth + 5, padding.top + chartHeight);

        if (this.data.length > 0) {
            const lastTime = this.data[this.data.length - 1].time;
            const firstTime = this.data[0].time;
            const timeStr = lastTime.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
            
            this.ctx.textAlign = 'right';
            this.ctx.fillText(timeStr, padding.left + chartWidth, padding.top + chartHeight + 15);
        }
    }

    drawPlaceholder() {
        this.ctx.fillStyle = this.colors.text;
        this.ctx.font = '11px sans-serif';
        this.ctx.textAlign = 'center';
        this.ctx.fillText('等待数据...', this.width / 2, this.height / 2);
    }

    setColorScheme(scheme) {
        if (scheme === 'water') {
            this.colors = {
                line1: '#4fc3f7',
                line2: '#81d4fa',
                fill: 'rgba(79, 195, 247, 0.2)',
                grid: 'rgba(79, 195, 247, 0.2)',
                text: '#90caf9'
            };
        } else if (scheme === 'flow') {
            this.colors = {
                line1: '#81c784',
                line2: '#a5d6a7',
                fill: 'rgba(129, 199, 132, 0.2)',
                grid: 'rgba(129, 199, 132, 0.2)',
                text: '#a5d6a7'
            };
        } else if (scheme === 'warning') {
            this.colors = {
                line1: '#ffb74d',
                line2: '#ffcc80',
                fill: 'rgba(255, 183, 77, 0.2)',
                grid: 'rgba(255, 183, 77, 0.2)',
                text: '#ffcc80'
            };
        }
    }
}
