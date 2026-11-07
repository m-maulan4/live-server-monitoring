// SERVMON - Live Metrics (SSE)
// lucide optional — kalau CDN gagal load, ikon hilang tapi dashboard tetap jalan
if (window.lucide) { try { lucide.createIcons(); } catch (_) { } }

// ---- state ----
const MAX_BUFFER_POINTS = 30;
let isStreaming = true;
let packetCounter = 0;
let autoScrollTerminal = true;

const fifoLabels = [], fifoCpuData = [], fifoRamData = [], fifoNetData = [];
const sparkCpuData = Array(10).fill(0);
const sparkRamData = Array(10).fill(0);

let mainChart = null, miniSparkCpu = null, miniSparkRam = null;

// ---- helpers ----
const $cache = Object.create(null);
const $ = id => (id in $cache ? $cache[id] : ($cache[id] = document.getElementById(id)));
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

// warna badge interval: ok <1.5s, warn <3s, bad >3s (satu tick normal ~1s)
function setIntervalBadge(ms) {
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
    const ramPct = d.ram.pct || 0;

    // semua rate/agregat sudah dihitung server; FE hanya tampil
    const net = d.network || {};
    const rxMbps = net.rx_mbps || 0;
    const txMbps = net.tx_mbps || 0;
    const netMbps = net.net_mbps || 0;
    const totalRx = net.rx_bytes || 0;
    const totalTx = net.tx_bytes || 0;

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

    // storage (agregat sudah dihitung server)
    const st = d.storage || {};
    const stUsed = st.used || 0;
    const stTotal = st.total || 0;
    const stPct = st.pct || 0;
    $('kpi-storage-gb').textContent = (stUsed / 1e9).toFixed(1);
    $('kpi-storage-total').textContent = 'Total: ' + fmtB(stTotal);
    $('kpi-storage-pct').textContent = stPct.toFixed(0) + '% Used';
    $('kpi-storage-bar').style.width = stPct + '%';

    // zram (ratio/pct sudah dihitung server)
    const zram = d.zram || {};
    $('kpi-zram-used').textContent = fmtB(zram.used);
    $('kpi-zram-total').textContent = '/ ' + fmtB(zram.disk_size);
    $('kpi-zram-pct').textContent = (zram.pct || 0).toFixed(0) + '% Used';
    $('kpi-zram-ratio').textContent = (zram.ratio || 0).toFixed(1) + '%';
    $('kpi-zram-bar').style.width = (zram.pct || 0) + '%';

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

    // interval = gap kedatangan pesan SSE → tampil di badge header
    const interval = now - (window._prevArrive || now);
    window._prevArrive = now;
    $('ws-status-text').textContent = 'interval: ' + Math.round(interval) + 'ms';
    setIntervalBadge(interval);

    // process list (top 10 by RAM)
    renderProcesses(d.processes || []);

    // terminal logs
    appendLogs(d.logs || []);
}

function renderProcesses(procs) {
    const tbody = $('nodes-table-body');
    if (!procs.length) {
        const tr = document.createElement('tr');
        const td = document.createElement('td');
        td.colSpan = 4;
        td.className = 'py-2.5 px-3 text-slate-500';
        td.textContent = 'menunggu data proses…';
        tr.appendChild(td);
        tbody.replaceChildren(tr);
        return;
    }
    const frag = document.createDocumentFragment();
    procs.forEach(p => {
        const tr = document.createElement('tr');
        tr.className = 'hover:bg-slate-800/40 transition';
        const cells = [
            ['py-2.5 px-3 text-slate-500', p.pid],
            ['py-2.5 px-3 font-bold text-slate-200', p.name],
            ['py-2.5 px-3 text-right text-cyan-400', p.cpu.toFixed(1) + '%'],
            ['py-2.5 px-3 text-right text-violet-400', fmtB(p.ram)],
        ];
        cells.forEach(([c, v]) => {
            const td = document.createElement('td');
            td.className = c;
            td.textContent = v;
            tr.appendChild(td);
        });
        frag.appendChild(tr);
    });
    tbody.replaceChildren(frag);
}

const MAX_LOGS = 50;                 // maksimal log tampil di terminal
const seenCursors = new Set();       // dedupe log (server kirim cache saat journalctl gagal)

function appendLogs(logs) {
    const body = $('terminal-body');
    $('term-packet-count').textContent = `${packetCounter} packets`;
    const frag = document.createDocumentFragment();
    logs.forEach(l => {
        if (l.cursor) {
            if (seenCursors.has(l.cursor)) return; // sudah pernah tampil
            seenCursors.add(l.cursor);
        }
        const t = (l.time || '').slice(11) || '--:--:--';
        const badge = l.priority === '3' ? ['[ERR]', 'text-rose-400']
            : l.priority === '4' ? ['[WARN]', 'text-yellow-300']
                : ['[SYS]', 'text-emerald-400'];

        const div = document.createElement('div');
        div.className = 'hover:bg-slate-900/60 p-1.5 rounded border-b border-slate-800/50';

        const title = document.createElement('div');
        title.className = 'flex items-center gap-2 text-[10px] mb-0.5';

        const time = document.createElement('span');
        time.className = 'text-slate-500 shrink-0 font-mono';
        time.textContent = t;

        const bad = document.createElement('span');
        bad.className = badge[1] + ' font-bold';
        bad.textContent = badge[0];

        const unit = document.createElement('span');
        unit.className = 'text-slate-400 truncate font-mono';
        unit.textContent = l.unit || '';

        const msg = document.createElement('div');
        msg.className = 'text-slate-200 break-all leading-relaxed';
        msg.textContent = stripAnsi(l.message);

        title.append(time, bad, unit);
        div.append(title, msg);
        frag.appendChild(div);
    });
    body.appendChild(frag);
    while (body.children.length > MAX_LOGS) body.removeChild(body.firstChild);
    // cap cursor set juga (buang yang tak akan muncul lagi)
    if (seenCursors.size > MAX_LOGS * 4) {
        const keep = new Set(logs.map(l => l.cursor).filter(Boolean));
        seenCursors.forEach(c => { if (!keep.has(c)) seenCursors.delete(c); });
    }
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
    setIntervalBadge(2000);
};
es.onopen = () => {
    $('ws-status-text').textContent = 'interval: --ms';
    setIntervalBadge(0);
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
