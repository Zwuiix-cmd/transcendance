import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';

async function fetchSplitGLB(modelName, partCount, signal) {
    const chunks = await Promise.all(
        Array.from({ length: partCount }, async (_, index) => {
            const partPath = `./models/${modelName}_parts/${modelName}.part${String(index).padStart(2, '0')}`;
            const response = await fetch(partPath, { signal });
            if (!response.ok)
                throw new Error(`Failed to fetch ${partPath} (${response.status})`);
            return response.arrayBuffer();
        }),
    );

    return new Response(new Blob(chunks, { type: 'model/gltf-binary' }));
}

export function createWorldControl(scene) {
    const loader = new GLTFLoader();
    const expandedTable = new URLSearchParams(window.location.search).has('seats');
    let essential = false;
    let world = null;
    let request = null;
    let resolveReady;
    const ready = new Promise((resolve) => {
        resolveReady = resolve;
    });

    function unload(model) {
        scene.remove(model);
        const geometries = new Set();
        const materials = new Set();
        const textures = new Set();
        model.traverse((object) => {
            if (!object.isMesh) return;
            geometries.add(object.geometry);
            const list = Array.isArray(object.material) ? object.material : [object.material];
            list.forEach((material) => materials.add(material));
        });
        for (const material of materials) {
            for (const value of Object.values(material)) {
                if (value?.isTexture)
                    textures.add(value);
            }
            material.dispose();
        }
        for (const geometry of geometries)
            geometry.dispose();
        const images = new Set();
        for (const texture of textures) {
            images.add(texture.source.data);
            texture.dispose();
        }
        for (const image of images)
            image?.close?.();
    }

    async function loadWorld() {
        request?.abort();
        const current = new AbortController();
        request = current;
        if (world) {
            unload(world);
            world = null;
        }

        try {
            const response = essential
                ? await fetch('./models/uno_essential.glb', { signal: current.signal })
                : await fetchSplitGLB('uno', 17, current.signal);
            if (!response.ok)
                throw new Error(`World model failed to load (${response.status})`);

            const data = await response.arrayBuffer();
            if (current.signal.aborted)
                return;

            const gltf = await loader.parseAsync(data, './models/');
            if (current.signal.aborted) {
                unload(gltf.scene);
                return;
            }

            world = gltf.scene;
            world.traverse((object) => {
                if (expandedTable && /outdoor_table_chair_set_01_(table|chair)/i.test(object.name)) {
                    object.visible = false;
                    return;
                }
                if (!object.isMesh) return;
                object.castShadow = true;
                object.receiveShadow = true;
            });
            scene.add(world);
            resolveReady();
        } catch (error) {
            if (!current.signal.aborted)
                console.error(error);
        }
    }

    window.addEventListener('keydown', (event) => {
        if (event.key !== 'F1') return;
        event.preventDefault();
        if (event.repeat) return;
        essential = !essential;
        loadWorld();
    });

    loadWorld();
    return ready;
}
