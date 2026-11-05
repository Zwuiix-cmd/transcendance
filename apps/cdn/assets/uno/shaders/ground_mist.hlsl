uniform float4 _Color, _Wind;
uniform float _Density, _NoiseScale, _NoiseAmount, _HeightFalloff, _EdgeFade, _Steps;

#include "shared_fog.hlsl"

float FogDensity(float3 p, float3 local)
{
    float3 q = (p - _Time.y * _Wind.xyz) * _NoiseScale;
    q += 0.25 * sin(q.zxy * 1.7 + _Time.y * 0.12);
    float noise = 0.7 * Noise(q) + 0.3 * Noise(q * 2.03 + 19.1);
    noise = smoothstep(0.2, 0.8, noise);
    float height = saturate(local.y + 0.5);
    float layer = pow(1.0 - height, _HeightFalloff);
    return _Density * layer * lerp(1.0, noise, _NoiseAmount);
}
