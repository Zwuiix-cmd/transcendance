uniform float4 _Color;
uniform float _Scale;
const float _Density = 5.0;
const float _Thickness = 0.55;
uniform float _Lifetime;
const float _NoiseScale = 2.2;
const float _NoiseAmount = 0.6;
const float _FogSpeed = 0.4;
uniform float _Steps;
uniform sampler2D _Trail;
uniform sampler2D _Origins;
uniform float4 _TrailMap;

#define CARD_FOG
#include "shared_fog.hlsl"

float3 ReadTrail(float2 uv)
{
    if (any(lessThan(uv, float2(0.0))) || any(greaterThan(uv, float2(1.0)))) return float3(0.0);
    float3 trail = textureLod(_Trail, uv, 0.0).rgb;
    if (trail.x <= 0.00001) return float3(0.0);
    return float3(trail.x, trail.yz / trail.x);
}

float FogDensity(float3 p)
{
    float2 uv = (p.xz - _TrailMap.xy) / _TrailMap.zw + 0.5;
    float3 trail = ReadTrail(uv);
    float density = trail.x;
    float height = trail.y;
    float seconds = trail.z;
    if (density == 0.0) return 0.0;
    float2 origin = textureLod(_Origins, uv, 0.0).xy / density;
    
    float expansion = 1.0 + seconds * max(_FogSpeed, 0.0);
    float3 q;
    q.xz = origin + (p.xz - origin) / expansion;
    q.y = p.y;
    q *= _NoiseScale / _Scale;
    q += 0.6 * sin(q.zxy * 1.7 + seconds * _FogSpeed * 0.5);
    float noise = 0.7 * Noise(q) + 0.3 * Noise(q * 2.03 + 19.1);
    float age = saturate(seconds / _Lifetime);
    float cloud = smoothstep(0.3 + age * 0.35, 0.65 + age * 0.25, noise);
    
    float maturity = smoothstep(0.02, 0.5, age);
    float thickness = max(_Thickness * _Scale, 0.001) * lerp(0.12, 1.0, maturity);
    float h = (p.y - height) / thickness;
    float vertical = exp2(-2.0 * h * h);
    
    float breakup = Noise(q * 0.75 + 47.3);
    float survival = smoothstep(age - 0.15, age + 0.15, breakup);
    float evaporation = lerp(1.0, survival, smoothstep(0.25, 0.6, age));
    
    return (_Density / _Scale) * density * vertical * evaporation * lerp(1.0, cloud, _NoiseAmount);
}
