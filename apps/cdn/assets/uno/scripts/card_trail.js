import * as THREE from 'three';
import { FullScreenQuad } from 'three/addons/postprocessing/Pass.js';
import { hlslDefines, loadHlsl } from './volume_shaders.js';

export async function createCardTrail(renderer, volumes) {
    const [compute, source, shared] = await Promise.all([
        loadHlsl('./shaders/card_trail_compute.hlsl'),
        loadHlsl('./shaders/card_fog_trail.hlsl'),
        loadHlsl('./shaders/shared_fog.hlsl'),
    ]);

    const scale = 0.16 / 1.5;
    const settings = {
        resolution: 256,
        cardLength: 1.5,
        lifetime: 3,
        spread: 0.06,
        centre: new THREE.Vector3(0, 1.665, 0),
        size: new THREE.Vector3(1.6, 0.7, 1.6),
    };
    const colors = { R: 0xed1c24, G: 0x50aa44, B: 0x0072bc, Y: 0xffde16, gray: 0x999999 };
    const map = new THREE.Vector4(settings.centre.x, settings.centre.z, settings.size.x, settings.size.z);
    const layers = new Map();
    const active = new Map();
    const geometry = new THREE.BoxGeometry(1, 1, 1);
    const shader = source.replace('#include "shared_fog.hlsl"', shared);
    const position = new THREE.Vector3();
    const axis = new THREE.Vector3();

    const uniforms = {
        _Scale: { value: scale },
        _CardLength: { value: settings.cardLength },
        _Lifetime: { value: settings.lifetime },
        _Spread: { value: settings.spread },
        _TrailRead: { value: null },
        _OriginsRead: { value: null },
        _TrailMap: { value: map },
        _Previous: { value: new THREE.Vector4() },
        _Current: { value: new THREE.Vector4() },
        _DeltaTime: { value: 0 },
        _PoseInterval: { value: 0 },
        _Stamp: { value: 0 },
        _StampSteps: { value: 1 },
        _Clear: { value: false },
        _Resolution: { value: new THREE.Vector2(settings.resolution, settings.resolution) },
    };
    const material = new THREE.ShaderMaterial({
        glslVersion: THREE.GLSL3,
        uniforms,
        vertexShader: `void main() { gl_Position = vec4(position.xy, 0.0, 1.0); }`,
        fragmentShader: `
            ${hlslDefines}
            ${compute}
            uniform bool _Clear;
            uniform vec2 _Resolution;
            layout(location = 0) out vec4 trail;
            layout(location = 1) out vec4 origins;
            void main() {
                if (_Clear)
                    ClearTrail(trail, origins);
                else
                    UpdateTrail(ivec2(gl_FragCoord.xy), ivec2(_Resolution), trail, origins);
            }
        `,
        depthTest: false,
        depthWrite: false,
        blending: THREE.NoBlending,
        toneMapped: false,
    });

    const pass = new FullScreenQuad(material);

    function dispatch(layer, seconds, stamp, clear = false) {
        const previousTarget = renderer.getRenderTarget();
        const autoClear = renderer.autoClear;
        uniforms._TrailRead.value = layer.read.textures[0];
        uniforms._OriginsRead.value = layer.read.textures[1];
        uniforms._DeltaTime.value = seconds;
        uniforms._Stamp.value = stamp ? 1 : 0;
        uniforms._Clear.value = clear;
        renderer.autoClear = false;
        renderer.setRenderTarget(layer.write);
        pass.render(renderer);

        renderer.setRenderTarget(previousTarget);
        renderer.autoClear = autoClear;

        [layer.read, layer.write] = [layer.write, layer.read];
        layer.mesh.material.uniforms._Trail.value = layer.read.textures[0];
        layer.mesh.material.uniforms._Origins.value = layer.read.textures[1];
    }

    function getLayer(color) {
        if (layers.has(color))
            return layers.get(color);
        const target = new THREE.WebGLRenderTarget(
            settings.resolution,
            settings.resolution,
            {
                count: 2,
                type: THREE.FloatType,
                minFilter: THREE.LinearFilter,
                magFilter: THREE.LinearFilter,
                depthBuffer: false,
            }
        );
        const tint = new THREE.Color(colors[color]);
        const colorVolumeMaterial = volumes.createMaterial(
            shader,
            {
                _Trail: target.textures[0],
                _Origins: target.textures[1],
                _TrailMap: map,
                _Color: new THREE.Vector4(tint.r, tint.g, tint.b, 1),
                _Scale: scale,
                _Lifetime: settings.lifetime,
                _Steps: 8,
            }
        );
        const colorMesh = new THREE.Mesh(geometry, colorVolumeMaterial);
        colorMesh.position.copy(settings.centre);
        colorMesh.scale.copy(settings.size);
        colorMesh.renderOrder = 2;
        colorMesh.visible = false;
        volumes.scene.add(colorMesh);
        const layer = {
            mesh: colorMesh,
            read: target,
            write: target.clone(),
            lastEmission: -Infinity,
        };
        dispatch(layer, 0, false, true);
        layers.set(color, layer);
        return layer;
    }

    function pose(card) {
        card.updateWorldMatrix(true, false);
        position.setFromMatrixPosition(card.matrixWorld);
        axis.set(1, 0, 0).transformDirection(card.matrixWorld);
        return new THREE.Vector4(position.x, position.y, position.z, Math.atan2(-axis.z, axis.x));
    }

    function submit(track, time) {
        const current = pose(track.card);
        const previous = track.previous;
        const angle = Math.atan2(Math.sin(current.w - previous.w), Math.cos(current.w - previous.w));
        current.w = previous.w + angle;
        
        const travelDistance = Math.hypot(current.x - previous.x, current.y - previous.y, current.z - previous.z);
        const travelRotation = Math.abs(angle) * (settings.cardLength * scale) * 0.6;
        const spacing = Math.min(map.z, map.w) / settings.resolution * 0.5;
        uniforms._StampSteps.value = THREE.MathUtils.clamp(
            Math.ceil((travelDistance + travelRotation) / spacing),
            1, 512);

        uniforms._Previous.value.copy(previous);
        uniforms._Current.value.copy(current);
        uniforms._PoseInterval.value = Math.max(0, time - track.time);
        dispatch(track.layer, 0, true);
        track.previous.copy(current);
        track.time = time;
        track.layer.lastEmission = time;
        track.layer.mesh.visible = true;
    }

    function start(card, action, duration) {
        const color = action === 'draw' ? 'gray' : (card.userData.chosenColor || card.userData.face[0]);
        if (!(color in colors)) return () => {};
        const time = performance.now() / 1000;
        const track = {
            card,
            layer: getLayer(color),
            previous: pose(card),
            time,
            end: time + duration,
        };
        active.set(card, track);
        submit(track, time);
        return () => {
            if (active.get(card) !== track) return;
            submit(track, performance.now() / 1000);
            active.delete(card);
        };
    }

    let previousTime = performance.now() / 1000;
    function update() {
        const time = performance.now() / 1000;
        const seconds = Math.max(0, time - previousTime);
        previousTime = time;
        const diffusion = settings.spread * scale ** 2;
        const width = map.z / settings.resolution;
        const height = map.w / settings.resolution;
        const rate = diffusion * (1 / width ** 2 + 1 / height ** 2);
        const steps = Math.max(1, Math.ceil(seconds * 2 * rate / 0.9));
        for (const layer of layers.values()) {
            if (!layer.mesh.visible) continue;
            if (seconds >= settings.lifetime || time - layer.lastEmission >= settings.lifetime) {
                dispatch(layer, 0, false, true);
                layer.mesh.visible = false;
            } else {
                for (let i = 0; i < steps; i++)
                    dispatch(layer, seconds / steps, false);
            }
        }
        for (const [card, track] of active) {
            submit(track, time);
            if (time >= track.end)
                active.delete(card);
        }
    }

    function clear() {
        active.clear();
        for (const layer of layers.values()) {
            dispatch(layer, 0, false, true);
            layer.mesh.visible = false;
            layer.lastEmission = -Infinity;
        }
    }

    function warmupSetup() {
        const warmupScene = new THREE.Scene();
        const warmupCamera = new THREE.PerspectiveCamera(60, 1, 0.01, 10);
        warmupCamera.position.copy(settings.centre);
        for (const color of Object.keys(colors)) {
            const layer = getLayer(color);
            dispatch(layer, 0, true);
            dispatch(layer, 1 / 60, false);
            const mesh = layer.mesh.clone();
            mesh.visible = true;
            warmupScene.add(mesh);
        }
        renderer.compile(warmupScene, warmupCamera);
        clear();
        previousTime = performance.now() / 1000;
    }

    warmupSetup();

    return { start, update, clear };
}
