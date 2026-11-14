import * as THREE from 'three';
import { createWorldControl } from './world_control.js?v=radial-table-20261105-v10';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';
import { HDRLoader } from 'three/addons/loaders/HDRLoader.js';
import { createCardsControl } from './cards_control.js?v=rules-20261114-v29';
import { createPlayerHeads } from './player_heads.js?v=shaders-20261114-v30';
import { createVolumeRenderer } from './volume_shaders.js?v=shaders-20261114-v30';
import { createFog } from './create_fog.js?v=shaders-20261114-v30';
import { createCardTrail } from './card_trail.js?v=shaders-20261114-v30';

const query = new URLSearchParams(window.location.search);
const gameID = query.get('gameId')?.trim() || '';
const spectatorMode = query.get('spectate') === '1';
const networkMode = Boolean(gameID) && window.parent !== window;
const roundTable = query.has('seats');
const seatCount = THREE.MathUtils.clamp(Number(query.get('seats')) || 4, 1, 8);
const startingHandSize = THREE.MathUtils.clamp(Number(query.get('hand')) || 7, 3, 14);
const tableRadius = 1.15 + Math.max(0, seatCount - 4) * 0.035;

function createNotice() {
    const element = document.querySelector('.notice');
    const prompt = document.querySelector('.prompt');
    let timer = 0;
    return {
        show(text) {
            if (!element) return;
            element.textContent = text;
            element.classList.add('is-visible');
            window.clearTimeout(timer);
            timer = window.setTimeout(() => element.classList.remove('is-visible'), 2600);
        },
        hold(text) {
            if (!prompt) return;
            prompt.textContent = text || '';
            prompt.classList.toggle('is-visible', Boolean(text));
        },
    };
}

const cardLabels = { S: 'Skip', R: 'Reverse', A2: '+2', WC: 'Wild', W4: 'Wild +4' };
const colorLabels = { R: 'red', G: 'green', B: 'blue', Y: 'yellow' };
function describeCard(face) {
    if (!face) return 'a card';
    if (face === 'WC' || face === 'W4') return cardLabels[face];
    const symbol = face.slice(1);
    return `${colorLabels[face[0]] || ''} ${cardLabels[symbol] || symbol}`.trim();
}

function createRuleFeedback(viewerID) {
    let lastEventVersion = null;
    const nameOf = (snapshot, id) => (id === viewerID ? 'You' : snapshot.players?.find((player) => player.id === id)?.name || 'A player');
    const eventMessage = (snapshot) => {
        const event = snapshot.lastEvent;
        if (!event?.type) return '';
        const actor = nameOf(snapshot, event.playerId);
        const target = nameOf(snapshot, event.targetId);
        switch (event.type) {
            case 'start': return `The round opens with ${describeCard(event.card)}.`;
            case 'play': {
                const verb = event.jump ? 'jumped in with' : 'played';
                if (!event.targetId) return `${actor} ${verb} ${describeCard(event.card)}${event.color && (event.card === 'WC' || event.card === 'W4') ? `, color is now ${colorLabels[event.color]}` : ''}.`;
                if (!event.count) return `${actor} ${verb} ${describeCard(event.card)}: ${target} skip${event.targetId === viewerID ? '' : 's'} a turn.`;
                const pending = snapshot.phase === 'penalty' || snapshot.phase === 'challenge';
                return `${actor} ${verb} ${describeCard(event.card)}${event.color ? ` (${colorLabels[event.color]})` : ''}: ${target} ${pending ? `face${event.targetId === viewerID ? '' : 's'} +${event.count}` : `draw${event.targetId === viewerID ? '' : 's'} ${event.count}`}.`;
            }
            case 'rotate': return `${actor} played ${describeCard(event.card)}: every hand moves to the next player!`;
            case 'swap': return `${actor} swapped hands with ${target}.`;
            case 'draw': return event.count > 1 ? `${actor} drew ${event.count} cards.` : `${actor} drew a card.`;
            case 'pass': return `${actor} passed.`;
            case 'accept': return `${actor} drew ${event.count} cards.`;
            case 'challenge_won': return `${actor} caught a bluff: ${target} draws ${event.count}.`;
            case 'challenge_lost': return `${actor} lost the challenge and draws ${event.count}.`;
            case 'uno': return `${actor} called UNO!`;
            case 'catch': return `${actor} caught ${target} without UNO: +${event.count} cards.`;
            case 'finish': return `${actor} won the round with ${event.count} points.`;
            default: return '';
        }
    };
    const promptMessage = (snapshot) => {
        if (snapshot.status === 'finished') return `${nameOf(snapshot, snapshot.winnerId)} won the round.`;
        const me = snapshot.players?.find((player) => player.id === viewerID);
        if (!me) return '';
        if (snapshot.catchable && snapshot.catchable !== viewerID) return `${nameOf(snapshot, snapshot.catchable)} forgot to call UNO. Click their hand to catch them.`;
        if (snapshot.turnPlayerId !== viewerID) {
            if (snapshot.catchable === viewerID) return 'You forgot UNO! Press U before someone catches you.';
            return snapshot.rules?.jumpIn && (snapshot.playable || []).length ? 'Jump-in! You hold the same card: play it now to steal the turn.' : '';
        }
        const stackHint = snapshot.rules?.stackDrawCards && (snapshot.playable || []).length ? ', or stack a draw card from your hand' : '';
        if (snapshot.phase === 'challenge') return `Wild +4 from ${nameOf(snapshot, snapshot.pendingFromId)}: Enter to draw ${snapshot.pendingDraw || 4}, C to challenge${stackHint}.`;
        if (snapshot.phase === 'penalty') return `+${snapshot.pendingDraw || 2} from ${nameOf(snapshot, snapshot.pendingFromId)}: press Enter to draw${stackHint}.`;
        if (snapshot.phase === 'swap') return 'You played a 7: click another player\'s hand to swap hands with them.';
        if (snapshot.phase === 'drawn') return snapshot.rules?.forcePlay ? 'Your drawn card fits and force play is on: play it.' : 'Your drawn card fits: play it, or click the draw pile again (or press Space) to pass.';
        if (me.cardCount === 2) return 'Your turn. Press U to call UNO before playing your second-to-last card.';
        if (snapshot.catchable === viewerID) return 'You forgot UNO! Press U before someone catches you.';
        return (snapshot.playable || []).length ? 'Your turn.' : 'Your turn: no playable card, click the draw pile.';
    };
    return {
        apply(snapshot) {
            if (!snapshot) return;
            if (snapshot.version != null && snapshot.version !== lastEventVersion) {
                lastEventVersion = snapshot.version;
                const skipped = (snapshot.skipped || []).map((id) => nameOf(snapshot, id));
                const message = eventMessage(snapshot);
                if (skipped.length) notice.show(`${message} ${skipped.join(', ')} ${skipped.length > 1 || skipped[0] === 'You' ? 'have' : 'has'} no playable card and nothing to draw: turn skipped.`.trim());
                else if (message) notice.show(message);
            }
            notice.hold(promptMessage(snapshot));
        },
    };
}

function createParentBridge() {
    const origin = window.location.origin;
    const pending = new Map();
    const handlers = { bootstrap: null, state: null };
    let nextID = 1;
    const post = (message) => window.parent.postMessage(message, origin);
    window.addEventListener('message', (event) => {
        if (event.origin !== origin || event.source !== window.parent) return;
        const data = event.data;
        if (!data || typeof data !== 'object') return;
        if (data.type === 'uno.bootstrap') handlers.bootstrap?.(data);
        else if (data.type === 'uno.state') handlers.state?.(data.snapshot);
        else if (data.type === 'uno.action.result') {
            const request = pending.get(data.id);
            if (!request) return;
            pending.delete(data.id);
            if (data.ok) request.resolve();
            else request.reject(new Error(data.message || 'The table refused this move.'));
        }
    });
    return {
        bootstrap() {
            return new Promise((resolve) => {
                const timer = window.setInterval(() => post({ type: 'uno.ready' }), 500);
                handlers.bootstrap = (data) => {
                    handlers.bootstrap = null;
                    window.clearInterval(timer);
                    resolve(data);
                };
                post({ type: 'uno.ready' });
            });
        },
        firstState() {
            return new Promise((resolve) => {
                const previous = handlers.state;
                handlers.state = (snapshot) => {
                    handlers.state = previous;
                    previous?.(snapshot);
                    resolve(snapshot);
                };
            });
        },
        onState(callback) {
            handlers.state = callback;
        },
        action(action) {
            return new Promise((resolve, reject) => {
                const id = nextID++;
                pending.set(id, { resolve, reject });
                post({ type: 'uno.action', id, action });
                window.setTimeout(() => {
                    if (pending.delete(id)) reject(new Error('The table did not answer in time.'));
                }, 15000);
            });
        },
    };
}

const notice = createNotice();
const bridge = networkMode ? createParentBridge() : null;
const reportProgress = (stage, ratio) => {
    if (!networkMode) return;
    window.parent.postMessage({ type: 'uno.progress', stage, ratio }, window.location.origin);
};
reportProgress('connecting', 0.05);
let initialUnoState = null;
let viewerID = query.get('playerId')?.trim() || '';
if (bridge) {
    const bootstrap = await bridge.bootstrap();
    viewerID = bootstrap.player?.id || viewerID;
    initialUnoState = bootstrap.snapshot || null;
    if (!initialUnoState && !spectatorMode) initialUnoState = await bridge.firstState();
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
reportProgress('lighting', 0.15);
await createWorldControl(scene);
reportProgress('table', 0.35);
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
    if (actionBusy || !bridge) return;
    actionBusy = true;
    try {
        await bridge.action(action.type === 'play' ? { type: 'play', cardIndex: action.cardIndex, color: action.color || null } : action);
    } catch (error) {
        notice.show(error.message);
    } finally {
        actionBusy = false;
    }
};
const {
    preloadCardTextures, createCard, displayDeck, hands, drawPile, discardPile,
    clickCard, updateHover, clearHover,
} = cardsControl = createCardsControl(scene, camera, canvas, myPlayer, cardEffects, seatCount, startingHandSize, roundTable, networkMode ? requestUnoAction : null);
reportProgress('cards', 0.55);
await preloadCardTextures(renderer);
reportProgress('players', 0.75);

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
reportProgress('ready', 0.95);
let firstFrameReported = false;
Object.assign(cardEffects, cardTrail);
function syncHeads(snapshot) {
    if (!snapshot || !headDisplay.setActive) return;
    snapshot.players?.forEach((player, index) => headDisplay.setActive(index, !player.finished));
}

const ruleFeedback = networkMode ? createRuleFeedback(viewerID) : null;
if (initialUnoState) {
    cardsControl.applyServerState(initialUnoState);
    syncHeads(initialUnoState);
    ruleFeedback?.apply(initialUnoState);
}

bridge?.onState((snapshot) => {
    cardsControl.applyServerState(snapshot);
    syncHeads(snapshot);
    ruleFeedback?.apply(snapshot);
    if (snapshot?.status === 'finished' && document.pointerLockElement === canvas) {
        window.setTimeout(() => document.exitPointerLock(), 3000);
    }
});

const actionKeys = { ' ': 'pass', enter: 'accept', c: 'challenge', u: 'uno' };
window.addEventListener('keydown', (event) => {
    if (!networkMode || spectatorMode || myPlayer < 0 || event.repeat) return;
    if (event.ctrlKey || event.metaKey || event.altKey) return;
    const type = actionKeys[event.key.toLowerCase()];
    if (!type) return;
    event.preventDefault();
    void requestUnoAction({ type });
});

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
    if (!firstFrameReported) {
        firstFrameReported = true;
        reportProgress('ready', 1);
        if (networkMode) window.parent.postMessage({ type: 'uno.scene.ready' }, window.location.origin);
    }
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
