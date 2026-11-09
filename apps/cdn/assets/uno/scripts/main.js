import * as THREE from 'three';
import { createWorldControl } from './world_control.js?v=radial-table-20261105-v10';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';
import { HDRLoader } from 'three/addons/loaders/HDRLoader.js';
import { createCardsControl } from './cards_control.js?v=radial-table-20261105-v21';
import { createPlayerHeads } from './player_heads.js?v=radial-table-20261105-v21';
import { createVolumeRenderer } from './volume_shaders.js';
import { createFog } from './create_fog.js';
import { createCardTrail } from './card_trail.js';

const query = new URLSearchParams(window.location.search);
const gameID = query.get('gameId')?.trim() || '';
const viewerID = query.get('playerId')?.trim() || '';
const spectatorMode = query.get('spectate') === '1';
const networkMode = Boolean(gameID);
const roundTable = query.has('seats');
const seatCount = THREE.MathUtils.clamp(Number(query.get('seats')) || 4, 1, 8);
const startingHandSize = THREE.MathUtils.clamp(Number(query.get('hand')) || 7, 3, 14);
const tableRadius = 1.15 + Math.max(0, seatCount - 4) * 0.035;

async function requestGame(path, options = {}) {
    const token = sessionStorage.getItem('card-arena.access-token');
    const headers = new Headers(options.headers || {});
    headers.set('Accept', 'application/json');
    if (token) headers.set('Authorization', `Bearer ${token}`);
    const response = await fetch(`/api/v1/games/${encodeURIComponent(gameID)}/uno/${path}`, { ...options, headers });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(payload.message || `UNO request failed (${response.status})`);
    return payload;
}

let initialUnoState = null;
if (networkMode) {
    try {
        initialUnoState = await requestGame('state');
    } catch (error) {
        console.error(error);
    }
}

async function addRoundTable(scene, count) {
    if (!roundTable) return;

    const loader = new GLTFLoader();

    const [tableGLB, chairGLB] = await Promise.all([
        loader.loadAsync('./models/table.glb'),
        loader.loadAsync('./models/chair.glb'),
    ]);

    const chairRadius = tableRadius + 0.22;

    function prepareModel(model) {
        model.traverse((object) => {
            if (!object.isMesh) return;
            object.castShadow = true;
            object.receiveShadow = true;
        });
        const group = new THREE.Group();
        group.add(model);
        return group;
    }

    const table = prepareModel(tableGLB.scene);
    table.scale.setScalar(1.44);
    scene.add(table);

    const chairTemplate = prepareModel(chairGLB.scene);
    chairTemplate.setScalar
    for (let id = 0; id < count; id++) {
        const angle = (id / count) * Math.PI * 2;
        const chair = chairTemplate.clone(true);
        chair.position.set(
            Math.sin(angle) * chairRadius,
            1,
            Math.cos(angle) * chairRadius
        );
        chair.rotation.y = angle;
        scene.add(chair);
    }
}


const renderer = new THREE.WebGLRenderer({ antialias: true });
renderer.setClearColor('#2a384c');
renderer.toneMapping = THREE.AgXToneMapping;
renderer.toneMappingExposure = 1;
renderer.shadowMap.enabled = true;
renderer.shadowMap.type = THREE.PCFSoftShadowMap;
document.body.appendChild(renderer.domElement);
renderer.setSize(innerWidth, innerHeight);
renderer.clear();

const scene = new THREE.Scene();
await createWorldControl(scene);
await addRoundTable(scene, seatCount);

new HDRLoader().load(
    './textures/misty_pines_4k.hdr',
    (hdri) => {
        const { data, width, height } = hdri.image;
        const rowSize = width * 4;
        const original = data.slice();

        for (let y = 0; y < height; y++) {
            const source = (height - 1 - y) * rowSize;
            data.set(original.subarray(source, source + rowSize), y * rowSize);
        }
        hdri.flipY = false;
        hdri.mapping = THREE.EquirectangularReflectionMapping;
        scene.background = hdri;
        scene.environment = null;
    }
);

const ambient = new THREE.HemisphereLight(0xbdbebc, 0xbdbebc, 1.5);
scene.add(ambient);

const sunlight = new THREE.DirectionalLight(0xbdbebc, 1.5);
sunlight.position.set(30, 50, 40);
sunlight.castShadow = true;
sunlight.shadow.mapSize.set(4096, 4096);
const shadowSize = 40;
sunlight.shadow.camera.left = -shadowSize;
sunlight.shadow.camera.right = shadowSize;
sunlight.shadow.camera.top = shadowSize;
sunlight.shadow.camera.bottom = -shadowSize;
sunlight.shadow.camera.near = 0.5;
sunlight.shadow.camera.far = 140;
sunlight.shadow.bias = -0.0005;
sunlight.shadow.normalBias = 0.03;
sunlight.shadow.camera.updateProjectionMatrix();
scene.add(sunlight);

const camera = new THREE.PerspectiveCamera(60, innerWidth / innerHeight, 0.1, 100);
const cameraDistance = roundTable ? tableRadius + 0.4 : 1.5;
camera.position.set(0, roundTable ? 2.15 : 2, cameraDistance);
camera.lookAt(0, roundTable ? 0.65 : 1, 0);
camera.rotation.reorder('YXZ');

const cameraBounds = {
    halfSize: 5,
    minHeight: 0.3,
    maxHeight: 6,
};

const canvas = renderer.domElement;
const serverPlayerIndex = initialUnoState?.players?.findIndex((player) => player.id === viewerID) ?? -1;
const myPlayer = networkMode ? (spectatorMode ? -1 : serverPlayerIndex) : 0;
const cardEffects = {};
let cardsControl;
let actionBusy = false;
const requestUnoAction = async (action) => {
    if (actionBusy || !networkMode) return;
    actionBusy = true;
    try {
        const next = await requestGame(action.type === 'draw' ? 'draw' : 'play', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: action.type === 'draw' ? undefined : JSON.stringify({ cardIndex: action.cardIndex, color: action.color || '' }),
        });
        cardsControl.applyServerState(next);
        syncHeads(next);
    } catch (error) {
        console.error(error);
    } finally {
        actionBusy = false;
    }
};
const {
    preloadCardTextures, createCard, displayDeck, hands, drawPile, discardPile,
    clickCard, updateHover, clearHover,
} = cardsControl = createCardsControl(scene, camera, canvas, myPlayer, cardEffects, seatCount, startingHandSize, roundTable, networkMode ? requestUnoAction : null);
await preloadCardTextures(renderer);

canvas.addEventListener('contextmenu', (event) => {
    event.preventDefault();
});

async function lockPointer() {
    if (document.pointerLockElement !== canvas) {
        try {
            await canvas.requestPointerLock();
        } catch (error) {
        }
    }
}

canvas.addEventListener('pointerdown', (event) => {
    if (!spectatorMode) {
        if (event.button === 2) {
            void lockPointer();
            return;
        }
        if (event.button === 0) void clickCard(event);
        return;
    }
    if (document.pointerLockElement !== canvas) {
        void lockPointer();
        return;
    }
    if (event.button === 0) void clickCard();
});

document.addEventListener('mousemove', (event) => {
    if (document.pointerLockElement !== canvas) return;

    camera.rotation.y -= event.movementX * 0.002;
    camera.rotation.x -= event.movementY * 0.002;

    const limit = Math.PI / 2 - 0.01;
    camera.rotation.x = Math.max(-limit, Math.min(limit, camera.rotation.x));
});

document.addEventListener('pointerlockchange', () => {
    if (document.pointerLockElement !== canvas) {
        keys.clear();
        clearHover();
    }
});

function resize() {
    renderer.setPixelRatio(Math.min(devicePixelRatio, 2));
    renderer.setSize(innerWidth, innerHeight);
    camera.aspect = innerWidth / innerHeight;
    camera.updateProjectionMatrix();
    const gl = renderer.getContext();
    const width = gl.drawingBufferWidth;
    const height = gl.drawingBufferHeight;
    const ratio = renderer.getPixelRatio();
    renderer.setViewport(0, 0, width / ratio, height / ratio);
}
window.addEventListener('resize', resize);
resize();

const keys = new Set();
window.addEventListener('keydown', (event) => {
    if (!spectatorMode) return;
    const key = event.key.toLowerCase();
    if (!['w', 'a', 's', 'd', 'q', 'e', 'arrowleft', 'arrowright', 'shift'].includes(key)) return;
    if (event.ctrlKey || event.metaKey || event.altKey) return;
    event.preventDefault();
    keys.add(key);
    if (!event.repeat) lockPointer();
});
window.addEventListener('keyup', (event) => keys.delete(event.key.toLowerCase()));
window.addEventListener('blur', () => keys.clear());
const viewCamera = camera;

if (!networkMode) {
    const centralCardScale = 1.9;
    const centralPileOffset = roundTable ? 0.28 : 0;
    const middleCard = createCard(-centralPileOffset, 0, 'R1', 180);
    middleCard.scale.setScalar(centralCardScale);
    middleCard.rotation.x = middleCard.userData.centralRotation;
    middleCard.userData.pile = 'discard';
    discardPile.push(middleCard);

    const drawFaces = ['R1', 'B2', 'G3', 'Y4', 'RA2', 'BR', 'GS', 'WC', 'W4', 'Y0'];
    for (let i = 0; i < 30; i++) {
        const card = createCard(centralPileOffset, 0, 'BACK', 180, i * 0.001);
        card.scale.setScalar(centralCardScale);
        card.rotation.x = card.userData.centralRotation;
        card.userData.face = drawFaces[i % drawFaces.length];
        card.userData.pile = 'draw';
        drawPile.push(card);
    }

    for (let id = 0; id < seatCount; id += 1) {
        const deck = Array.from({ length: startingHandSize }, (_, cardIndex) => drawFaces[(id * startingHandSize + cardIndex) % drawFaces.length]);
        displayDeck(id, deck);
    }
}

const volumes = createVolumeRenderer(renderer, scene, sunlight);
const headDisplay = await createPlayerHeads(volumes, scene, myPlayer, seatCount, roundTable);
const fog = await createFog(volumes);
const cardTrail = await createCardTrail(renderer, volumes);
Object.assign(cardEffects, cardTrail);
function syncHeads(snapshot) {
    if (!snapshot || !headDisplay.setActive) return;
    snapshot.players?.forEach((player, index) => headDisplay.setActive(index, !player.finished));
}

if (initialUnoState) {
    cardsControl.applyServerState(initialUnoState);
    syncHeads(initialUnoState);
}

if (networkMode) {
    window.setInterval(async () => {
        try {
            const next = await requestGame('state');
            cardsControl.applyServerState(next);
            syncHeads(next);
        } catch (error) {
            console.error(error);
        }
    }, 1200);
}

let shaderOn = true;
window.addEventListener('keydown', (event) => {
    if (event.key !== 'F2') return;
    event.preventDefault();
    if (event.repeat) return;
    shaderOn = !shaderOn;
});

const movement = new THREE.Vector3();
let previousTime = 0;
renderer.setAnimationLoop((milliseconds) => {
    const t = milliseconds / 1000;
    const delta = Math.min(t - previousTime, 0.05);
    previousTime = t;

    movement.set(
        Number(keys.has('d')) - Number(keys.has('a')),
        Number(keys.has('e')) - Number(keys.has('q')),
        Number(keys.has('s')) - Number(keys.has('w'))
    ).normalize().multiplyScalar(2 * (keys.has('shift') ? 3 : 1) * delta);
    camera.translateX(movement.x);
    camera.translateY(movement.y);
    camera.translateZ(movement.z);

    camera.position.x = THREE.MathUtils.clamp(camera.position.x, -cameraBounds.halfSize, cameraBounds.halfSize);
    camera.position.y = THREE.MathUtils.clamp(camera.position.y, cameraBounds.minHeight, cameraBounds.maxHeight);
    camera.position.z = THREE.MathUtils.clamp(camera.position.z, -cameraBounds.halfSize, cameraBounds.halfSize);

    updateHover();

    headDisplay.update();

    if (shaderOn) {
        cardTrail.update();
        volumes.render(t, viewCamera);
    } else {
        renderer.render(scene, viewCamera);
    }
});
