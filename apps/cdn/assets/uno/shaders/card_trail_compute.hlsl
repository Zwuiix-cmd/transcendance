uniform sampler2D _TrailRead, _OriginsRead;
uniform float4 _TrailMap, _Previous, _Current;
uniform float _DeltaTime, _PoseInterval, _Scale;
uniform float _CardLength, _Lifetime, _Spread;
const float _Softness = 0.2;
uniform int _Stamp, _StampSteps;

float4 Read(sampler2D field, int2 pixel, int2 size)
{
    return any(lessThan(pixel, int2(0))) || any(greaterThanEqual(pixel, size)) ? float4(0.0) : texelFetch(field, pixel, 0);
}

float4 Diffuse(sampler2D field, int2 pixel, int2 size, float2 weight)
{
    float4 centre = Read(field, pixel, size);
    if (_Stamp != 0) return centre;
    float4 left = Read(field, pixel + int2(-1, 0), size);
    float4 right = Read(field, pixel + int2(1, 0), size);
    float4 up = Read(field, pixel + int2(0, -1), size);
    float4 down = Read(field, pixel + int2(0, 1), size);
    return centre - weight.x * (2.0 * centre - (left + right)) - weight.y * (2.0 * centre - (up + down));
}

void ClearTrail(out float4 trail, out float4 origins)
{
    trail = float4(0.0);
    origins = float4(0.0);
}

void UpdateTrail(int2 pixel, int2 size, out float4 trail, out float4 origins)
{
    float2 texelSize = _TrailMap.zw / float2(size);
    float2 weight = (_Spread * _Scale * _Scale) * _DeltaTime / (texelSize * texelSize);
    float4 old = Diffuse(_TrailRead, pixel, size, weight);
    float inverseDensity = old.x > 1e-12 ? 1.0 / old.x : 0.0;
    float2 origin = Diffuse(_OriginsRead, pixel, size, weight).xy * inverseDensity;
    float elevation = old.y * inverseDensity;
    float age = old.z * inverseDensity;
    float remaining = old.x * saturate(1.0 - _DeltaTime / max(_Lifetime - age, 0.00001));
    age += _DeltaTime;
    float2 world = _TrailMap.xy + ((float2(pixel) + 0.5) / float2(size) - 0.5) * _TrailMap.zw;
    if (_Stamp != 0)
    {
        float2 segment = _Current.xz - _Previous.xz;
        float2 fromStart = world - _Previous.xz;
        float segmentLength = dot(segment, segment);
        float along = segmentLength > 0.0 ? saturate(dot(fromStart, segment) / segmentLength) : 0.0;
        float2 nearest = fromStart - segment * along;
        float reach = (length(float2(166.0 / 257.0, 1.0)) * (_CardLength * _Scale) * 0.5 + _Softness * _Scale) * 1.01;
        if (dot(nearest, nearest) <= reach * reach)
        {
            for (int i = 0; i <= _StampSteps; i++)
            {
                float t = float(i) / float(_StampSteps);
                float4 pose = lerp(_Previous, _Current, t);
                float2 offset = world - pose.xz;
                float s = sin(pose.w), c = cos(pose.w);
                float2 local = float2(c * offset.x - s * offset.y, s * offset.x + c * offset.y);
                float2 edge = abs(local) - float2(166.0 / 257.0, 1.0) * (_CardLength * _Scale) * 0.5;
                float outside = length(max(edge, 0.0));
                float shape = 1.0 - smoothstep(0.0, _Softness * _Scale, outside);
                float stampAge = (1.0 - t) * _PoseInterval;
                float strength = shape * saturate(1.0 - stampAge / _Lifetime);
                if (strength > remaining)
                {
                    float influence = (strength - remaining) / strength;
                    elevation = lerp(elevation, pose.y, influence);
                    age = lerp(age, stampAge, influence);
                    origin = lerp(origin, pose.xz, influence);
                    remaining = strength;
                }
            }
        }
    }
    trail = float4(remaining, elevation * remaining, age * remaining, 0.0);
    origins = float4(origin * remaining, 0.0, 0.0);
}
