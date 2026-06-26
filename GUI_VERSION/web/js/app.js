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

$$('.nav-btn').forEach(btn => {
  btn.addEventListener('click', () => showPage(btn.dataset.page));
});

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
    if ($('#page-download').classList.contains('active')) {
      updateDownloadUI(data);
    }
    updateGaugeSpeed(data.speed || 0);
  };
}

function updateDownloadUI(d) {
  const status = $('#dl-status');
  status.textContent = (d.state || 'idle').toUpperCase();
  status.className = 'status-badge ' + (d.paused ? 'paused' : d.state);

  $('#dl-filename').textContent = d.filename || '—';
  $('#dl-bar').style.width = (d.percent || 0) + '%';
  $('#dl-percent').textContent = (d.percent || 0).toFixed(1) + '%';
  $('#dl-size').textContent = `${formatBytes(d.progress || 0)} / ${formatBytes(d.total || 0)}`;
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

  if (corrupt) {
    $('#corr-pct').textContent = (d.corruptionPercent || 0).toFixed(1);
  }
  if (complete) {
    $('#meter-integrity').style.width = (d.integrity || 0) + '%';
    $('#meter-trust').style.width = (d.trust || 0) + '%';
    $('#val-integrity').textContent = (d.integrity || 0).toFixed(1) + '%';
    $('#val-trust').textContent = (d.trust || 0).toFixed(1) + '%';
    $('#dl-output').textContent = d.outputPath || '';
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

function drawGauge(speed) {
  const canvas = $('#speed-gauge');
  if (!canvas) return;
  const ctx = canvas.getContext('2d');
  const w = canvas.width, h = canvas.height;
  const cx = w / 2, cy = h / 2 + 10;
  const r = 80;
  ctx.clearRect(0, 0, w, h);

  ctx.beginPath();
  ctx.arc(cx, cy, r, Math.PI, 2 * Math.PI);
  ctx.strokeStyle = '#243352';
  ctx.lineWidth = 14;
  ctx.stroke();

  const max = 100;
  const pct = Math.min(speed / max, 1);
  const grad = ctx.createLinearGradient(cx - r, cy, cx + r, cy);
  grad.addColorStop(0, '#3b82f6');
  grad.addColorStop(1, '#06b6d4');

  ctx.beginPath();
  ctx.arc(cx, cy, r, Math.PI, Math.PI + Math.PI * pct);
  ctx.strokeStyle = grad;
  ctx.lineWidth = 14;
  ctx.lineCap = 'round';
  ctx.stroke();

  ctx.fillStyle = '#8ba3c7';
  ctx.font = '13px sans-serif';
  ctx.textAlign = 'center';
  ctx.fillText('0', cx - r + 10, cy + 20);
  ctx.fillText('100', cx + r - 14, cy + 20);
}

function updateGaugeSpeed(speed) {
  drawGauge(speed);
  const el = $('#gauge-speed');
  if (el) el.textContent = formatSpeed(speed);
}

async function loadStats() {
  try {
    const res = await fetch('/api/stats');
    const data = await res.json();
    $('#st-total').textContent = data.totalDownloads || 0;
    $('#st-bytes').textContent = formatBytes(data.totalBytes || 0);
    $('#st-time').textContent = formatDuration(data.totalDuration || 0);
    $('#st-avg').textContent = formatSpeed(data.avgSpeed || 0);
    drawHistoryChart(data.recent || []);
    drawTrustChart(data.recent || []);
    if (data.recent && data.recent.length) {
      updateGaugeSpeed(data.recent[data.recent.length - 1].speed || 0);
    }
  } catch {}
}

function drawHistoryChart(recent) {
  const canvas = $('#history-chart');
  if (!canvas) return;
  const ctx = canvas.getContext('2d');
  const w = canvas.parentElement.clientWidth - 32;
  canvas.width = w;
  const h = canvas.height;
  ctx.clearRect(0, 0, w, h);
  if (!recent.length) return;

  const pad = 30;
  const maxSpeed = Math.max(...recent.map(r => r.speed), 1);
  const barW = Math.max(20, (w - pad * 2) / recent.length - 6);

  recent.forEach((r, i) => {
    const bh = ((r.speed / maxSpeed) * (h - pad * 2));
    const x = pad + i * (barW + 6);
    const y = h - pad - bh;
    const grad = ctx.createLinearGradient(x, y, x, h - pad);
    grad.addColorStop(0, '#06b6d4');
    grad.addColorStop(1, '#3b82f6');
    ctx.fillStyle = grad;
    ctx.beginPath();
    ctx.roundRect(x, y, barW, bh, 4);
    ctx.fill();
  });

  ctx.fillStyle = '#8ba3c7';
  ctx.font = '11px sans-serif';
  ctx.fillText('Speed per download (MB/s)', pad, 16);
}

function drawTrustChart(recent) {
  const canvas = $('#trust-chart');
  if (!canvas) return;
  const ctx = canvas.getContext('2d');
  const w = canvas.parentElement.clientWidth - 32;
  canvas.width = w;
  const h = canvas.height;
  ctx.clearRect(0, 0, w, h);
  if (!recent.length) return;

  const pad = 30;
  const barW = Math.max(20, (w - pad * 2) / recent.length - 6);

  recent.forEach((r, i) => {
    const x = pad + i * (barW + 6);
    const ih = ((r.integrity / 100) * (h - pad * 2));
    const th = ((r.trust / 100) * (h - pad * 2));
    ctx.fillStyle = 'rgba(34,197,94,.7)';
    ctx.fillRect(x, h - pad - ih, barW / 2 - 1, ih);
    ctx.fillStyle = 'rgba(59,130,246,.7)';
    ctx.fillRect(x + barW / 2 + 1, h - pad - th, barW / 2 - 1, th);
  });

  ctx.fillStyle = '#8ba3c7';
  ctx.font = '11px sans-serif';
  ctx.fillText('■ Integrity  ■ Trust', pad, 16);
}

connectSSE();
drawGauge(0);
