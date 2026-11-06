// SERVMON - Live Metrics (SSE)
// lucide optional — kalau CDN gagal load, ikon hilang tapi dashboard tetap jalan
if (window.lucide) { try { lucide.createIcons(); } catch (_) {} }

// ---- state ----
const MAX_BUFFER_POINTS = 30;
let isStreaming = true;
let packetCounter = 0;
let autoScrollTerminal = true;
let lastSample = null;

const fifoLabels = [], fifoCpuData = [], fifoRamData = [], fifoNetData = [];
const sparkCpuData = Array(10).fill(0);
const sparkRamData = Array(10).fill(0);

let mainChart = null, miniSparkCpu = null, miniSparkRam = null;

// ---- helpers ----
const $ = id => document.getElementById(id);
const fmtB = n => {
    if (n == null) return '0 B';
    const u = ['B', 'KB', 'MB', 'GB', 'TB']; let i = 0;
    while (n >= 1024 && i < u.length - 1) { n /= 1024; i++; }
    return n.toFixed(i ? 1 : 0) + ' ' + u[i];
};
const fmtUpt = s => {
    const d = Math.floor(s / 86400), h = Math.floor(s % 86400 / 3600), m = Math.floor(s % 3600 / 60);
    return `${d}d ${h}h ${m}m`;
};
const stripAnsi = s => (s || '').replace(/[\u001b\u009b][[()#;?]*(?:[0-9]{1,4}(?:;[0-9]{0,4})*)?[0-9A-ORZcf-nqry=><]/g, '');

// warna badge latency: ok <1.5s, warn <3s, bad >3s (satu tick normal ~1s)
function setLatencyBadge(ms) {
    const lvl = ms < 1500 ? 'ok' : ms < 3000 ? 'warn' : 'bad';
    const c = {
        ok: { bg: 'bg-emerald-950/40 border-emerald-500/40', text: 'text-emerald-400', ping: 'bg-emerald-400', dot: 'bg-emerald-500', glow: 'glow-emerald' },
        warn: { bg: 'bg-amber-950/40 border-amber-500/40', text: 'text-amber-400', ping: 'bg-amber-400', dot: 'bg-amber-500', glow: '' },
        bad: { bg: 'bg-rose-950/40 border-rose-500/40', text: 'text-rose-400', ping: 'bg-rose-400', dot: 'bg-rose-500', glow: '' },
    }[lvl];
    $('ws-badge').className = (`flex items-center gap-2 border ${c.bg} px-3 py-1.5 rounded-full ${c.glow}`).replace(/\s+/g, ' ').trim();
    $('ws-status-text').className = `text-xs font-bold tracking-wider font-mono ${c.text}`;
    $('ws-ping-dot').className = `animate-pulse-fast absolute inline-flex h-full w-full rounded-full ${c.ping} opacity-75`;
    $('ws-dot').className = `relative inline-flex rounded-full h-2.5 w-2.5 ${c.dot}`;
}

// ---- clock ----
function updateClock() {
    const now = new Date();
    $('realtime-clock').textContent = now.toLocaleTimeString('id-ID', { hour12: false });
}
setInterval(updateClock, 1000);

// ---- charts ----
function initCharts() {
    // Chart.js wajib untuk grafik; kalau CDN gagal, graph kosong tapi KPI/terminal tetap hidup
    if (typeof Chart === 'undefined') { console.warn('Chart.js gagal load — cek koneksi/adblock'); return; }
    const canvas = $('liveFifoChart');
    if (!canvas) { console.warn('canvas #liveFifoChart tidak ada'); return; }
    const ctx = canvas.getContext('2d');
    const mk = c => { const g = ctx.createLinearGradient(0, 0, 0, 300); g.addColorStop(0, c + '59'); g.addColorStop(1, c + '00'); return g; };
    mainChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels: fifoLabels,
            datasets: [
                { label: 'CPU (%)', data: fifoCpuData, borderColor: '#06b6d4', backgroundColor: mk('#06b6d4'), borderWidth: 2.5, fill: true, tension: .35, pointRadius: 2 },
                { label: 'RAM (%)', data: fifoRamData, borderColor: '#8b5cf6', backgroundColor: mk('#8b5cf6'), borderWidth: 2.5, fill: true, tension: .35, pointRadius: 2 },
                { label: 'Network (Mbps)', data: fifoNetData, borderColor: '#10b981', backgroundColor: mk('#10b981'), borderWidth: 2.5, fill: true, tension: .35, pointRadius: 2 }
            ]
        },
        options: {
            responsive: true, maintainAspectRatio: false, animation: { duration: 300 },
            plugins: { legend: { display: false }, tooltip: { backgroundColor: '#0b1120', titleColor: '#06b6d4', bodyColor: '#f8fafc', borderColor: '#1e293b', borderWidth: 1 } },
            scales: {
                x: { grid: { color: 'rgba(30,41,59,.5)' }, ticks: { color: '#64748b' } },
                y: { min: 0, max: 100, grid: { color: 'rgba(30,41,59,.5)' }, ticks: { color: '#64748b', callback: v => v + '%' } }
            }
        }
    });

    const spark = (id, color) => {
        const c = $(id).getContext('2d');
        return new Chart(c, {
            type: 'line',
            data: { labels: Array(10).fill(''), datasets: [{ data: Array(10).fill(0), borderColor: color, borderWidth: 2, pointRadius: 0, fill: false, tension: .3 }] },
            options: { responsive: true, maintainAspectRatio: false, plugins: { legend: { display: false }, tooltip: { enabled: false } }, scales: { x: { display: false }, y: { display: false, min: 0, max: 100 } } }
        });
    };
    miniSparkCpu = spark('mini-sparkline-cpu', '#06b6d4');
    miniSparkRam = spark('mini-sparkline-ram', '#8b5cf6');
}

// ---- real data ingest (SSE) ----
function onData(d) {
    packetCounter++;
    const now = new Date();
    const timeTag = now.toLocaleTimeString('id-ID', { hour12: false });

    const cpu = d.cpu.usage || 0;
    const cores = d.cpu.cores || 0;
    const ramPct = d.ram.total ? (d.ram.used / d.ram.total * 100) : 0;

    // network rate (delta cumulative bytes / dt), excludes first sample
    let netMbps = 0, rxMbps = 0, txMbps = 0;
    const totalRx = (d.network || []).reduce((a, n) => a + (n.rx_bytes || 0), 0);
    const totalTx = (d.network || []).reduce((a, n) => a + (n.tx_bytes || 0), 0);
    if (lastSample) {
        const dt = (now - lastSample.t) / 1000;
        if (dt > 0) {
            rxMbps = Math.max(0, (totalRx - lastSample.rx) * 8 / dt / 1e6);
            txMbps = Math.max(0, (totalTx - lastSample.tx) * 8 / dt / 1e6);
            netMbps = rxMbps + txMbps;
        }
    }
    lastSample = { t: now, rx: totalRx, tx: totalTx };

    // FIFO
    fifoLabels.push(timeTag); fifoCpuData.push(cpu); fifoRamData.push(ramPct); fifoNetData.push(Math.min(100, netMbps));
    if (fifoLabels.length > MAX_BUFFER_POINTS) {
        fifoLabels.shift(); fifoCpuData.shift(); fifoRamData.shift(); fifoNetData.shift();
    }

    // sparklines (guard: chart mungkin belum siap)
    if (miniSparkCpu && miniSparkRam && mainChart) {
        sparkCpuData.shift(); sparkCpuData.push(cpu);
        sparkRamData.shift(); sparkRamData.push(ramPct);
        miniSparkCpu.data.datasets[0].data = sparkCpuData; miniSparkCpu.update('none');
        miniSparkRam.data.datasets[0].data = sparkRamData; miniSparkRam.update('none');
        mainChart.update('none');
    }

    // KPI cards
    $('kpi-cpu-value').textContent = cpu.toFixed(0) + '%';
    $('kpi-cpu-bar').style.width = cpu + '%';
    $('kpi-cpu-peak').textContent = Math.max(...fifoCpuData).toFixed(0) + '%';
    $('kpi-cpu-cores').textContent = cores + ' Cores';

    $('kpi-ram-gb').textContent = (d.ram.used / 1e9).toFixed(1);
    $('kpi-ram-total').textContent = 'GB / ' + fmtB(d.ram.total);
    $('kpi-ram-pct').textContent = ramPct.toFixed(0) + '% Used';
    $('kpi-ram-avail').textContent = 'Available: ' + fmtB(d.ram.available);
    $('kpi-ram-bar').style.width = ramPct + '%';

    // storage (agregat semua mount)
    const st = d.storage || [];
    const stUsed = st.reduce((a, s) => a + (s.used || 0), 0);
    const stTotal = st.reduce((a, s) => a + (s.total || 0), 0);
    const stPct = stTotal ? stUsed / stTotal * 100 : 0;
    $('kpi-storage-gb').textContent = (stUsed / 1e9).toFixed(1);
    $('kpi-storage-total').textContent = 'Total: ' + fmtB(stTotal);
    $('kpi-storage-pct').textContent = stPct.toFixed(0) + '% Used';
    $('kpi-storage-bar').style.width = stPct + '%';
    $('kpi-storage-mounts').textContent = 'mounts: ' + st.map(s => s.path).join(', ');

    // zram
    const zram = d.zram || {};
    const ratio = (zram.compressed && zram.original) ? (zram.original / zram.compressed) : 0;
    $('kpi-zram-comp').textContent = fmtB(zram.compressed) + ' / ' + fmtB(zram.disk_size);
    $('kpi-zram-ratio').textContent = (ratio ? ratio.toFixed(1) : '0') + 'x';
    // bar = % data terkompresi (compressed/original)
    $('kpi-zram-bar').style.width = (zram.original ? Math.min(100, zram.compressed / zram.original * 100) : 0) + '%';

    $('kpi-net-total').textContent = netMbps.toFixed(1);
    $('kpi-net-rx').textContent = rxMbps.toFixed(1) + ' Mbps';
    $('kpi-net-tx').textContent = txMbps.toFixed(1) + ' Mbps';
    // bar selalu terbagi 2 penuh (100%) sesuai proporsi RX : TX
    $('kpi-net-rx-bar').style.width = (netMbps > 0 ? rxMbps / netMbps * 100 : 0) + '%';
    $('kpi-net-tx-bar').style.width = (netMbps > 0 ? txMbps / netMbps * 100 : 0) + '%';

    // total traffic (akumulasi byte sejak boot)
    $('kpi-rx-total').textContent = fmtB(totalRx);
    $('kpi-tx-total').textContent = fmtB(totalTx);
    const totBytes = totalRx + totalTx;
    $('kpi-rx-total-bar').style.width = (totBytes > 0 ? totalRx / totBytes * 100 : 50) + '%';
    $('kpi-tx-total-bar').style.width = (totBytes > 0 ? totalTx / totBytes * 100 : 50) + '%';

    // system info widget
    $('info-hostname').textContent = d.server.hostname + ' · ' + (d.server.os || '-');
    $('info-cpumodel').textContent = d.server.cpu_model || '-';
    $('info-cores').textContent = (d.server.cores || 0) + ' cores · ' + (d.server.threads || 0) + ' threads';
    $('info-uptime').textContent = fmtUpt(d.server.uptime || 0);
    $('info-load').textContent = [d.server.load1, d.server.load5, d.server.load15].map(v => (v || 0).toFixed(2)).join(' / ');

    // avg stats
    $('buffer-count-badge').textContent = `${fifoLabels.length} / ${MAX_BUFFER_POINTS} pts`;

    // latency = message gap (approx) → tampil di badge header
    const latency = now - (window._prevArrive || now);
    window._prevArrive = now;
    $('ws-status-text').textContent = 'latency: ' + Math.round(latency) + 'ms';
    setLatencyBadge(latency);

    // process list (top 10 by RAM)
    renderProcesses(d.processes || []);

    // terminal logs
    appendLogs(d.logs || []);
}

function renderProcesses(procs) {
    const tbody = $('nodes-table-body');
    tbody.innerHTML = '';
    if (!procs.length) {
        tbody.innerHTML = '<tr><td colspan="4" class="py-2.5 px-3 text-slate-500">menunggu data proses…</td></tr>';
        return;
    }
    procs.forEach(p => {
        const tr = document.createElement('tr');
        tr.className = 'hover:bg-slate-800/40 transition';
        tr.innerHTML = `
            <td class="py-2.5 px-3 text-slate-500">${p.pid}</td>
            <td class="py-2.5 px-3 font-bold text-slate-200">${p.name}</td>
            <td class="py-2.5 px-3 text-right text-cyan-400">${p.cpu.toFixed(1)}%</td>
            <td class="py-2.5 px-3 text-right text-violet-400">${fmtB(p.ram)}</td>
        `;
        tbody.appendChild(tr);
    });
}

function appendLogs(logs) {
    const body = $('terminal-body');
    $('term-packet-count').textContent = `${packetCounter} packets`;
    logs.forEach(l => {
        const div = document.createElement('div');
        const t = (l.time || '').slice(11) || '--:--:--';
        const badge = l.priority === '3' ? '<span class="text-rose-400 font-bold">[ERR]</span>'
            : l.priority === '4' ? '<span class="text-yellow-300 font-bold">[WARN]</span>'
                : '<span class="text-emerald-400 font-bold">[SYS]</span>';
        div.className = 'hover:bg-slate-900/60 p-1.5 rounded border-b border-slate-800/50';
        div.innerHTML = `
            <div class="flex items-center gap-2 text-[10px] mb-0.5">
                <span class="text-slate-500 shrink-0 font-mono">${t}</span>${badge}
                <span class="text-slate-400 truncate font-mono">${l.unit || ''}</span>
            </div>
            <div class="text-slate-200 break-all leading-relaxed">${stripAnsi(l.message)}</div>
        `;
        body.appendChild(div);
    });
    while (body.children.length > 50) body.removeChild(body.firstChild);
    if (autoScrollTerminal) body.scrollTop = body.scrollHeight;
}

// ---- SSE ----
const es = new EventSource('/health');
es.onmessage = e => {
    if (!isStreaming) return;
    try {
        const d = JSON.parse(e.data);
        window._lastData = d;
        onData(d);
    } catch (err) { console.error('render error:', err); }
};
es.onerror = () => {
    $('ws-status-text').textContent = 'SSE RECONNECT';
    setLatencyBadge(2000);
};
es.onopen = () => {
    $('ws-status-text').textContent = 'latency: --ms';
    setLatencyBadge(0);
};

// ---- controls ----
// script ada di akhir <body> — DOM sudah siap, init langsung (tanpa nunggu DOMContentLoaded)
initCharts();

document.addEventListener('DOMContentLoaded', () => {
    $('btn-clear-terminal').addEventListener('click', () => { $('terminal-body').innerHTML = ''; });

    $('btn-autoscroll').addEventListener('click', () => {
        autoScrollTerminal = !autoScrollTerminal;
        const b = $('btn-autoscroll');
        b.textContent = autoScrollTerminal ? 'Auto-scroll ON' : 'Auto-scroll OFF';
        b.className = autoScrollTerminal
            ? 'text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded border border-emerald-500/30 font-semibold'
            : 'text-slate-400 bg-slate-800 px-2 py-0.5 rounded border border-slate-700';
    });
});
