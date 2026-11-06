import * as THREE from 'three';
import { gsap } from 'gsap';

export function createCardsControl(scene, camera, canvas, myPlayer, effects = {}, seatCount = 4, startingHandSize = 7, roundTable = false, onAction = null) {
    const cardTextureLoader = new THREE.TextureLoader();
    const textures = new Map();
    const tableHeight = roundTable ? 1.455 : 1.365;
    const tableRadius = roundTable ? 1.15 + Math.max(0, seatCount - 4) * 0.035 : 0.8;
    const centralCardScale = roundTable ? 2.1 : 1;
    const centralCardRotation = Math.PI / 2;
    const playerCardScale = roundTable ? 1.5 : 1;
    const drawPileX = roundTable ? 0.28 : 0;
    const discardPileX = roundTable ? -0.28 : 0;
    const hoverDistance = 0.06;
    const colorPickHeight = 0.08;
    const jumpHeight = 0.05;
    const wildPixels = new Map();
    const hands = Array.from({ length: seatCount }, () => []);
    const cards = [];
    const drawPile = [];
    const discardPile = [];
    let hovered = null;
    let selected = null;
    let restarting = false;
    let winning = false;
    let appliedVersion = null;
    const networkMode = typeof onAction === 'function';

    const tableBlocker = new THREE.Mesh(
        new THREE.CircleGeometry(tableRadius, 64),
        new THREE.MeshBasicMaterial({ side: THREE.DoubleSide })
    );
    tableBlocker.rotation.x = -Math.PI / 2;
    tableBlocker.position.set(0, tableHeight - 0.01, 0);
    tableBlocker.visible = false;
    scene.add(tableBlocker);

    function cardTexture(face) {
        if (!textures.has(face)) {
            const texture = cardTextureLoader.load(`./cards/${face}.png`);
            texture.colorSpace = THREE.SRGBColorSpace;
            textures.set(face, texture);
        }
        return textures.get(face);
    }

    async function preloadCardTextures(renderer) {
        const colors = ['R', 'G', 'B', 'Y'];
        const values = ['0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'A2', 'R', 'S'];
        const faces = [
            'BACK', 'BLANK', 'WC', 'W4',
            ...colors.flatMap((color) => values.map((value) => color + value)),
            ...colors.flatMap((color) => [`WC_${color}`, `W4_${color}`]),
        ];
        await Promise.all(faces.map(async (face) => {
            const texture = await cardTextureLoader.loadAsync(`./cards/${face}.png`);
            texture.colorSpace = THREE.SRGBColorSpace;
            texture.anisotropy = renderer.capabilities.getMaxAnisotropy();
            await texture.image.decode();
            renderer.initTexture(texture);
            textures.set(face, texture);
        }));
        getWildPixels('WC');
        getWildPixels('W4');
    }

    function animate(target, values, animationDuration = 0.3) {
        return gsap.to(target, {
            ...values,
            duration: animationDuration,
            ease: 'power2.out',
            overwrite: 'auto',
        });
    }

    function throwCard(card, destination, duration) {
        gsap.killTweensOf(card.position);
        const start = card.position.clone();
        const progress = { t: 0 };
        const throwHeight = 0.3;

        return gsap.to(progress, {
            t: 1,
            duration,
            ease: 'none',
            onUpdate: () => {
                const t = progress.t;
                card.position.lerpVectors(start, destination, t);
                card.position.y += 4 * throwHeight * t * (1 - t);
            },
            onComplete: () => {
                card.userData.moving = false;
            },
        });
    }

    function isWild(card) {
        return card && ['WC', 'W4'].includes(card.userData.face);
    }

    function setCardPriority(card, active) {
        if (!card) return;
        card.renderOrder = active ? 1000 : 0;
        card.material.depthTest = !active;
        card.material.depthWrite = !active;
    }

    function selectCard(card) {
        if (myPlayer < 0) return;
        const previous = selected;
        if (isWild(selected)) {
            selected.userData.chosenColor = null;
            selected.userData.previewColor = null;
            selected.material.map = cardTexture(selected.userData.face);
        }
        setCardPriority(previous, false);
        selected = selected === card ? null : card;
        setCardPriority(selected, true);
        layoutHand(myPlayer);
    }

    function getWildPixels(face) {
        if (!wildPixels.has(face)) {
            const image = cardTexture(face).image;
            if (!image?.complete || !image.naturalWidth) return;
            const canvas = document.createElement('canvas');
            canvas.width = image.width;
            canvas.height = image.height;
            const context = canvas.getContext('2d');
            context.drawImage(image, 0, 0);
            wildPixels.set(face, context.getImageData(0, 0, canvas.width, canvas.height));
        }
        return wildPixels.get(face);
    }

    function wildColorAt(hit) {
        if (!isWild(selected) || hit?.object !== selected) return;
        if (selected.position.y < selected.userData.hitArea.position.y + colorPickHeight - 0.002) return;

        const face = selected.userData.face;
        const pixels = getWildPixels(face);
        if (!pixels) return;
        const x = THREE.MathUtils.clamp(Math.floor(hit.uv.x * pixels.width), 0, pixels.width - 1);
        const y = THREE.MathUtils.clamp(Math.floor((1 - hit.uv.y) * pixels.height), 0, pixels.height - 1);
        const offset = (x + y * pixels.width) * 4;

        const [r, g, b, a] = pixels.data.slice(offset, offset + 4);
        const tolerance = 20;
        const matches = (red, green, blue) =>
            Math.abs(r - red) <= tolerance &&
            Math.abs(g - green) <= tolerance &&
            Math.abs(b - blue) <= tolerance;

        if (matches(237, 28, 36)) return 'R';
        if (matches(255, 222, 22)) return 'Y';
        if (matches(0, 114, 188)) return 'B';
        if (matches(80, 170, 68)) return 'G';
    }

    function chooseWildColor(hit, confirm = false) {
        if (!isWild(selected)) return;
        const color = wildColorAt(hit);
        const data = selected.userData;

        if (hit?.object !== selected)
            data.previewColor = null;
        else if (color)
            data.previewColor = color;

        if (confirm && hit?.object === selected && data.previewColor)
            data.chosenColor = data.previewColor;

        const preview = data.previewColor || data.chosenColor;
        selected.material.map = cardTexture(preview ? `${data.face}_${preview}` : data.face);
    }

    const cardGeometry = new THREE.PlaneGeometry(0.18 * (166 / 257), 0.18);

    function createCard(x, z, faceName, rotationZ = 0, elevation = 0) {
        const material = new THREE.MeshStandardMaterial({
            map: cardTexture(faceName),
            roughness: 0.8,
            metalness: 0,
            emissive: 0xffffff,
            emissiveIntensity: 0,
            alphaTest: 0.5,
            alphaToCoverage: true,
            side: THREE.DoubleSide,
        });

        const card = new THREE.Mesh(cardGeometry, material);
        card.castShadow = true;
        card.receiveShadow = true;
        card.position.set(x, tableHeight + elevation, z);

        card.rotation.set(Math.PI / 2, Math.PI, THREE.MathUtils.degToRad(rotationZ));

        scene.add(card);
        card.userData = { owner: null, face: faceName, pile: null, centralScale: centralCardScale, centralRotation: centralCardRotation };
        cards.push(card);
        return card;
    }

    function layoutHand(id, animationDuration = 0.3) {
        const hand = hands[id];
        if (!hand) return Promise.resolve();
        return Promise.all(hand.flatMap((card, i) => {
            const position = (i - (hand.length - 1) / 2) * 0.11 / (hand.length / 7 + 1);
            const angle = (id / seatCount) * Math.PI * 2;
            const radialX = Math.sin(angle);
            const radialZ = Math.cos(angle);
            const tangentX = Math.cos(angle);
            const tangentZ = -Math.sin(angle);
            const handRadius = tableRadius * (id === myPlayer ? 0.68 : 0.72);
            const x = roundTable ? radialX * handRadius + tangentX * position : [position, -0.47, position, 0.47][id];
            const z = roundTable ? radialZ * handRadius + tangentZ * position : [0.47, position, -0.47, position][id];
            const y = tableHeight + 0.0005 * (id < 2 ? i : hand.length - i);
            const rotation = roundTable ? angle + Math.PI : THREE.MathUtils.degToRad(-(id + 2) * 90);

            let hitArea = card.userData.hitArea;
            if (!hitArea) {
                hitArea = new THREE.Mesh(card.geometry, card.material);
                hitArea.visible = false;
                hitArea.userData.card = card;
                scene.add(hitArea);
                card.userData.hitArea = hitArea;
            }
            hitArea.position.set(x, y, z);
            hitArea.rotation.set(Math.PI / 2, Math.PI, rotation);

            const active = card === hovered || card === selected;
            card.scale.setScalar(card.userData.owner === myPlayer ? playerCardScale : 1);
            hitArea.scale.copy(card.scale);
            const slide = active ? hoverDistance : 0;
            const destination = {
                x: x + (roundTable ? radialX : [0, 1, 0, -1][id]) * slide,
                y: y + (card === selected && isWild(card) ? colorPickHeight : 0),
                z: z + (roundTable ? radialZ : [-1, 0, 1, 0][id]) * slide,
            };
            return [
                animate(card.position, destination, animationDuration),
                animate(card.rotation, { z: rotation }, animationDuration),
            ];
        }));
    }

    function displayDeck(id, deck) {
        hands[id] = deck.map((face) => {
            const card = createCard(0, 0, id === myPlayer ? face : 'BACK', -(id + 2) * 90 + 5 * 360);
            card.userData.face = face;
            card.userData.owner = id;
            card.scale.setScalar(id === myPlayer ? playerCardScale : 1);
            return card;
        });
        layoutHand(id, 2);
        return hands[id];
    }

    const raycaster = new THREE.Raycaster();
    const screenCenter = new THREE.Vector2(0, 0);

    function hitAtCenter(event = null) {
        camera.updateMatrixWorld();
        scene.updateMatrixWorld();

        screenCenter.set(0, 0);
        if (event && document.pointerLockElement !== canvas) {
            const bounds = canvas.getBoundingClientRect();
            screenCenter.set(
                ((event.clientX - bounds.left) / bounds.width) * 2 - 1,
                -((event.clientY - bounds.top) / bounds.height) * 2 + 1,
            );
        }
        raycaster.setFromCamera(screenCenter, camera);
        const hitAreas = myPlayer >= 0 ? hands[myPlayer].map((card) => card.userData.hitArea) : [];
        return raycaster.intersectObjects([tableBlocker, ...cards, ...hitAreas], false)[0];
    }

    function cardAtCenter(hit) {
        if (!hit || hit.object === tableBlocker) return null;

        const card = hit.object.userData.card || hit.object;
        if (card.userData.moving) return null;
        if (card.userData.pile === 'draw') return drawPile.at(-1);
        if (card.userData.pile === 'discard') return discardPile.at(-1);
        if (card.userData.owner !== myPlayer) return null;
        return card;
    }

    function setHovered(card) {
        if (hovered === card) return;
        hovered = card;
        if (myPlayer >= 0) layoutHand(myPlayer);

        const drawHighlight = card?.userData.pile === 'draw' ? 0.2 : 0;
        drawPile.forEach((card) => animate(card.material, { emissiveIntensity: drawHighlight }));

        const canPlaySelected = selected && (!isWild(selected) || selected.userData.chosenColor);
        const discardHighlight = card?.userData.pile === 'discard' && canPlaySelected ? 0.2 : 0;
        discardPile.forEach((card) => animate(card.material, { emissiveIntensity: discardHighlight }));
    }

    function updateHover() {
        if (!restarting && !winning && document.pointerLockElement === canvas) {
            const hit = hitAtCenter();
            setHovered(cardAtCenter(hit));
            chooseWildColor(hit);
        }
    }

    async function playCard(playerId, cardIndex, color = null) {
        if (restarting || winning) return;
        const hand = hands[playerId];
        if (!hand) return;
        const card = hand[cardIndex];
        if (!card) return;
        if (isWild(card)) {
            color = color ?? card.userData.chosenColor;
            if (!['R', 'G', 'B', 'Y'].includes(color)) return;
            card.userData.chosenColor = color;
        }

        hands[playerId].splice(cardIndex, 1);

        if (selected === card) {
            setCardPriority(card, false);
            selected = null;
        }
        if (hovered === card) hovered = null;

        if (card.userData.hitArea) scene.remove(card.userData.hitArea);

        card.userData.hitArea = null;
        card.userData.owner = null;
        card.userData.pile = 'discard';
        card.userData.moving = true;
        card.scale.setScalar(centralCardScale);
        card.rotation.x = centralCardRotation;

        card.material.map = cardTexture(
            isWild(card)
                ? `${card.userData.face}_${card.userData.chosenColor}`
                : card.userData.face
        );
        discardPile.push(card);

        const won = hands[playerId].length === 0;
        if (won) {
            winning = true;
            selectCard(null);
            setHovered(null);
        }
        const moveDuration = won ? 2 : 0.3;
        const destination = new THREE.Vector3(discardPileX, tableHeight + (discardPile.length - 1) * 0.001, 0);
        const stopTrail = effects.start?.(card, 'play', moveDuration);
        await Promise.all([
            won ? throwCard(card, destination, moveDuration) : animate(card.position, {
                x: destination.x,
                y: destination.y,
                z: destination.z,
                onComplete: () => {
                    card.userData.moving = false;
                },
            }, moveDuration),
            animate(card.rotation, { z: Math.PI * (won ? 3 : 1) }, moveDuration),
            layoutHand(playerId),
        ]);
        stopTrail?.();
        if (won) {
            await winningAnimation();
            winning = false;
        }
    }

    function drawCard(playerId, dealing = false) {
        if ((restarting && !dealing) || winning) return;
        const card = drawPile.pop();
        if (!card) return;
        card.userData.chosenColor = null;
        card.userData.previewColor = null;
        gsap.killTweensOf(card.material);
        card.material.emissiveIntensity = 0;
        card.material.map = cardTexture(
            playerId === myPlayer ? card.userData.face : 'BACK'
        );
        card.userData.pile = null;
        card.userData.owner = playerId;
        card.scale.setScalar(playerId === myPlayer ? playerCardScale : 1);
        card.rotation.x = Math.PI / 2;
        hands[playerId].push(card);
        const stopTrail = !dealing ? effects.start?.(card, 'draw', 0.3) : null;
        return layoutHand(playerId).then(() => { stopTrail?.(); });
    }

    function clearHover() {
        if (restarting || winning) return;
        setHovered(null);
        chooseWildColor(null);
    }

    async function restartGame() {
        if (restarting || winning) return;
        if (cards.length < hands.length * startingHandSize + 1) return;
        effects.clear?.();
        restarting = true;
        selected = null;
        hovered = null;

        hands.forEach((hand) => {
            hand.length = 0;
        });
        drawPile.length = 0;
        discardPile.length = 0;
        cards.forEach((card) => {
            gsap.killTweensOf([card.position, card.rotation, card.material]);
            if (card.userData.hitArea) scene.remove(card.userData.hitArea);
            card.userData.hitArea = null;
            card.userData.owner = null;
            card.userData.pile = null;
            card.userData.moving = false;
            card.userData.chosenColor = null;
            card.userData.previewColor = null;
            card.scale.setScalar(1);
            card.rotation.x = Math.PI / 2;
            if (isWild(card)) card.material.map = cardTexture(card.userData.face);
            card.material.emissiveIntensity = 0;
        });

        drawPile.push(...cards);
        cards.forEach((card) => {
            card.userData.pile = 'draw';
        });

        const shuffleSpacing = 0.007;

        function shuffleMiddle(spacing = shuffleSpacing) {
            for (let i = drawPile.length - 1; i > 0; i--) {
                const j = Math.floor(Math.random() * (i + 1));
                [drawPile[i], drawPile[j]] = [drawPile[j], drawPile[i]];
            }
            return Promise.all(drawPile.flatMap((card, i) => [
                animate(card.position, {
                    x: 0,
                    y: tableHeight + i * spacing,
                    z: 0,
                }, 0.6),
                animate(card.rotation, { z: Math.PI }, 0.6),
            ]));
        }

        await shuffleMiddle();
        cards.forEach((card) => {
            card.material.map = cardTexture('BACK');
        });
        for (let shuffle = 0; shuffle < 4; shuffle++) {
            await shuffleMiddle(shuffle === 3 ? 0.001 : shuffleSpacing);
        }

        for (let round = 0; round < startingHandSize; round++) {
            for (let playerId = 0; playerId < hands.length; playerId++) {
                await drawCard(playerId, true);
            }
        }

        await Promise.all(drawPile.map((card, i) =>
            animate(card.position, { x: drawPileX, y: tableHeight + i * 0.001, z: 0 }, 0.6)
        ));
        drawPile.forEach((card) => { card.scale.setScalar(centralCardScale); card.rotation.x = centralCardRotation; });

        const firstCard = drawPile.pop();
        firstCard.userData.pile = 'discard';
        firstCard.scale.setScalar(centralCardScale);
        firstCard.rotation.x = centralCardRotation;
        firstCard.material.map = cardTexture(firstCard.userData.face);
        discardPile.push(firstCard);
        await animate(firstCard.position, { x: discardPileX, y: tableHeight, z: 0 });
        restarting = false;
    }

    async function clickCard(event = null) {
        if (restarting || winning) return;
        if (networkMode && myPlayer < 0) return;
        const hit = hitAtCenter(event);
        const target = cardAtCenter(hit);
        if (!target) {
            if (!hit || hit.object === tableBlocker) selectCard(null);
            return;
        }

        if (target === selected && isWild(selected) && hit.object === selected) {
            chooseWildColor(hit, true);
            return;
        }

        if (target.userData.owner === myPlayer) {
            selectCard(target);
        } else if (target.userData.pile === 'discard' && selected) {
            const cardIndex = hands[myPlayer].indexOf(selected);
            if (networkMode) {
                if (isWild(selected) && !selected.userData.chosenColor) return;
                await onAction({ type: 'play', cardIndex, color: selected.userData.chosenColor ?? null });
            } else {
                await playCard(myPlayer, cardIndex);
            }
        } else if (target.userData.pile === 'draw' && drawPile.length) {
            if (networkMode) await onAction({ type: 'draw' });
            else drawCard(myPlayer);
        }
    }

    const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

    async function waveAnimation(card, speed) {
        const startY = card.position.y;
        await animate(card.position, { y: startY + jumpHeight }, speed);
        await wait(speed * 300 / 3);
        await animate(card.position, { y: startY }, speed);
    }

    async function handWaveAnimation(hand) {
        if (hand.length === 0) return;

        const speed = 3.3 / hand.length;
        const animations = [];

        let left = Math.floor((hand.length - 1) / 2);
        let right = Math.floor(hand.length / 2);

        while (left >= 0) {
            animations.push(waveAnimation(hand[left], speed));

            if (right !== left) {
                animations.push(waveAnimation(hand[right], speed));
            }

            left--;
            right++;
            if (left >= 0) await wait(100);
        }

        await Promise.all(animations);
    }

    async function raiseDrawPileAnimation() {
        const animations = [];
        const startHeight = drawPile.map((card) => card.position.y);

        for (const [id, card] of [...drawPile].reverse().entries()) {
            const index = drawPile.length - 1 - id;
            animations.push(animate(card.position, { y: startHeight[index] + jumpHeight }, 0.3));
            await wait(20);
        }

        await Promise.all(animations);
        await wait(20);
        animations.length = 0;

        for (const [id, card] of drawPile.entries()) {
            animations.push(animate(card.position, { y: startHeight[id] }, 0.3));
            await wait(20);
        }

        await Promise.all(animations);
    }

    async function winningAnimation() {
        if (!winning) return;
        await Promise.all([...hands.map(handWaveAnimation), raiseDrawPileAnimation()]);
    }

    function clearSceneState() {
        effects.clear?.();
        selected = null;
        hovered = null;
        hands.forEach((hand) => { hand.length = 0; });
        drawPile.length = 0;
        discardPile.length = 0;
        cards.forEach((card) => {
            gsap.killTweensOf([card.position, card.rotation, card.material]);
            if (card.userData.hitArea) scene.remove(card.userData.hitArea);
            scene.remove(card);
        });
        cards.length = 0;
    }

    function applyServerState(snapshot) {
        if (!snapshot) return;
        if (snapshot.version != null && snapshot.version === appliedVersion) return;
        appliedVersion = snapshot.version ?? null;
        clearSceneState();
        const topFace = snapshot.topCard || 'R1';
        const topCard = createCard(discardPileX, 0, topFace, 180);
        topCard.scale.setScalar(centralCardScale);
        topCard.rotation.x = centralCardRotation;
        topCard.userData.pile = 'discard';
        if (isWild(topCard) && snapshot.currentColor)
            topCard.material.map = cardTexture(`${topFace}_${snapshot.currentColor}`);
        discardPile.push(topCard);

        const visibleDrawCards = Math.min(Math.max(snapshot.drawCount || 0, 0), 30);
        for (let index = 0; index < visibleDrawCards; index += 1) {
            const card = createCard(drawPileX, 0, 'BACK', 180, index * 0.001);
            card.scale.setScalar(centralCardScale);
            card.rotation.x = centralCardRotation;
            card.userData.pile = 'draw';
            drawPile.push(card);
        }

        (snapshot.players || []).slice(0, hands.length).forEach((player, id) => {
            if (player.finished) return;
            const deck = id === myPlayer && Array.isArray(player.hand)
                ? player.hand
                : Array.from({ length: player.cardCount || 0 }, () => 'BACK');
            displayDeck(id, deck);
        });
    }

    return {
        preloadCardTextures, createCard, displayDeck, hands, drawPile, discardPile,
        selectCard, playCard, drawCard, restartGame,
        clickCard, updateHover, clearHover, applyServerState,
    };
}
