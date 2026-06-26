const $ = (s) => document.querySelector(s);
const $$ = (s) => document.querySelectorAll(s);

let evtSource = null;
let metaTimer = null;
let metadata = null;
let lastDownloadState = null;
let homeRestoring = false;
let downloadActive = false;

const homeState = {
  url: '',
  metaHtml: '',
  metaVisible: false,
  btnReady: false,
  threads: true,
  integrity: false,
  clear: false,
  checksum: '',
};

function formatBytes(n) {
  if (n < 0) n = 0;
  const u = ['B', 'KB', 'MB', 'GB', 'TB'];
  let s = n, i = 0;
  while (s >= 1024 && i < u.length - 1) { s /= 1024; i++; }
  return i === 0 ? `${n} ${u[0]}` : `${s.toFixed(2)} ${u[i]}`;
}

function formatDuration(sec) {
  if (sec < 0) sec = 0;
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  const s = Math.floor(sec % 60);
  if (h > 0) return `${String(h).padStart(2,'0')}:${String(m).padStart(2,'0')}:${String(s).padStart(2,'0')}`;
  return `${String(m).padStart(2,'0')}:${String(s).padStart(2,'0')}`;
}

function formatSpeed(mbps) {
  return `${mbps < 0.01 ? '0.00' : mbps.toFixed(2)} MB/s`;
}

function isValidURL(str) {
  try {
    const u = new URL(str);
    return u.protocol === 'http:' || u.protocol === 'https:';
  } catch { return false; }
}

function saveHomeState() {
  homeState.url = urlInput.value;
  homeState.metaHtml = metaPreview.innerHTML;
  homeState.metaVisible = !metaPreview.classList.contains('hidden');
  homeState.btnReady = startBtn.classList.contains('ready');
  homeState.threads = $('#opt-threads').checked;
  homeState.integrity = $('#opt-integrity').checked;
  homeState.clear = $('#opt-clear').checked;
  homeState.checksum = checksumInput.value;
}

function restoreHomeState() {
  homeRestoring = true;
  urlInput.value = homeState.url;
  $('#opt-threads').checked = homeState.threads;
  $('#opt-integrity').checked = homeState.integrity;
  $('#opt-clear').checked = homeState.clear;
  checksumInput.value = homeState.checksum;
  checksumInput.classList.toggle('hidden', !homeState.integrity);

  if (homeState.metaVisible && homeState.metaHtml) {
    metaPreview.innerHTML = homeState.metaHtml;
    metaPreview.classList.remove('hidden');
  } else {
    metaPreview.classList.add('hidden');
  }

  if (downloadActive) {
    startBtn.disabled = true;
    startBtn.className = 'start-btn disabled';
    const label = lastDownloadState?.paused ? 'Download paused' : 'Download in progress';
    startBtn.textContent = label;
  } else if (homeState.btnReady && isValidURL(homeState.url)) {
    startBtn.disabled = false;
    startBtn.className = 'start-btn ready';
    startBtn.textContent = 'Start Download';
  } else if (homeState.url && isValidURL(homeState.url)) {
    startBtn.disabled = true;
    startBtn.className = 'start-btn disabled';
    startBtn.textContent = 'Checking...';
    metadata = null;
    clearTimeout(metaTimer);
    metaTimer = setTimeout(() => fetchMeta(homeState.url), 200);
  } else {
    startBtn.disabled = true;
    startBtn.className = 'start-btn disabled';
    startBtn.textContent = 'Enter a valid URL';
  }
  homeRestoring = false;
}

function isSessionActive(state) {
  return ['downloading', 'paused', 'merging', 'corruption'].includes(state);
}

function isLiveDownload(state, paused) {
  return (state === 'downloading' || state === 'merging') && !paused;
}

let metricsSnapshot = null;

function snapshotFrom(d) {
  return {
    percent: d.percent || 0,
    progress: d.progress || 0,
    total: d.total || 0,
    speed: d.speed || 0,
    elapsed: d.elapsed || 0,
    eta: d.eta || 0,
    workers: d.workers || 0,
    filename: d.filename || '—',
  };
}

const idleMetrics = {
  percent: 0, progress: 0, total: 0, speed: 0, elapsed: 0, eta: 0, workers: 0, filename: '—',
};

function applyMetrics(m) {
  $('#dl-percent').textContent = m.percent.toFixed(1) + '%';
  $('#dl-size').textContent = `${formatBytes(m.progress)} / ${formatBytes(m.total)}`;
  $('#dl-speed').textContent = formatSpeed(m.speed);
  $('#dl-elapsed').textContent = formatDuration(m.elapsed);
  $('#dl-eta').textContent = m.eta > 0 ? formatDuration(m.eta) : '--:--';
  $('#dl-workers').textContent = m.workers || 0;
  $('#dl-filename').textContent = m.filename;
  const track = $('#animal-track');
  const runner = $('#runner');
  const trackFill = $('#track-fill');
  if (track && runner && trackFill) {
    const pos = Math.min(Math.max(m.percent, 2), 98);
    runner.style.left = pos + '%';
    trackFill.style.width = m.percent + '%';
  }
}

function setBadge(state, paused) {
  let badge = state || 'idle';
  if (paused && badge === 'downloading') badge = 'paused';
  $('#dl-status').textContent = badge.toUpperCase();
  $('#dl-status').className = 'status-badge ' + (paused ? 'paused' : badge);
}

function setAnimalRunning(running, paused) {
  const track = $('#animal-track');
  if (!track) return;
  track.classList.toggle('running', running);
  track.classList.toggle('paused', paused && !running);
}

function idleDownloadUI() {
  metricsSnapshot = null;
  applyMetrics(idleMetrics);
  setBadge('idle');
  setAnimalRunning(false, false);
  $('#dl-controls').classList.add('hidden');
  $('#corruption-panel').classList.add('hidden');
  $('#complete-panel').classList.add('hidden');
}

function updateChromeOnly(d) {
  const paused = d.paused || d.state === 'paused';
  setBadge(d.state, paused);
  if (d.filename) $('#dl-filename').textContent = d.filename;
  setAnimalRunning(false, paused || d.state === 'paused');
  const showControls = ['downloading', 'paused', 'merging'].includes(d.state);
  $('#dl-controls').classList.toggle('hidden', !showControls);
}

function showPage(name) {
  $$('.page').forEach(p => p.classList.remove('active'));
  $$('.nav-btn').forEach(b => b.classList.toggle('active', b.dataset.page === name));
  $(`#page-${name}`).classList.add('active');
  if (name === 'home') restoreHomeState();
  if (name === 'stats') loadStats();
  if (name === 'download') {
    connectSSE();
    if (lastDownloadState) updateDownloadUI(lastDownloadState, true);
    else idleDownloadUI();
  }
}

$$('.nav-btn').forEach(btn => btn.addEventListener('click', () => showPage(btn.dataset.page)));

const urlInput = $('#url-input');
const startBtn = $('#start-btn');
const metaPreview = $('#meta-preview');
const checksumInput = $('#checksum-input');

urlInput.addEventListener('input', () => {
  if (homeRestoring || downloadActive) return;
  clearTimeout(metaTimer);
  const v = urlInput.value.trim();
  saveHomeState();
  if (!isValidURL(v)) {
    startBtn.disabled = true;
    startBtn.className = 'start-btn disabled';
    startBtn.textContent = 'Enter a valid URL';
    metaPreview.classList.add('hidden');
    metadata = null;
    return;
  }
  startBtn.disabled = true;
  startBtn.className = 'start-btn disabled';
  startBtn.textContent = 'Checking...';
  metaTimer = setTimeout(() => fetchMeta(v), 400);
});

$('#opt-integrity').addEventListener('change', (e) => {
  checksumInput.classList.toggle('hidden', !e.target.checked);
  saveHomeState();
});

$('#opt-threads').addEventListener('change', saveHomeState);
$('#opt-clear').addEventListener('change', saveHomeState);
checksumInput.addEventListener('input', saveHomeState);

async function fetchMeta(url) {
  try {
    const res = await fetch('/api/metadata', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url }),
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Failed');
    metadata = data;
    metaPreview.classList.remove('hidden');
    metaPreview.innerHTML = `<strong>${data.filename}</strong> — ${formatBytes(data.size)}` +
      (data.supportsRange ? ' · Multi-thread available' : ' · Sequential only');
    startBtn.disabled = false;
    startBtn.className = 'start-btn ready';
    startBtn.textContent = 'Start Download';
    saveHomeState();
    homeState.btnReady = true;
  } catch (e) {
    if (downloadActive) return;
    metaPreview.classList.remove('hidden');
    metaPreview.innerHTML = `<span style="color:var(--red)">${e.message}</span>`;
    startBtn.disabled = true;
    startBtn.className = 'start-btn disabled';
    startBtn.textContent = 'Invalid URL';
    metadata = null;
    saveHomeState();
  }
}

startBtn.addEventListener('click', async () => {
  if (startBtn.disabled || downloadActive) return;
  const body = {
    url: urlInput.value.trim(),
    useThreads: $('#opt-threads').checked,
    checkIntegrity: $('#opt-integrity').checked,
    checksum: checksumInput.value.trim(),
    clearTarget: $('#opt-clear').checked,
  };
  saveHomeState();
  startBtn.disabled = true;
  startBtn.textContent = 'Starting...';
  try {
    const res = await fetch('/api/download/start', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Failed');
    downloadActive = true;
    lastDownloadState = data;
    showPage('download');
    updateDownloadUI(data);
  } catch (e) {
    alert(e.message);
    restoreHomeState();
  }
});

function connectSSE() {
  if (evtSource) return;
  evtSource = new EventSource('/api/download/events');
  evtSource.onmessage = (e) => {
    const data = JSON.parse(e.data);
    if (data.state === 'idle' && lastDownloadState && isSessionActive(lastDownloadState.state)) {
      return;
    }
    if (data.state !== 'idle' || data.filename) {
      lastDownloadState = data;
    }
    const wasActive = downloadActive;
    downloadActive = isSessionActive(data.state);
    if (wasActive && !downloadActive && ['stopped', 'completed', 'failed'].includes(data.state)) {
      if ($('#page-home').classList.contains('active')) restoreHomeState();
    }
    if ($('#page-download').classList.contains('active')) {
      const d = lastDownloadState || data;
      if (isLiveDownload(d.state, d.paused)) {
        updateDownloadUI(d);
      } else if (d.state === 'completed' || d.state === 'corruption') {
        updateDownloadUI(d);
      } else if (metricsSnapshot && (d.paused || d.state === 'paused' || d.state === 'stopped')) {
        updateChromeOnly(d);
      } else if (d.state === 'idle' || !d.filename) {
        idleDownloadUI();
      } else {
        updateDownloadUI(d);
      }
    }
    const speed = (lastDownloadState || data).speed || 0;
    if ($('#page-stats').classList.contains('active') || speed > 0) {
      Charts.drawSpeedometer($('#speed-gauge'), speed);
      const el = $('#gauge-speed');
      if (el) el.textContent = formatSpeed(speed);
    }
  };
  evtSource.onerror = () => {
    if (evtSource.readyState === EventSource.CLOSED) {
      evtSource = null;
      setTimeout(connectSSE, 1000);
    }
  };
}

function updateDownloadUI(d, fromNav) {
  if (!d) { idleDownloadUI(); return; }

  if (d.state === 'completed') {
    metricsSnapshot = snapshotFrom({ ...d, percent: 100, progress: d.total, eta: 0 });
    applyMetrics(metricsSnapshot);
    setBadge('completed');
    setAnimalRunning(false, false);
    $('#dl-controls').classList.add('hidden');
    $('#corruption-panel').classList.add('hidden');
    $('#complete-panel').classList.remove('hidden');
    $('#meter-integrity').style.width = (d.integrity || 0) + '%';
    $('#meter-trust').style.width = (d.trust || 0) + '%';
    $('#val-integrity').textContent = (d.integrity || 0).toFixed(1) + '%';
    $('#val-trust').textContent = (d.trust || 0).toFixed(1) + '%';
    $('#dl-output').textContent = d.outputPath || '';
    return;
  }

  if (d.state === 'corruption') {
    if (!metricsSnapshot) metricsSnapshot = snapshotFrom(d);
    applyMetrics(metricsSnapshot);
    setBadge('corruption');
    setAnimalRunning(false, false);
    $('#dl-filename').textContent = d.filename || '—';
    $('#corruption-panel').classList.remove('hidden');
    $('#dl-controls').classList.add('hidden');
    $('#complete-panel').classList.add('hidden');
    $('#corr-pct').textContent = (d.corruptionPercent || 0).toFixed(1);
    return;
  }

  if (isLiveDownload(d.state, d.paused)) {
    metricsSnapshot = snapshotFrom(d);
    applyMetrics(metricsSnapshot);
    setBadge('downloading');
    setAnimalRunning(true, false);
    $('#dl-controls').classList.remove('hidden');
    $('#corruption-panel').classList.add('hidden');
    $('#complete-panel').classList.add('hidden');
    return;
  }

  if (d.paused || d.state === 'paused' || d.state === 'stopped') {
    if (!fromNav || !metricsSnapshot) metricsSnapshot = snapshotFrom(d);
    applyMetrics(metricsSnapshot);
    updateChromeOnly(d);
    $('#corruption-panel').classList.add('hidden');
    $('#complete-panel').classList.add('hidden');
    if (d.state === 'stopped') $('#dl-controls').classList.add('hidden');
    return;
  }

  idleDownloadUI();
}

async function sendControl(action) {
  try {
    const res = await fetch('/api/download/control', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action }),
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Failed');
    lastDownloadState = data;
    downloadActive = isSessionActive(data.state);
    updateDownloadUI(data);
    if ($('#page-home').classList.contains('active')) restoreHomeState();
  } catch (e) {
    console.error(e);
  }
}

$$('.ctrl-btn[data-action]').forEach(btn => {
  btn.addEventListener('click', () => sendControl(btn.dataset.action));
});

$$('.ctrl-btn[data-recover]').forEach(btn => {
  btn.addEventListener('click', async () => {
    try {
      const res = await fetch('/api/download/recover', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action: btn.dataset.recover }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed');
      lastDownloadState = data;
      downloadActive = true;
      updateDownloadUI(data);
      $('#corruption-panel').classList.add('hidden');
      $('#dl-controls').classList.remove('hidden');
    } catch (e) {
      console.error(e);
    }
  });
});

$('#back-home').addEventListener('click', () => {
  saveHomeState();
  showPage('home');
});

async function loadStats() {
  try {
    const res = await fetch('/api/stats');
    const data = await res.json();
    $('#st-total').textContent = data.totalDownloads || 0;
    $('#st-bytes').textContent = formatBytes(data.totalBytes || 0);
    $('#st-time').textContent = formatDuration(data.totalDuration || 0);
    $('#st-avg').textContent = formatSpeed(data.avgSpeed || 0);

    const maxDl = 20;
    Charts.drawRing($('#ring-downloads'), Math.min((data.totalDownloads || 0) / maxDl, 1), 'DL', '#3b82f6');
    const maxGB = 10 * 1024 * 1024 * 1024;
    Charts.drawRing($('#ring-data'), Math.min((data.totalBytes || 0) / maxGB, 1), 'DATA', '#06b6d4');
    Charts.drawHistoryChart($('#history-chart'), data.recent || []);
    Charts.drawTrustRadar($('#trust-chart'), data.recent || []);

    const liveSpeed = lastDownloadState?.speed ?? data.recent?.[data.recent.length - 1]?.speed ?? 0;
    Charts.drawSpeedometer($('#speed-gauge'), liveSpeed);
    $('#gauge-speed').textContent = formatSpeed(liveSpeed);
  } catch {}
}

connectSSE();
idleDownloadUI();
