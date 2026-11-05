import * as THREE from 'three';
import { FullScreenQuad } from 'three/addons/postprocessing/Pass.js';

export const hlslDefines = `
        #define float2 vec2
        #define float3 vec3
        #define float4 vec4
        #define half4 vec4
        #define float3x3 mat3
        #define frac fract
        #define lerp mix
        #ifndef saturate
            #define saturate(x) clamp(x, 0.0, 1.0)
        #endif
        #define mul(a, b) ((a) * (b))
        #define UNITY_REVERSED_Z 0
        #define UNITY_NEAR_CLIP_VALUE -1.0

        #define int2 ivec2
        #define TEXTURE2D(name) uniform sampler2D name
        #define SAMPLER(name) const int name = 0
        #define SAMPLE_TEXTURE2D_LOD(tex, samp, uv, lod) textureLod(tex, uv, float(lod))
`;

export async function loadHlsl(path) {
    const response = await fetch(path);
    if (!response.ok) throw new Error(`Shader failed to load: ${path} (${response.status})`);
    return response.text();
}

export function createVolumeRenderer(renderer, scene, sunlight) {
    const volumeScene = new THREE.Scene();
    const depthTexture = new THREE.DepthTexture(1, 1, THREE.UnsignedIntType);
    const worldTarget = new THREE.WebGLRenderTarget(1, 1, {
        type: THREE.HalfFloatType,
        samples: 4,
        depthTexture: depthTexture,
    });
    const sharedUniforms = {
        _ScaledScreenParams: { value: new THREE.Vector4() },
        _CameraDepthTexture: { value: depthTexture },
        UNITY_MATRIX_I_VP: { value: new THREE.Matrix4() },
        _MainLightDirection: { value: new THREE.Vector3() },
        _MainLightColor: { value: new THREE.Color() },
    };

    const vertexShader = `
        void main() {
            gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
        }
    `;

    const fragmentWrapper = (hlsl) => `
        ${hlslDefines}

        uniform vec4 _Time;
        uniform vec4 _ScaledScreenParams;
        uniform sampler2D _CameraDepthTexture;
        uniform mat4 UNITY_MATRIX_I_VP;
        uniform mat4 unity_WorldToObject;
        uniform mat4 unity_ObjectToWorld;
        uniform vec3 _MainLightDirection;
        uniform vec3 _MainLightColor;

        vec2 GetNormalizedScreenSpaceUV(vec4 positionCS) {
            return positionCS.xy / _ScaledScreenParams.xy;
        }
        float SampleSceneDepth(vec2 uv) {
            return texture(_CameraDepthTexture, uv).r;
        }
        vec3 ComputeWorldSpacePosition(vec2 uv, float depth, mat4 inverseVP) {
            vec4 world = inverseVP * vec4(uv * 2.0 - 1.0, depth, 1.0);
            return world.xyz / world.w;
        }
        vec3 TransformWorldToObject(vec3 point) {
            return (unity_WorldToObject * vec4(point, 1.0)).xyz;
        }
        vec3 TransformObjectToWorld(vec3 point) {
            return (unity_ObjectToWorld * vec4(point, 1.0)).xyz;
        }
        vec3 SafeNormalize(vec3 v) {
            return v * inversesqrt(max(dot(v, v), 1e-12));
        }

        struct Light { vec3 direction; vec3 color; };
        Light GetMainLight() {
            return Light(_MainLightDirection, _MainLightColor);
        }

        ${hlsl}

        out vec4 outColor;
        void main() {
            outColor = Frag(Varyings(gl_FragCoord));
            outColor.rgb /= max(outColor.a, 0.000001);
            #define gl_FragColor outColor
            #include <tonemapping_fragment>
            #include <colorspace_fragment>
            outColor.rgb *= outColor.a;
        }
    `;

    function createMaterial(hlsl, settings) {
        const uniforms = {};
        for (const name in settings)
            uniforms[name] = { value: settings[name] };

        return new THREE.ShaderMaterial({
            glslVersion: THREE.GLSL3,
            uniforms: {
                ...sharedUniforms,
                _Time: { value: new THREE.Vector4() },
                unity_WorldToObject: { value: new THREE.Matrix4() },
                unity_ObjectToWorld: { value: new THREE.Matrix4() },
                ...uniforms,
            },
            vertexShader,
            fragmentShader: fragmentWrapper(hlsl),
            side: THREE.BackSide,
            depthTest: false,
            depthWrite: false,
            transparent: true,
            blending: THREE.CustomBlending,
            blendSrc: THREE.OneFactor,
            blendDst: THREE.OneMinusSrcAlphaFactor,
            blendSrcAlpha: THREE.OneFactor,
            blendDstAlpha: THREE.OneMinusSrcAlphaFactor,
        });
    }

    const backgroundMaterial = new THREE.ShaderMaterial({
        glslVersion: THREE.GLSL3,
        uniforms: {
            image: { value: worldTarget.texture }
        },
        vertexShader: `
            out vec2 vUv;
            void main() {
                vUv = uv;
                gl_Position = vec4(position.xy, 0.0, 1.0);
            }
        `,

        fragmentShader: `
            uniform sampler2D image;
            in vec2 vUv;
            out vec4 outColor;
            void main() {
                outColor = texture(image, vUv);
                #define gl_FragColor outColor
                #include <tonemapping_fragment>
                #include <colorspace_fragment>
            }
        `,
        depthTest: false,
        depthWrite: false,
    });
    const background = new FullScreenQuad(backgroundMaterial);
    const size = new THREE.Vector2();
    const lightPosition = new THREE.Vector3();
    const lightTarget = new THREE.Vector3();
    function render(time, camera) {
        renderer.getDrawingBufferSize(size);
        if (worldTarget.width !== size.x || worldTarget.height !== size.y)
            worldTarget.setSize(size.x, size.y);
        sharedUniforms._ScaledScreenParams.value.set(size.x, size.y, 1 + 1 / size.x, 1 + 1 / size.y);

        camera.updateMatrixWorld();
        sharedUniforms.UNITY_MATRIX_I_VP.value.multiplyMatrices(camera.matrixWorld, camera.projectionMatrixInverse);

        sunlight.getWorldPosition(lightPosition);
        sunlight.target.getWorldPosition(lightTarget);
        sharedUniforms._MainLightDirection.value.subVectors(lightPosition, lightTarget).normalize();

        sharedUniforms._MainLightColor.value.copy(sunlight.color).multiplyScalar(sunlight.intensity);

        volumeScene.updateMatrixWorld(true);
        for (const volume of volumeScene.children) {
            const uniforms = volume.material.uniforms;
            const t = time + (volume.userData.timeOffset ?? 0);
            uniforms._Time.value.set(t / 20, t, t * 2, t * 3);
            uniforms.unity_ObjectToWorld.value.copy(volume.matrixWorld);
            uniforms.unity_WorldToObject.value.copy(volume.matrixWorld).invert();
        }

        renderer.setRenderTarget(worldTarget);
        renderer.render(scene, camera);

        renderer.setRenderTarget(null);
        background.render(renderer);

        renderer.autoClear = false;
        renderer.render(volumeScene, camera);
        renderer.autoClear = true;
    }

    return { scene: volumeScene, createMaterial, render };
}
