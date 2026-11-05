<?php

declare(strict_types=1);

require_once dirname(__DIR__) . '/src/AdminSession.php';

header('Cache-Control: no-store');
header('X-Content-Type-Options: nosniff');
header('X-Frame-Options: DENY');

$requestedPath = (string) ($_GET['path'] ?? '');
$path = parse_url($requestedPath, PHP_URL_PATH);
$query = parse_url($requestedPath, PHP_URL_QUERY);
$path = is_string($path) ? $path : '';
$forwardPath = $path . ($query ? '?' . $query : '');
$method = strtoupper($_SERVER['REQUEST_METHOD'] ?? 'GET');

if (!adminProxyRouteAllowed($path, $method)) {
    http_response_code(404);
    header('Content-Type: application/json');
    echo json_encode([
        'code' => 'not_found',
        'message' => 'This admin API route is not available.',
    ], JSON_THROW_ON_ERROR);
    exit;
}

$body = file_get_contents('php://input') ?: '';
if (strlen($body) > 1_048_576) {
    adminJsonError(413, 'request_too_large', 'The request body is too large.');
}

if (!adminRequestOriginAllowed($method)) {
    adminJsonError(403, 'invalid_origin', 'The request origin is not allowed.');
}

$authorization = adminSessionAuthorization();
$proxy = adminApiProxy();
$result = $proxy->forward($method, $forwardPath, $body, $authorization);

if ($path === '/auth/login' && $result['status'] >= 200 && $result['status'] < 300) {
    $result = attachAdminSessionCookie($result);
}
if ($path === '/auth/logout' || in_array($result['status'], [401, 403], true)) {
    adminClearSessionCookie();
}

http_response_code($result['status']);
header('Content-Type: ' . $result['contentType']);
foreach ($result['headers'] as $responseHeader) {
    if (preg_match('/^(X-Pagination-(?:Page|Limit|Has-More)):\s*(.+)$/i', $responseHeader, $matches) === 1) {
        header($matches[1] . ': ' . trim($matches[2]));
    }
}
echo $result['body'];

function adminProxyRouteAllowed(string $path, string $method): bool
{
    if ($method === 'POST' && in_array($path, ['/auth/login', '/auth/logout'], true)) {
        return true;
    }

    if ($method === 'GET' && $path === '/auth/me') {
        return true;
    }

    if ($method === 'GET' && preg_match('#^/admin/(dashboard|users|logs|logs/stats)$#', $path) === 1) {
        return true;
    }

    if ($method === 'GET' && preg_match('#^/admin/users/[A-Za-z0-9_-]+/analytics$#', $path) === 1) {
        return true;
    }

    if ($method === 'PUT' && preg_match('#^/admin/users/[A-Za-z0-9_-]+/(elo|role)$#', $path) === 1) {
        return true;
    }

    return $method === 'DELETE' && preg_match('#^/admin/users/[A-Za-z0-9_-]+$#', $path) === 1;
}

/** @return array{status: int, contentType: string, body: string} */
function attachAdminSessionCookie(array $result): array
{
    try {
        $payload = json_decode($result['body'], true, 512, JSON_THROW_ON_ERROR);
    } catch (Throwable) {
        return $result;
    }

    if (!is_array($payload) || !is_string($payload['accessToken'] ?? null)) {
        return $result;
    }

    adminSetSessionCookie($payload['accessToken'], is_string($payload['expiresAt'] ?? null) ? $payload['expiresAt'] : null);
    unset($payload['accessToken']);
    $result['body'] = json_encode($payload, JSON_THROW_ON_ERROR);

    return $result;
}

function adminRequestOriginAllowed(string $method): bool
{
    if (in_array($method, ['GET', 'HEAD', 'OPTIONS'], true)) {
        return true;
    }

    $origin = trim((string) ($_SERVER['HTTP_ORIGIN'] ?? ''));
    if ($origin === '') {
        $origin = trim((string) ($_SERVER['HTTP_REFERER'] ?? ''));
    }
    if ($origin === '') {
        return true;
    }

    $originHost = parse_url($origin, PHP_URL_HOST);
    $requestHost = parse_url('http://' . ($_SERVER['HTTP_HOST'] ?? ''), PHP_URL_HOST);

    return is_string($originHost) && is_string($requestHost) && strcasecmp($originHost, $requestHost) === 0;
}

function adminJsonError(int $status, string $code, string $message): never
{
    http_response_code($status);
    header('Content-Type: application/json');
    echo json_encode(['code' => $code, 'message' => $message], JSON_THROW_ON_ERROR);
    exit;
}
