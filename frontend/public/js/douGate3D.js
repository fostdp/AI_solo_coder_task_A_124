class DouGate3D {
    constructor(canvas) {
        this.canvas = canvas;
        this.scene = null;
        this.camera = null;
        this.renderer = null;
        this.controls = null;
        this.gateGroup = null;
        this.waterUp = null;
        this.waterDown = null;
        this.gateDoor = null;
        this.isAutoRotate = false;
        this.animationId = null;
        this.gateConfig = {
            width: 6,
            height: 4.5,
            chamberLength: 60,
            chamberWidth: 10,
            wallHeight: 6,
            wallThickness: 1.5
        };
        
        this.waterLevelUp = 7.5;
        this.waterLevelDown = 3.5;
        this.gateOpening = 0;
        this.flowRate = 0;
        
        this.init();
    }

    init() {
        this.scene = new THREE.Scene();
        this.scene.background = new THREE.Color(0x0a1628);
        this.scene.fog = new THREE.Fog(0x0a1628, 50, 150);

        const rect = this.canvas.getBoundingClientRect();
        this.camera = new THREE.PerspectiveCamera(60, rect.width / rect.height, 0.1, 1000);
        this.camera.position.set(40, 25, 40);
        this.camera.lookAt(0, 5, 0);

        this.renderer = new THREE.WebGLRenderer({ 
            canvas: this.canvas, 
            antialias: true,
            alpha: true 
        });
        this.renderer.setSize(rect.width, rect.height);
        this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
        this.renderer.shadowMap.enabled = true;
        this.renderer.shadowMap.type = THREE.PCFSoftShadowMap;

        this.setupLights();
        this.createGate();
        this.createWater();
        this.createEnvironment();
        this.setupControls();

        window.addEventListener('resize', () => this.onResize());
        
        this.animate();
    }

    setupLights() {
        const ambientLight = new THREE.AmbientLight(0x404060, 0.5);
        this.scene.add(ambientLight);

        const directionalLight = new THREE.DirectionalLight(0xffffff, 0.8);
        directionalLight.position.set(30, 50, 30);
        directionalLight.castShadow = true;
        directionalLight.shadow.mapSize.width = 2048;
        directionalLight.shadow.mapSize.height = 2048;
        directionalLight.shadow.camera.near = 0.5;
        directionalLight.shadow.camera.far = 200;
        directionalLight.shadow.camera.left = -60;
        directionalLight.shadow.camera.right = 60;
        directionalLight.shadow.camera.top = 60;
        directionalLight.shadow.camera.bottom = -60;
        this.scene.add(directionalLight);

        const fillLight = new THREE.DirectionalLight(0x4488cc, 0.3);
        fillLight.position.set(-20, 20, -20);
        this.scene.add(fillLight);

        const pointLight1 = new THREE.PointLight(0x4fc3f7, 0.5, 50);
        pointLight1.position.set(0, 10, 0);
        this.scene.add(pointLight1);
    }

    createGate() {
        this.gateGroup = new THREE.Group();
        
        const { chamberLength, chamberWidth, wallHeight, wallThickness } = this.gateConfig;
        
        const wallMaterial = new THREE.MeshStandardMaterial({
            color: 0x8b7355,
            roughness: 0.8,
            metalness: 0.1
        });
        
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

        const stoneTexture = this.createStoneTexture();
        const stoneMaterial = new THREE.MeshStandardMaterial({
            map: stoneTexture,
            roughness: 0.9,
            metalness: 0.05
        });

        for (let i = 0; i < 5; i++) {
            const zPos = -chamberLength / 2 + i * (chamberLength / 4);
            const stepHeight = wallHeight * 0.2 + i * wallHeight * 0.15;
            
            const leftStep = new THREE.Mesh(
                new THREE.BoxGeometry(wallThickness * 0.6, 0.5, 3),
                stoneMaterial
            );
            leftStep.position.set(-chamberWidth / 2 - wallThickness / 2 + 0.3, stepHeight, zPos);
            this.gateGroup.add(leftStep);

            const rightStep = new THREE.Mesh(
                new THREE.BoxGeometry(wallThickness * 0.6, 0.5, 3),
                stoneMaterial
            );
            rightStep.position.set(chamberWidth / 2 + wallThickness / 2 - 0.3, stepHeight, zPos);
            this.gateGroup.add(rightStep);
        }

        this.createGateDoor();
        this.createGateMechanism();

        this.scene.add(this.gateGroup);
    }

    createStoneTexture() {
        const canvas = document.createElement('canvas');
        canvas.width = 256;
        canvas.height = 256;
        const ctx = canvas.getContext('2d');
        
        ctx.fillStyle = '#8b7355';
        ctx.fillRect(0, 0, 256, 256);
        
        for (let i = 0; i < 50; i++) {
            const x = Math.random() * 256;
            const y = Math.random() * 256;
            const w = 20 + Math.random() * 40;
            const h = 15 + Math.random() * 30;
            
            ctx.fillStyle = `rgba(${100 + Math.random() * 30}, ${80 + Math.random() * 30}, ${60 + Math.random() * 20}, 0.5)`;
            ctx.fillRect(x, y, w, h);
            
            ctx.strokeStyle = 'rgba(60, 50, 40, 0.3)';
            ctx.lineWidth = 1;
            ctx.strokeRect(x, y, w, h);
        }
        
        const texture = new THREE.CanvasTexture(canvas);
        texture.wrapS = THREE.RepeatWrapping;
        texture.wrapT = THREE.RepeatWrapping;
        return texture;
    }

    createGateDoor() {
        const { width, height, chamberWidth } = this.gateConfig;
        
        const doorMaterial = new THREE.MeshStandardMaterial({
            color: 0x5a4a3a,
            roughness: 0.7,
            metalness: 0.2
        });
        
        this.gateDoor = new THREE.Group();
        
        const doorLeft = new THREE.Mesh(
            new THREE.BoxGeometry(0.4, height, width / 2 - 0.2),
            doorMaterial
        );
        doorLeft.position.set(-chamberWidth / 2 + 0.2, height / 2, 0);
        doorLeft.castShadow = true;
        this.gateDoor.add(doorLeft);

        const doorRight = new THREE.Mesh(
            new THREE.BoxGeometry(0.4, height, width / 2 - 0.2),
            doorMaterial
        );
        doorRight.position.set(chamberWidth / 2 - 0.2, height / 2, 0);
        doorRight.castShadow = true;
        this.gateDoor.add(doorRight);
        
        const beamMaterial = new THREE.MeshStandardMaterial({
            color: 0x4a3a2a,
            roughness: 0.6,
            metalness: 0.3
        });
        
        for (let i = 0; i < 4; i++) {
            const yPos = height * 0.2 + i * height * 0.25;
            
            const beamLeft = new THREE.Mesh(
                new THREE.BoxGeometry(0.5, 0.3, width / 2 + 1),
                beamMaterial
            );
            beamLeft.position.set(-chamberWidth / 2 + 0.25, yPos, 0);
            this.gateDoor.add(beamLeft);

            const beamRight = new THREE.Mesh(
                new THREE.BoxGeometry(0.5, 0.3, width / 2 + 1),
                beamMaterial
            );
            beamRight.position.set(chamberWidth / 2 - 0.25, yPos, 0);
            this.gateDoor.add(beamRight);
        }

        this.gateDoor.position.z = -this.gateConfig.chamberLength / 2;
        this.gateGroup.add(this.gateDoor);
    }

    createGateMechanism() {
        const { chamberWidth, chamberLength, wallHeight } = this.gateConfig;
        
        const mechanismMaterial = new THREE.MeshStandardMaterial({
            color: 0x3a3a3a,
            roughness: 0.5,
            metalness: 0.7
        });
        
        const topBeam = new THREE.Mesh(
            new THREE.BoxGeometry(chamberWidth + 6, 1, 4),
            mechanismMaterial
        );
        topBeam.position.set(0, wallHeight + 0.5, -chamberLength / 2);
        topBeam.castShadow = true;
        this.gateGroup.add(topBeam);

        const wheelMaterial = new THREE.MeshStandardMaterial({
            color: 0x5a4a3a,
            roughness: 0.6,
            metalness: 0.3
        });

        const wheel1 = new THREE.Mesh(
            new THREE.CylinderGeometry(1.5, 1.5, 0.3, 16),
            wheelMaterial
        );
        wheel1.rotation.z = Math.PI / 2;
        wheel1.position.set(-3, wallHeight + 1, -chamberLength / 2);
        this.gateGroup.add(wheel1);

        const wheel2 = new THREE.Mesh(
            new THREE.CylinderGeometry(1.5, 1.5, 0.3, 16),
            wheelMaterial
        );
        wheel2.rotation.z = Math.PI / 2;
        wheel2.position.set(3, wallHeight + 1, -chamberLength / 2);
        this.gateGroup.add(wheel2);
    }

    createWater() {
        const { chamberLength, chamberWidth, wallThickness } = this.gateConfig;
        
        const waterMaterial = new THREE.MeshPhongMaterial({
            color: 0x4488cc,
            transparent: true,
            opacity: 0.7,
            side: THREE.DoubleSide,
            shininess: 100
        });

        this.waterUp = new THREE.Mesh(
            new THREE.BoxGeometry(chamberWidth + wallThickness * 2, 10, chamberLength / 2 + 10),
            waterMaterial
        );
        this.waterUp.position.set(0, -5, -chamberLength * 0.75);
        this.scene.add(this.waterUp);

        this.waterDown = new THREE.Mesh(
            new THREE.BoxGeometry(chamberWidth + wallThickness * 2, 10, chamberLength / 2 + 30),
            waterMaterial.clone()
        );
        this.waterDown.position.set(0, -5, chamberLength * 0.5);
        this.scene.add(this.waterDown);

        this.updateWaterLevels(this.waterLevelUp, this.waterLevelDown);
    }

    createEnvironment() {
        const groundGeometry = new THREE.PlaneGeometry(300, 300, 50, 50);
        const groundMaterial = new THREE.MeshStandardMaterial({
            color: 0x2a3a2a,
            roughness: 1,
            metalness: 0
        });
        
        const positions = groundGeometry.attributes.position;
        for (let i = 0; i < positions.count; i++) {
            const x = positions.getX(i);
            const y = positions.getY(i);
            const z = Math.sin(x * 0.05) * Math.cos(y * 0.05) * 2;
            positions.setZ(i, z);
        }
        groundGeometry.computeVertexNormals();

        const ground = new THREE.Mesh(groundGeometry, groundMaterial);
        ground.rotation.x = -Math.PI / 2;
        ground.position.y = -1;
        ground.receiveShadow = true;
        this.scene.add(ground);

        const treeMaterial = new THREE.MeshStandardMaterial({
            color: 0x1a4a1a,
            roughness: 0.9
        });
        const trunkMaterial = new THREE.MeshStandardMaterial({
            color: 0x4a3a2a,
            roughness: 0.8
        });

        for (let i = 0; i < 30; i++) {
            const angle = Math.random() * Math.PI * 2;
            const radius = 40 + Math.random() * 60;
            const x = Math.cos(angle) * radius;
            const z = Math.sin(angle) * radius;

            const trunk = new THREE.Mesh(
                new THREE.CylinderGeometry(0.5, 0.7, 4, 8),
                trunkMaterial
            );
            trunk.position.set(x, 1, z);
            trunk.castShadow = true;
            this.scene.add(trunk);

            const tree = new THREE.Mesh(
                new THREE.ConeGeometry(2 + Math.random(), 5 + Math.random() * 3, 8),
                treeMaterial
            );
            tree.position.set(x, 5 + Math.random(), z);
            tree.castShadow = true;
            this.scene.add(tree);
        }

        const mountainMaterial = new THREE.MeshStandardMaterial({
            color: 0x3a4a5a,
            roughness: 1
        });

        for (let i = 0; i < 8; i++) {
            const angle = (i / 8) * Math.PI * 2;
            const radius = 80 + Math.random() * 40;
            const x = Math.cos(angle) * radius;
            const z = Math.sin(angle) * radius;

            const mountain = new THREE.Mesh(
                new THREE.ConeGeometry(15 + Math.random() * 10, 20 + Math.random() * 15, 6),
                mountainMaterial
            );
            mountain.position.set(x, 8 + Math.random() * 5, z);
            mountain.castShadow = true;
            this.scene.add(mountain);
        }
    }

    setupControls() {
        let isDragging = false;
        let previousMousePosition = { x: 0, y: 0 };
        let spherical = { theta: Math.PI / 4, phi: Math.PI / 4, radius: 60 };
        let target = new THREE.Vector3(0, 5, 0);

        const updateCamera = () => {
            this.camera.position.x = target.x + spherical.radius * Math.sin(spherical.phi) * Math.cos(spherical.theta);
            this.camera.position.y = target.y + spherical.radius * Math.cos(spherical.phi);
            this.camera.position.z = target.z + spherical.radius * Math.sin(spherical.phi) * Math.sin(spherical.theta);
            this.camera.lookAt(target);
        };

        this.canvas.addEventListener('mousedown', (e) => {
            isDragging = true;
            previousMousePosition = { x: e.clientX, y: e.clientY };
        });

        this.canvas.addEventListener('mousemove', (e) => {
            if (!isDragging) return;

            const deltaX = e.clientX - previousMousePosition.x;
            const deltaY = e.clientY - previousMousePosition.y;

            spherical.theta -= deltaX * 0.01;
            spherical.phi = Math.max(0.1, Math.min(Math.PI - 0.1, spherical.phi + deltaY * 0.01));

            updateCamera();
            previousMousePosition = { x: e.clientX, y: e.clientY };
        });

        this.canvas.addEventListener('mouseup', () => {
            isDragging = false;
        });

        this.canvas.addEventListener('mouseleave', () => {
            isDragging = false;
        });

        this.canvas.addEventListener('wheel', (e) => {
            e.preventDefault();
            spherical.radius = Math.max(20, Math.min(120, spherical.radius + e.deltaY * 0.1));
            updateCamera();
        });

        updateCamera();

        this.controls = {
            update: () => {
                if (this.isAutoRotate) {
                    spherical.theta += 0.002;
                    updateCamera();
                }
            },
            reset: () => {
                spherical = { theta: Math.PI / 4, phi: Math.PI / 4, radius: 60 };
                target = new THREE.Vector3(0, 5, 0);
                updateCamera();
            },
            target
        };
    }

    updateWaterLevels(up, down) {
        this.waterLevelUp = up;
        this.waterLevelDown = down;
        
        if (this.waterUp) {
            this.waterUp.scale.y = Math.max(0.1, up / 5);
            this.waterUp.position.y = up / 2 - 0.5;
        }
        
        if (this.waterDown) {
            this.waterDown.scale.y = Math.max(0.1, down / 5);
            this.waterDown.position.y = down / 2 - 0.5;
        }
    }

    setGateOpening(opening) {
        this.gateOpening = opening;
        if (this.gateDoor) {
            const maxMove = this.gateConfig.height * 0.8;
            this.gateDoor.position.y = opening * maxMove;
        }
    }

    setGateConfig(config) {
        if (config.gate_width) this.gateConfig.width = config.gate_width;
        if (config.gate_height) this.gateConfig.height = config.gate_height;
        if (config.chamber_length) this.gateConfig.chamberLength = config.chamber_length;
        if (config.chamber_width) this.gateConfig.chamberWidth = config.chamber_width;
    }

    toggleAutoRotate() {
        this.isAutoRotate = !this.isAutoRotate;
        return this.isAutoRotate;
    }

    resetView() {
        if (this.controls && this.controls.reset) {
            this.controls.reset();
        }
    }

    onResize() {
        const rect = this.canvas.getBoundingClientRect();
        this.camera.aspect = rect.width / rect.height;
        this.camera.updateProjectionMatrix();
        this.renderer.setSize(rect.width, rect.height);
    }

    animate() {
        this.animationId = requestAnimationFrame(() => this.animate());
        
        if (this.controls && this.controls.update) {
            this.controls.update();
        }
        
        const time = Date.now() * 0.001;
        if (this.waterUp) {
            this.waterUp.material.opacity = 0.65 + Math.sin(time * 2) * 0.05;
        }
        if (this.waterDown) {
            this.waterDown.material.opacity = 0.65 + Math.sin(time * 2 + 1) * 0.05;
        }

        this.renderer.render(this.scene, this.camera);
    }

    dispose() {
        if (this.animationId) {
            cancelAnimationFrame(this.animationId);
        }
        this.renderer.dispose();
    }
}
