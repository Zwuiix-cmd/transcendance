import * as THREE from 'three';
import { loadHlsl } from './volume_shaders.js?v=shaders-20261114-v30';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';

export async function createPlayerHeads(volumes, scene, myPlayer, seatCount = 4, roundTable = false) {
    const orb_head_hlsl = await loadHlsl('./shaders/orb_head.hlsl');
    const headGeometry = new THREE.BoxGeometry(1, 1, 1);
    const loader = new GLTFLoader();
    const glassesgltf = await loader.loadAsync('./models/glasses.glb');
    const glassesModel = glassesgltf.scene;
    glassesModel.traverse((object) => {
        if (!object.isMesh) return;
        object.castShadow = true;
        object.receiveShadow = true;
    });

    const glassesBox = new THREE.Box3();
    glassesBox.setFromObject(glassesModel);
    const glassesSize = new THREE.Vector3();
    glassesBox.getSize(glassesSize);
    const glassesSettings = { scale: 0.7, distance: 0.38, height: 0.08 };

    const coreColorHeads = [
        new THREE.Vector4(1, 0.439, 0.847, 1),
        new THREE.Vector4(1, 0.439, 0.847, 1),
        new THREE.Vector4(1, 0.439, 0.847, 1),
        new THREE.Vector4(1, 0.439, 0.847, 1),
    ];
    const heads = [];
    const headRadius = 1.15 + Math.max(0, seatCount - 4) * 0.035 + 0.1;
    const positions = !roundTable && seatCount <= 4
        ? [[0, 0.8], [-0.8, 0], [0, -0.8], [0.8, 0]]
        : Array.from({ length: seatCount }, (_, id) => {
            const angle = (id / seatCount) * Math.PI * 2;
            return [Math.sin(angle) * headRadius, Math.cos(angle) * headRadius];
        });
    for (let id = 0; id < seatCount; id++) {
        if (id === myPlayer) {
            heads.push(null);
            continue;
        }
        const headMaterial = volumes.createMaterial(
            orb_head_hlsl,
            {
                _InnerColor: coreColorHeads[id % coreColorHeads.length],
                _Steps: 12,
                _ShadowSteps: 2,
            }
        );

        const head = new THREE.Mesh(headGeometry, headMaterial);
        head.renderOrder = 1;
        head.scale.setScalar(0.6);
        head.position.set(positions[id][0], 2.15, positions[id][1]);
        head.userData.playerId = id;
        head.userData.timeOffset = id * 17.3;
        head.userData.lookDirection = new THREE.Vector3(-head.position.x, 0, -head.position.z).normalize();

        const model = glassesModel.clone(true);
        model.scale.setScalar(glassesSettings.scale / glassesSize.x);
        model.position.y = glassesSettings.height;
        const glasses = new THREE.Group();
        glasses.add(model);
        head.userData.glasses = glasses;
        scene.add(glasses);
        volumes.scene.add(head);
        heads.push(head);
    }

    const lookDirection = new THREE.Vector3();
    const lookTarget = new THREE.Vector3();

    function update() {
        for (const head of heads) {
            if (!head) continue;
            const glasses = head.userData.glasses;
            glasses.visible = head.visible;

            lookDirection.copy(head.userData.lookDirection);
            if (lookDirection.length() === 0)
                lookDirection.set(0, 0, 1);
            lookDirection.normalize();
            head.lookAt(head.getWorldPosition(lookTarget).add(lookDirection));

            head.updateMatrixWorld(true);
            glasses.position.set(0, 0, glassesSettings.distance).applyMatrix4(head.matrixWorld);
            head.getWorldQuaternion(glasses.quaternion);
            glasses.scale.setFromMatrixScale(head.matrixWorld);
        }
    }

    function setActive(id, active) {
        const head = heads[id];
        if (head) head.visible = active;
    }

    return { heads, update, setActive };
}
