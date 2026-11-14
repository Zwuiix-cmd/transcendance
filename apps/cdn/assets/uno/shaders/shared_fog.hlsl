struct Varyings { float4 positionCS; };

float Hash(float3 p)
{
    return frac(sin(dot(p, float3(127.1, 311.7, 74.7))) * 43758.5453);
}

float Noise(float3 p)
{
    float3 cell = floor(p);
    float3 f = frac(p);
    f = f * f * (3.0 - 2.0 * f);
    float bottom = lerp(lerp(Hash(cell), Hash(cell + float3(1.0,0.0,0.0)), f.x),
                        lerp(Hash(cell + float3(0.0,1.0,0.0)), Hash(cell + float3(1.0,1.0,0.0)), f.x), f.y);
    float top = lerp(lerp(Hash(cell + float3(0.0,0.0,1.0)), Hash(cell + float3(1.0,0.0,1.0)), f.x),
                     lerp(Hash(cell + float3(0.0,1.0,1.0)), Hash(cell + float3(1.0,1.0,1.0)), f.x), f.y);
    return lerp(bottom, top, f.z);
}

#ifdef CARD_FOG
    float FogDensity(float3 positionWS);
#else
    float FogDensity(float3 positionWS, float3 positionOS);
#endif

half4 Frag(Varyings varyings)
{
    float2 uv = GetNormalizedScreenSpaceUV(varyings.positionCS);
    float depth = SampleSceneDepth(uv);
    #if UNITY_REVERSED_Z
        float nearDepth = 1.0;
    #else
        float nearDepth = UNITY_NEAR_CLIP_VALUE;
        depth = lerp(UNITY_NEAR_CLIP_VALUE, 1.0, depth);
    #endif
    float3 nearWS = ComputeWorldSpacePosition(uv, nearDepth, UNITY_MATRIX_I_VP);
    float3 sceneWS = ComputeWorldSpacePosition(uv, depth, UNITY_MATRIX_I_VP);
    float sceneDistance = length(sceneWS - nearWS);
    float3 direction = SafeNormalize(sceneWS - nearWS);

    float3 localOrigin = TransformWorldToObject(nearWS);
    float3 localDirection = mul(float3x3(unity_WorldToObject), direction);
    float3 safeDirection = (step(0.0, localDirection) * 2.0 - 1.0) * max(abs(localDirection), 0.00001);
    float3 a = (-0.5 - localOrigin) / safeDirection;
    float3 b = ( 0.5 - localOrigin) / safeDirection;
    float3 entry = min(a, b);
    float3 exit = max(a, b);
    float start = max(max(entry.x, max(entry.y, entry.z)), 0.0);
    float end = min(min(exit.x, min(exit.y, exit.z)), sceneDistance);
    if (end <= start) return half4(0.0);

    int steps = int(_Steps);
    float stepLength = (end - start) / float(steps);
    float jitter = frac(52.9829189 * frac(dot(varyings.positionCS.xy, float2(0.06711056, 0.00583715))));
    float opticalDepth = 0.0;
    for (int step = 0; step < steps; step++)
    {
        float travel = start + (float(step) + jitter) * stepLength;
        float3 p = nearWS + direction * travel;
        #ifdef CARD_FOG
            float density = max(0.0, FogDensity(p));
        #else
            float3 local = localOrigin + localDirection * travel;
            float3 edge = 0.5 - abs(local);
            float fade = smoothstep(0.0, _EdgeFade, min(edge.x, min(edge.y, edge.z)));
            float density = max(0.0, FogDensity(p, local)) * fade;
        #endif
        opticalDepth += density * stepLength;
        if (opticalDepth > 5.521460917862246) break;
    }
    float opacity = 1.0 - exp(-opticalDepth);
    return half4(_Color.rgb * opacity, opacity);
}
