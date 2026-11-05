import * as THREE from 'three';
import { loadHlsl } from './volume_shaders.js';

export async function createFog(volumes) {
    const [shared, groundSource, forestSource] = await Promise.all([
        loadHlsl('./shaders/shared_fog.hlsl'),
        loadHlsl('./shaders/ground_mist.hlsl'),
        loadHlsl('./shaders/forest_fog.hlsl'),
    ]);
    const geometry = new THREE.BoxGeometry(1, 1, 1);

    function createBox(source, dimensions, position, uniforms) {
        const hlsl = source.replace('#include "shared_fog.hlsl"', shared);
        const box = new THREE.Mesh(geometry, volumes.createMaterial(hlsl, uniforms));
        box.scale.set(...dimensions);
        box.position.set(...position);
        volumes.scene.add(box);
        return box;
    }

    const groundMist = createBox(
        groundSource,
        [44, 1.4, 44],
        [0, 0.6, 0],
        {
            _Color: new THREE.Vector4(0.74, 0.74, 0.74, 1),
            _Density: 0.37,
            _NoiseScale: 0.65,
            _NoiseAmount: 0.9,
            _Wind: new THREE.Vector4(0.12, 0, 0.06, 0),
            _HeightFalloff: 2,
            _EdgeFade: 0.08,
            _Steps: 8,
        }
    );

    const forestFog = createBox(
        forestSource,
        [48, 20, 48],
        [0, 9, 0],
        {
            _Color: new THREE.Vector4(0.74, 0.74, 0.74, 1),
            _Density: 0.14,
            _ClearingRadius: 6,
            _Transition: 4,
            _BoundaryNoise: 2,
            _NoiseScale: 0.18,
            _NoiseAmount: 0.35,
            _Wind: new THREE.Vector4(0.04, 0, 0.02, 0),
            _EdgeFade: 0.08,
            _Steps: 8,
        }
    );

    return { groundMist, forestFog };
}
