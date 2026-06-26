const $ = (s) => document.querySelector(s);
const $$ = (s) => document.querySelectorAll(s);

let evtSource = null;
let metaTimer = null;
let metadata = null;

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

function showPage(name) {
  $$('.page').forEach(p => p.classList.remove('active'));
  $$('.nav-btn').forEach(b => b.classList.toggle('active', b.dataset.page === name));
  $(`#page-${name}`).classList.add('active');
  if (name === 'stats') loadStats();
  if (name === 'download') connectSSE();
}

$$('.nav-btn').forEach(btn => btn.addEventListener('click', () => showPage(btn.dataset.page)));

const urlInput = $('#url-input');
const startBtn = $('#start-btn');
const metaPreview = $('#meta-preview');
const checksumInput = $('#checksum-input');

urlInput.addEventListener('input', () => {
  clearTimeout(metaTimer);
  const v = urlInput.value.trim();
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
});

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
  } catch (e) {
    metaPreview.classList.remove('hidden');
    metaPreview.innerHTML = `<span style="color:var(--red)">${e.message}</span>`;
    startBtn.disabled = true;
    startBtn.className = 'start-btn disabled';
    startBtn.textContent = 'Invalid URL';
    metadata = null;
  }
}

startBtn.addEventListener('click', async () => {
  if (startBtn.disabled) return;
  const body = {
    url: urlInput.value.trim(),
    useThreads: $('#opt-threads').checked,
    checkIntegrity: $('#opt-integrity').checked,
    checksum: checksumInput.value.trim(),
    clearTarget: $('#opt-clear').checked,
  };
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
    showPage('download');
    updateDownloadUI(data);
  } catch (e) {
    alert(e.message);
    startBtn.disabled = false;
    startBtn.className = 'start-btn ready';
    startBtn.textContent = 'Start Download';
  }
});

function connectSSE() {
  if (evtSource) return;
  evtSource = new EventSource('/api/download/events');
  evtSource.onmessage = (e) => {
    const data = JSON.parse(e.data);
    if ($('#page-download').classList.contains('active')) updateDownloadUI(data);
    Charts.drawSpeedometer($('#speed-gauge'), data.speed || 0);
    const el = $('#gauge-speed');
    if (el) el.textContent = formatSpeed(data.speed || 0);
  };
}

function updateDownloadUI(d) {
  const status = $('#dl-status');
  status.textContent = (d.state || 'idle').toUpperCase();
  status.className = 'status-badge ' + (d.paused ? 'paused' : d.state);

  $('#dl-filename').textContent = d.filename || '—';
  const pct = d.percent || 0;
  $('#dl-percent').textContent = pct.toFixed(1) + '%';
  $('#dl-size').textContent = `${formatBytes(d.progress || 0)} / ${formatBytes(d.total || 0)}`;

  const track = $('#animal-track');
  const runner = $('#runner');
  const trackFill = $('#track-fill');
  if (track && runner && trackFill) {
    const pos = Math.min(Math.max(pct, 2), 98);
    runner.style.left = pos + '%';
    trackFill.style.width = pos + '%';
    const active = ['downloading', 'merging'].includes(d.state) && !d.paused;
    track.classList.toggle('running', active);
    track.classList.toggle('paused', d.paused);
  }

  $('#dl-speed').textContent = formatSpeed(d.speed || 0);
  $('#dl-elapsed').textContent = formatDuration(d.elapsed || 0);
  $('#dl-eta').textContent = d.eta > 0 ? formatDuration(d.eta) : '--:--';
  $('#dl-workers').textContent = d.workers || 0;

  const corrupt = d.state === 'corruption';
  const complete = d.state === 'completed';
  const active = ['downloading', 'paused', 'merging'].includes(d.state);

  $('#dl-controls').classList.toggle('hidden', !active);
  $('#corruption-panel').classList.toggle('hidden', !corrupt);
  $('#complete-panel').classList.toggle('hidden', !complete);

  if (corrupt) $('#corr-pct').textContent = (d.corruptionPercent || 0).toFixed(1);
  if (complete) {
    $('#meter-integrity').style.width = (d.integrity || 0) + '%';
    $('#meter-trust').style.width = (d.trust || 0) + '%';
    $('#val-integrity').textContent = (d.integrity || 0).toFixed(1) + '%';
    $('#val-trust').textContent = (d.trust || 0).toFixed(1) + '%';
    $('#dl-output').textContent = d.outputPath || '';
    if (track) track.classList.remove('running');
  }
}

$$('.ctrl-btn[data-action]').forEach(btn => {
  btn.addEventListener('click', async () => {
    await fetch('/api/download/control', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action: btn.dataset.action }),
    });
  });
});

$$('.ctrl-btn[data-recover]').forEach(btn => {
  btn.addEventListener('click', async () => {
    await fetch('/api/download/recover', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action: btn.dataset.recover }),
    });
    $('#corruption-panel').classList.add('hidden');
    $('#dl-controls').classList.remove('hidden');
  });
});

$('#back-home').addEventListener('click', () => showPage('home'));

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

    if (data.recent?.length) {
      Charts.drawSpeedometer($('#speed-gauge'), data.recent[data.recent.length - 1].speed || 0);
      $('#gauge-speed').textContent = formatSpeed(data.recent[data.recent.length - 1].speed || 0);
    }
  } catch {}
}

connectSSE();
Charts.drawSpeedometer($('#speed-gauge'), 0);
