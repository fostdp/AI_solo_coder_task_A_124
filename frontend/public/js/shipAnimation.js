class ShipAnimation {
    constructor(scene, gateConfig) {
        this.scene = scene;
        this.gateConfig = gateConfig;
        this.ship = null;
        this.isAnimating = false;
        this.direction = 'upstream';
        this.progress = 0;
        this.speed = 0.002;
        this.callback = null;
        
        this.chamberProgress = 0;
        this.phase = 'approach';
        this.animationId = null;
        
        this.init();
    }

    init() {
        this.createShip();
    }

    createShip() {
        this.ship = new THREE.Group();
        
        const hullMaterial = new THREE.MeshStandardMaterial({
            color: 0x8b4513,
            roughness: 0.7,
            metalness: 0.1
        });
        
        const hull = new THREE.Mesh(
            new THREE.BoxGeometry(4, 1.5, 15),
            hullMaterial
        );
        hull.position.y = 0.75;
        hull.castShadow = true;
        this.ship.add(hull);

        const bow = new THREE.Mesh(
            new THREE.ConeGeometry(2, 3, 4),
            hullMaterial
        );
        bow.rotation.x = Math.PI / 2;
        bow.rotation.z = Math.PI / 4;
        bow.position.set(0, 0.75, 9);
        bow.castShadow = true;
        this.ship.add(bow);

        const stern = new THREE.Mesh(
            new THREE.BoxGeometry(3.5, 1.2, 2),
            hullMaterial
        );
        stern.position.set(0, 0.6, -8);
        stern.castShadow = true;
        this.ship.add(stern);

        const cabinMaterial = new THREE.MeshStandardMaterial({
            color: 0xd4a574,
            roughness: 0.6,
            metalness: 0.2
        });

        const cabin = new THREE.Mesh(
            new THREE.BoxGeometry(3, 2, 6),
            cabinMaterial
        );
        cabin.position.set(0, 2.2, -1);
        cabin.castShadow = true;
        this.ship.add(cabin);

        const roof = new THREE.Mesh(
            new THREE.BoxGeometry(3.2, 0.2, 6.2),
            new THREE.MeshStandardMaterial({ color: 0x8b4513, roughness: 0.8 })
        );
        roof.position.set(0, 3.2, -1);
        roof.castShadow = true;
        this.ship.add(roof);

        const windowMaterial = new THREE.MeshStandardMaterial({
            color: 0x87ceeb,
            transparent: true,
            opacity: 0.7,
            roughness: 0.3,
            metalness: 0.5
        });

        for (let i = -1; i <= 1; i++) {
            const windowLeft = new THREE.Mesh(
                new THREE.BoxGeometry(0.1, 0.8, 1),
                windowMaterial
            );
            windowLeft.position.set(-1.51, 2.2, i * 2);
            this.ship.add(windowLeft);

            const windowRight = new THREE.Mesh(
                new THREE.BoxGeometry(0.1, 0.8, 1),
                windowMaterial
            );
            windowRight.position.set(1.51, 2.2, i * 2);
            this.ship.add(windowRight);
        }

        const mastMaterial = new THREE.MeshStandardMaterial({
            color: 0x5a4a3a,
            roughness: 0.7
        });

        const mast = new THREE.Mesh(
            new THREE.CylinderGeometry(0.1, 0.15, 6, 8),
            mastMaterial
        );
        mast.position.set(0, 5, 2);
        mast.castShadow = true;
        this.ship.add(mast);

        const sailMaterial = new THREE.MeshStandardMaterial({
            color: 0xf5f5dc,
            side: THREE.DoubleSide,
            roughness: 0.9
        });

        const sail = new THREE.Mesh(
            new THREE.PlaneGeometry(3, 4),
            sailMaterial
        );
        sail.position.set(0, 5, 2.1);
        sail.rotation.y = 0.2;
        this.ship.add(sail);

        this.ship.position.set(0, 0, 50);
        this.ship.visible = false;
        this.scene.add(this.ship);
    }

    setDirection(direction) {
        this.direction = direction;
        if (direction === 'upstream') {
            this.ship.rotation.y = Math.PI;
        } else {
            this.ship.rotation.y = 0;
        }
    }

    startPassage(direction, onComplete) {
        if (this.isAnimating) return;
        
        this.isAnimating = true;
        this.direction = direction;
        this.callback = onComplete;
        this.progress = 0;
        this.phase = 'approach';
        this.ship.visible = true;
        
        if (direction === 'upstream') {
            this.ship.rotation.y = Math.PI;
            this.ship.position.set(0, 0, 50);
        } else {
            this.ship.rotation.y = 0;
            this.ship.position.set(0, 0, -50);
        }
        
        this.animate();
    }

    animate() {
        if (!this.isAnimating) return;
        
        const { chamberLength } = this.gateConfig;
        const speed = this.speed;
        
        if (this.direction === 'upstream') {
            if (this.phase === 'approach') {
                this.progress += speed;
                this.ship.position.z = 50 - this.progress * 100;
                
                this.ship.position.y = Math.sin(this.progress * Math.PI * 4) * 0.2;
                this.ship.rotation.z = Math.sin(this.progress * Math.PI * 3) * 0.02;
                
                if (this.progress >= 0.4) {
                    this.phase = 'entering';
                }
            } else if (this.phase === 'entering') {
                this.progress += speed * 0.5;
                this.ship.position.z = 10 - (this.progress - 0.4) * 100;
                
                if (this.progress >= 0.6) {
                    this.phase = 'waiting';
                }
            } else if (this.phase === 'waiting') {
                this.progress += speed * 0.2;
                
                this.ship.position.y = 0 + (this.progress - 0.6) * 20;
                this.ship.position.z = -10 + Math.sin(this.progress * 10) * 0.3;
                
                if (this.progress >= 0.85) {
                    this.phase = 'exiting';
                }
            } else if (this.phase === 'exiting') {
                this.progress += speed;
                this.ship.position.z = -10 - (this.progress - 0.85) * 200;
                this.ship.position.y = 5 + (this.progress - 0.85) * 20;
                
                if (this.progress >= 1) {
                    this.complete();
                }
            }
        } else {
            if (this.phase === 'approach') {
                this.progress += speed;
                this.ship.position.z = -50 + this.progress * 100;
                
                this.ship.position.y = Math.sin(this.progress * Math.PI * 4) * 0.2;
                this.ship.rotation.z = Math.sin(this.progress * Math.PI * 3) * 0.02;
                
                if (this.progress >= 0.4) {
                    this.phase = 'entering';
                }
            } else if (this.phase === 'entering') {
                this.progress += speed * 0.5;
                this.ship.position.z = -10 + (this.progress - 0.4) * 100;
                
                if (this.progress >= 0.6) {
                    this.phase = 'waiting';
                }
            } else if (this.phase === 'waiting') {
                this.progress += speed * 0.2;
                
                this.ship.position.y = 5 - (this.progress - 0.6) * 20;
                this.ship.position.z = 10 + Math.sin(this.progress * 10) * 0.3;
                
                if (this.progress >= 0.85) {
                    this.phase = 'exiting';
                }
            } else if (this.phase === 'exiting') {
                this.progress += speed;
                this.ship.position.z = 10 + (this.progress - 0.85) * 200;
                this.ship.position.y = 0 - (this.progress - 0.85) * 20;
                
                if (this.progress >= 1) {
                    this.complete();
                }
            }
        }
        
        this.animationId = requestAnimationFrame(() => this.animate());
    }

    complete() {
        this.isAnimating = false;
        this.ship.visible = false;
        this.progress = 0;
        this.phase = 'approach';
        
        if (this.animationId) {
            cancelAnimationFrame(this.animationId);
        }
        
        if (this.callback) {
            this.callback();
        }
    }

    pause() {
        this.isAnimating = false;
        if (this.animationId) {
            cancelAnimationFrame(this.animationId);
        }
    }

    resume() {
        if (!this.isAnimating && this.progress < 1 && this.progress > 0) {
            this.isAnimating = true;
            this.animate();
        }
    }

    reset() {
        this.isAnimating = false;
        this.progress = 0;
        this.phase = 'approach';
        this.ship.visible = false;
        if (this.animationId) {
            cancelAnimationFrame(this.animationId);
        }
    }

    setWaterLevelOffset(offset) {
        if (this.ship) {
            this.ship.position.y = offset + Math.sin(this.progress * Math.PI * 4) * 0.2;
        }
    }
}
