const Charts = (() => {
  const MAX_SPEED = 150;

  function drawSpeedometer(canvas, speed) {
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    const w = canvas.width;
    const h = canvas.height;
    const cx = w / 2;
    const cy = h - 22;
    const r = Math.min(w * 0.36, h * 0.72);
    const start = Math.PI;
    const end = 2 * Math.PI;
    const span = end - start;

    ctx.clearRect(0, 0, w, h);

    ctx.fillStyle = '#0f172a';
    ctx.fillRect(0, 0, w, h);

    ctx.beginPath();
    ctx.arc(cx, cy, r + 20, 0, Math.PI * 2);
    const bezel = ctx.createRadialGradient(cx, cy - r * 0.3, r * 0.2, cx, cy, r + 22);
    bezel.addColorStop(0, '#334155');
    bezel.addColorStop(1, '#0f172a');
    ctx.fillStyle = bezel;
    ctx.fill();

    ctx.beginPath();
    ctx.arc(cx, cy, r + 10, start, end, false);
    ctx.strokeStyle = '#1e293b';
    ctx.lineWidth = 18;
    ctx.lineCap = 'round';
    ctx.stroke();

    const redAt = start + span * 0.82;
    ctx.beginPath();
    ctx.arc(cx, cy, r + 10, redAt, end, false);
    ctx.strokeStyle = 'rgba(239,68,68,.5)';
    ctx.lineWidth = 18;
    ctx.stroke();

    const pct = Math.min(Math.max(speed / MAX_SPEED, 0), 1);
    const needleAngle = start + span * pct;

    if (pct > 0.005) {
      const arcGrad = ctx.createLinearGradient(cx - r, cy - r, cx + r, cy - r);
      arcGrad.addColorStop(0, '#06b6d4');
      arcGrad.addColorStop(0.5, '#3b82f6');
      arcGrad.addColorStop(1, '#a855f7');
      ctx.beginPath();
      ctx.arc(cx, cy, r + 10, start, needleAngle, false);
      ctx.strokeStyle = arcGrad;
      ctx.lineWidth = 18;
      ctx.lineCap = 'round';
      ctx.shadowColor = '#06b6d4';
      ctx.shadowBlur = 12;
      ctx.stroke();
      ctx.shadowBlur = 0;
    }

    for (let i = 0; i <= 10; i++) {
      const t = i / 10;
      const ang = start + span * t;
      const major = i % 2 === 0;
      const inner = r + (major ? 4 : 10);
      const outer = r + 16;
      ctx.beginPath();
      ctx.moveTo(cx + Math.cos(ang) * inner, cy + Math.sin(ang) * inner);
      ctx.lineTo(cx + Math.cos(ang) * outer, cy + Math.sin(ang) * outer);
      ctx.strokeStyle = major ? '#94a3b8' : '#475569';
      ctx.lineWidth = major ? 2 : 1;
      ctx.stroke();
      if (major) {
        const labelR = r + 30;
        const tx = cx + Math.cos(ang) * labelR;
        const ty = cy + Math.sin(ang) * labelR;
        ctx.fillStyle = '#8ba3c7';
        ctx.font = '10px system-ui, sans-serif';
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
        ctx.fillText(String(Math.round(t * MAX_SPEED)), tx, ty);
      }
    }

    ctx.save();
    ctx.translate(cx, cy);
    ctx.rotate(needleAngle);
    ctx.beginPath();
    ctx.moveTo(-8, 0);
    ctx.lineTo(r + 4, 0);
    ctx.strokeStyle = '#ef4444';
    ctx.lineWidth = 3;
    ctx.lineCap = 'round';
    ctx.stroke();
    ctx.beginPath();
    ctx.arc(0, 0, 8, 0, Math.PI * 2);
    ctx.fillStyle = '#1e293b';
    ctx.fill();
    ctx.strokeStyle = '#64748b';
    ctx.lineWidth = 2;
    ctx.stroke();
    ctx.beginPath();
    ctx.arc(0, 0, 3, 0, Math.PI * 2);
    ctx.fillStyle = '#ef4444';
    ctx.fill();
    ctx.restore();

    const readoutY = cy - r * 0.35;
    ctx.fillStyle = '#e2e8f4';
    ctx.font = 'bold 24px system-ui, sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(speed < 0.01 ? '0.00' : speed.toFixed(2), cx, readoutY);
    ctx.fillStyle = '#06b6d4';
    ctx.font = '11px system-ui, sans-serif';
    ctx.fillText('MB/s', cx, readoutY + 18);
  }

  function drawRing(canvas, pct, label, color) {
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    const w = canvas.width, h = canvas.height;
    const cx = w / 2, cy = h / 2, r = 36;
    ctx.clearRect(0, 0, w, h);

    ctx.beginPath();
    ctx.arc(cx, cy, r, 0, Math.PI * 2);
    ctx.strokeStyle = '#243352';
    ctx.lineWidth = 8;
    ctx.stroke();

    if (pct > 0) {
      const grad = ctx.createLinearGradient(0, 0, w, h);
      grad.addColorStop(0, color);
      grad.addColorStop(1, '#06b6d4');
      ctx.beginPath();
      ctx.arc(cx, cy, r, -Math.PI / 2, -Math.PI / 2 + Math.PI * 2 * Math.min(pct, 1));
      ctx.strokeStyle = grad;
      ctx.lineWidth = 8;
      ctx.lineCap = 'round';
      ctx.shadowColor = color;
      ctx.shadowBlur = 8;
      ctx.stroke();
      ctx.shadowBlur = 0;
    }

    ctx.fillStyle = '#e2e8f4';
    ctx.font = 'bold 16px sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(Math.round(pct * 100) + '%', cx, cy - 4);
    ctx.fillStyle = '#8ba3c7';
    ctx.font = '8px sans-serif';
    ctx.fillText(label, cx, cy + 12);
  }

  function drawHistoryChart(canvas, recent) {
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    const w = canvas.parentElement.clientWidth - 32;
    canvas.width = w;
    const h = canvas.height;
    ctx.clearRect(0, 0, w, h);

    if (!recent.length) {
      ctx.fillStyle = '#64748b';
      ctx.font = '13px sans-serif';
      ctx.textAlign = 'center';
      ctx.fillText('No downloads yet', w / 2, h / 2);
      return;
    }

    const pad = { t: 28, r: 20, b: 32, l: 44 };
    const maxSpeed = Math.max(...recent.map(r => r.speed), 10);
    const chartW = w - pad.l - pad.r;
    const chartH = h - pad.t - pad.b;

    ctx.strokeStyle = '#2e4060';
    ctx.lineWidth = 1;
    for (let i = 0; i <= 4; i++) {
      const y = pad.t + (chartH / 4) * i;
      ctx.beginPath();
      ctx.moveTo(pad.l, y);
      ctx.lineTo(w - pad.r, y);
      ctx.stroke();
      ctx.fillStyle = '#64748b';
      ctx.font = '10px sans-serif';
      ctx.textAlign = 'right';
      ctx.fillText((maxSpeed * (1 - i / 4)).toFixed(0), pad.l - 6, y + 3);
    }

    const points = recent.map((r, i) => ({
      x: pad.l + (i / Math.max(recent.length - 1, 1)) * chartW,
      y: pad.t + chartH - (r.speed / maxSpeed) * chartH,
    }));

    const areaGrad = ctx.createLinearGradient(0, pad.t, 0, h - pad.b);
    areaGrad.addColorStop(0, 'rgba(6,182,212,.45)');
    areaGrad.addColorStop(1, 'rgba(6,182,212,0)');

    ctx.beginPath();
    ctx.moveTo(points[0].x, h - pad.b);
    points.forEach(p => ctx.lineTo(p.x, p.y));
    ctx.lineTo(points[points.length - 1].x, h - pad.b);
    ctx.closePath();
    ctx.fillStyle = areaGrad;
    ctx.fill();

    const lineGrad = ctx.createLinearGradient(pad.l, 0, w - pad.r, 0);
    lineGrad.addColorStop(0, '#3b82f6');
    lineGrad.addColorStop(1, '#06b6d4');

    ctx.beginPath();
    points.forEach((p, i) => i === 0 ? ctx.moveTo(p.x, p.y) : ctx.lineTo(p.x, p.y));
    ctx.strokeStyle = lineGrad;
    ctx.lineWidth = 3;
    ctx.shadowColor = '#06b6d4';
    ctx.shadowBlur = 10;
    ctx.stroke();
    ctx.shadowBlur = 0;

    points.forEach(p => {
      ctx.beginPath();
      ctx.arc(p.x, p.y, 5, 0, Math.PI * 2);
      ctx.fillStyle = '#06b6d4';
      ctx.fill();
      ctx.strokeStyle = '#e2e8f4';
      ctx.lineWidth = 2;
      ctx.stroke();
    });

    ctx.fillStyle = '#8ba3c7';
    ctx.font = '11px sans-serif';
    ctx.textAlign = 'left';
    ctx.fillText('Download speed over time (MB/s)', pad.l, 18);
  }

  function drawTrustRadar(canvas, recent) {
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    const w = canvas.parentElement.clientWidth - 32;
    canvas.width = w;
    const h = canvas.height;
    ctx.clearRect(0, 0, w, h);

    if (!recent.length) {
      ctx.fillStyle = '#64748b';
      ctx.font = '13px sans-serif';
      ctx.textAlign = 'center';
      ctx.fillText('No integrity data yet', w / 2, h / 2);
      return;
    }

    const pad = 40;
    const barW = Math.max(24, (w - pad * 2) / recent.length - 8);
    const chartH = h - pad - 30;

    recent.forEach((r, i) => {
      const x = pad + i * (barW + 8);
      const iH = (r.integrity / 100) * chartH;
      const tH = (r.trust / 100) * chartH;

      const iGrad = ctx.createLinearGradient(x, h - pad - iH, x, h - pad);
      iGrad.addColorStop(0, '#22c55e');
      iGrad.addColorStop(1, 'rgba(34,197,94,.3)');
      ctx.fillStyle = iGrad;
      ctx.beginPath();
      ctx.roundRect(x, h - pad - iH, barW * 0.45, iH, [4, 4, 0, 0]);
      ctx.fill();

      const tGrad = ctx.createLinearGradient(x, h - pad - tH, x, h - pad);
      tGrad.addColorStop(0, '#8b5cf6');
      tGrad.addColorStop(1, 'rgba(139,92,246,.3)');
      ctx.fillStyle = tGrad;
      ctx.beginPath();
      ctx.roundRect(x + barW * 0.55, h - pad - tH, barW * 0.45, tH, [4, 4, 0, 0]);
      ctx.fill();

      ctx.fillStyle = '#64748b';
      ctx.font = '9px sans-serif';
      ctx.textAlign = 'center';
      ctx.fillText('#' + (i + 1), x + barW / 2, h - 12);
    });

    ctx.fillStyle = '#8ba3c7';
    ctx.font = '11px sans-serif';
    ctx.textAlign = 'left';
    ctx.fillText('■ Integrity   ■ Trust', pad, 18);
  }

  return { drawSpeedometer, drawRing, drawHistoryChart, drawTrustRadar, MAX_SPEED };
})();
