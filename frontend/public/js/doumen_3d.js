class DouGateScene {
    constructor(containerId) {
        this.container = document.getElementById(containerId);
        if (!this.container) {
            console.error('Container not found:', containerId);
            return;
        }
        this.scene = null;
        this.camera = null;
        this.renderer = null;
        this.gateGroup = null;
        this.gateDoor = null;
        this.gateHeight = 8;
        this.gateWidth = 6;
        this.waterUp = null;
        this.waterDown = null;
        this.waterLevelUp = 0.7;
        this.waterLevelDown = 0.4;
        this.gateOpening = 0.8;
        this.isAutoRotate = false;
        this.autoRotateSpeed = 0.002;
        this.theta = 0.5;
        this.phi = Math.PI / 3;
        this.radius = 25;
        this.isDragging = false;
        this.lastMouseX = 0;
        this.lastMouseY = 0;
        this.clock = null;
        this.waterParticles = null;
        this.shipManager = null;
        this.targetPosition = new THREE.Vector3(0, 2, 0);
        this.animationId = null;
        this.isRunning = false;
        this.init();
    }

    init() {
        const width = this.container.clientWidth;
        const height = this.container.clientHeight;

        this.scene = new THREE.Scene();
        this.scene.background = new THREE.Color(0x87ceeb);
        this.scene.fog = new THREE.Fog(0x87ceeb, 40, 80);

        this.camera = new THREE.PerspectiveCamera(60, width / height, 0.1, 1000);
        this.updateCamera();

        this.renderer = new THREE.WebGLRenderer({ antialias: true });
        this.renderer.setSize(width, height);
        this.renderer.shadowMap.enabled = true;
        this.renderer.shadowMap.type = THREE.PCFSoftShadowMap;
        this.container.appendChild(this.renderer.domElement);

        this.setupLighting();
        this.createGate();
        this.createWater();
        this.createEnvironment();
        this.setupControls();
        this.clock = new THREE.Clock();

        window.addEventListener('resize', () => this.onResize());

        this.animate();
    }

    setupLighting() {
        const ambientLight = new THREE.AmbientLight(0xffffff, 0.5);
        this.scene.add(ambientLight);

        const directionalLight = new THREE.DirectionalLight(0xffffff, 0.8);
        directionalLight.position.set(10, 20, 10);
        directionalLight.castShadow = true;
        directionalLight.shadow.mapSize.width = 2048;
        directionalLight.shadow.mapSize.height = 2048;
        directionalLight.shadow.camera.near = 0.5;
        directionalLight.shadow.camera.far = 100;
        directionalLight.shadow.camera.left = -30;
        directionalLight.shadow.camera.right = 30;
        directionalLight.shadow.camera.top = 30;
        directionalLight.shadow.camera.bottom = -30;
        this.scene.add(directionalLight);

        const fillLight = new THREE.DirectionalLight(0x88ccff, 0.3);
        fillLight.position.set(-10, 10, -10);
        this.scene.add(fillLight);

        const pointLight = new THREE.PointLight(0xffffcc, 0.3, 30);
        pointLight.position.set(0, 15, 0);
        this.scene.add(pointLight);
    }

    createGate() {
        this.gateGroup = new THREE.Group();

        const stoneTexture = this.createStoneTexture();
        const wallMaterial = new THREE.MeshPhongMaterial({
            map: stoneTexture,
            specular: 0x111111,
            shininess: 5
        });

        const chamberLength = 18;
        const chamberWidth = this.gateWidth;
        const wallHeight = 12;
        const wallThickness = 2;

        const leftWall = new THREE.Mesh(
            new THREE.BoxGeometry(wallThickness, wallHeight, chamberLength),
            wallMaterial
        );
        leftWall.position.set(-chamberWidth / 2 - wallThickness / 2, wallHeight / 2, 0);
        leftWall.castShadow = true;
        leftWall.receiveShadow = true;
        this.gateGroup.add(leftWall);

        const rightWall = new THREE.Mesh(
            new THREE.BoxGeometry(wallThickness, wallHeight, chamberLength),
            wallMaterial
        );
        rightWall.position.set(chamberWidth / 2 + wallThickness / 2, wallHeight / 2, 0);
        rightWall.castShadow = true;
        rightWall.receiveShadow = true;
        this.gateGroup.add(rightWall);

        const stepMaterial = wallMaterial.clone();
        for (let side = -1; side <= 1; side += 2) {
            for (let i = 0; i < 5; i++) {
                const stepHeight = 0.8;
                const stepWidth = 0.8;
                const step = new THREE.Mesh(
                    new THREE.BoxGeometry(stepWidth, stepHeight, chamberLength * 0.9),
                    stepMaterial
                );
                step.position.set(
                    side * (chamberWidth / 2 + wallThickness - stepWidth / 2),
                    2 + i * stepHeight,
                    0
                );
                step.receiveShadow = true;
                this.gateGroup.add(step);
            }
        }

        const gateMaterial = new THREE.MeshPhongMaterial({
            color: 0x4a3728,
            specular: 0x222222,
            shininess: 10
        });

        const gateDoorWidth = chamberWidth * 0.95;
        const gateDoorHeight = this.gateHeight;
        const gateDoorThickness = 0.6;

        this.gateDoor = new THREE.Group();

        const leftDoor = new THREE.Mesh(
            new THREE.BoxGeometry(gateDoorWidth / 2, gateDoorHeight, gateDoorThickness),
            gateMaterial
        );
        leftDoor.position.set(-gateDoorWidth / 4, gateDoorHeight / 2, 0);
        leftDoor.castShadow = true;
        this.gateDoor.add(leftDoor);

        const rightDoor = new THREE.Mesh(
            new THREE.BoxGeometry(gateDoorWidth / 2, gateDoorHeight, gateDoorThickness),
            gateMaterial
        );
        rightDoor.position.set(gateDoorWidth / 4, gateDoorHeight / 2, 0);
        rightDoor.castShadow = true;
        this.gateDoor.add(rightDoor);

        for (let i = 0; i < 4; i++) {
            const beam = new THREE.Mesh(
                new THREE.BoxGeometry(gateDoorWidth * 0.9, 0.15, gateDoorThickness + 0.2),
                new THREE.MeshPhongMaterial({ color: 0x3d2b1f })
            );
            beam.position.set(0, (i + 1) * gateDoorHeight / 5, 0);
            this.gateDoor.add(beam);
        }

        this.gateDoor.position.set(0, 0, -chamberLength / 2 + gateDoorThickness / 2);
        this.gateGroup.add(this.gateDoor);

        const topBeam = new THREE.Mesh(
            new THREE.BoxGeometry(gateDoorWidth + 1, 1.5, 2),
            new THREE.MeshPhongMaterial({ color: 0x5d4e37 })
        );
        topBeam.position.set(0, gateDoorHeight + 0.5, -chamberLength / 2 + 0.5);
        topBeam.castShadow = true;
        this.gateGroup.add(topBeam);

        const winchMaterial = new THREE.MeshPhongMaterial({ color: 0x654321 });
        for (let side = -1; side <= 1; side += 2) {
            const winch = new THREE.Mesh(
                new THREE.CylinderGeometry(0.8, 0.8, 0.4, 16),
                winchMaterial
            );
            winch.rotation.z = Math.PI / 2;
            winch.position.set(side * gateDoorWidth / 3, gateDoorHeight + 1, -chamberLength / 2 + 0.5);
            this.gateGroup.add(winch);
        }

        this.gateGroup.position.y = 0;
        this.scene.add(this.gateGroup);
    }

    createStoneTexture() {
        const canvas = document.createElement('canvas');
        canvas.width = 256;
        canvas.height = 256;
        const ctx = canvas.getContext('2d');

        ctx.fillStyle = '#8b7355';
        ctx.fillRect(0, 0, 256, 256);

        for (let i = 0; i < 500; i++) {
            const x = Math.random() * 256;
            const y = Math.random() * 256;
            const size = Math.random() * 4 + 1;
            const alpha = Math.random() * 0.3;
            ctx.fillStyle = `rgba(60, 50, 30, ${alpha})`;
            ctx.fillRect(x, y, size, size);
        }

        for (let i = 0; i < 10; i++) {
            const y = Math.random() * 256;
            ctx.strokeStyle = 'rgba(50, 40, 20, 0.2)';
            ctx.lineWidth = 1;
            ctx.beginPath();
            ctx.moveTo(0, y);
            ctx.lineTo(256, y + Math.random() * 10 - 5);
            ctx.stroke();
        }

        const texture = new THREE.CanvasTexture(canvas);
        texture.wrapS = THREE.RepeatWrapping;
        texture.wrapT = THREE.RepeatWrapping;
        return texture;
    }

    createWater() {
        const waterMaterial = new THREE.MeshPhongMaterial({
            color: 0x4da6ff,
            transparent: true,
            opacity: 0.7,
            side: THREE.DoubleSide,
            specular: 0xffffff,
            shininess: 100
        });

        const waterGeomUp = new THREE.PlaneGeometry(30, 30);
        this.waterUp = new THREE.Mesh(waterGeomUp, waterMaterial.clone());
        this.waterUp.rotation.x = -Math.PI / 2;
        this.waterUp.position.set(0, this.waterLevelUp * 5, -15);
        this.waterUp.receiveShadow = true;
        this.scene.add(this.waterUp);

        const waterGeomDown = new THREE.PlaneGeometry(30, 30);
        this.waterDown = new THREE.Mesh(waterGeomDown, waterMaterial.clone());
        this.waterDown.rotation.x = -Math.PI / 2;
        this.waterDown.position.set(0, this.waterLevelDown * 5, 15);
        this.waterDown.receiveShadow = true;
        this.scene.add(this.waterDown);
    }

    createEnvironment() {
        const groundGeom = new THREE.PlaneGeometry(100, 100, 50, 50);
        const positions = groundGeom.attributes.position;
        for (let i = 0; i < positions.count; i++) {
            const x = positions.getX(i);
            const y = positions.getY(i);
            const z = Math.sin(x * 0.05) * Math.cos(y * 0.05) * 1.5;
            positions.setZ(i, z);
        }
        groundGeom.computeVertexNormals();

        const groundMaterial = new THREE.MeshPhongMaterial({ color: 0x6b8e23 });
        const ground = new THREE.Mesh(groundGeom, groundMaterial);
        ground.rotation.x = -Math.PI / 2;
        ground.position.y = -1;
        ground.receiveShadow = true;
        this.scene.add(ground);

        const treePositions = [
            [-12, 0, 12], [10, 0, 14], [-10, 0, -12], [12, 0, -10],
            [-8, 0, 16], [14, 0, 8], [-14, 0, -8], [8, 0, -14],
            [-16, 0, 4], [6, 0, 18], [-6, 0, -16], [18, 0, -4],
            [-20, 0, -4], [16, 0, -12], [-18, 0, 10], [4, 0, -18],
            [-14, 0, 16], [18, 0, 14], [-20, 0, 16], [20, 0, 16],
            [-16, 0, -18], [14, 0, -18], [-18, 0, -14], [18, 0, -16],
            [-10, 0, 20], [8, 0, 22], [-8, 0, -22], [10, 0, -22],
            [-22, 0, 0], [22, 0, 0]
        ];

        treePositions.forEach(([x, y, z]) => {
            const tree = this.createTree();
            tree.position.set(x, y, z);
            const scale = 0.7 + Math.random() * 0.6;
            tree.scale.set(scale, scale, scale);
            this.scene.add(tree);
        });

        for (let i = 0; i < 8; i++) {
            const mountain = this.createMountain();
            const angle = (i / 8) * Math.PI * 2;
            const distance = 35 + Math.random() * 10;
            mountain.position.set(
                Math.cos(angle) * distance,
                -1 + Math.random() * 2,
                Math.sin(angle) * distance
            );
            const scale = 2 + Math.random() * 3;
            mountain.scale.set(scale, scale * (1 + Math.random()), scale);
            this.scene.add(mountain);
        }
    }

    createTree() {
        const tree = new THREE.Group();

        const trunkGeom = new THREE.CylinderGeometry(0.2, 0.3, 2, 8);
        const trunkMaterial = new THREE.MeshPhongMaterial({ color: 0x8b4513 });
        const trunk = new THREE.Mesh(trunkGeom, trunkMaterial);
        trunk.position.y = 1;
        trunk.castShadow = true;
        tree.add(trunk);

        const crownGeom = new THREE.ConeGeometry(1.2, 3, 8);
        const crownMaterial = new THREE.MeshPhongMaterial({ color: 0x228b22 });
        const crown = new THREE.Mesh(crownGeom, crownMaterial);
        crown.position.y = 3;
        crown.castShadow = true;
        tree.add(crown);

        const crown2 = new THREE.Mesh(
            new THREE.ConeGeometry(0.9, 2, 8),
            crownMaterial.clone()
        );
        crown2.position.y = 4.5;
        crown2.castShadow = true;
        tree.add(crown2);

        return tree;
    }

    createMountain() {
        const mountainGeom = new THREE.ConeGeometry(3, 6, 6);
        const mountainMaterial = new THREE.MeshPhongMaterial({ color: 0x6b7b5b });
        const mountain = new THREE.Mesh(mountainGeom, mountainMaterial);
        mountain.castShadow = true;
        return mountain;
    }

    setupControls() {
        const canvas = this.renderer.domElement;

        canvas.addEventListener('mousedown', (e) => {
            this.isDragging = true;
            this.lastMouseX = e.clientX;
            this.lastMouseY = e.clientY;
        });

        window.addEventListener('mouseup', () => {
            this.isDragging = false;
        });

        window.addEventListener('mousemove', (e) => {
            if (!this.isDragging) return;

            const deltaX = e.clientX - this.lastMouseX;
            const deltaY = e.clientY - this.lastMouseY;

            this.theta -= deltaX * 0.01;
            this.phi -= deltaY * 0.01;

            this.phi = Math.max(0.1, Math.min(Math.PI / 2 - 0.1, this.phi));

            this.updateCamera();

            this.lastMouseX = e.clientX;
            this.lastMouseY = e.clientY;
        });

        canvas.addEventListener('wheel', (e) => {
            e.preventDefault();
            this.radius += e.deltaY * 0.05;
            this.radius = Math.max(10, Math.min(60, this.radius));
            this.updateCamera();
        });

        canvas.addEventListener('touchstart', (e) => {
            if (e.touches.length === 1) {
                this.isDragging = true;
                this.lastMouseX = e.touches[0].clientX;
                this.lastMouseY = e.touches[0].clientY;
            }
        });

        canvas.addEventListener('touchend', () => {
            this.isDragging = false;
        });

        canvas.addEventListener('touchmove', (e) => {
            if (!this.isDragging || e.touches.length !== 1) return;
            e.preventDefault();

            const deltaX = e.touches[0].clientX - this.lastMouseX;
            const deltaY = e.touches[0].clientY - this.lastMouseY;

            this.theta -= deltaX * 0.01;
            this.phi -= deltaY * 0.01;
            this.phi = Math.max(0.1, Math.min(Math.PI / 2 - 0.1, this.phi));

            this.updateCamera();

            this.lastMouseX = e.touches[0].clientX;
            this.lastMouseY = e.touches[0].clientY;
        });
    }

    updateCamera() {
        const x = this.radius * Math.sin(this.phi) * Math.cos(this.theta);
        const y = this.radius * Math.cos(this.phi);
        const z = this.radius * Math.sin(this.phi) * Math.sin(this.theta);

        this.camera.position.set(
            this.targetPosition.x + x,
            this.targetPosition.y + y,
            this.targetPosition.z + z
        );
        this.camera.lookAt(this.targetPosition);
    }

    setGateOpening(opening) {
        this.gateOpening = Math.max(0, Math.min(1, opening));
        if (this.gateDoor) {
            const travel = this.gateHeight * this.gateOpening;
            this.gateDoor.position.y = -travel;
        }
    }

    setWaterLevels(upLevel, downLevel) {
        this.waterLevelUp = Math.max(0, Math.min(1, upLevel));
        this.waterLevelDown = Math.max(0, Math.min(1, downLevel));

        const maxWater = 6;
        if (this.waterUp) {
            this.waterUp.position.y = 0.5 + this.waterLevelUp * maxWater;
            this.waterUp.scale.y = this.waterLevelUp * 0.5 + 0.5;
        }
        if (this.waterDown) {
            this.waterDown.position.y = 0.5 + this.waterLevelDown * maxWater;
            this.waterDown.scale.y = this.waterLevelDown * 0.5 + 0.5;
        }
    }

    setAutoRotate(enabled) {
        this.isAutoRotate = enabled;
    }

    setTargetPosition(x, y, z) {
        this.targetPosition.set(x, y, z);
        this.updateCamera();
    }

    onResize() {
        if (!this.container || !this.camera || !this.renderer) return;
        const width = this.container.clientWidth;
        const height = this.container.clientHeight;
        this.camera.aspect = width / height;
        this.camera.updateProjectionMatrix();
        this.renderer.setSize(width, height);
    }

    animate() {
        this.animationId = requestAnimationFrame(() => this.animate());

        if (this.isAutoRotate) {
            this.theta += this.autoRotateSpeed;
            this.updateCamera();
        }

        const delta = this.clock ? this.clock.getDelta() : 0.016;
        const time = this.clock ? this.clock.getElapsedTime() : 0;

        if (this.waterUp && this.waterUp.material) {
            this.waterUp.material.opacity = 0.65 + Math.sin(time * 1.5) * 0.05;
        }
        if (this.waterDown && this.waterDown.material) {
            this.waterDown.material.opacity = 0.65 + Math.sin(time * 1.5 + 1) * 0.05;
        }

        if (this.shipManager) {
            this.shipManager.update(delta);
        }

        this.renderer.render(this.scene, this.camera);
    }

    addShip(shipData) {
        if (!this.shipManager) {
            this.shipManager = new ShipManager(this.scene);
        }
        return this.shipManager.addShip(shipData);
    }

    removeShip(shipId) {
        if (this.shipManager) {
            this.shipManager.removeShip(shipId);
        }
    }

    clearShips() {
        if (this.shipManager) {
            this.shipManager.clear();
        }
    }

    setWaterParticlesEnabled(enabled, particleCanvasId) {
        if (enabled && particleCanvasId) {
            this.waterParticles = new WaterParticleSystem(
                document.getElementById(particleCanvasId)
            );
            this.waterParticles.start();
        } else if (this.waterParticles) {
            this.waterParticles.stop();
            this.waterParticles = null;
        }
    }

    dispose() {
        if (this.animationId) {
            cancelAnimationFrame(this.animationId);
        }
        if (this.waterParticles) {
            this.waterParticles.stop();
        }
        if (this.renderer) {
            this.renderer.dispose();
        }
    }
}

class ShipManager {
    constructor(scene) {
        this.scene = scene;
        this.ships = new Map();
        this.pathPoints = [];
    }

    addShip(shipData) {
        const ship = this.createShipModel(shipData);
        const id = shipData.id || Date.now() + Math.random();
        this.ships.set(id, {
            mesh: ship,
            data: shipData,
            progress: 0,
            state: 'approach',
            targetY: shipData.targetY || 0,
            startY: shipData.startY || 0
        });
        return id;
    }

    removeShip(shipId) {
        const ship = this.ships.get(shipId);
        if (ship) {
            this.scene.remove(ship.mesh);
            this.ships.delete(shipId);
        }
    }

    clear() {
        this.ships.forEach((ship) => {
            this.scene.remove(ship.mesh);
        });
        this.ships.clear();
    }

    createShipModel(shipData) {
        const shipGroup = new THREE.Group();

        const hullLength = 8 + (shipData.length || 0) * 0.1;
        const hullWidth = 2;
        const hullHeight = 1.5;

        const hullGeom = new THREE.BoxGeometry(hullLength, hullHeight, hullWidth);
        const hullMaterial = new THREE.MeshPhongMaterial({
            color: shipData.color || 0x8b4513
        });
        const hull = new THREE.Mesh(hullGeom, hullMaterial);
        hull.position.y = hullHeight / 2;
        hull.castShadow = true;
        shipGroup.add(hull);

        const bowGeom = new THREE.ConeGeometry(hullWidth * 0.4, 1.5, 4);
        const bow = new THREE.Mesh(bowGeom, hullMaterial);
        bow.rotation.z = -Math.PI / 2;
        bow.position.set(hullLength / 2 + 0.75, hullHeight / 2, 0);
        bow.castShadow = true;
        shipGroup.add(bow);

        const sternGeom = new THREE.BoxGeometry(0.8, hullHeight * 0.8, hullWidth);
        const stern = new THREE.Mesh(sternGeom, hullMaterial);
        stern.position.set(-hullLength / 2 - 0.4, hullHeight * 0.4, 0);
        stern.castShadow = true;
        shipGroup.add(stern);

        const cabinLength = hullLength * 0.4;
        const cabinWidth = hullWidth * 0.8;
        const cabinHeight = 1.2;
        const cabinGeom = new THREE.BoxGeometry(cabinLength, cabinHeight, cabinWidth);
        const cabinMaterial = new THREE.MeshPhongMaterial({ color: 0x654321 });
        const cabin = new THREE.Mesh(cabinGeom, cabinMaterial);
        cabin.position.set(-hullLength * 0.1, hullHeight + cabinHeight / 2, 0);
        cabin.castShadow = true;
        shipGroup.add(cabin);

        const cabinRoofGeom = new THREE.BoxGeometry(cabinLength * 1.1, 0.2, cabinWidth * 1.1);
        const cabinRoof = new THREE.Mesh(cabinRoofGeom, cabinMaterial);
        cabinRoof.position.set(-hullLength * 0.1, hullHeight + cabinHeight + 0.1, 0);
        shipGroup.add(cabinRoof);

        const windowMaterial = new THREE.MeshPhongMaterial({
            color: 0xadd8e6,
            transparent: true,
            opacity: 0.8
        });
        for (let i = 0; i < 3; i++) {
            const windowGeom = new THREE.BoxGeometry(0.4, 0.5, 0.05);
            const window = new THREE.Mesh(windowGeom, windowMaterial);
            window.position.set(
                -hullLength * 0.1 + (i - 1) * cabinLength * 0.35,
                hullHeight + cabinHeight * 0.5,
                cabinWidth / 2 + 0.03
            );
            shipGroup.add(window);

            const window2 = window.clone();
            window2.position.z = -cabinWidth / 2 - 0.03;
            shipGroup.add(window2);
        }

        const mastGeom = new THREE.CylinderGeometry(0.05, 0.05, 3, 8);
        const mastMaterial = new THREE.MeshPhongMaterial({ color: 0x5d4e37 });
        const mast = new THREE.Mesh(mastGeom, mastMaterial);
        mast.position.set(hullLength * 0.2, hullHeight + 1.5, 0);
        mast.castShadow = true;
        shipGroup.add(mast);

        const sailGeom = new THREE.PlaneGeometry(2.5, 3);
        const sailMaterial = new THREE.MeshPhongMaterial({
            color: 0xf5f5dc,
            side: THREE.DoubleSide,
            transparent: true,
            opacity: 0.9
        });
        const sail = new THREE.Mesh(sailGeom, sailMaterial);
        sail.position.set(hullLength * 0.2 + 0.5, hullHeight + 1.5, 0);
        sail.rotation.y = Math.PI / 6;
        shipGroup.add(sail);

        shipGroup.position.set(-20, 0.5, 0);
        return shipGroup;
    }

    update(delta) {
        this.ships.forEach((ship, id) => {
            const speed = 0.05;
            ship.progress += speed * delta;

            if (ship.progress < 0.4) {
                ship.state = 'approach';
                const t = ship.progress / 0.4;
                ship.mesh.position.x = -20 + t * 12;
                ship.mesh.position.z = Math.sin(t * Math.PI * 2) * 0.5;
                ship.mesh.rotation.z = Math.sin(t * Math.PI) * 0.03;
                ship.mesh.rotation.y = Math.sin(t * Math.PI) * 0.05;
            } else if (ship.progress < 0.6) {
                ship.state = 'entering';
                const t = (ship.progress - 0.4) / 0.2;
                ship.mesh.position.x = -8 + t * 10;
                ship.mesh.position.z = Math.sin(t * Math.PI) * 0.3;
            } else if (ship.progress < 0.85) {
                ship.state = 'waiting';
                const t = (ship.progress - 0.6) / 0.25;
                ship.mesh.position.x = 2;
                ship.mesh.position.y = ship.startY + t * (ship.targetY - ship.startY) * 0.8;
                ship.mesh.rotation.z = Math.sin(Date.now() * 0.002) * 0.01;
                ship.mesh.position.x = 2 + Math.sin(t * Math.PI * 3) * 0.2;
            } else {
                ship.state = 'exiting';
                const t = (ship.progress - 0.85) / 0.15;
                ship.mesh.position.x = 2 + t * 18;
                ship.mesh.position.z = Math.sin(t * Math.PI) * 0.3;
                if (ship.targetY > ship.startY) {
                    ship.mesh.position.y = ship.targetY;
                }
            }
        });
    }
}
