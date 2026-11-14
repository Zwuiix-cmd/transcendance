uniform float4 _Color, _Wind;
uniform float _Density, _ClearingRadius, _Transition, _BoundaryNoise;
uniform float _NoiseScale, _NoiseAmount, _EdgeFade, _Steps;

#include "shared_fog.hlsl"

float FogDensity(float3 p, float3 local)
{
    float3 q = (p - _Time.y * _Wind.xyz) * _NoiseScale;
    float noise = 0.7 * Noise(q) + 0.3 * Noise(q * 2.03 + 19.1);
    float2 centre = unity_ObjectToWorld[3].xz;
    float radius = length(p.xz - centre) + (noise - 0.5) * _BoundaryNoise;
    float ring = smoothstep(_ClearingRadius, _ClearingRadius + _Transition, radius);
    float topFade = 1.0 - smoothstep(0.25, 0.5, local.y);
    return _Density * ring * topFade * lerp(1.0, noise, _NoiseAmount);
}
