<?php

declare(strict_types=1);

require_once dirname(__DIR__) . '/src/AdminSession.php';

header('Cache-Control: no-store');
header('X-Content-Type-Options: nosniff');
header('X-Frame-Options: DENY');
header('Referrer-Policy: no-referrer');
header('Permissions-Policy: camera=(), microphone=(), geolocation=()');
header('Cross-Origin-Opener-Policy: same-origin');
header('Cross-Origin-Resource-Policy: same-origin');
header("Content-Security-Policy: default-src 'self'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'; script-src 'self'; style-src 'self'");

$hasAdminSession = adminSessionIsAdmin(adminApiProxy());
if (adminSessionToken() !== null && !$hasAdminSession) {
    adminClearSessionCookie();
}
$bodyClass = $hasAdminSession ? '' : 'admin-locked';
$sessionState = $hasAdminSession ? 'admin' : 'anonymous';
?><!doctype html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Uno Ranked — Admin</title>
    <link rel="stylesheet" href="admin.css">
</head>
<body class="<?= $bodyClass ?>" data-admin-session="<?= $sessionState ?>">
    <div id="admin-app" data-api-endpoint="api.php" data-admin-session="<?= $sessionState ?>">
        <aside class="admin-sidebar">
            <a class="admin-brand" href="/dashboard" aria-label="Back to Uno Ranked">
                <span class="admin-brand-mark">UR</span>
                <span><strong>Uno Ranked</strong><small>Administration</small></span>
            </a>
            <nav class="admin-nav" aria-label="Administration">
                <button type="button" data-view="overview">Overview</button>
                <button type="button" data-view="requests">Requests</button>
                <button type="button" data-view="user-activity">User activity</button>
                <button type="button" data-view="games">Games</button>
                <button type="button" data-view="users">Users</button>
                <button type="button" data-view="logs">Audit logs</button>
                <button type="button" data-view="actors">Actors</button>
            </nav>
            <div class="admin-sidebar-footer">
                <a href="/dashboard">Back to site</a>
                <button type="button" data-action="logout">Sign out</button>
            </div>
        </aside>
        <div class="admin-workspace">
            <header class="admin-topbar">
                <div>
                    <span class="admin-context">Uno Ranked / Admin</span>
                    <strong id="admin-view-title">Overview</strong>
                </div>
                <div class="admin-topbar-actions">
                    <span id="admin-user" class="admin-user-label">Not signed in</span>
                    <button type="button" class="admin-button admin-button-secondary" data-action="refresh">Refresh</button>
                </div>
            </header>
            <main id="admin-content" class="admin-content" aria-live="polite"></main>
        </div>
    </div>
    <script src="admin.js" defer></script>
</body>
</html>
