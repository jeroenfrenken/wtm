// WebSocket connection
let ws = null;
let logs = [];
let logCount = 0;
const MAX_LOGS = 1000;

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    connectWebSocket();
    updateStatus();
    setInterval(updateStatus, 5000);
});

// WebSocket
function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(`${protocol}//${window.location.host}/ws`);

    ws.onopen = () => {
        console.log('WebSocket connected');
    };

    ws.onmessage = (event) => {
        const lines = event.data.split('\n');
        lines.forEach(line => {
            if (!line) return;
            try {
                const msg = JSON.parse(line);
                handleMessage(msg);
            } catch (e) {
                console.error('Failed to parse message:', e);
            }
        });
    };

    ws.onclose = () => {
        console.log('WebSocket disconnected, reconnecting...');
        setTimeout(connectWebSocket, 3000);
    };

    ws.onerror = (error) => {
        console.error('WebSocket error:', error);
    };
}

function handleMessage(msg) {
    switch (msg.type) {
        case 'log':
            addLog(msg.data);
            break;
        case 'status':
            updateServiceStatus(msg.data);
            break;
        case 'worktree_status':
            updateWorktreeStatus(msg.data);
            break;
        case 'event':
            handleEvent(msg.data);
            break;
    }
}

// Logs
function addLog(log) {
    logs.push(log);
    if (logs.length > MAX_LOGS) {
        logs.shift();
    }
    logCount++;
    renderLogs();
}

function renderLogs() {
    const container = document.getElementById('logs');
    const filter = document.getElementById('log-filter')?.value?.toLowerCase() || '';
    const serviceFilter = document.getElementById('log-service')?.value || '';

    // Filter logs
    let filtered = logs;
    if (filter) {
        filtered = filtered.filter(log =>
            log.message.toLowerCase().includes(filter) ||
            log.service.toLowerCase().includes(filter)
        );
    }
    if (serviceFilter) {
        filtered = filtered.filter(log => log.service === serviceFilter);
    }

    // Render last 100
    const toRender = filtered.slice(-100);

    if (toRender.length === 0) {
        container.innerHTML = '<p class="muted">No logs yet...</p>';
    } else {
        container.innerHTML = toRender.map(log => {
            const time = new Date(log.timestamp).toLocaleTimeString();
            const streamClass = log.stream === 'stderr' ? 'stderr' : '';
            return `
                <div class="log-line ${streamClass}">
                    <span class="log-service">${escapeHtml(log.service)}</span>
                    <span class="log-time">${time}</span>
                    <span class="log-message">${escapeHtml(log.message)}</span>
                </div>
            `;
        }).join('');
    }

    // Auto-scroll
    container.scrollTop = container.scrollHeight;

    // Update count
    document.querySelectorAll('.log-count').forEach(el => {
        el.textContent = logCount;
    });
}

function clearLogs() {
    logs = [];
    logCount = 0;
    renderLogs();
}

// Events
function handleEvent(event) {
    console.log('Event:', event);
    updateStatus();
}

// Status
async function updateStatus() {
    try {
        const response = await fetch('/api/status');
        const result = await response.json();
        if (result.success && result.data.worktrees) {
            updateWorktreeStatus(result.data.worktrees);
        }
    } catch (e) {
        console.error('Failed to fetch status:', e);
    }
}

function updateServiceStatus(services) {
    let runningCount = 0;
    let stoppedCount = 0;

    const domain = window.wtmConfig?.domain || 'local';
    const proxyPort = window.wtmConfig?.proxyPort || 8080;
    const selectedWT = window.wtmConfig?.selectedWT || 'main';

    Object.entries(services).forEach(([name, status]) => {
        const card = document.querySelector(`.service-card[data-service="${name}"]`);
        if (card) {
            // Update status dot
            const dot = card.querySelector('.status-dot');
            if (dot) {
                dot.setAttribute('data-status', status.health || status.state);
            }

            // Update card health attribute for top border color
            card.setAttribute('data-health', status.health || status.state);

            // Update port display
            const portEl = card.querySelector('.service-port');
            if (portEl && status.port) {
                portEl.textContent = `:${status.port}`;
            }

            // Update domain link
            const domainEl = card.querySelector('.service-domain');
            if (domainEl && status.state === 'running' && status.port) {
                const serviceDomain = `${name}.${selectedWT}.${domain}`;
                domainEl.innerHTML = `<a href="http://${serviceDomain}:${proxyPort}" target="_blank">${serviceDomain}</a>`;
            } else if (domainEl) {
                domainEl.innerHTML = '';
            }

            // Count running/stopped
            if (status.state === 'running') {
                runningCount++;
            } else {
                stoppedCount++;
            }
        }
    });

    // Update summary counts
    document.getElementById('running-count').textContent = runningCount;
    document.getElementById('stopped-count').textContent = stoppedCount;
}

// Worktree status (for tabs)
function updateWorktreeStatus(worktrees) {
    let totalRunning = 0;
    let totalStopped = 0;

    Object.entries(worktrees).forEach(([name, info]) => {
        // Update tab status dot
        const tabContainer = document.querySelector(`.worktree-tab-container[data-worktree="${name}"]`);
        if (tabContainer) {
            const statusDot = tabContainer.querySelector('.wt-status-dot');
            if (statusDot) {
                statusDot.setAttribute('data-status', info.status || 'stopped');
            }

            // Update toggle button icon
            const toggleBtn = tabContainer.querySelector('.wt-btn-toggle');
            if (toggleBtn) {
                toggleBtn.textContent = info.status === 'running' || info.status === 'partial' ? '■' : '▶';
            }
        }

        // Count services for summary
        if (info.services) {
            Object.values(info.services).forEach(svc => {
                if (svc.state === 'running') {
                    totalRunning++;
                } else {
                    totalStopped++;
                }
            });
        }

        // Update service cards if this is the selected worktree
        if (name === window.wtmConfig?.selectedWT && info.services) {
            updateServiceStatus(info.services);
        }
    });

    // Update summary counts
    document.getElementById('running-count').textContent = totalRunning;
    document.getElementById('stopped-count').textContent = totalStopped;
}

// Service actions
async function startService(name) {
    await serviceAction(name, 'start');
}

async function stopService(name) {
    await serviceAction(name, 'stop');
}

async function restartService(name) {
    await serviceAction(name, 'restart');
}

async function serviceAction(name, action) {
    try {
        const response = await fetch(`/api/services/${name}/${action}`, {
            method: 'POST'
        });
        const result = await response.json();
        if (!result.success) {
            showNotification(result.error || 'Action failed', 'error');
        } else {
            showNotification(`Service ${name} ${action}ed`, 'success');
        }
        updateStatus();
    } catch (e) {
        showNotification('Failed to perform action: ' + e.message, 'error');
    }
}

// Global service actions
async function startAllServices() {
    try {
        const response = await fetch('/api/services/all/start', { method: 'POST' });
        const result = await response.json();
        if (result.success) {
            showNotification('Starting all services...', 'success');
        } else {
            showNotification(result.error || 'Failed to start services', 'error');
        }
        updateStatus();
    } catch (e) {
        showNotification('Failed to start services: ' + e.message, 'error');
    }
}

async function stopAllServices() {
    try {
        const response = await fetch('/api/services/all/stop', { method: 'POST' });
        const result = await response.json();
        if (result.success) {
            showNotification('Stopping all services...', 'success');
        } else {
            showNotification(result.error || 'Failed to stop services', 'error');
        }
        updateStatus();
    } catch (e) {
        showNotification('Failed to stop services: ' + e.message, 'error');
    }
}

async function restartAllServices() {
    try {
        const response = await fetch('/api/services/all/restart', { method: 'POST' });
        const result = await response.json();
        if (result.success) {
            showNotification('Restarting all services...', 'success');
        } else {
            showNotification(result.error || 'Failed to restart services', 'error');
        }
        updateStatus();
    } catch (e) {
        showNotification('Failed to restart services: ' + e.message, 'error');
    }
}

async function syncHosts() {
    try {
        const response = await fetch('/api/hosts/sync', { method: 'POST' });
        const result = await response.json();
        if (result.success) {
            showNotification('Hosts synced successfully!', 'success');
        } else {
            showNotification(result.error || 'Failed to sync hosts', 'error');
        }
    } catch (e) {
        showNotification('Failed to sync hosts: ' + e.message, 'error');
    }
}

function viewLogs(name) {
    const select = document.getElementById('log-service');
    if (select) {
        select.value = name;
        renderLogs();
        // Scroll to logs
        document.querySelector('.logs-section')?.scrollIntoView({ behavior: 'smooth' });
    }
}

async function openTerminal(name) {
    try {
        const response = await fetch(`/api/terminal/${name}`, { method: 'POST' });
        const result = await response.json();
        if (result.success) {
            showNotification(`Opening terminal for ${name}...`, 'success');
        } else {
            showNotification(result.error || 'Failed to open terminal', 'error');
        }
    } catch (e) {
        showNotification('Failed to open terminal: ' + e.message, 'error');
    }
}

// Tasks
async function runTask(name) {
    showNotification(`Run "wtm task ${name}" in your terminal`, 'info');
}

// Worktrees
async function selectWorktree(name) {
    try {
        const response = await fetch(`/api/worktrees/${name}/switch`, { method: 'POST' });
        const result = await response.json();
        if (result.success) {
            // Update active tab styling
            document.querySelectorAll('.worktree-tab').forEach(tab => {
                tab.classList.remove('active');
                if (tab.dataset.worktree === name) {
                    tab.classList.add('active');
                }
            });
            // Update config
            if (window.wtmConfig) {
                window.wtmConfig.selectedWT = name;
            }
            // Reload page to update service cards
            window.location.reload();
        } else {
            showNotification(result.error || 'Failed to select worktree', 'error');
        }
    } catch (e) {
        showNotification('Failed to select worktree: ' + e.message, 'error');
    }
}

// Alias for backwards compatibility
async function switchWorktree(name) {
    await selectWorktree(name);
}

async function toggleWorktree(name) {
    try {
        // Get current status
        const statusResp = await fetch('/api/status');
        const statusResult = await statusResp.json();

        let isRunning = false;
        if (statusResult.success && statusResult.data.worktrees && statusResult.data.worktrees[name]) {
            const status = statusResult.data.worktrees[name].status;
            isRunning = status === 'running' || status === 'partial';
        }

        // Toggle based on current state
        const action = isRunning ? 'stop' : 'start';
        const response = await fetch(`/api/worktrees/${name}/${action}`, { method: 'POST' });
        const result = await response.json();

        if (result.success) {
            showNotification(`${action === 'start' ? 'Starting' : 'Stopping'} ${name}...`, 'success');
        } else {
            showNotification(result.error || `Failed to ${action} worktree`, 'error');
        }
    } catch (e) {
        showNotification('Failed to toggle worktree: ' + e.message, 'error');
    }
}

async function openWorktreeTerminal(name) {
    try {
        const response = await fetch(`/api/worktrees/${name}/terminal`, { method: 'POST' });
        const result = await response.json();
        if (result.success) {
            showNotification(`Opening terminal for ${name}...`, 'success');
        } else {
            showNotification(result.error || 'Failed to open terminal', 'error');
        }
    } catch (e) {
        showNotification('Failed to open terminal: ' + e.message, 'error');
    }
}

function showCreateWorktree() {
    document.getElementById('create-modal').classList.add('active');
    document.getElementById('wt-name').focus();
}

function hideModal() {
    document.getElementById('create-modal').classList.remove('active');
    document.getElementById('wt-name').value = '';
    document.getElementById('wt-branch').value = '';
}

async function createWorktree(event) {
    event.preventDefault();

    const name = document.getElementById('wt-name').value.trim();
    const branch = document.getElementById('wt-branch').value.trim();

    if (!name) {
        showNotification('Name is required', 'error');
        return;
    }

    try {
        const response = await fetch('/api/worktrees', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name, branch, syncHosts: true })
        });
        const result = await response.json();
        if (result.success) {
            hideModal();
            showNotification(`Worktree "${name}" created! Hosts synced.`, 'success');
            setTimeout(() => window.location.reload(), 1000);
        } else {
            showNotification(result.error || 'Failed to create worktree', 'error');
        }
    } catch (e) {
        showNotification('Failed to create worktree: ' + e.message, 'error');
    }
}

async function deleteWorktree(name) {
    if (!confirm(`Are you sure you want to delete worktree "${name}"?`)) {
        return;
    }

    try {
        const response = await fetch(`/api/worktrees/${name}`, {
            method: 'DELETE'
        });
        const result = await response.json();
        if (result.success) {
            showNotification(`Worktree "${name}" deleted`, 'success');
            setTimeout(() => { window.location.href = '/'; }, 1000);
        } else {
            showNotification(result.error || 'Failed to delete worktree', 'error');
        }
    } catch (e) {
        showNotification('Failed to delete worktree: ' + e.message, 'error');
    }
}

// Notifications
function showNotification(message, type = 'info') {
    // Remove existing notification
    const existing = document.querySelector('.notification');
    if (existing) existing.remove();

    const notification = document.createElement('div');
    notification.className = `notification notification-${type}`;
    notification.style.cssText = `
        position: fixed;
        bottom: 20px;
        right: 20px;
        padding: 12px 20px;
        background: var(--bg-secondary);
        border: 2px solid var(--border);
        border-radius: var(--radius);
        color: var(--text);
        font-family: inherit;
        font-weight: 700;
        z-index: 2000;
        animation: slideIn 0.3s ease;
        max-width: 400px;
    `;

    if (type === 'error') {
        notification.style.borderColor = 'var(--danger)';
        notification.style.color = 'var(--danger)';
    } else if (type === 'success') {
        notification.style.borderColor = 'var(--success)';
        notification.style.color = 'var(--success)';
    }

    notification.textContent = message;
    document.body.appendChild(notification);

    setTimeout(() => {
        notification.style.animation = 'slideOut 0.3s ease';
        setTimeout(() => notification.remove(), 300);
    }, 3000);
}

// Add animation keyframes
const style = document.createElement('style');
style.textContent = `
    @keyframes slideIn {
        from { transform: translateX(100%); opacity: 0; }
        to { transform: translateX(0); opacity: 1; }
    }
    @keyframes slideOut {
        from { transform: translateX(0); opacity: 1; }
        to { transform: translateX(100%); opacity: 0; }
    }
`;
document.head.appendChild(style);

// Utilities
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Filter logs on input
document.addEventListener('input', (e) => {
    if (e.target.id === 'log-filter' || e.target.id === 'log-service') {
        renderLogs();
    }
});

// Close modal on escape
document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
        hideModal();
    }
});

// Close modal on backdrop click
document.addEventListener('click', (e) => {
    if (e.target.classList.contains('modal')) {
        hideModal();
    }
});
