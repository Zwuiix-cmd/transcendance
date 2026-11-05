<?php

declare(strict_types=1);

final class ApiProxy
{
    public function __construct(
        private readonly string $baseUrl,
        private readonly string $apiKey,
    ) {
    }

    /** @return array{status: int, contentType: string, body: string, headers: list<string>} */
    public function forward(string $method, string $path, string $body, ?string $authorization): array
    {
        $headers = [
            'Accept: application/json',
            'X-API-Key: ' . $this->apiKey,
        ];

        if ($authorization !== null && $authorization !== '') {
            $headers[] = 'Authorization: ' . $authorization;
        }

        if ($body !== '') {
            $headers[] = 'Content-Type: application/json';
        }

        $context = stream_context_create([
            'http' => [
                'method' => $method,
                'header' => implode("\r\n", $headers) . "\r\n",
                'content' => $body,
                'ignore_errors' => true,
                'timeout' => 10,
            ],
        ]);

        $response = @file_get_contents(rtrim($this->baseUrl, '/') . $path, false, $context);
        $responseHeaders = $http_response_header ?? [];

        if ($response === false) {
            return [
                'status' => 502,
                'contentType' => 'application/json',
                'body' => json_encode([
                    'code' => 'admin_api_unavailable',
                    'message' => 'The API could not be reached.',
                ], JSON_THROW_ON_ERROR),
                'headers' => [],
            ];
        }

        return [
            'status' => $this->statusCode($responseHeaders),
            'contentType' => $this->contentType($responseHeaders),
            'body' => $response,
            'headers' => $responseHeaders,
        ];
    }

    /** @param list<string> $headers */
    private function statusCode(array $headers): int
    {
        foreach (array_reverse($headers) as $header) {
            if (preg_match('/^HTTP\/\S+\s+(\d{3})/', $header, $matches) === 1) {
                return (int) $matches[1];
            }
        }

        return 502;
    }

    /** @param list<string> $headers */
    private function contentType(array $headers): string
    {
        foreach ($headers as $header) {
            if (str_starts_with(strtolower($header), 'content-type:')) {
                return trim(substr($header, strlen('content-type:')));
            }
        }

        return 'application/json';
    }
}
