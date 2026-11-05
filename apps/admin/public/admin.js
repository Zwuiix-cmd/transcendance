(function () {
    'use strict'

    const root = document.getElementById('admin-app')
    const content = document.getElementById('admin-content')
    const endpoint = root?.dataset.apiEndpoint || '/admin/api.php'
    const numberFormatter = new Intl.NumberFormat(undefined)
    const dateFormatter = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' })
    const dayFormatter = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' })
    const viewTitles = {
        overview: 'Overview',
        requests: 'Requests',
        'user-activity': 'User activity',
        games: 'Games',
        users: 'Users',
        'user-detail': 'User profile',
        logs: 'Audit logs',
        actors: 'Actors',
    }
    const roles = ['player', 'staff', 'developer', 'admin']
    const state = {
        authenticated: root?.dataset.adminSession === 'admin',
        me: null,
        dashboard: null,
        users: [],
        usersPage: 1,
        usersHasMore: false,
        logs: [],
        logsPage: 1,
        logsHasMore: false,
        logStats: null,
        selectedUser: null,
        selectedAnalytics: null,
        view: 'overview',
        loading: false,
        logsLoading: false,
        busy: false,
        paginationLoading: false,
        error: '',
        notice: '',
        userSearch: '',
        userRole: 'all',
        logFilters: { days: '14', category: 'all', status: 'all', actor: '', search: '' },
    }

    function escapeHtml(value) {
        return String(value ?? '')
            .replaceAll('&', '&amp;')
            .replaceAll('<', '&lt;')
            .replaceAll('>', '&gt;')
            .replaceAll('"', '&quot;')
            .replaceAll("'", '&#039;')
    }

    function number(value) {
        return numberFormatter.format(Number(value || 0))
    }

    function date(value, fallback = '—') {
        if (!value) return fallback
        const parsed = new Date(value)
        return Number.isNaN(parsed.getTime()) ? fallback : dateFormatter.format(parsed)
    }

    function day(value) {
        if (!value) return '—'
        const parsed = new Date(String(value).length === 10 ? value + 'T12:00:00Z' : value)
        return Number.isNaN(parsed.getTime()) ? value : dayFormatter.format(parsed)
    }

    function roleLabel(role) {
        return String(role || 'player').replace(/^./, (letter) => letter.toUpperCase())
    }

    function actionLabel(action) {
        return String(action || '').replaceAll('.', ' / ').replaceAll('_', ' ')
    }

    function initials(username) {
        return String(username || '?').slice(0, 2).toUpperCase()
    }

    function errorMessage(error, fallback) {
        return error && error.message ? error.message : fallback
    }

    async function apiRequest(path, options = {}) {
        const headers = { Accept: 'application/json' }
        if (options.body !== undefined) headers['Content-Type'] = 'application/json'
        const response = await fetch(endpoint + '?path=' + encodeURIComponent(path), {
            method: options.method || 'GET',
            headers,
            credentials: 'same-origin',
            body: options.body === undefined ? undefined : JSON.stringify(options.body),
        })
        const text = await response.text()
        let payload = null
        try { payload = text ? JSON.parse(text) : null } catch (_) { payload = null }
        if (!response.ok) {
            const error = new Error(payload?.message || 'The request could not be completed.')
            error.status = response.status
            throw error
        }
        if (Array.isArray(payload)) {
            payload.pagination = {
                hasMore: response.headers.get('X-Pagination-Has-More') === 'true',
                page: Number(response.headers.get('X-Pagination-Page') || 1),
            }
        }
        return payload
    }

    function messageMarkup() {
        if (state.error) return '<div class="admin-message error" role="alert">' + escapeHtml(state.error) + '</div>'
        if (state.notice) return '<div class="admin-message success" role="status">' + escapeHtml(state.notice) + '</div>'
        return ''
    }

    function renderLogin() {
        document.body.classList.add('admin-locked')
        document.getElementById('admin-user').textContent = 'Authentication required'
        document.getElementById('admin-view-title').textContent = 'Sign in'
        document.querySelectorAll('[data-view]').forEach((button) => { button.classList.remove('active') })
        content.innerHTML = '<section class="admin-auth admin-panel">' +
            '<h1>Admin sign in</h1>' +
            '<p>Use an Uno Ranked account with the admin role. Access is verified by the API before any administrative data is returned.</p>' +
            messageMarkup() +
            '<form data-login-form>' +
            '<label class="admin-field"><span>Email</span><input class="admin-input" name="email" type="email" autocomplete="username" required></label>' +
            '<label class="admin-field"><span>Password</span><input class="admin-input" name="password" type="password" autocomplete="current-password" required></label>' +
            '<button class="admin-button" type="submit">Sign in</button>' +
            '</form>' +
            '</section>'
    }

    function updateShell() {
        document.body.classList.remove('admin-locked')
        document.getElementById('admin-user').textContent = state.me ? state.me.username + ' · ' + roleLabel(state.me.role) : 'Loading'
        document.getElementById('admin-view-title').textContent = viewTitles[state.view] || 'Admin'
        document.querySelectorAll('[data-view]').forEach((button) => {
            const activeView = state.view === 'user-detail' ? 'users' : state.view
            button.classList.toggle('active', button.dataset.view === activeView)
        })
    }

    function render() {
        if (!state.authenticated) {
            renderLogin()
            return
        }
        updateShell()
        if (state.loading) {
            content.innerHTML = '<div class="admin-loading">Loading administration data…</div>'
            return
        }
        const view = renderView()
        content.innerHTML = messageMarkup() + view
        drawCharts()
    }

    function renderPageHeader(title, subtitle, action = '') {
        return '<header class="admin-page-header"><div><h1>' + escapeHtml(title) + '</h1>' +
            (subtitle ? '<p>' + escapeHtml(subtitle) + '</p>' : '') +
            '</div>' + action + '</header>'
    }

    function renderKpi(label, value, detail = '') {
        return '<article class="admin-kpi"><span>' + escapeHtml(label) + '</span><strong>' + escapeHtml(number(value)) + '</strong>' +
            (detail ? '<small>' + escapeHtml(detail) + '</small>' : '') + '</article>'
    }

    function renderMetric(label, value) {
        return '<div class="admin-metric"><strong>' + escapeHtml(number(value)) + '</strong><span>' + escapeHtml(label) + '</span></div>'
    }

    function renderDistribution(items, green = false) {
        if (!items || items.length === 0) return '<div class="admin-empty"><strong>No data</strong>Nothing has been recorded yet.</div>'
        const max = Math.max(...items.map((item) => Number(item.value || 0)), 1)
        return '<div class="admin-stat-list">' + items.map((item) => '<div class="admin-stat-row"><div class="admin-stat-row-header"><span>' + escapeHtml(item.label) + '</span><strong>' + escapeHtml(number(item.value)) + '</strong></div><div class="admin-progress ' + (green ? 'green' : '') + '"><span style="width:' + Math.max(2, Number(item.value || 0) / max * 100) + '%"></span></div></div>').join('') + '</div>'
    }

    function chartPanel(title, subtitle, chartName, legend) {
        return '<section class="admin-panel"><div class="admin-panel-header"><div><h2>' + escapeHtml(title) + '</h2><p>' + escapeHtml(subtitle) + '</p></div><small>Hover points</small></div><div class="admin-chart-wrap"><canvas class="admin-chart" data-chart="' + chartName + '" aria-label="' + escapeHtml(title) + '"></canvas><div class="admin-chart-tooltip" data-chart-tooltip hidden></div></div><div class="admin-legend">' + legend.map((item) => '<span style="--legend:' + item.color + '">' + escapeHtml(item.label) + '</span>').join('') + '</div></section>'
    }

    function renderOverview() {
        const dashboard = state.dashboard
        const overview = dashboard?.overview || {}
        return renderPageHeader('Overview', 'Platform activity and current operating figures.') +
            '<div class="admin-kpi-grid">' +
            renderKpi('Users', overview.totalUsers, number(overview.activeUsers) + ' active recently') +
            renderKpi('Average ELO', overview.averageElo, 'Highest ' + number(overview.highestElo)) +
            renderKpi('Recorded games', overview.recordedGames, number(overview.totalGames) + ' total tables') +
            renderKpi('Requests today', dashboard?.logsToday || 0, number(overview.adminUsers) + ' admin accounts') +
            '</div>' +
            '<div class="admin-grid">' +
            chartPanel('Request activity', 'Requests, registrations and logins over the selected window.', 'requests', [
                { label: 'Requests', color: '#d4a45c' }, { label: 'Accounts', color: '#76b89a' }, { label: 'Logins', color: '#8faeb5' },
            ]) +
            '<section class="admin-panel"><div class="admin-panel-header"><div><h2>Game state</h2><p>Tables currently recorded by the API.</p></div></div><div class="admin-metric-strip">' +
            renderMetric('Waiting', overview.waitingGames) + renderMetric('In progress', overview.activeGames) + renderMetric('Finished', overview.finishedGames) + renderMetric('Admins', overview.adminUsers) +
            '</div><div style="height:22px"></div><h2 style="font-size:13px;margin:0 0 14px">ELO distribution</h2>' + renderDistribution(dashboard?.eloDistribution) + '</section>' +
            '</div>' +
            '<div class="admin-grid-three"><section class="admin-panel"><div class="admin-panel-header"><div><h2>Modes</h2><p>Completed tables by mode.</p></div></div>' + renderDistribution(dashboard?.modeDistribution, true) + '</section>' +
            '<section class="admin-panel" style="grid-column: span 2"><div class="admin-panel-header"><div><h2>Recent audit events</h2><p>Latest events recorded by the API.</p></div><button class="admin-button admin-button-secondary" data-view="logs" type="button">Open logs</button></div>' + renderLogsTable((dashboard?.recentLogs || []).slice(0, 8), true) + '</section></div>'
    }

    function renderActivityTable(points) {
        if (!points || points.length === 0) return '<div class="admin-empty"><strong>No activity</strong>Daily values will appear when the platform receives traffic.</div>'
        return '<div class="admin-table-wrap"><table class="admin-table"><thead><tr><th>Date</th><th>Requests</th><th>Accounts</th><th>Logins</th><th>Games</th><th>Admin</th><th>Security</th></tr></thead><tbody>' + points.map((point) => '<tr><td><strong>' + escapeHtml(day(point.date)) + '</strong></td><td>' + number(point.totalRequests) + '</td><td>' + number(point.registrations) + '</td><td>' + number(point.logins) + '</td><td>' + number(point.gameActions) + '</td><td>' + number(point.adminActions) + '</td><td>' + number(point.securityEvents) + '</td></tr>').join('') + '</tbody></table></div>'
    }

    function renderRequests() {
        const points = state.dashboard?.activity || []
        return renderPageHeader('Requests', 'Traffic volume and API request categories.') +
            chartPanel('Request activity', 'Move across the chart to inspect each day.', 'requests', [
                { label: 'Requests', color: '#d4a45c' }, { label: 'Games', color: '#76b89a' }, { label: 'Security', color: '#cf8b7e' },
            ]) +
            '<section class="admin-panel"><div class="admin-panel-header"><div><h2>Daily request table</h2></div><small>' + number(points.length) + ' days</small></div>' + renderActivityTable(points) + '</section>'
    }

    function renderUserActivity() {
        const points = state.dashboard?.activity || []
        const accounts = points.reduce((total, point) => total + Number(point.registrations || 0), 0)
        const logins = points.reduce((total, point) => total + Number(point.logins || 0), 0)
        const active = new Set(points.filter((point) => Number(point.registrations || 0) + Number(point.logins || 0) > 0).map((point) => point.date)).size
        return renderPageHeader('User activity', 'Account creation and authentication activity over time.') +
            '<div class="admin-kpi-grid">' + renderKpi('New accounts', accounts) + renderKpi('Logins', logins) + renderKpi('Active days', active) + renderKpi('Users', state.dashboard?.overview?.totalUsers || 0) + '</div>' +
            chartPanel('User activity', 'Compare registrations, logins and game actions per day.', 'user-activity', [
                { label: 'Accounts', color: '#76b89a' }, { label: 'Logins', color: '#8faeb5' }, { label: 'Games', color: '#d4a45c' },
            ]) +
            '<section class="admin-panel"><div class="admin-panel-header"><div><h2>Daily user activity</h2></div></div>' + renderActivityTable(points) + '</section>'
    }

    function renderGames() {
        const overview = state.dashboard?.overview || {}
        return renderPageHeader('Games', 'Operational game counts. A record is created after matchmaking or custom table creation.') +
            '<div class="admin-kpi-grid">' + renderKpi('Total tables', overview.totalGames) + renderKpi('Waiting', overview.waitingGames) + renderKpi('In progress', overview.activeGames) + renderKpi('Finished', overview.finishedGames) + '</div>' +
            '<div class="admin-grid-three"><section class="admin-panel"><div class="admin-panel-header"><div><h2>Mode distribution</h2></div></div>' + renderDistribution(state.dashboard?.modeDistribution, true) + '</section><section class="admin-panel"><div class="admin-panel-header"><div><h2>Table pipeline</h2></div></div><div class="admin-metric-strip">' + renderMetric('Waiting', overview.waitingGames) + renderMetric('Playing', overview.activeGames) + renderMetric('Finished', overview.finishedGames) + renderMetric('Recorded', overview.recordedGames) + '</div></section><section class="admin-panel"><div class="admin-panel-header"><div><h2>Rating distribution</h2></div></div>' + renderDistribution(state.dashboard?.eloDistribution) + '</section></div>'
    }

    function filteredUsers() {
        const query = state.userSearch.trim().toLowerCase()
        return state.users.filter((user) => {
            const matchesQuery = !query || (user.username + ' ' + user.email + ' ' + user.id).toLowerCase().includes(query)
            const matchesRole = state.userRole === 'all' || user.role === state.userRole
            return matchesQuery && matchesRole
        })
    }

    function renderUsers() {
        const users = filteredUsers()
        return renderPageHeader('Users', 'Search an account, then open its profile for the complete history and controls.') +
            '<section class="admin-panel"><form class="admin-toolbar" data-users-filter><input class="admin-input" name="search" value="' + escapeHtml(state.userSearch) + '" placeholder="Search username, email or ID"><select class="admin-select" name="role"><option value="all">All roles</option>' + roles.map((role) => '<option value="' + role + '" ' + (state.userRole === role ? 'selected' : '') + '>' + roleLabel(role) + '</option>').join('') + '</select><button class="admin-button admin-button-secondary" type="submit">Filter</button></form>' +
            '<div class="admin-table-wrap"><table class="admin-table"><thead><tr><th>Manage</th><th>User</th><th>Email</th><th>ELO</th><th>Record</th><th>Role</th><th>Joined</th></tr></thead><tbody>' +
            (users.length ? users.map((user) => '<tr><td><button class="admin-button admin-button-secondary" data-action="manage-user" data-user-id="' + escapeHtml(user.id) + '" type="button">Manage</button></td><td><div class="admin-user-cell"><span class="admin-avatar">' + escapeHtml(initials(user.username)) + '</span><span><strong>' + escapeHtml(user.username) + '</strong><small>' + escapeHtml(user.id.slice(0, 12)) + '</small></span></div></td><td>' + escapeHtml(user.email) + '</td><td><strong>' + number(user.elo) + '</strong></td><td>' + number(user.gamesWon) + ' / ' + number(user.gamesPlayed) + '</td><td>' + escapeHtml(roleLabel(user.role)) + '</td><td>' + escapeHtml(date(user.createdAt)) + '</td></tr>').join('') : '<tr><td colspan="7"><div class="admin-empty"><strong>No users found</strong>Try another search or role.</div></td></tr>') +
            '</tbody></table></div>' + (state.usersHasMore ? '<div class="admin-load-more"><button class="admin-button admin-button-secondary" data-action="load-users-more" type="button" ' + (state.paginationLoading ? 'disabled' : '') + '>' + (state.paginationLoading ? 'Loading…' : 'Load more users') + '</button></div>' : '') + '</section>'
    }

    function renderUserDetail() {
        const user = state.selectedUser
        const analytics = state.selectedAnalytics
        if (!user) return renderUsers()
        const summary = analytics?.summary || {}
        const eloHistory = analytics?.eloHistory || []
        const activity = analytics?.activity || []
        return renderPageHeader('User profile', 'Manage this account and inspect every recorded rating event.', '<button class="admin-button admin-button-secondary" data-action="back-users" type="button">Back to users</button>') +
            '<div class="admin-user-detail"><section class="admin-panel"><div class="admin-profile-heading"><span class="admin-avatar">' + escapeHtml(initials(user.username)) + '</span><div><h2>' + escapeHtml(user.username) + '</h2><p>' + escapeHtml(user.email) + ' · ' + escapeHtml(roleLabel(user.role)) + '</p></div></div><div class="admin-detail-meta"><div><strong>' + number(user.elo) + '</strong><span>Current ELO</span></div><div><strong>' + number(user.gamesPlayed) + '</strong><span>Games played</span></div><div><strong>' + number(user.gamesWon) + '</strong><span>Games won</span></div></div><div style="height:24px"></div><div class="admin-analytics-grid">' +
            analyticsMetric('Average ELO', summary.averageElo) + analyticsMetric('Peak ELO', summary.peakElo) + analyticsMetric('Lowest ELO', summary.lowestElo) + analyticsMetric('Net ELO Δ', summary.netEloDelta, summary.netEloDelta >= 0 ? 'admin-positive' : 'admin-negative') +
            analyticsMetric('Win rate', summary.winRate == null ? '—' : Math.round(Number(summary.winRate) * 100) + '%') + analyticsMetric('Volatility', summary.volatility) + analyticsMetric('Rank', summary.rank ? '#' + summary.rank : '—') + analyticsMetric('Active days', activity.filter((point) => Number(point.games || 0) > 0).length) +
            '</div></section><section class="admin-panel admin-user-actions"><div class="admin-panel-header"><div><h2>Account controls</h2><p>Changes are audited by the API.</p></div></div><form class="admin-field" data-elo-form data-user-id="' + escapeHtml(user.id) + '"><span>ELO</span><input id="user-elo" class="admin-input" name="elo" type="number" min="0" max="5000" value="' + escapeHtml(user.elo) + '"><button class="admin-button" type="submit" ' + (state.busy ? 'disabled' : '') + '>Save ELO</button></form><form class="admin-field" data-role-form data-user-id="' + escapeHtml(user.id) + '"><span>Role</span><select class="admin-select" name="role" ' + (user.id === state.me?.id ? 'disabled' : '') + '>' + roles.map((role) => '<option value="' + role + '" ' + (user.role === role ? 'selected' : '') + '>' + roleLabel(role) + '</option>').join('') + '</select><button class="admin-button admin-button-secondary" type="submit" ' + (state.busy || user.id === state.me?.id ? 'disabled' : '') + '>Save role</button></form><hr><button class="admin-button admin-button-danger" data-action="delete-user" data-user-id="' + escapeHtml(user.id) + '" type="button" ' + (state.busy || user.id === state.me?.id ? 'disabled' : '') + '>Delete account</button></section></div>' +
            (eloHistory.length ? '<section class="admin-panel"><div class="admin-panel-header"><div><h2>ELO history</h2><p>Every rating step, including administrative adjustments.</p></div><small>' + number(eloHistory.length) + ' events</small></div><div class="admin-chart-wrap"><canvas class="admin-chart" data-chart="elo" aria-label="ELO history"></canvas><div class="admin-chart-tooltip" data-chart-tooltip hidden></div></div><div class="admin-table-wrap"><table class="admin-table"><thead><tr><th>Date</th><th>Reason</th><th>Before</th><th>After</th><th>Change</th><th>Game</th></tr></thead><tbody>' + eloHistory.slice().reverse().map((point) => '<tr><td>' + escapeHtml(date(point.createdAt)) + '</td><td>' + escapeHtml(String(point.reason || '').replaceAll('_', ' ')) + '</td><td>' + number(point.previousElo) + '</td><td><strong>' + number(point.newElo) + '</strong></td><td class="' + (Number(point.delta) >= 0 ? 'admin-positive' : 'admin-negative') + '">' + (Number(point.delta) > 0 ? '+' : '') + number(point.delta) + '</td><td><code>' + escapeHtml(point.gameId ? point.gameId.slice(0, 10) : '—') + '</code></td></tr>').join('') + '</tbody></table></div></section>' : '<section class="admin-panel"><div class="admin-empty"><strong>No ELO history</strong>Rating events will appear after ranked matches or an audited adjustment.</div></section>') +
            '<section class="admin-panel"><div class="admin-panel-header"><div><h2>Activity history</h2><p>Games, wins, score and ELO delta grouped by day.</p></div></div>' + renderPlayerActivityTable(activity) + '</section>'
    }

    function analyticsMetric(label, value, className = '') {
        const text = typeof value === 'number' ? number(value) : String(value ?? '—')
        return '<div class="admin-analytics-metric"><strong class="' + className + '">' + escapeHtml(text) + '</strong><span>' + escapeHtml(label) + '</span></div>'
    }

    function renderPlayerActivityTable(points) {
        if (!points.length) return '<div class="admin-empty"><strong>No activity history</strong>Finished games will appear here.</div>'
        return '<div class="admin-table-wrap"><table class="admin-table"><thead><tr><th>Date</th><th>Games</th><th>Wins</th><th>Losses</th><th>Score</th><th>ELO Δ</th></tr></thead><tbody>' + points.slice().reverse().map((point) => '<tr><td>' + escapeHtml(day(point.date)) + '</td><td>' + number(point.games) + '</td><td>' + number(point.wins) + '</td><td>' + number(point.losses) + '</td><td>' + number(point.score) + '</td><td>' + (Number(point.eloDelta) > 0 ? '+' : '') + number(point.eloDelta) + '</td></tr>').join('') + '</tbody></table></div>'
    }

    function renderLogsTable(logs, compact = false) {
        if (!logs || logs.length === 0) return '<div class="admin-empty"><strong>No logs found</strong>The audit stream is empty for the current filters.</div>'
        return '<div class="admin-table-wrap"><table class="admin-table"><thead><tr><th>Time</th><th>Event</th><th>Actor</th><th>Target</th><th>HTTP</th><th>Details</th></tr></thead><tbody>' + logs.slice(0, compact ? 8 : 250).map((log) => '<tr><td>' + escapeHtml(date(log.createdAt)) + '</td><td><strong>' + escapeHtml(actionLabel(log.action)) + '</strong></td><td>' + (log.actorUsername ? '<button class="admin-link-button" data-action="actor-filter" data-actor="' + escapeHtml(log.actorUsername) + '" type="button">' + escapeHtml(log.actorUsername) + '</button>' : '<span class="admin-muted">System</span>') + '</td><td>' + escapeHtml(log.targetType || '—') + (log.targetId ? ' / ' + escapeHtml(log.targetId.slice(0, 10)) : '') + '</td><td>' + escapeHtml(log.statusCode ?? '—') + '</td><td class="admin-log-details"><details><summary>' + escapeHtml(log.details || 'No details') + '</summary><div>Target ID: ' + escapeHtml(log.targetId || '—') + '<br>IP: ' + escapeHtml(log.ipAddress || '—') + '<br>User agent: ' + escapeHtml(log.userAgent || '—') + '</div></details></td></tr>').join('') + '</tbody></table></div>'
    }

    function renderLogs() {
        const stats = state.logStats || {}
        return renderPageHeader('Audit logs', 'Filter request, authentication, game and administrative events.') +
            '<section class="admin-panel"><form class="admin-toolbar" data-logs-form><select class="admin-select" name="days"><option value="1" ' + (state.logFilters.days === '1' ? 'selected' : '') + '>24 hours</option><option value="7" ' + (state.logFilters.days === '7' ? 'selected' : '') + '>7 days</option><option value="14" ' + (state.logFilters.days === '14' ? 'selected' : '') + '>14 days</option><option value="30" ' + (state.logFilters.days === '30' ? 'selected' : '') + '>30 days</option><option value="90" ' + (state.logFilters.days === '90' ? 'selected' : '') + '>90 days</option></select><select class="admin-select" name="category"><option value="all" ' + (state.logFilters.category === 'all' ? 'selected' : '') + '>All categories</option><option value="requests" ' + (state.logFilters.category === 'requests' ? 'selected' : '') + '>Requests</option><option value="security" ' + (state.logFilters.category === 'security' ? 'selected' : '') + '>Security</option><option value="admin" ' + (state.logFilters.category === 'admin' ? 'selected' : '') + '>Admin</option><option value="auth" ' + (state.logFilters.category === 'auth' ? 'selected' : '') + '>Authentication</option><option value="games" ' + (state.logFilters.category === 'games' ? 'selected' : '') + '>Games</option></select><select class="admin-select" name="status"><option value="all" ' + (state.logFilters.status === 'all' ? 'selected' : '') + '>All HTTP status</option><option value="success" ' + (state.logFilters.status === 'success' ? 'selected' : '') + '>Success</option><option value="client_error" ' + (state.logFilters.status === 'client_error' ? 'selected' : '') + '>Client errors</option><option value="server_error" ' + (state.logFilters.status === 'server_error' ? 'selected' : '') + '>Server errors</option></select><input class="admin-input" name="actor" value="' + escapeHtml(state.logFilters.actor) + '" placeholder="Actor"><input class="admin-input" name="search" value="' + escapeHtml(state.logFilters.search) + '" placeholder="Search event, target or IP"><button class="admin-button" type="submit">Apply</button><button class="admin-button admin-button-secondary" data-action="export-logs" type="button">Export JSON</button></form><div class="admin-metric-strip">' + renderMetric('Events', stats.totalEvents) + renderMetric('Requests', stats.totalRequests) + renderMetric('Errors', Number(stats.clientErrors || 0) + Number(stats.serverErrors || 0)) + renderMetric('Security', stats.securityEvents) + '</div></section>' +
            '<section class="admin-panel"><div class="admin-panel-header"><div><h2>Event stream</h2></div><small>' + number(state.logs.length) + ' results</small></div>' + (state.logsLoading ? '<div class="admin-loading">Loading logs…</div>' : renderLogsTable(state.logs) + (state.logsHasMore ? '<div class="admin-load-more"><button class="admin-button admin-button-secondary" data-action="load-logs-more" type="button" ' + (state.paginationLoading ? 'disabled' : '') + '>' + (state.paginationLoading ? 'Loading…' : 'Load more logs') + '</button></div>' : '')) + '</section>'
    }

    function renderActors() {
        const stats = state.logStats || {}
        const actors = stats.actors || []
        return renderPageHeader('Actors', 'Request volume grouped by the account or system actor that emitted it.') +
            '<div class="admin-kpi-grid">' + renderKpi('Active actors', stats.activeActors) + renderKpi('Requests', stats.totalRequests) + renderKpi('Errors', Number(stats.clientErrors || 0) + Number(stats.serverErrors || 0)) + renderKpi('Security events', stats.securityEvents) + '</div>' +
            '<section class="admin-panel"><div class="admin-table-wrap"><table class="admin-table"><thead><tr><th>Actor</th><th>Events</th><th>Requests</th><th>Success</th><th>Errors</th><th>Security</th><th>Last seen</th></tr></thead><tbody>' + (actors.length ? actors.map((actor) => '<tr><td><button class="admin-link-button" data-action="actor-filter" data-actor="' + escapeHtml(actor.username) + '" type="button">' + escapeHtml(actor.username) + '</button></td><td>' + number(actor.totalEvents) + '</td><td>' + number(actor.requests) + '</td><td>' + number(actor.successfulRequests) + '</td><td>' + number(actor.failedRequests) + '</td><td>' + number(actor.securityEvents) + '</td><td>' + escapeHtml(date(actor.lastSeen)) + '</td></tr>').join('') : '<tr><td colspan="7"><div class="admin-empty"><strong>No actor data</strong>There are no matching audit events.</div></td></tr>') + '</tbody></table></div></section>'
    }

    function renderView() {
        if (state.view === 'overview') return renderOverview()
        if (state.view === 'requests') return renderRequests()
        if (state.view === 'user-activity') return renderUserActivity()
        if (state.view === 'games') return renderGames()
        if (state.view === 'users') return renderUsers()
        if (state.view === 'user-detail') return renderUserDetail()
        if (state.view === 'logs') return renderLogs()
        if (state.view === 'actors') return renderActors()
        return renderOverview()
    }

    function chartConfig(name) {
        const points = state.dashboard?.activity || []
        if (name === 'requests') return { points, series: [{ key: 'totalRequests', label: 'Requests', color: '#d4a45c' }, { key: 'gameActions', label: 'Games', color: '#76b89a' }, { key: 'securityEvents', label: 'Security', color: '#cf8b7e' }] }
        if (name === 'user-activity') return { points, series: [{ key: 'registrations', label: 'Accounts', color: '#76b89a' }, { key: 'logins', label: 'Logins', color: '#8faeb5' }, { key: 'gameActions', label: 'Games', color: '#d4a45c' }] }
        const history = state.selectedAnalytics?.eloHistory || []
        return { points: history.map((point) => ({ date: point.createdAt, previousElo: point.previousElo, newElo: point.newElo })), series: [{ key: 'previousElo', label: 'Before', color: '#8faeb5' }, { key: 'newElo', label: 'After', color: '#d4a45c' }] }
    }

    function drawCharts() {
        document.querySelectorAll('canvas[data-chart]').forEach((canvas) => {
            const config = chartConfig(canvas.dataset.chart)
            const context = canvas.getContext('2d')
            if (!context) return
            const rect = canvas.getBoundingClientRect()
            const ratio = window.devicePixelRatio || 1
            const width = Math.max(280, rect.width)
            const height = 270
            canvas.width = width * ratio
            canvas.height = height * ratio
            context.setTransform(ratio, 0, 0, ratio, 0, 0)
            context.clearRect(0, 0, width, height)
            const left = 12
            const right = width - 12
            const top = 18
            const bottom = height - 28
            context.strokeStyle = '#2a2f34'
            context.lineWidth = 1
            for (let index = 0; index < 4; index += 1) {
                const y = top + (bottom - top) * index / 3
                context.beginPath(); context.moveTo(left, y); context.lineTo(right, y); context.stroke()
            }
            const values = config.points.flatMap((point) => config.series.map((series) => Number(point[series.key] || 0)))
            const max = Math.max(...values, 1)
            const positions = config.points.map((point, index) => ({ x: config.points.length <= 1 ? (left + right) / 2 : left + (right - left) * index / (config.points.length - 1), point }))
            config.series.forEach((series) => {
                context.strokeStyle = series.color
                context.lineWidth = 2
                context.beginPath()
                positions.forEach((position, index) => {
                    const value = Number(position.point[series.key] || 0)
                    const y = bottom - value / max * (bottom - top)
                    if (index === 0) context.moveTo(position.x, y)
                    else context.lineTo(position.x, y)
                })
                if (positions.length) context.stroke()
                positions.forEach((position) => {
                    const value = Number(position.point[series.key] || 0)
                    const y = bottom - value / max * (bottom - top)
                    context.fillStyle = series.color
                    context.beginPath(); context.arc(position.x, y, 3, 0, Math.PI * 2); context.fill()
                })
            })
            canvas._adminChart = { config, positions, left, right }
            if (!canvas._adminChartBound) {
                canvas._adminChartBound = true
                canvas.addEventListener('mousemove', chartMove)
                canvas.addEventListener('mouseleave', chartLeave)
            }
            if (!config.points.length) {
                context.fillStyle = '#8e969b'
                context.font = '12px system-ui'
                context.textAlign = 'center'
                context.fillText('No data for this period', width / 2, height / 2)
            }
        })
    }

    function chartMove(event) {
        const canvas = event.currentTarget
        const chart = canvas._adminChart
        if (!chart || !chart.positions.length) return
        const bounds = canvas.getBoundingClientRect()
        const ratio = Math.max(0, Math.min(1, (event.clientX - bounds.left) / bounds.width))
        const index = chart.positions.length <= 1 ? 0 : Math.round(ratio * (chart.positions.length - 1))
        const point = chart.positions[index].point
        const tooltip = canvas.parentElement.querySelector('[data-chart-tooltip]')
        tooltip.hidden = false
        tooltip.style.left = Math.min(82, Math.max(3, ratio * 100)) + '%'
        tooltip.style.top = '10px'
        tooltip.innerHTML = '<strong>' + escapeHtml(day(point.date)) + '</strong>' + chart.config.series.map((series) => '<span>' + escapeHtml(series.label) + ': ' + escapeHtml(number(point[series.key])) + '</span>').join('')
    }

    function chartLeave(event) {
        const tooltip = event.currentTarget.parentElement.querySelector('[data-chart-tooltip]')
        tooltip.hidden = true
    }

    async function loadDashboardData() {
        state.dashboard = await apiRequest('/admin/dashboard?days=14&logs=12')
    }

    async function loadUsersData(reset = true) {
        const page = reset ? 1 : state.usersPage + 1
        const query = new URLSearchParams({ limit: '50', page: String(page), q: state.userSearch.trim(), role: state.userRole })
        const users = await apiRequest('/admin/users?' + query.toString())
        if (reset) state.users = users || []
        else state.users = state.users.concat(users || [])
        state.usersPage = page
        state.usersHasMore = Boolean(users?.pagination?.hasMore)
    }

    async function loadLogsData(reset = true) {
        state.logsLoading = true
        render()
        const page = reset ? 1 : state.logsPage + 1
        const query = new URLSearchParams({ limit: '50', page: String(page), days: state.logFilters.days, category: state.logFilters.category, status: state.logFilters.status })
        if (state.logFilters.actor.trim()) query.set('actor', state.logFilters.actor.trim())
        if (state.logFilters.search.trim()) query.set('search', state.logFilters.search.trim())
        try {
            const requests = [apiRequest('/admin/logs?' + query.toString())]
            if (reset) requests.push(apiRequest('/admin/logs/stats?' + new URLSearchParams({ days: state.logFilters.days, category: state.logFilters.category, status: state.logFilters.status, actor: state.logFilters.actor.trim(), search: state.logFilters.search.trim() }).toString()))
            const [logs, stats] = await Promise.all(requests)
            state.logs = reset ? (logs || []) : state.logs.concat(logs || [])
            state.logsPage = page
            state.logsHasMore = Boolean(logs?.pagination?.hasMore)
            if (reset) state.logStats = stats || null
            state.error = ''
        } catch (error) {
            state.error = errorMessage(error, 'Could not load audit logs.')
        } finally {
            state.logsLoading = false
            render()
        }
    }

    async function loadSelectedAnalytics() {
        if (!state.selectedUser) return
        state.selectedAnalytics = null
        render()
        try {
            state.selectedAnalytics = await apiRequest('/admin/users/' + encodeURIComponent(state.selectedUser.id) + '/analytics')
            state.error = ''
        } catch (error) {
            state.error = errorMessage(error, 'Could not load this user analytics.')
        }
        render()
    }

    async function boot() {
        if (!state.authenticated) {
            render()
            return
        }
        state.loading = true
        state.error = ''
        render()
        try {
            const [me, dashboard] = await Promise.all([apiRequest('/auth/me'), apiRequest('/admin/dashboard?days=14&logs=12')])
            if (me.role !== 'admin') throw new Error('The admin role is required for this workspace.')
            state.me = me
            state.dashboard = dashboard
            await loadUsersData()
        } catch (error) {
            if (error.status === 401 || error.status === 403) {
                state.authenticated = false
                state.me = null
            }
            state.error = errorMessage(error, 'Could not load the admin workspace.')
        } finally {
            state.loading = false
            render()
        }
    }

    async function showView(view) {
        state.view = view
        state.error = ''
        state.notice = ''
        render()
        if (view === 'logs' || view === 'actors') await loadLogsData()
    }

    async function openUser(id) {
        state.selectedUser = state.users.find((user) => user.id === id) || null
        if (!state.selectedUser) return
        state.view = 'user-detail'
        state.notice = ''
        state.error = ''
        await loadSelectedAnalytics()
    }

    async function refresh() {
        if (!state.authenticated || state.busy) return
        state.loading = true
        state.error = ''
        render()
        try {
            const requests = [loadDashboardData(), loadUsersData()]
            if (state.view === 'logs' || state.view === 'actors') requests.push(loadLogsData())
            if (state.view === 'user-detail') requests.push(loadSelectedAnalytics())
            await Promise.all(requests)
            state.notice = 'Data refreshed.'
        } catch (error) {
            state.error = errorMessage(error, 'Could not refresh the workspace.')
        } finally {
            state.loading = false
            render()
        }
    }

    async function saveElo(form) {
        const id = form.dataset.userId
        const elo = Number(new FormData(form).get('elo'))
        if (!Number.isInteger(elo) || elo < 0 || elo > 5000) {
            state.error = 'ELO must be an integer between 0 and 5000.'
            render()
            return
        }
        state.busy = true; state.error = ''; state.notice = ''; render()
        try {
            await apiRequest('/admin/users/' + encodeURIComponent(id) + '/elo', { method: 'PUT', body: { elo } })
            await loadDashboardData(); await loadUsersData()
            state.selectedUser = state.users.find((user) => user.id === id) || state.selectedUser
            await loadSelectedAnalytics()
            state.notice = 'ELO updated and reflected in the profile.'
        } catch (error) {
            state.error = errorMessage(error, 'Could not update this ELO.')
        } finally {
            state.busy = false
            render()
        }
    }

    async function saveRole(form) {
        const id = form.dataset.userId
        const role = new FormData(form).get('role')
        state.busy = true; state.error = ''; state.notice = ''; render()
        try {
            await apiRequest('/admin/users/' + encodeURIComponent(id) + '/role', { method: 'PUT', body: { role } })
            await loadUsersData()
            state.selectedUser = state.users.find((user) => user.id === id) || state.selectedUser
            state.notice = 'Role updated.'
        } catch (error) {
            state.error = errorMessage(error, 'Could not update this role.')
        } finally {
            state.busy = false
            render()
        }
    }

    async function deleteUser(id) {
        if (!window.confirm('Delete this account and all linked data? This cannot be undone.')) return
        state.busy = true; state.error = ''; state.notice = ''; render()
        try {
            await apiRequest('/admin/users/' + encodeURIComponent(id), { method: 'DELETE' })
            await loadDashboardData(); await loadUsersData()
            state.selectedUser = null; state.selectedAnalytics = null; state.view = 'users'; state.notice = 'Account deleted.'
        } catch (error) {
            state.error = errorMessage(error, 'Could not delete this account.')
        } finally {
            state.busy = false
            render()
        }
    }

    function exportLogs() {
        const blob = new Blob([JSON.stringify(state.logs, null, 2)], { type: 'application/json' })
        const link = document.createElement('a')
        link.href = URL.createObjectURL(blob)
        link.download = 'uno-ranked-audit-logs.json'
        link.click()
        URL.revokeObjectURL(link.href)
    }

    document.addEventListener('click', async (event) => {
        const target = event.target instanceof Element ? event.target.closest('[data-view], [data-action]') : null
        if (!target) return
        const view = target.dataset.view
        const action = target.dataset.action
        if (view) {
            event.preventDefault()
            await showView(view)
            return
        }
        if (action === 'refresh') await refresh()
        if (action === 'logout') {
            try { await apiRequest('/auth/logout', { method: 'POST' }) } catch (_) { /* The session is cleared locally below. */ }
            state.authenticated = false
            state.me = null
            state.error = ''
            state.notice = ''
            render()
        }
        if (action === 'manage-user') await openUser(target.dataset.userId)
        if (action === 'load-users-more') {
            state.paginationLoading = true
            render()
            try { await loadUsersData(false) } catch (error) { state.error = errorMessage(error, 'Could not load more users.') }
            state.paginationLoading = false
            render()
        }
        if (action === 'load-logs-more') {
            state.paginationLoading = true
            render()
            try { await loadLogsData(false) } catch (error) { state.error = errorMessage(error, 'Could not load more logs.') }
            state.paginationLoading = false
            render()
        }
        if (action === 'back-users') { state.view = 'users'; state.selectedUser = null; state.selectedAnalytics = null; render() }
        if (action === 'delete-user') await deleteUser(target.dataset.userId)
        if (action === 'export-logs') exportLogs()
        if (action === 'actor-filter') { state.logFilters.actor = target.dataset.actor || ''; state.view = 'logs'; await loadLogsData() }
    })

    document.addEventListener('submit', async (event) => {
        const form = event.target
        if (!(form instanceof HTMLFormElement)) return
        event.preventDefault()
        if (form.matches('[data-login-form]')) {
            state.error = ''; state.notice = ''
            try {
                await apiRequest('/auth/login', { method: 'POST', body: { email: form.elements.email.value, password: form.elements.password.value } })
                state.authenticated = true
                await boot()
            } catch (error) {
                state.error = errorMessage(error, 'Invalid credentials.')
                render()
            }
        }
        if (form.matches('[data-users-filter]')) {
            const values = new FormData(form)
            state.userSearch = String(values.get('search') || '')
            state.userRole = String(values.get('role') || 'all')
            try { await loadUsersData() } catch (error) { state.error = errorMessage(error, 'Could not filter users.') }
            render()
        }
        if (form.matches('[data-logs-form]')) {
            const values = new FormData(form)
            state.logFilters.days = String(values.get('days') || '14')
            state.logFilters.category = String(values.get('category') || 'all')
            state.logFilters.status = String(values.get('status') || 'all')
            state.logFilters.actor = String(values.get('actor') || '')
            state.logFilters.search = String(values.get('search') || '')
            await loadLogsData()
        }
        if (form.matches('[data-elo-form]')) await saveElo(form)
        if (form.matches('[data-role-form]')) await saveRole(form)
    })

    boot()
}())
