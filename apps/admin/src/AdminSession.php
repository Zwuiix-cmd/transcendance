<?php

declare(strict_types=1);

require_once __DIR__ . '/ApiProxy.php';

function adminApiProxy(): ApiProxy
{
    return new ApiProxy(
        getenv('ADMIN_API_URL') ?: 'http://api:8080/api/v1',
        getenv('API_KEY') ?: '',
    );
}

function adminSessionCookieName(): string
{
    return 'uno_ranked_admin_session';
}

function adminSessionToken(): ?string
{
    $value = trim((string) ($_COOKIE[adminSessionCookieName()] ?? ''));

    return preg_match('/^[A-Za-z0-9_-]{16,512}$/', $value) === 1 ? $value : null;
}

function adminSessionIsAdmin(ApiProxy $proxy): bool
{
    $token = adminSessionToken();
    if ($token === null) {
        return false;
    }

    $result = $proxy->forward('GET', '/auth/me', '', 'Bearer ' . $token);
    if ($result['status'] !== 200) {
        return false;
    }

    try {
        $payload = json_decode($result['body'], true, 512, JSON_THROW_ON_ERROR);
    } catch (Throwable) {
        return false;
    }

    return is_array($payload) && ($payload['role'] ?? null) === 'admin';
}

function adminSessionAuthorization(): ?string
{
    $token = adminSessionToken();

    return $token === null ? null : 'Bearer ' . $token;
}

function adminSetSessionCookie(string $token, ?string $expiresAt): void
{
    $expires = time() + 86_400;
    if ($expiresAt !== null) {
        try {
            $parsed = new DateTimeImmutable($expiresAt);
            $expires = $parsed->getTimestamp();
        } catch (Throwable) {
            // Keep the bounded default when the API response has no valid expiry.
        }
    }

    setcookie(adminSessionCookieName(), $token, [
        'expires' => max(time() + 60, $expires),
        'path' => '/admin',
        'secure' => adminCookieSecure(),
        'httponly' => true,
        'samesite' => 'Strict',
    ]);
}

function adminClearSessionCookie(): void
{
    setcookie(adminSessionCookieName(), '', [
        'expires' => time() - 3600,
        'path' => '/admin',
        'secure' => adminCookieSecure(),
        'httponly' => true,
        'samesite' => 'Strict',
    ]);
}

function adminCookieSecure(): bool
{
    return filter_var(getenv('ADMIN_COOKIE_SECURE') ?: 'true', FILTER_VALIDATE_BOOLEAN);
}
