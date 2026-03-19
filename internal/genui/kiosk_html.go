package genui

// KioskHTML is the complete kiosk single-page application.
// Template variables ({{WS_URL}}, {{TITLE}}, etc.) are replaced at runtime by KioskHandler.
const KioskHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1, user-scalable=no">
<title>{{TITLE}}</title>
<style>
/* === Color System === */
:root {
  --bg-void: #050a12;
  --bg-glass: rgba(13, 17, 23, 0.72);
  --bg-glass-hover: rgba(13, 17, 23, 0.85);
  --bg-solid: #0d1117;
  --text-primary: #e6edf3;
  --text-dim: #6e7681;
  --text-secondary: #8b949e;
  --accent: #00d4aa;
  --accent-hover: #00f0c0;
  --accent-glow: rgba(0, 212, 170, 0.3);
  --accent-glow-strong: rgba(0, 212, 170, 0.6);
  --accent-alt: #7c3aed;
  --danger: #f85149;
  --border-glow: rgba(0, 212, 170, 0.15);
  --border-dim: rgba(48, 54, 61, 0.6);
  --stage-active: #00d4aa;
  --stage-done: #3fb950;
  --stage-pending: #30363d;
  --stage-error: #f85149;
  --sidebar-width: 280px;
}

/* Cyberpunk theme override */
.theme-cyberpunk {
  --accent: #ff2d6b;
  --accent-hover: #ff5a8a;
  --accent-glow: rgba(255, 45, 107, 0.3);
  --accent-glow-strong: rgba(255, 45, 107, 0.6);
  --border-glow: rgba(255, 45, 107, 0.15);
  --stage-active: #ff2d6b;
}

/* Clean theme override */
.theme-clean {
  --bg-void: #0d1117;
  --bg-glass: rgba(22, 27, 34, 0.95);
  --accent: #58a6ff;
  --accent-hover: #79c0ff;
  --accent-glow: rgba(88, 166, 255, 0.2);
  --accent-glow-strong: rgba(88, 166, 255, 0.4);
  --border-glow: rgba(88, 166, 255, 0.1);
  --stage-active: #58a6ff;
}

/* === Base Reset === */
*, *::before, *::after { margin: 0; padding: 0; box-sizing: border-box; }
html, body {
  height: 100%;
  background: var(--bg-void);
  color: var(--text-primary);
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, sans-serif;
  overflow: hidden;
  -webkit-font-smoothing: antialiased;
}

/* === Neural Background Canvas === */
#neuralCanvas {
  position: fixed;
  top: 0; left: 0;
  width: 100%; height: 100%;
  z-index: 0;
  pointer-events: none;
}

/* === Layout === */
.layout {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: var(--sidebar-width) 1fr;
  grid-template-rows: 1fr auto;
  height: 100vh;
}
.layout.no-sidebar {
  grid-template-columns: 1fr;
}
.layout.no-sidebar .sidebar { display: none; }
.layout.no-sidebar .main-area { grid-column: 1; }
.layout.no-sidebar .bottom-bar { grid-column: 1; }

/* === Glassmorphism Base === */
.glass-panel {
  background: var(--bg-glass);
  backdrop-filter: blur(16px) saturate(1.2);
  -webkit-backdrop-filter: blur(16px) saturate(1.2);
  border: 1px solid var(--border-glow);
  border-radius: 12px;
  box-shadow: 0 0 20px rgba(0, 212, 170, 0.03), inset 0 1px 0 rgba(255,255,255,0.03);
}

/* === Sci-Fi Corner Accents === */
.sci-frame { position: relative; }
.sci-frame::before, .sci-frame::after {
  content: '';
  position: absolute;
  width: 16px; height: 16px;
  border-color: var(--accent);
  border-style: solid;
  opacity: 0.4;
  animation: corner-pulse 3s ease-in-out infinite alternate;
  pointer-events: none;
}
.sci-frame::before { top: -1px; left: -1px; border-width: 2px 0 0 2px; border-radius: 4px 0 0 0; }
.sci-frame::after  { bottom: -1px; right: -1px; border-width: 0 2px 2px 0; border-radius: 0 0 4px 0; }
@keyframes corner-pulse {
  0% { opacity: 0.25; }
  100% { opacity: 0.6; }
}

/* === Sidebar === */
.sidebar {
  background: var(--bg-glass);
  backdrop-filter: blur(20px) saturate(1.2);
  -webkit-backdrop-filter: blur(20px) saturate(1.2);
  border-right: 1px solid var(--border-glow);
  grid-row: 1 / -1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  animation: fadeSlideIn 0.4s ease;
}
@keyframes fadeSlideIn {
  from { opacity: 0; transform: translateX(-10px); }
  to { opacity: 1; transform: translateX(0); }
}
.sidebar-header {
  padding: 16px;
  border-bottom: 1px solid var(--border-glow);
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-shrink: 0;
}
.sidebar-header h1 {
  font-size: 13px;
  font-weight: 700;
  color: var(--accent);
  letter-spacing: 0.08em;
  text-transform: uppercase;
  text-shadow: 0 0 10px var(--accent-glow);
}
.sidebar-collapse-btn {
  background: none;
  border: none;
  color: var(--text-dim);
  cursor: pointer;
  font-size: 16px;
  padding: 4px 6px;
  line-height: 1;
  border-radius: 4px;
  transition: all 0.15s;
}
.sidebar-collapse-btn:hover { color: var(--text-primary); background: rgba(255,255,255,0.05); }

.sidebar-body {
  flex: 1;
  overflow-y: auto;
  padding: 12px 14px;
  scrollbar-width: thin;
  scrollbar-color: var(--border-glow) transparent;
}
.sidebar-body::-webkit-scrollbar { width: 4px; }
.sidebar-body::-webkit-scrollbar-track { background: transparent; }
.sidebar-body::-webkit-scrollbar-thumb { background: var(--border-glow); border-radius: 4px; }

/* === Agent Status Ring === */
.agent-status {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  margin-bottom: 12px;
  background: rgba(0,0,0,0.2);
  border-radius: 10px;
  border: 1px solid var(--border-glow);
}
.agent-ring {
  width: 36px; height: 36px;
  border-radius: 50%;
  border: 2px solid var(--accent);
  display: flex;
  align-items: center;
  justify-content: center;
  animation: ring-breathe 3s ease-in-out infinite;
  flex-shrink: 0;
  box-shadow: 0 0 12px var(--accent-glow);
}
.agent-ring-inner {
  width: 10px; height: 10px;
  border-radius: 50%;
  background: var(--accent);
  box-shadow: 0 0 8px var(--accent-glow-strong);
}
@keyframes ring-breathe {
  0%, 100% { transform: scale(1); opacity: 0.8; }
  50% { transform: scale(1.06); opacity: 1; }
}
.agent-ring.processing {
  animation: ring-spin 1.5s linear infinite;
  border-style: dashed;
}
@keyframes ring-spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
.agent-status-text { font-size: 12px; color: var(--text-secondary); }
.agent-status-text strong { color: var(--text-primary); font-weight: 600; display: block; font-size: 13px; }

/* === Connection Status === */
.conn-status {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  background: rgba(0,0,0,0.15);
  border-radius: 8px;
  margin-bottom: 12px;
  font-size: 12px;
  color: var(--text-secondary);
}
.conn-dot {
  width: 8px; height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
  transition: background 0.3s;
}
.conn-dot.connected { background: var(--stage-done); box-shadow: 0 0 6px rgba(63, 185, 80, 0.5); }
.conn-dot.disconnected { background: var(--danger); }
.conn-dot.reconnecting { background: #d29922; animation: pulse-dot 1s infinite; }
@keyframes pulse-dot {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.3; }
}
.cached-badge {
  display: inline-block;
  font-size: 9px;
  font-weight: 700;
  color: #d29922;
  background: rgba(210, 153, 34, 0.15);
  padding: 2px 5px;
  border-radius: 3px;
  margin-left: auto;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

/* === Metrics Panel === */
.metrics-panel {
  padding: 10px 12px;
  background: rgba(0,0,0,0.15);
  border-radius: 8px;
  margin-bottom: 12px;
  font-size: 11px;
}
.metric-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 3px 0;
  color: var(--text-secondary);
}
.metric-row .metric-val { color: var(--text-primary); font-weight: 600; font-family: 'SF Mono', 'Fira Code', monospace; font-size: 11px; }
.metric-bar {
  height: 3px;
  background: var(--stage-pending);
  border-radius: 2px;
  margin-top: 6px;
  overflow: hidden;
}
.metric-bar-fill {
  height: 100%;
  background: var(--accent);
  border-radius: 2px;
  transition: width 0.6s ease;
  box-shadow: 0 0 6px var(--accent-glow);
}

/* === Section Labels === */
.section-label {
  font-size: 10px;
  font-weight: 700;
  color: var(--text-dim);
  text-transform: uppercase;
  letter-spacing: 0.08em;
  margin-bottom: 6px;
  margin-top: 8px;
}

/* === Task History === */
.task-list { list-style: none; }
.task-list li {
  padding: 5px 8px;
  font-size: 11px;
  color: var(--text-dim);
  border-radius: 4px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: 'SF Mono', 'Fira Code', monospace;
  cursor: default;
  transition: all 0.15s;
}
.task-list li:hover {
  background: rgba(255,255,255,0.04);
  color: var(--text-primary);
}

/* === Control Toggles === */
.controls-section {
  margin-top: auto;
  padding-top: 12px;
  border-top: 1px solid var(--border-glow);
}
.toggle-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 0;
  font-size: 11px;
  color: var(--text-secondary);
}
.toggle-switch {
  width: 32px; height: 18px;
  background: var(--stage-pending);
  border-radius: 9px;
  cursor: pointer;
  position: relative;
  transition: background 0.2s;
  border: none;
  padding: 0;
}
.toggle-switch::after {
  content: '';
  position: absolute;
  top: 2px; left: 2px;
  width: 14px; height: 14px;
  background: #fff;
  border-radius: 50%;
  transition: transform 0.2s;
}
.toggle-switch.on {
  background: var(--accent);
  box-shadow: 0 0 8px var(--accent-glow);
}
.toggle-switch.on::after {
  transform: translateX(14px);
}

/* === Main Area === */
.main-area {
  position: relative;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.main-area iframe {
  flex: 1;
  width: 100%;
  border: none;
  background: transparent;
}

/* === Pipeline HUD === */
.pipeline-hud {
  padding: 10px 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0;
  flex-shrink: 0;
  background: rgba(0,0,0,0.2);
  border-bottom: 1px solid var(--border-glow);
}
.pipeline-node {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 3px;
  position: relative;
}
.pipeline-circle {
  width: 22px; height: 22px;
  border-radius: 50%;
  border: 2px solid var(--stage-pending);
  background: transparent;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 8px;
  font-weight: 700;
  color: var(--text-dim);
  transition: all 0.3s ease;
  position: relative;
}
.pipeline-circle.active {
  border-color: var(--stage-active);
  color: var(--stage-active);
  box-shadow: 0 0 12px var(--accent-glow-strong);
  animation: stage-pulse 1.2s ease-in-out infinite;
}
@keyframes stage-pulse {
  0%, 100% { box-shadow: 0 0 8px var(--accent-glow); }
  50% { box-shadow: 0 0 20px var(--accent-glow-strong); }
}
.pipeline-circle.done {
  border-color: var(--stage-done);
  background: var(--stage-done);
  color: #fff;
}
.pipeline-circle.error {
  border-color: var(--stage-error);
  background: var(--stage-error);
  color: #fff;
}
.pipeline-label {
  font-size: 8px;
  color: var(--text-dim);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  transition: color 0.3s;
}
.pipeline-circle.active + .pipeline-label,
.pipeline-circle.done + .pipeline-label { color: var(--text-secondary); }
.pipeline-connector {
  width: 16px;
  height: 2px;
  background: var(--stage-pending);
  align-self: center;
  margin-bottom: 14px;
  transition: background 0.3s;
  border-radius: 1px;
}
.pipeline-connector.lit {
  background: var(--stage-done);
  box-shadow: 0 0 4px rgba(63, 185, 80, 0.4);
}

/* === Empty State === */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  flex: 1;
  color: var(--text-dim);
  gap: 12px;
}
.empty-state-icon {
  width: 48px; height: 48px;
  border: 2px solid var(--border-glow);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  opacity: 0.4;
  animation: ring-breathe 3s ease-in-out infinite;
}
.empty-state-text { font-size: 14px; }

/* === Bottom Bar === */
.bottom-bar {
  grid-column: 2;
  background: var(--bg-glass);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border-top: 1px solid var(--border-glow);
  padding: 10px 16px;
  display: flex;
  gap: 8px;
  align-items: center;
}
.layout.no-sidebar .bottom-bar { grid-column: 1; }

.chat-field {
  flex: 1;
  background: rgba(0,0,0,0.3);
  border: 1px solid var(--border-dim);
  border-radius: 10px;
  padding: 10px 14px;
  color: var(--text-primary);
  font-size: 14px;
  outline: none;
  transition: all 0.2s;
}
.chat-field:focus {
  border-color: var(--accent);
  box-shadow: 0 0 12px var(--accent-glow);
}
.chat-field:disabled { opacity: 0.4; cursor: not-allowed; }
.chat-field::placeholder { color: var(--text-dim); }

.btn-send {
  background: var(--accent);
  color: var(--bg-void);
  border: none;
  border-radius: 10px;
  padding: 10px 20px;
  font-size: 13px;
  font-weight: 700;
  cursor: pointer;
  transition: all 0.15s;
  white-space: nowrap;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.btn-send:hover { background: var(--accent-hover); transform: translateY(-1px); box-shadow: 0 4px 12px var(--accent-glow); }
.btn-send:active { transform: translateY(0); }
.btn-send:disabled { opacity: 0.4; cursor: not-allowed; transform: none; }

.btn-stop {
  background: var(--danger);
  color: #fff;
  border: none;
  border-radius: 10px;
  padding: 10px 16px;
  font-size: 13px;
  font-weight: 700;
  cursor: pointer;
  transition: all 0.15s;
  white-space: nowrap;
}
.btn-stop:hover { background: #ff6b61; transform: translateY(-1px); }
.btn-stop:active { transform: scale(0.96); }
.btn-stop.pulse { animation: stop-pulse 0.4s ease; }
@keyframes stop-pulse {
  0% { transform: scale(1); }
  30% { transform: scale(1.08); }
  100% { transform: scale(1); }
}

/* === Sidebar Toggle === */
.sidebar-open-btn {
  position: fixed;
  top: 12px; left: 12px;
  z-index: 200;
  background: var(--bg-glass);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border: 1px solid var(--border-glow);
  color: var(--text-primary);
  border-radius: 8px;
  padding: 8px 10px;
  font-size: 16px;
  cursor: pointer;
  display: none;
  line-height: 1;
  transition: all 0.15s;
}
.sidebar-open-btn:hover { background: var(--bg-glass-hover); }
.layout.no-sidebar ~ .sidebar-open-btn { display: block; }

/* === Mobile Sidebar Overlay === */
.sidebar-overlay {
  display: none;
  position: fixed;
  inset: 0;
  z-index: 300;
}
.sidebar-overlay.visible { display: block; }
.sidebar-overlay .overlay-bg {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  backdrop-filter: blur(4px);
}
.sidebar-overlay .overlay-panel {
  position: absolute;
  top: 0; left: 0;
  width: 280px; height: 100%;
  background: var(--bg-glass);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border-right: 1px solid var(--border-glow);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  animation: slideIn 0.25s ease;
}
@keyframes slideIn {
  from { transform: translateX(-100%); }
  to { transform: translateX(0); }
}
.sidebar-overlay .overlay-panel .sidebar-header,
.sidebar-overlay .overlay-panel .sidebar-body { padding: 16px 14px; }

/* === CRT Mode === */
.crt-mode::after {
  content: '';
  position: fixed;
  inset: 0;
  background: repeating-linear-gradient(transparent 0px, rgba(0,0,0,0.04) 1px, transparent 2px);
  pointer-events: none;
  z-index: 9999;
}
.crt-mode .sidebar-header h1,
.crt-mode .agent-status-text strong {
  text-shadow: 0 0 4px var(--accent), 0 0 8px var(--accent-glow);
}

/* === Responsive: Desktop (>1024px) === */
/* Default is desktop — full command center */

/* === Responsive: Tablet (481-1024px) === */
@media (max-width: 1024px) {
  .layout { grid-template-columns: 1fr; }
  .sidebar { display: none; }
  .sidebar-open-btn { display: block; }
  .bottom-bar { grid-column: 1; }
  .main-area { grid-column: 1; }
  .pipeline-connector { width: 10px; }
  .pipeline-circle { width: 18px; height: 18px; font-size: 7px; }
}

/* === Responsive: Phone (≤480px) — Wearable Companion === */
@media (max-width: 480px) {
  .layout { height: 100dvh; height: -webkit-fill-available; }
  .pipeline-hud { display: none; }
  .main-area { grid-template-rows: 1fr; }
  .bottom-bar { padding: 8px 10px; }
  .chat-field { font-size: 16px; padding: 10px 12px; }
  .btn-send { padding: 10px 14px; font-size: 12px; }
}

/* === Touch Optimizations === */
.touch-mode .chat-field { font-size: 16px; padding: 12px 16px; }
.touch-mode .btn-send { min-height: 44px; min-width: 44px; padding: 12px 22px; }
.touch-mode .btn-stop { min-height: 44px; min-width: 44px; padding: 12px 18px; }
.touch-mode .sidebar-open-btn { min-height: 44px; min-width: 44px; }
.touch-mode .sidebar-collapse-btn { min-height: 44px; min-width: 44px; }
.touch-mode .task-list li { padding: 10px 8px; }
@media (hover: none) and (pointer: coarse) {
  .chat-field { font-size: 16px; padding: 12px 16px; }
  .btn-send { min-height: 44px; min-width: 44px; }
  .btn-stop { min-height: 44px; min-width: 44px; }
  .sidebar-open-btn { min-height: 44px; min-width: 44px; }
}

/* === Scrollbar === */
::-webkit-scrollbar { width: 4px; }
::-webkit-scrollbar-track { background: transparent; }
::-webkit-scrollbar-thumb { background: var(--border-glow); border-radius: 4px; }
</style>
</head>
<body>

<!-- Neural Network Background Canvas -->
<canvas id="neuralCanvas"></canvas>

<div id="app" class="layout">
  <!-- Sidebar -->
  <aside class="sidebar" id="sidebar">
    <div class="sidebar-header">
      <h1>{{TITLE2}}</h1>
      <button class="sidebar-collapse-btn" id="sidebarCollapseBtn" title="Collapse sidebar">&#x2715;</button>
    </div>
    <div class="sidebar-body">
      <!-- Agent Status -->
      <div class="agent-status" id="agentStatus">
        <div class="agent-ring" id="agentRing"><div class="agent-ring-inner"></div></div>
        <div class="agent-status-text">
          <strong id="agentStateLabel">Idle</strong>
          <span id="connLabel">Disconnected</span>
        </div>
      </div>

      <!-- Connection -->
      <div class="conn-status" id="connStatus">
        <span class="conn-dot disconnected" id="connDot"></span>
        <span id="connStatusText">Disconnected</span>
      </div>

      <!-- Metrics -->
      <div class="section-label">Metrics</div>
      <div class="metrics-panel" id="metricsPanel">
        <div class="metric-row"><span>Quality</span><span class="metric-val" id="metricQuality">--</span></div>
        <div class="metric-bar"><div class="metric-bar-fill" id="metricQualityBar" style="width:0%"></div></div>
        <div class="metric-row"><span>Cost</span><span class="metric-val" id="metricCost">--</span></div>
        <div class="metric-row"><span>Duration</span><span class="metric-val" id="metricDuration">--</span></div>
      </div>

      <!-- Task History -->
      <div class="section-label">Task History</div>
      <ul class="task-list" id="taskList"></ul>

      <!-- Controls -->
      <div class="controls-section">
        <div class="section-label">Controls</div>
        <div class="toggle-row"><span>Sound</span><button class="toggle-switch" id="toggleSound" data-key="ovh_sound"></button></div>
        <div class="toggle-row"><span>CRT Mode</span><button class="toggle-switch" id="toggleCRT" data-key="ovh_crt"></button></div>
        <div class="toggle-row"><span>Theme</span>
          <select id="themeSelect" style="background:var(--stage-pending);color:var(--text-primary);border:none;border-radius:4px;padding:3px 6px;font-size:10px;cursor:pointer;">
            <option value="scifi">Sci-Fi</option>
            <option value="cyberpunk">Cyberpunk</option>
            <option value="clean">Clean</option>
          </select>
        </div>
      </div>
    </div>
  </aside>

  <!-- Main Content -->
  <main class="main-area" id="mainArea">
    <!-- Pipeline HUD -->
    <div class="pipeline-hud" id="pipelineHUD"></div>

    <!-- Content area -->
    <div class="empty-state" id="emptyState">
      <div class="empty-state-icon">&#x25C9;</div>
      <div class="empty-state-text">Awaiting signal...</div>
    </div>
  </main>

  <!-- Bottom Bar -->
  <div class="bottom-bar" id="bottomBar">
    <input type="text" class="chat-field" id="chatInput" placeholder="Type a message..." disabled autocomplete="off">
    <button class="btn-send" id="btnSend" disabled>Send</button>
    <button class="btn-stop" id="btnStop" style="display:none;">&#x23F9; Stop</button>
  </div>
</div>

<!-- Sidebar open button -->
<button class="sidebar-open-btn" id="sidebarOpenBtn">&#x2630;</button>

<!-- Mobile sidebar overlay -->
<div class="sidebar-overlay" id="sidebarOverlay">
  <div class="overlay-bg" id="overlayBg"></div>
  <div class="overlay-panel">
    <div class="sidebar-header">
      <h1>{{TITLE2}}</h1>
      <button class="sidebar-collapse-btn" id="overlayCloseBtn">&#x2715;</button>
    </div>
    <div class="sidebar-body">
      <div class="conn-status">
        <span class="conn-dot disconnected" id="connDotOverlay"></span>
        <span id="connLabelOverlay">Disconnected</span>
      </div>
      <div class="section-label">Task History</div>
      <ul class="task-list" id="taskListOverlay"></ul>
    </div>
  </div>
</div>

<script>
(function() {
  "use strict";

  // ==== PIPELINE STAGE NAMES ====
  var STAGES = [
    {n:1, key:"intake",     label:"IN"},
    {n:2, key:"clarify",    label:"CL"},
    {n:3, key:"plan",       label:"PL"},
    {n:4, key:"agent_selection", label:"AG"},
    {n:5, key:"execute",    label:"EX"},
    {n:6, key:"review",     label:"RV"},
    {n:7, key:"memory_update",   label:"ME"},
    {n:8, key:"pattern_tracking",label:"PT"},
    {n:9, key:"reflection", label:"RF"},
    {n:10,key:"goal_update",label:"GO"}
  ];

  // ==== CONFIGURATION ====
  var CONFIG = {
    wsURL: "{{WS_URL}}",
    darkMode: {{DARK_MODE}},
    showSidebar: {{SHOW_SIDEBAR}},
    touchMode: {{TOUCH_MODE}},
    emergencyStop: {{EMERGENCY_STOP}},
    theme: "{{THEME}}",
    soundEnabled: {{SOUND_ENABLED}},
    pingInterval: 25000,
    reconnectBase: 1000,
    reconnectMax: 30000,
    feedbackTimeout: 60000,
    maxTaskHistory: 10,
    cacheKeyUI: "overhuman_last_ui",
    cacheKeyTasks: "overhuman_task_history"
  };

  // ==== STATE ====
  var state = {
    ws: null,
    connected: false,
    reconnectDelay: CONFIG.reconnectBase,
    reconnectTimer: null,
    pingTimer: null,
    currentTaskID: "",
    taskHistory: [],
    uiDeliveredAt: 0,
    firstActionTime: 0,
    actionsUsed: [],
    scrolled: false,
    feedbackTimer: null,
    feedbackSent: false,
    streamBuffer: "",
    isCached: false,
    pipelineActive: false,
    stageStates: {} // stage number → "started"|"completed"|"error"
  };

  // ==== DOM REFS ====
  var dom = {
    app: document.getElementById("app"),
    sidebar: document.getElementById("sidebar"),
    sidebarCollapseBtn: document.getElementById("sidebarCollapseBtn"),
    sidebarOpenBtn: document.getElementById("sidebarOpenBtn"),
    sidebarOverlay: document.getElementById("sidebarOverlay"),
    overlayBg: document.getElementById("overlayBg"),
    overlayCloseBtn: document.getElementById("overlayCloseBtn"),
    connDot: document.getElementById("connDot"),
    connLabel: document.getElementById("connLabel"),
    connStatusText: document.getElementById("connStatusText"),
    connDotOverlay: document.getElementById("connDotOverlay"),
    connLabelOverlay: document.getElementById("connLabelOverlay"),
    taskList: document.getElementById("taskList"),
    taskListOverlay: document.getElementById("taskListOverlay"),
    mainArea: document.getElementById("mainArea"),
    emptyState: document.getElementById("emptyState"),
    chatInput: document.getElementById("chatInput"),
    btnSend: document.getElementById("btnSend"),
    btnStop: document.getElementById("btnStop"),
    bottomBar: document.getElementById("bottomBar"),
    connStatus: document.getElementById("connStatus"),
    pipelineHUD: document.getElementById("pipelineHUD"),
    agentRing: document.getElementById("agentRing"),
    agentStateLabel: document.getElementById("agentStateLabel"),
    metricQuality: document.getElementById("metricQuality"),
    metricQualityBar: document.getElementById("metricQualityBar"),
    metricCost: document.getElementById("metricCost"),
    metricDuration: document.getElementById("metricDuration"),
    toggleSound: document.getElementById("toggleSound"),
    toggleCRT: document.getElementById("toggleCRT"),
    themeSelect: document.getElementById("themeSelect")
  };

  // ==== NEURAL BACKGROUND ====
  var neuralCanvas = document.getElementById("neuralCanvas");
  var neuralCtx = neuralCanvas.getContext("2d");
  var particles = [];
  var neuralSpeed = 0.3; // base speed, increases when pipeline active

  function initNeural() {
    resizeCanvas();
    var count = window.innerWidth <= 768 ? 30 : 65;
    particles = [];
    for (var i = 0; i < count; i++) {
      particles.push({
        x: Math.random() * neuralCanvas.width,
        y: Math.random() * neuralCanvas.height,
        vx: (Math.random() - 0.5) * neuralSpeed,
        vy: (Math.random() - 0.5) * neuralSpeed,
        r: 1.5 + Math.random() * 1
      });
    }
    requestAnimationFrame(drawNeural);
  }

  function resizeCanvas() {
    neuralCanvas.width = window.innerWidth;
    neuralCanvas.height = window.innerHeight;
  }
  window.addEventListener("resize", resizeCanvas);

  function drawNeural() {
    if (document.hidden) { requestAnimationFrame(drawNeural); return; }
    var w = neuralCanvas.width, h = neuralCanvas.height;
    neuralCtx.clearRect(0, 0, w, h);
    var maxDist = 120;
    var speed = state.pipelineActive ? 1.2 : 0.3;
    var accentRGB = getAccentRGB();

    // Update positions
    for (var i = 0; i < particles.length; i++) {
      var p = particles[i];
      p.x += p.vx * (speed / 0.3);
      p.y += p.vy * (speed / 0.3);
      if (p.x < 0 || p.x > w) p.vx *= -1;
      if (p.y < 0 || p.y > h) p.vy *= -1;
      p.x = Math.max(0, Math.min(w, p.x));
      p.y = Math.max(0, Math.min(h, p.y));
    }

    // Draw connections
    neuralCtx.lineWidth = 0.5;
    for (var i = 0; i < particles.length; i++) {
      for (var j = i + 1; j < particles.length; j++) {
        var dx = particles[i].x - particles[j].x;
        var dy = particles[i].y - particles[j].y;
        var dist = Math.sqrt(dx * dx + dy * dy);
        if (dist < maxDist) {
          var alpha = (1 - dist / maxDist) * 0.15;
          neuralCtx.strokeStyle = "rgba(" + accentRGB + "," + alpha + ")";
          neuralCtx.beginPath();
          neuralCtx.moveTo(particles[i].x, particles[i].y);
          neuralCtx.lineTo(particles[j].x, particles[j].y);
          neuralCtx.stroke();
        }
      }
    }

    // Draw particles
    for (var i = 0; i < particles.length; i++) {
      var p = particles[i];
      neuralCtx.fillStyle = "rgba(" + accentRGB + ",0.3)";
      neuralCtx.beginPath();
      neuralCtx.arc(p.x, p.y, p.r, 0, Math.PI * 2);
      neuralCtx.fill();
    }

    requestAnimationFrame(drawNeural);
  }

  function getAccentRGB() {
    var style = getComputedStyle(document.documentElement);
    var accent = style.getPropertyValue("--accent").trim();
    // Parse hex
    if (accent.charAt(0) === "#") {
      var r = parseInt(accent.substr(1,2),16);
      var g = parseInt(accent.substr(3,2),16);
      var b = parseInt(accent.substr(5,2),16);
      return r+","+g+","+b;
    }
    return "0,212,170";
  }

  // ==== PIPELINE HUD ====
  function buildPipelineHUD() {
    var html = "";
    for (var i = 0; i < STAGES.length; i++) {
      if (i > 0) html += '<div class="pipeline-connector" id="pconn' + i + '"></div>';
      html += '<div class="pipeline-node">' +
        '<div class="pipeline-circle" id="pstage' + STAGES[i].n + '">' + STAGES[i].n + '</div>' +
        '<div class="pipeline-label">' + STAGES[i].label + '</div></div>';
    }
    dom.pipelineHUD.innerHTML = html;
  }

  function updatePipelineStage(stage, status) {
    state.stageStates[stage] = status;
    var el = document.getElementById("pstage" + stage);
    if (!el) return;

    el.className = "pipeline-circle";
    if (status === "started") {
      el.classList.add("active");
      state.pipelineActive = true;
      dom.agentRing.classList.add("processing");
      dom.agentStateLabel.textContent = "Processing";
    } else if (status === "completed") {
      el.classList.add("done");
      el.textContent = "\u2713";
      // Light connector
      var conn = document.getElementById("pconn" + stage);
      if (conn) conn.classList.add("lit");
    } else if (status === "error") {
      el.classList.add("error");
      el.textContent = "\u2717";
    }

    // Check if all done
    if (stage === 10 && (status === "completed" || status === "error")) {
      state.pipelineActive = false;
      dom.agentRing.classList.remove("processing");
      dom.agentStateLabel.textContent = "Idle";
      soundPlay("pipelineDone");
    } else if (status === "completed") {
      soundPlay("stageComplete");
    } else if (status === "error") {
      soundPlay("error");
    }
  }

  function resetPipeline() {
    state.stageStates = {};
    state.pipelineActive = false;
    for (var i = 0; i < STAGES.length; i++) {
      var el = document.getElementById("pstage" + STAGES[i].n);
      if (el) { el.className = "pipeline-circle"; el.textContent = STAGES[i].n; }
      if (i > 0) {
        var conn = document.getElementById("pconn" + i);
        if (conn) conn.classList.remove("lit");
      }
    }
  }

  // ==== SOUND ENGINE (Web Audio API) ====
  var audioCtx = null;
  function getAudioCtx() {
    if (!audioCtx) { try { audioCtx = new (window.AudioContext || window.webkitAudioContext)(); } catch(e) {} }
    return audioCtx;
  }

  function soundPlay(type) {
    if (!isSoundOn()) return;
    var ctx = getAudioCtx();
    if (!ctx) return;
    var now = ctx.currentTime;
    var g = ctx.createGain();
    g.connect(ctx.destination);

    if (type === "connect") {
      var o = ctx.createOscillator(); o.type = "sine"; o.frequency.setValueAtTime(300, now); o.frequency.linearRampToValueAtTime(800, now + 0.15); g.gain.setValueAtTime(0.08, now); g.gain.linearRampToValueAtTime(0, now + 0.2); o.connect(g); o.start(now); o.stop(now + 0.2);
    } else if (type === "disconnect") {
      var o = ctx.createOscillator(); o.type = "sine"; o.frequency.setValueAtTime(600, now); o.frequency.linearRampToValueAtTime(200, now + 0.15); g.gain.setValueAtTime(0.06, now); g.gain.linearRampToValueAtTime(0, now + 0.2); o.connect(g); o.start(now); o.stop(now + 0.2);
    } else if (type === "stageComplete") {
      var o = ctx.createOscillator(); o.type = "sine"; o.frequency.setValueAtTime(1200, now); g.gain.setValueAtTime(0.04, now); g.gain.linearRampToValueAtTime(0, now + 0.05); o.connect(g); o.start(now); o.stop(now + 0.06);
    } else if (type === "pipelineDone") {
      [523, 659, 784].forEach(function(f, i) { var o = ctx.createOscillator(); o.type = "sine"; o.frequency.value = f; var gg = ctx.createGain(); gg.gain.setValueAtTime(0.05, now); gg.gain.linearRampToValueAtTime(0, now + 0.3); o.connect(gg); gg.connect(ctx.destination); o.start(now + i * 0.03); o.stop(now + 0.35); });
    } else if (type === "error") {
      var o = ctx.createOscillator(); o.type = "sawtooth"; o.frequency.value = 80; g.gain.setValueAtTime(0.06, now); g.gain.linearRampToValueAtTime(0, now + 0.25); o.connect(g); o.start(now); o.stop(now + 0.25);
    } else if (type === "send") {
      var o = ctx.createOscillator(); o.type = "triangle"; o.frequency.setValueAtTime(400, now); o.frequency.linearRampToValueAtTime(800, now + 0.08); g.gain.setValueAtTime(0.04, now); g.gain.linearRampToValueAtTime(0, now + 0.1); o.connect(g); o.start(now); o.stop(now + 0.1);
    }
  }

  function isSoundOn() {
    return dom.toggleSound && dom.toggleSound.classList.contains("on");
  }

  // ==== THEME ====
  function applyTheme(theme) {
    document.body.classList.remove("theme-cyberpunk", "theme-clean");
    if (theme === "cyberpunk") document.body.classList.add("theme-cyberpunk");
    else if (theme === "clean") document.body.classList.add("theme-clean");
    try { localStorage.setItem("ovh_theme", theme); } catch(e) {}
  }

  function loadTheme() {
    var saved = null;
    try { saved = localStorage.getItem("ovh_theme"); } catch(e) {}
    var theme = saved || CONFIG.theme || "scifi";
    dom.themeSelect.value = theme;
    applyTheme(theme);
  }

  // ==== TOGGLES ====
  function initToggles() {
    [dom.toggleSound, dom.toggleCRT].forEach(function(btn) {
      if (!btn) return;
      var key = btn.getAttribute("data-key");
      var saved = false;
      try { saved = localStorage.getItem(key) === "true"; } catch(e) {}
      if (saved || (key === "ovh_sound" && CONFIG.soundEnabled)) btn.classList.add("on");
      btn.addEventListener("click", function() {
        btn.classList.toggle("on");
        try { localStorage.setItem(key, btn.classList.contains("on")); } catch(e) {}
        if (key === "ovh_crt") {
          document.body.classList.toggle("crt-mode", btn.classList.contains("on"));
        }
      });
    });
    // Apply CRT on load if saved
    if (dom.toggleCRT && dom.toggleCRT.classList.contains("on")) {
      document.body.classList.add("crt-mode");
    }
    // Theme select
    dom.themeSelect.addEventListener("change", function() {
      applyTheme(dom.themeSelect.value);
    });
  }

  // ==== INITIALIZATION ====
  function init() {
    if (CONFIG.touchMode || isTouchDevice()) document.body.classList.add("touch-mode");
    if (!CONFIG.showSidebar) dom.app.classList.add("no-sidebar");
    if (CONFIG.emergencyStop) dom.btnStop.style.display = "";

    buildPipelineHUD();
    loadTheme();
    initToggles();
    loadTaskHistory();
    loadCachedUI();
    bindEvents();
    initNeural();
    connectWS();
  }

  function isTouchDevice() {
    return ("ontouchstart" in window) || (navigator.maxTouchPoints > 0);
  }

  // ==== EVENTS ====
  function bindEvents() {
    dom.sidebarCollapseBtn.addEventListener("click", function() { dom.app.classList.add("no-sidebar"); });
    dom.sidebarOpenBtn.addEventListener("click", function() {
      if (window.innerWidth <= 1024) { dom.sidebarOverlay.classList.add("visible"); }
      else { dom.app.classList.remove("no-sidebar"); }
    });
    dom.overlayBg.addEventListener("click", closeMobileOverlay);
    dom.overlayCloseBtn.addEventListener("click", closeMobileOverlay);
    dom.chatInput.addEventListener("keydown", function(e) {
      if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); sendChatMessage(); }
    });
    dom.btnSend.addEventListener("click", sendChatMessage);
    dom.btnStop.addEventListener("click", sendEmergencyStop);
    window.addEventListener("message", handleIframeMessage);
  }

  function closeMobileOverlay() { dom.sidebarOverlay.classList.remove("visible"); }

  // ==== WEBSOCKET ====
  function connectWS() {
    if (state.ws) { try { state.ws.close(); } catch(e) {} }
    setConnectionStatus("reconnecting");
    try { state.ws = new WebSocket(CONFIG.wsURL); } catch(e) { scheduleReconnect(); return; }

    state.ws.onopen = function() {
      state.connected = true;
      state.reconnectDelay = CONFIG.reconnectBase;
      setConnectionStatus("connected");
      dom.chatInput.disabled = false;
      dom.btnSend.disabled = false;
      startPing();
      soundPlay("connect");
    };
    state.ws.onclose = function() {
      state.connected = false;
      setConnectionStatus("disconnected");
      dom.chatInput.disabled = true;
      dom.btnSend.disabled = true;
      stopPing();
      scheduleReconnect();
      soundPlay("disconnect");
    };
    state.ws.onerror = function() {};
    state.ws.onmessage = function(evt) { handleWSMessage(evt.data); };
  }

  function scheduleReconnect() {
    if (state.reconnectTimer) return;
    setConnectionStatus("reconnecting");
    state.reconnectTimer = setTimeout(function() {
      state.reconnectTimer = null;
      state.reconnectDelay = Math.min(state.reconnectDelay * 2, CONFIG.reconnectMax);
      connectWS();
    }, state.reconnectDelay);
  }

  function startPing() {
    stopPing();
    state.pingTimer = setInterval(function() { wsSend({ type: "ping", payload: {} }); }, CONFIG.pingInterval);
  }
  function stopPing() { if (state.pingTimer) { clearInterval(state.pingTimer); state.pingTimer = null; } }

  function wsSend(msg) {
    if (state.ws && state.ws.readyState === WebSocket.OPEN) { state.ws.send(JSON.stringify(msg)); return true; }
    return false;
  }

  // ==== CONNECTION STATUS ====
  function setConnectionStatus(status) {
    var dotClass = "conn-dot " + status;
    var label = status === "connected" ? "Connected" : status === "reconnecting" ? "Reconnecting..." : "Disconnected";
    dom.connDot.className = dotClass;
    dom.connStatusText.textContent = label;
    dom.connLabel.textContent = label;
    dom.connDotOverlay.className = dotClass;
    dom.connLabelOverlay.textContent = label;
  }

  // ==== MESSAGE HANDLING ====
  function handleWSMessage(raw) {
    var msg;
    try { msg = JSON.parse(raw); } catch(e) { return; }
    if (!msg || !msg.type) return;

    switch (msg.type) {
      case "ui_full": handleUIFull(msg.payload); break;
      case "ui_stream": handleUIStream(msg.payload); break;
      case "action_result": handleActionResult(msg.payload); break;
      case "error": handleError(msg.payload); break;
      case "pipeline_stage": handlePipelineStage(msg.payload); break;
      case "pong": break;
    }
  }

  function handlePipelineStage(payload) {
    if (!payload) return;
    if (payload.status === "started" && payload.stage === 1) {
      resetPipeline();
    }
    updatePipelineStage(payload.stage, payload.status);
  }

  function handleUIFull(payload) {
    if (!payload) return;
    maybeSendFeedback();
    var taskID = payload.task_id || "";
    state.currentTaskID = taskID;
    state.uiDeliveredAt = Date.now();
    state.firstActionTime = 0;
    state.actionsUsed = [];
    state.scrolled = false;
    state.feedbackSent = false;
    state.streamBuffer = "";
    state.isCached = false;

    if (taskID) addTaskToHistory(taskID);
    renderSandboxedUI(payload.html || "");
    cacheUI(payload);
    startFeedbackTimer();
    updateMetrics(payload.thought);
    removeCachedBadge();
  }

  function handleUIStream(payload) {
    if (!payload) return;
    state.streamBuffer += (payload.chunk || "");
    if (payload.done) { renderSandboxedUI(state.streamBuffer); state.streamBuffer = ""; }
  }

  function handleActionResult(payload) {
    if (!payload) return;
    var iframe = dom.mainArea.querySelector("iframe");
    if (iframe && iframe.contentWindow) {
      iframe.contentWindow.postMessage({ type: "action_result", payload: payload }, "*");
    }
  }

  function handleError(payload) {
    if (!payload) return;
    soundPlay("error");
    var message = payload.message || "Unknown error";
    var code = payload.code || 0;
    var errorHTML = '<div style="padding:24px;color:#f85149;font-family:sans-serif;">' +
      '<h3 style="margin-bottom:8px;">Error' + (code ? ' (' + code + ')' : '') + '</h3>' +
      '<p>' + escapeHTML(message) + '</p></div>';
    renderSandboxedUI(errorHTML);
  }

  // ==== METRICS ====
  function updateMetrics(thought) {
    if (!thought) return;
    if (thought.total_ms) dom.metricDuration.textContent = (thought.total_ms / 1000).toFixed(1) + "s";
    if (typeof thought.total_cost === "number") dom.metricCost.textContent = "$" + thought.total_cost.toFixed(4);
    // Estimate quality from stages
    var stages = thought.stages;
    if (stages && stages.length > 0) {
      // Find review stage for quality
      for (var i = 0; i < stages.length; i++) {
        if (stages[i].name === "review" && stages[i].summary) {
          var match = stages[i].summary.match(/quality=([\d.]+)/);
          if (match) {
            var q = parseFloat(match[1]);
            dom.metricQuality.textContent = (q * 100).toFixed(0) + "%";
            dom.metricQualityBar.style.width = (q * 100) + "%";
          }
        }
      }
    }
  }

  // ==== SANDBOXED UI RENDERING ====
  function renderSandboxedUI(html) {
    if (dom.emptyState) dom.emptyState.style.display = "none";
    var existing = dom.mainArea.querySelector("iframe");
    if (existing) existing.remove();

    var bridgeScript = '<script>' +
      'window.addEventListener("scroll", function() { parent.postMessage({type:"iframe_scroll"}, "*"); });' +
      'document.addEventListener("click", function(e) {' +
        'var btn = e.target.closest("[data-action]");' +
        'if (btn) { parent.postMessage({ type: "iframe_action", actionId: btn.getAttribute("data-action"), data: btn.getAttribute("data-payload") || "{}" }, "*"); }' +
      '});' +
      'window.addEventListener("message", function(e) {' +
        'if (e.data && e.data.type === "action_result") { var evt = new CustomEvent("actionResult", {detail: e.data.payload}); document.dispatchEvent(evt); }' +
      '});' +
      'document.querySelectorAll("table").forEach(function(t) {' +
        'var w = document.createElement("div"); w.className = "table-scroll"; t.parentNode.insertBefore(w, t); w.appendChild(t);' +
      '});' +
      '<\/script>';

    var fullHTML = '<!DOCTYPE html><html><head>' +
      '<meta charset="utf-8">' +
      '<meta http-equiv="Content-Security-Policy" content="default-src ' + "'none'" + '; style-src ' + "'unsafe-inline'" + '; script-src ' + "'unsafe-inline'" + '; img-src data:;">' +
      '<meta name="viewport" content="width=device-width, initial-scale=1">' +
      '<style>' +
        '*, *::before, *::after { box-sizing: border-box; }' +
        'html { height: 100%; overflow-x: hidden; overflow-y: auto; -webkit-text-size-adjust: 100%; }' +
        'body { margin: 0; padding: clamp(8px, 3vw, 24px); min-height: 100%; max-width: 100vw; overflow-x: hidden; background: #050a12; color: #e6edf3; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; overflow-wrap: break-word; word-break: break-word; }' +
        'h1, h2, h3 { font-size: clamp(1.1rem, 4vw, 2rem) !important; line-height: 1.2; overflow-wrap: break-word; }' +
        'img, svg, video, canvas { max-width: 100%; height: auto; }' +
        'table { border-collapse: collapse; }' +
        '.table-scroll { overflow-x: auto; -webkit-overflow-scrolling: touch; max-width: 100%; }' +
        'td, th { padding: 6px 8px; text-align: left; white-space: normal; word-break: break-word; }' +
        'pre, code { max-width: 100%; overflow-x: auto; white-space: pre-wrap; }' +
        'a { color: #00d4aa; }' +
        '[data-action] { cursor: pointer; }' +
      '</style></head><body>' + html + bridgeScript + '</body></html>';

    var iframe = document.createElement("iframe");
    iframe.sandbox = "allow-scripts";
    iframe.style.cssText = "flex:1;width:100%;border:none;background:transparent;";
    iframe.srcdoc = fullHTML;
    dom.mainArea.appendChild(iframe);
  }

  // ==== IFRAME MESSAGE HANDLER ====
  function handleIframeMessage(e) {
    if (!e.data || !e.data.type) return;
    if (e.data.type === "iframe_action") {
      trackAction(e.data.actionId);
      wsSend({ type: "action", payload: { action_id: e.data.actionId, data: safeParseJSON(e.data.data) } });
    } else if (e.data.type === "iframe_scroll") {
      state.scrolled = true;
    }
  }

  // ==== CHAT ====
  function sendChatMessage() {
    var text = dom.chatInput.value.trim();
    if (!text || !state.connected) return;
    wsSend({ type: "input", payload: { text: text } });
    dom.chatInput.value = "";
    dom.chatInput.focus();
    soundPlay("send");
  }

  function sendEmergencyStop() {
    wsSend({ type: "cancel", payload: { reason: "user" } });
    dom.btnStop.classList.add("pulse");
    setTimeout(function() { dom.btnStop.classList.remove("pulse"); }, 400);
  }

  // ==== FEEDBACK ====
  function trackAction(actionId) {
    if (!state.firstActionTime && state.uiDeliveredAt) state.firstActionTime = Date.now();
    if (actionId && state.actionsUsed.indexOf(actionId) === -1) state.actionsUsed.push(actionId);
  }
  function startFeedbackTimer() {
    clearFeedbackTimer();
    state.feedbackTimer = setTimeout(function() { maybeSendFeedback(); }, CONFIG.feedbackTimeout);
  }
  function clearFeedbackTimer() {
    if (state.feedbackTimer) { clearTimeout(state.feedbackTimer); state.feedbackTimer = null; }
  }
  function maybeSendFeedback() {
    if (state.feedbackSent || !state.currentTaskID) return;
    state.feedbackSent = true;
    clearFeedbackTimer();
    var tta = 0;
    if (state.firstActionTime && state.uiDeliveredAt) tta = state.firstActionTime - state.uiDeliveredAt;
    wsSend({ type: "ui_feedback", payload: { task_id: state.currentTaskID, scrolled: state.scrolled, time_to_action_ms: tta, actions_used: state.actionsUsed, dismissed: false } });
  }

  // ==== TASK HISTORY ====
  function loadTaskHistory() {
    try { var raw = localStorage.getItem(CONFIG.cacheKeyTasks); if (raw) { state.taskHistory = JSON.parse(raw); if (!Array.isArray(state.taskHistory)) state.taskHistory = []; } } catch(e) { state.taskHistory = []; }
    renderTaskHistory();
  }
  function addTaskToHistory(taskID) {
    var idx = state.taskHistory.indexOf(taskID);
    if (idx !== -1) state.taskHistory.splice(idx, 1);
    state.taskHistory.unshift(taskID);
    if (state.taskHistory.length > CONFIG.maxTaskHistory) state.taskHistory = state.taskHistory.slice(0, CONFIG.maxTaskHistory);
    try { localStorage.setItem(CONFIG.cacheKeyTasks, JSON.stringify(state.taskHistory)); } catch(e) {}
    renderTaskHistory();
  }
  function renderTaskHistory() {
    var html = "";
    for (var i = 0; i < state.taskHistory.length; i++) {
      html += '<li title="' + escapeAttr(state.taskHistory[i]) + '">' + escapeHTML(state.taskHistory[i]) + '</li>';
    }
    dom.taskList.innerHTML = html;
    dom.taskListOverlay.innerHTML = html;
  }

  // ==== CACHE ====
  function cacheUI(payload) { try { localStorage.setItem(CONFIG.cacheKeyUI, JSON.stringify(payload)); } catch(e) {} }
  function loadCachedUI() {
    try {
      var raw = localStorage.getItem(CONFIG.cacheKeyUI);
      if (!raw) return;
      var payload = JSON.parse(raw);
      if (payload && payload.html) {
        state.isCached = true;
        state.currentTaskID = payload.task_id || "";
        renderSandboxedUI(payload.html);
        var badge = document.createElement("span");
        badge.className = "cached-badge";
        badge.textContent = "Cached";
        badge.id = "cachedBadge";
        dom.connStatus.appendChild(badge);
      }
    } catch(e) {}
  }
  function removeCachedBadge() { var b = document.getElementById("cachedBadge"); if (b) b.remove(); }

  // ==== UTILITIES ====
  function escapeHTML(str) { var d = document.createElement("div"); d.appendChild(document.createTextNode(str)); return d.innerHTML; }
  function escapeAttr(str) { return str.replace(/&/g,"&amp;").replace(/"/g,"&quot;").replace(/</g,"&lt;").replace(/>/g,"&gt;"); }
  function safeParseJSON(str) { try { return JSON.parse(str); } catch(e) { return {}; } }

  // ==== START ====
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", init);
  else init();
})();
</script>
</body>
</html>` + ""
