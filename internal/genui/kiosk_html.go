package genui

// KioskHTML is the complete kiosk single-page application.
// Template variables ({{WS_URL}}, {{TITLE}}, etc.) are replaced at runtime by KioskHandler.
const KioskHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, user-scalable=no">
<title>{{TITLE}}</title>
<style>
:root {
  --bg-primary: #0d1117;
  --bg-secondary: #161b22;
  --bg-surface: #21262d;
  --text-primary: #e6edf3;
  --text-secondary: #8b949e;
  --accent: #00d4aa;
  --accent-hover: #00f0c0;
  --danger: #f85149;
  --border: #30363d;
  --sidebar-width: 260px;
}
*, *::before, *::after { margin: 0; padding: 0; box-sizing: border-box; }
html, body {
  height: 100%;
  background: var(--bg-primary);
  color: var(--text-primary);
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, sans-serif;
  overflow: hidden;
}

/* Layout */
.layout {
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

/* Sidebar */
.sidebar {
  background: var(--bg-secondary);
  border-right: 1px solid var(--border);
  grid-row: 1 / -1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.sidebar-header {
  padding: 16px;
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-shrink: 0;
}
.sidebar-header h1 {
  font-size: 15px;
  font-weight: 700;
  color: var(--accent);
  letter-spacing: 0.04em;
  text-transform: uppercase;
}
.sidebar-collapse-btn {
  background: none;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 18px;
  padding: 4px;
  line-height: 1;
}
.sidebar-collapse-btn:hover { color: var(--text-primary); }

.sidebar-body {
  flex: 1;
  overflow-y: auto;
  padding: 12px 16px;
}

/* Connection status */
.conn-status {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  background: var(--bg-surface);
  border-radius: 8px;
  margin-bottom: 16px;
  font-size: 13px;
}
.conn-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}
.conn-dot.connected { background: #3fb950; }
.conn-dot.disconnected { background: var(--danger); }
.conn-dot.reconnecting { background: #d29922; animation: pulse-dot 1s infinite; }
@keyframes pulse-dot {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}

/* Section headers */
.section-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.06em;
  margin-bottom: 8px;
}

/* Task history */
.task-list {
  list-style: none;
}
.task-list li {
  padding: 6px 8px;
  font-size: 12px;
  color: var(--text-secondary);
  border-radius: 4px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: 'SF Mono', 'Fira Code', monospace;
}
.task-list li:hover {
  background: var(--bg-surface);
  color: var(--text-primary);
}

.cached-badge {
  display: inline-block;
  font-size: 10px;
  font-weight: 600;
  color: #d29922;
  background: rgba(210, 153, 34, 0.15);
  padding: 2px 6px;
  border-radius: 4px;
  margin-left: 8px;
  vertical-align: middle;
}

/* Main area */
.main-area {
  position: relative;
  overflow: hidden;
}
.main-area iframe {
  width: 100%;
  height: 100%;
  border: none;
  background: var(--bg-primary);
}
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--text-secondary);
  gap: 12px;
}
.empty-state-icon {
  font-size: 48px;
  opacity: 0.3;
}
.empty-state-text {
  font-size: 15px;
}

/* Bottom bar */
.bottom-bar {
  grid-column: 2;
  background: var(--bg-secondary);
  border-top: 1px solid var(--border);
  padding: 10px 16px;
  display: flex;
  gap: 8px;
  align-items: center;
}
.layout.no-sidebar .bottom-bar { grid-column: 1; }

.chat-field {
  flex: 1;
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 10px 14px;
  color: var(--text-primary);
  font-size: 14px;
  outline: none;
  transition: border-color 0.15s;
}
.chat-field:focus { border-color: var(--accent); }
.chat-field:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-send {
  background: var(--accent);
  color: var(--bg-primary);
  border: none;
  border-radius: 8px;
  padding: 10px 20px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s;
  white-space: nowrap;
}
.btn-send:hover { background: var(--accent-hover); }
.btn-send:disabled { opacity: 0.5; cursor: not-allowed; }

.btn-stop {
  background: var(--danger);
  color: #fff;
  border: none;
  border-radius: 8px;
  padding: 10px 16px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: transform 0.1s, background 0.15s;
  white-space: nowrap;
}
.btn-stop:hover { background: #ff6b61; }
.btn-stop:active { transform: scale(0.95); }
.btn-stop.pulse {
  animation: stop-pulse 0.4s ease;
}
@keyframes stop-pulse {
  0% { transform: scale(1); }
  30% { transform: scale(1.08); }
  100% { transform: scale(1); }
}

/* Sidebar toggle (shown when sidebar is hidden) */
.sidebar-open-btn {
  position: fixed;
  top: 10px;
  left: 10px;
  z-index: 200;
  background: var(--bg-surface);
  border: 1px solid var(--border);
  color: var(--text-primary);
  border-radius: 6px;
  padding: 8px 10px;
  font-size: 18px;
  cursor: pointer;
  display: none;
  line-height: 1;
}
.layout.no-sidebar ~ .sidebar-open-btn { display: block; }

/* Responsive */
@media (max-width: 768px) {
  .layout {
    grid-template-columns: 1fr;
  }
  .sidebar { display: none; }
  .sidebar-open-btn { display: block; }
  .bottom-bar { grid-column: 1; }
  .main-area { grid-column: 1; }
}

/* Sidebar overlay on mobile when opened */
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
  background: rgba(0, 0, 0, 0.5);
}
.sidebar-overlay .overlay-panel {
  position: absolute;
  top: 0;
  left: 0;
  width: 280px;
  height: 100%;
  background: var(--bg-secondary);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.sidebar-overlay .overlay-panel .sidebar-header,
.sidebar-overlay .overlay-panel .sidebar-body {
  padding: 16px;
}

/* Touch optimizations */
.touch-mode .chat-field { font-size: 16px; padding: 12px 16px; }
.touch-mode .btn-send { min-height: 44px; min-width: 44px; padding: 12px 22px; }
.touch-mode .btn-stop { min-height: 44px; min-width: 44px; padding: 12px 18px; }
.touch-mode .sidebar-open-btn { min-height: 44px; min-width: 44px; padding: 10px 12px; }
.touch-mode .sidebar-collapse-btn { min-height: 44px; min-width: 44px; }
.touch-mode .task-list li { padding: 10px 8px; }

@media (hover: none) and (pointer: coarse) {
  .chat-field { font-size: 16px; padding: 12px 16px; }
  .btn-send { min-height: 44px; min-width: 44px; }
  .btn-stop { min-height: 44px; min-width: 44px; }
  .sidebar-open-btn { min-height: 44px; min-width: 44px; }
}
</style>
</head>
<body>

<div id="app" class="layout">
  <!-- Sidebar -->
  <aside class="sidebar" id="sidebar">
    <div class="sidebar-header">
      <h1>{{TITLE2}}</h1>
      <button class="sidebar-collapse-btn" id="sidebarCollapseBtn" title="Collapse sidebar">&#x2715;</button>
    </div>
    <div class="sidebar-body">
      <div class="conn-status" id="connStatus">
        <span class="conn-dot disconnected" id="connDot"></span>
        <span id="connLabel">Disconnected</span>
      </div>
      <div class="section-label">Task History</div>
      <ul class="task-list" id="taskList"></ul>
    </div>
  </aside>

  <!-- Main content -->
  <main class="main-area" id="mainArea">
    <div class="empty-state" id="emptyState">
      <div class="empty-state-icon">&#x25CE;</div>
      <div class="empty-state-text">Waiting for UI...</div>
    </div>
  </main>

  <!-- Bottom bar -->
  <div class="bottom-bar" id="bottomBar">
    <input type="text" class="chat-field" id="chatInput" placeholder="Type a message..." disabled autocomplete="off">
    <button class="btn-send" id="btnSend" disabled>Send</button>
    <button class="btn-stop" id="btnStop" style="display:none;">&#x23F9; Stop</button>
  </div>
</div>

<!-- Sidebar open button (visible when sidebar is collapsed) -->
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

  // ---- Configuration ----
  var CONFIG = {
    wsURL: "{{WS_URL}}",
    darkMode: {{DARK_MODE}},
    showSidebar: {{SHOW_SIDEBAR}},
    touchMode: {{TOUCH_MODE}},
    emergencyStop: {{EMERGENCY_STOP}},
    pingInterval: 25000,
    reconnectBase: 1000,
    reconnectMax: 30000,
    feedbackTimeout: 60000,
    maxTaskHistory: 10,
    cacheKeyUI: "overhuman_last_ui",
    cacheKeyTasks: "overhuman_task_history"
  };

  // ---- State ----
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
    isCached: false
  };

  // ---- DOM refs ----
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
    connStatus: document.getElementById("connStatus")
  };

  // ---- Initialization ----
  function init() {
    // Apply touch mode
    if (CONFIG.touchMode || isTouchDevice()) {
      document.body.classList.add("touch-mode");
    }

    // Apply sidebar visibility
    if (!CONFIG.showSidebar) {
      dom.app.classList.add("no-sidebar");
    }

    // Show/hide emergency stop
    if (CONFIG.emergencyStop) {
      dom.btnStop.style.display = "";
    }

    // Load cached data
    loadTaskHistory();
    loadCachedUI();

    // Bind events
    bindEvents();

    // Start WebSocket
    connectWS();
  }

  function isTouchDevice() {
    return ("ontouchstart" in window) || (navigator.maxTouchPoints > 0);
  }

  // ---- Events ----
  function bindEvents() {
    // Sidebar collapse/open
    dom.sidebarCollapseBtn.addEventListener("click", function() {
      dom.app.classList.add("no-sidebar");
    });
    dom.sidebarOpenBtn.addEventListener("click", function() {
      if (window.innerWidth <= 768) {
        dom.sidebarOverlay.classList.add("visible");
      } else {
        dom.app.classList.remove("no-sidebar");
      }
    });
    dom.overlayBg.addEventListener("click", closeMobileOverlay);
    dom.overlayCloseBtn.addEventListener("click", closeMobileOverlay);

    // Chat input
    dom.chatInput.addEventListener("keydown", function(e) {
      if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        sendChatMessage();
      }
    });
    dom.btnSend.addEventListener("click", sendChatMessage);

    // Emergency stop
    dom.btnStop.addEventListener("click", sendEmergencyStop);

    // Listen for postMessage from sandboxed iframe
    window.addEventListener("message", handleIframeMessage);
  }

  function closeMobileOverlay() {
    dom.sidebarOverlay.classList.remove("visible");
  }

  // ---- WebSocket ----
  function connectWS() {
    if (state.ws) {
      try { state.ws.close(); } catch(e) {}
    }

    setConnectionStatus("reconnecting");

    try {
      state.ws = new WebSocket(CONFIG.wsURL);
    } catch(e) {
      scheduleReconnect();
      return;
    }

    state.ws.onopen = function() {
      state.connected = true;
      state.reconnectDelay = CONFIG.reconnectBase;
      setConnectionStatus("connected");
      dom.chatInput.disabled = false;
      dom.btnSend.disabled = false;
      startPing();
    };

    state.ws.onclose = function() {
      state.connected = false;
      setConnectionStatus("disconnected");
      dom.chatInput.disabled = true;
      dom.btnSend.disabled = true;
      stopPing();
      scheduleReconnect();
    };

    state.ws.onerror = function() {
      // onclose will fire after this
    };

    state.ws.onmessage = function(evt) {
      handleWSMessage(evt.data);
    };
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
    state.pingTimer = setInterval(function() {
      wsSend({ type: "ping", payload: {} });
    }, CONFIG.pingInterval);
  }

  function stopPing() {
    if (state.pingTimer) {
      clearInterval(state.pingTimer);
      state.pingTimer = null;
    }
  }

  function wsSend(msg) {
    if (state.ws && state.ws.readyState === WebSocket.OPEN) {
      state.ws.send(JSON.stringify(msg));
      return true;
    }
    return false;
  }

  // ---- Connection Status ----
  function setConnectionStatus(status) {
    var dotClass = "conn-dot " + status;
    var label = status === "connected" ? "Connected" :
                status === "reconnecting" ? "Reconnecting..." : "Disconnected";

    dom.connDot.className = dotClass;
    dom.connLabel.textContent = label;
    dom.connDotOverlay.className = dotClass;
    dom.connLabelOverlay.textContent = label;
  }

  // ---- Message Handling ----
  function handleWSMessage(raw) {
    var msg;
    try {
      msg = JSON.parse(raw);
    } catch(e) {
      return;
    }

    if (!msg || !msg.type) return;

    switch (msg.type) {
      case "ui_full":
        handleUIFull(msg.payload);
        break;
      case "ui_stream":
        handleUIStream(msg.payload);
        break;
      case "action_result":
        handleActionResult(msg.payload);
        break;
      case "error":
        handleError(msg.payload);
        break;
      case "pong":
        // keepalive acknowledged
        break;
    }
  }

  function handleUIFull(payload) {
    if (!payload) return;

    // Send feedback for previous task if needed
    maybeSendFeedback();

    // Reset feedback tracking
    var taskID = payload.task_id || "";
    state.currentTaskID = taskID;
    state.uiDeliveredAt = Date.now();
    state.firstActionTime = 0;
    state.actionsUsed = [];
    state.scrolled = false;
    state.feedbackSent = false;
    state.streamBuffer = "";
    state.isCached = false;

    // Add to task history
    if (taskID) {
      addTaskToHistory(taskID);
    }

    // Render HTML in sandbox
    renderSandboxedUI(payload.html || "");

    // Cache the payload
    cacheUI(payload);

    // Start feedback timer
    startFeedbackTimer();
  }

  function handleUIStream(payload) {
    if (!payload) return;
    state.streamBuffer += (payload.chunk || "");
    if (payload.done) {
      renderSandboxedUI(state.streamBuffer);
      state.streamBuffer = "";
    }
  }

  function handleActionResult(payload) {
    if (!payload) return;
    // Forward to iframe via postMessage
    var iframe = dom.mainArea.querySelector("iframe");
    if (iframe && iframe.contentWindow) {
      iframe.contentWindow.postMessage({
        type: "action_result",
        payload: payload
      }, "*");
    }
  }

  function handleError(payload) {
    if (!payload) return;
    var message = payload.message || "Unknown error";
    var code = payload.code || 0;
    var errorHTML = '<div style="padding:24px;color:#f85149;font-family:sans-serif;">' +
      '<h3 style="margin-bottom:8px;">Error' + (code ? ' (' + code + ')' : '') + '</h3>' +
      '<p>' + escapeHTML(message) + '</p></div>';
    renderSandboxedUI(errorHTML);
  }

  // ---- Sandboxed UI Rendering ----
  function renderSandboxedUI(html) {
    // Remove empty state
    if (dom.emptyState) {
      dom.emptyState.style.display = "none";
    }

    // Remove existing iframe
    var existing = dom.mainArea.querySelector("iframe");
    if (existing) {
      existing.remove();
    }

    // Build the iframe content with CSP meta tag and postMessage bridge
    var bridgeScript = '<script>' +
      'window.addEventListener("scroll", function() {' +
        'parent.postMessage({type:"iframe_scroll"}, "*");' +
      '});' +
      'document.addEventListener("click", function(e) {' +
        'var btn = e.target.closest("[data-action]");' +
        'if (btn) {' +
          'parent.postMessage({' +
            'type: "iframe_action",' +
            'actionId: btn.getAttribute("data-action"),' +
            'data: btn.getAttribute("data-payload") || "{}"' +
          '}, "*");' +
        '}' +
      '});' +
      'window.addEventListener("message", function(e) {' +
        'if (e.data && e.data.type === "action_result") {' +
          'var evt = new CustomEvent("actionResult", {detail: e.data.payload});' +
          'document.dispatchEvent(evt);' +
        '}' +
      '});' +
      '<\/script>';

    var fullHTML = '<!DOCTYPE html><html><head>' +
      '<meta charset="utf-8">' +
      '<meta http-equiv="Content-Security-Policy" content="default-src ' + "'none'" + '; style-src ' + "'unsafe-inline'" + '; script-src ' + "'unsafe-inline'" + '; img-src data:;">' +
      '<meta name="viewport" content="width=device-width, initial-scale=1">' +
      '<style>' +
        'html, body { margin: 0; padding: 0; background: #0d1117; color: #e6edf3; font-family: -apple-system, BlinkMacSystemFont, sans-serif; }' +
        'a { color: #00d4aa; }' +
        '[data-action] { cursor: pointer; }' +
      '</style>' +
      '</head><body>' + html + bridgeScript + '</body></html>';

    // Create sandboxed iframe
    var iframe = document.createElement("iframe");
    iframe.sandbox = "allow-scripts";
    iframe.style.width = "100%";
    iframe.style.height = "100%";
    iframe.style.border = "none";
    iframe.style.background = "var(--bg-primary)";
    iframe.srcdoc = fullHTML;

    dom.mainArea.appendChild(iframe);
  }

  // ---- Iframe postMessage Handler ----
  function handleIframeMessage(e) {
    if (!e.data || !e.data.type) return;

    switch (e.data.type) {
      case "iframe_action":
        trackAction(e.data.actionId);
        wsSend({
          type: "action",
          payload: {
            action_id: e.data.actionId,
            data: safeParseJSON(e.data.data)
          }
        });
        break;

      case "iframe_scroll":
        state.scrolled = true;
        break;
    }
  }

  // ---- Chat Input ----
  function sendChatMessage() {
    var text = dom.chatInput.value.trim();
    if (!text || !state.connected) return;
    wsSend({ type: "input", payload: { text: text } });
    dom.chatInput.value = "";
    dom.chatInput.focus();
  }

  // ---- Emergency Stop ----
  function sendEmergencyStop() {
    wsSend({ type: "cancel", payload: { reason: "user" } });

    // Visual pulse feedback
    dom.btnStop.classList.add("pulse");
    setTimeout(function() {
      dom.btnStop.classList.remove("pulse");
    }, 400);
  }

  // ---- UI Feedback ----
  function trackAction(actionId) {
    if (!state.firstActionTime && state.uiDeliveredAt) {
      state.firstActionTime = Date.now();
    }
    if (actionId && state.actionsUsed.indexOf(actionId) === -1) {
      state.actionsUsed.push(actionId);
    }
  }

  function startFeedbackTimer() {
    clearFeedbackTimer();
    state.feedbackTimer = setTimeout(function() {
      maybeSendFeedback();
    }, CONFIG.feedbackTimeout);
  }

  function clearFeedbackTimer() {
    if (state.feedbackTimer) {
      clearTimeout(state.feedbackTimer);
      state.feedbackTimer = null;
    }
  }

  function maybeSendFeedback() {
    if (state.feedbackSent || !state.currentTaskID) return;
    state.feedbackSent = true;
    clearFeedbackTimer();

    var timeToAction = 0;
    if (state.firstActionTime && state.uiDeliveredAt) {
      timeToAction = state.firstActionTime - state.uiDeliveredAt;
    }

    wsSend({
      type: "ui_feedback",
      payload: {
        task_id: state.currentTaskID,
        scrolled: state.scrolled,
        time_to_action_ms: timeToAction,
        actions_used: state.actionsUsed,
        dismissed: false
      }
    });
  }

  // ---- Task History (localStorage) ----
  function loadTaskHistory() {
    try {
      var raw = localStorage.getItem(CONFIG.cacheKeyTasks);
      if (raw) {
        state.taskHistory = JSON.parse(raw);
        if (!Array.isArray(state.taskHistory)) {
          state.taskHistory = [];
        }
      }
    } catch(e) {
      state.taskHistory = [];
    }
    renderTaskHistory();
  }

  function addTaskToHistory(taskID) {
    // Remove duplicate if exists
    var idx = state.taskHistory.indexOf(taskID);
    if (idx !== -1) {
      state.taskHistory.splice(idx, 1);
    }
    // Add to front
    state.taskHistory.unshift(taskID);
    // Trim to max
    if (state.taskHistory.length > CONFIG.maxTaskHistory) {
      state.taskHistory = state.taskHistory.slice(0, CONFIG.maxTaskHistory);
    }
    // Save
    try {
      localStorage.setItem(CONFIG.cacheKeyTasks, JSON.stringify(state.taskHistory));
    } catch(e) {}
    renderTaskHistory();
  }

  function renderTaskHistory() {
    var html = "";
    for (var i = 0; i < state.taskHistory.length; i++) {
      html += "<li title=\"" + escapeAttr(state.taskHistory[i]) + "\">" +
        escapeHTML(state.taskHistory[i]) + "</li>";
    }
    dom.taskList.innerHTML = html;
    dom.taskListOverlay.innerHTML = html;
  }

  // ---- Offline Cache (localStorage) ----
  function cacheUI(payload) {
    try {
      localStorage.setItem(CONFIG.cacheKeyUI, JSON.stringify(payload));
    } catch(e) {}
  }

  function loadCachedUI() {
    try {
      var raw = localStorage.getItem(CONFIG.cacheKeyUI);
      if (!raw) return;
      var payload = JSON.parse(raw);
      if (payload && payload.html) {
        state.isCached = true;
        state.currentTaskID = payload.task_id || "";
        renderSandboxedUI(payload.html);

        // Show cached badge in the connection status area
        var badge = document.createElement("span");
        badge.className = "cached-badge";
        badge.textContent = "Cached";
        badge.id = "cachedBadge";
        dom.connStatus.appendChild(badge);
      }
    } catch(e) {}
  }

  function removeCachedBadge() {
    var badge = document.getElementById("cachedBadge");
    if (badge) badge.remove();
  }

  // Remove cached badge when fresh UI arrives
  var origHandleUIFull = handleUIFull;
  handleUIFull = function(payload) {
    removeCachedBadge();
    origHandleUIFull(payload);
  };

  // ---- Utilities ----
  function escapeHTML(str) {
    var div = document.createElement("div");
    div.appendChild(document.createTextNode(str));
    return div.innerHTML;
  }

  function escapeAttr(str) {
    return str.replace(/&/g, "&amp;").replace(/"/g, "&quot;")
              .replace(/</g, "&lt;").replace(/>/g, "&gt;");
  }

  function safeParseJSON(str) {
    try { return JSON.parse(str); } catch(e) { return {}; }
  }

  // ---- Start ----
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
</script>
</body>
</html>`
