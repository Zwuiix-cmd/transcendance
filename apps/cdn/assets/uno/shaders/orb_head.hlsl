uniform float4 _InnerColor;
const float4 _FogColor = float4(0.45, 0.5, 0.65, 1.0);
const float _Intensity = 35.454545;
const float _CoreEdgeFade = 0.665;
uniform float _Steps;
const float _FogDensity = 10.4;
const float _FogNoiseScale = 5.56;
const float _FogNoiseAmount = 0.891;
const float _FogSpeed = 0.65;
const float _FogEdgeFade = 0.421;
const float _BoundaryTurbulence = 0.278;
const float _FogGlow = 2.34;
const float _ShadowStrength = 2.34;
uniform float _ShadowSteps;
const float _AmbientFill = 0.3;

struct Varyings { float4 positionCS; };

float fogTime;
float2 tiltRotation;

float2 RotateBy(float2 p, float2 sc)
{
    return float2(sc.y * p.x + sc.x * p.y, -sc.x * p.x + sc.y * p.y);
}

float2 Rotate(float2 p, float angle)
{
    return RotateBy(p, float2(sin(angle), cos(angle)));
}

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

float FogDensity(float3 p)
{
    if (_FogDensity <= 0.0) return 0.0;
    float radius = length(p);
    if (radius >= 1.0) return 0.0;
    float time = fogTime;
    float3 q = p * _FogNoiseScale;
    q.xz = Rotate(q.xz, time * 0.4 + 0.6 * sin(p.y * 2.0 + time * 0.7));
    q.yz = RotateBy(q.yz, tiltRotation);
    q += 0.4 * sin(q.yzx * 1.8 + time * float3(0.7, -0.9, 0.6));
    float noise = 0.65 * Noise(q) + 0.35 * Noise(q * 2.03 + 19.1);
    float cloud = smoothstep(0.25, 0.75, noise);
    float edgeDistance = 1.0 - radius - _BoundaryTurbulence * (1.0 - cloud);
    float fade = smoothstep(0.0, _FogEdgeFade, edgeDistance);
    float density = fade * lerp(1.0, cloud, _FogNoiseAmount);
    return _FogDensity * density;
}

float LightTransmission(float3 p, float3 toLight)
{
    if (_ShadowStrength <= 0.0) return 1.0;
    int steps = int(_ShadowSteps);
    float stepLength = length(toLight) / float(steps);
    float opticalDepth = 0.0;
    for (int i = 0; i < steps; i++)
    {
        opticalDepth += FogDensity(p + toLight * ((float(i) + 0.5) / float(steps))) * stepLength;
    }
    return exp(-opticalDepth * _ShadowStrength);
}

float3 CoreEmission(float3 p)
{
    float fade = smoothstep(0.0, _CoreEdgeFade, 1.0 - length(p));
    return _InnerColor.rgb * _Intensity * fade / (1.0 + dot(p, p) * 1081.6);
}

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
    float3 origin = TransformWorldToObject(nearWS) / 0.48;
    float3 scene = TransformWorldToObject(sceneWS) / 0.48;
    float sceneDistance = length(scene - origin);
    float3 direction = SafeNormalize(scene - origin);

    float projection = dot(-origin, direction);
    float closestSquared = max(dot(origin, origin) - projection * projection, 0.0);
    float discriminant = 1.0 - closestSquared;
    if (discriminant <= 0.0) return half4(0.0);
    float halfChord = sqrt(discriminant);
    float start = max(projection - halfChord, 0.0);
    float end = min(projection + halfChord, sceneDistance);
    if (end <= start) return half4(0.0);

    int steps = int(_Steps);
    float stepLength = (end - start) / float(steps);
    float jitter = frac(52.9829189 * frac(dot(varyings.positionCS.xy, float2(0.06711056, 0.00583715))));
    float3 accumulated = float3(0.0);
    float transmission = 1.0;
    Light mainLight = GetMainLight();
    float3 lightDirection = SafeNormalize(mul(float3x3(unity_WorldToObject), mainLight.direction));
    fogTime = _Time.y * _FogSpeed;
    tiltRotation = float2(sin(fogTime * 0.23), cos(fogTime * 0.23));
    for (int step = 0; step < steps; step++)
    {
        float travel = start + (float(step) + jitter) * stepLength;
        float3 p = origin + direction * travel;
        float density = FogDensity(p);
        float3 fogColor = _FogColor.rgb + _InnerColor.rgb * _FogGlow / (1.0 + dot(p, p) * 6.76);
        if (density > 0.0026 && _ShadowStrength > 0.0)
        {
            float alongLight = dot(p, lightDirection);
            float lightDistance = -alongLight + sqrt(max(0.0,
                alongLight * alongLight + 1.0 - dot(p, p)));
            float outsideLight = LightTransmission(p, lightDirection * lightDistance);
            float coreLight = LightTransmission(p, -p);
            float3 ambient = _FogColor.rgb * _AmbientFill;
            float3 externalColor = _FogColor.rgb * mainLight.color * outsideLight;
            float3 internalColor = _InnerColor.rgb * _FogGlow * coreLight / (1.0 + dot(p, p) * 6.76);
            float3 shadowedColor = ambient + externalColor + internalColor;
            fogColor = lerp(fogColor, shadowedColor, saturate(_ShadowStrength));
        }

        float3 emission = CoreEmission(p);
        float fogStepTransmission = exp(-density * stepLength);
        float emissionLength = density > 0.00026 ? (1.0 - fogStepTransmission) / density : stepLength;
            accumulated += transmission * (emission * emissionLength + fogColor * (1.0 - fogStepTransmission));
        transmission *= fogStepTransmission;
        if (transmission < 0.004) break;
    }
    return half4(accumulated, 1.0 - transmission);
}
