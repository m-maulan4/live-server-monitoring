// SERVMON — single-server health monitor (SSE)
// lucide optional: kalau CDN gagal, ikon hilang tapi dashboard tetap jalan.
if (window.lucide) { try { lucide.createIcons(); } catch (_) { } }

// ---- helpers ----
const $ = id => document.getElementById(id);
const fmtB = n => {
    if (n == null) return '0 B';
    const u = ['B', 'KB', 'MB', 'GB', 'TB']; let i = 0;
    while (n >= 1024 && i < u.length - 1) { n /= 1024; i++; }
    return n.toFixed(i ? 1 : 0) + ' ' + u[i];
};
const fmtUp = s => `${Math.floor(s / 86400)}d ${Math.floor(s % 86400 / 3600)}h`;
const stripAnsi = s => (s || '').replace(/[\u001b\u009b][[()#;?]*(?:[0-9]{1,4}(?:;[0-9]{0,4})*)?[0-9A-ORZcf-nqry=><]/g, '');
// gauge ring: atur panjang busur dari nilai 0-100
const setRing = (el, pct) => {
    const C = parseFloat(el.getAttribute('stroke-dasharray'));
    el.style.strokeDashoffset = C * (1 - Math.max(0, Math.min(100, pct)) / 100);
};
const setBullet = (el, pct) => { el.style.width = Math.max(0, Math.min(100, pct)) + '%'; };

// ---- stream chart ----
const WIN = 30;
const cpuSeries = [], memSeries = [], labels = [];
let streamChart = null;
function initStream() {
    if (typeof Chart === 'undefined') { console.warn('Chart.js gagal load — graph kosong, KPI tetap hidup'); return; }
    const ctx = $('streamChart').getContext('2d');
    const grad = c => { const g = ctx.createLinearGradient(0, 0, 0, 260); g.addColorStop(0, c + '55'); g.addColorStop(1, c + '00'); return g; };
    streamChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels,
            datasets: [
                { label: 'CPU', data: cpuSeries, borderColor: '#6366F1', backgroundColor: grad('#6366F1'), fill: true, tension: .4, borderWidth: 2, pointRadius: 0 },
                { label: 'MEM', data: memSeries, borderColor: '#A78BFA', backgroundColor: grad('#A78BFA'), fill: true, tension: .4, borderWidth: 2, pointRadius: 0 },
            ]
        },
        options: {
            responsive: true, maintainAspectRatio: false, animation: { duration: 250 },
            interaction: { mode: 'index', intersect: false },
            plugins: {
                legend: { display: false },
                tooltip: {
                    backgroundColor: '#192134', titleColor: '#A78BFA', bodyColor: '#fff',
                    borderColor: 'rgba(255,255,255,.1)', borderWidth: 1, cornerRadius: 10, padding: 10,
                    titleFont: { family: 'Inter' }, bodyFont: { family: 'Inter' },
                }
            },
            scales: {
                x: { grid: { color: 'rgba(255,255,255,.04)' }, ticks: { color: '#64748B', maxTicksLimit: 6, font: { size: 10 } } },
                y: { min: 0, max: 100, grid: { color: 'rgba(255,255,255,.04)' }, ticks: { color: '#64748B', callback: v => v + '%', maxTicksLimit: 5, font: { size: 10 } } }
            }
        }
    });
}

// ---- health score: gabungan berbobot (bukan angka dari server) ----
// bobot: CPU 30%, RAM 30%, disk 25%, zram 15%. 100 = ideal, makin penuh makin turun.
function healthScore(cpu, memPct, diskPct, zramPct) {
    const s = 100 - (0.30 * cpu + 0.30 * memPct + 0.25 * diskPct + 0.15 * zramPct);
    return Math.max(0, Math.min(100, Math.round(s)));
}

// ---- clock (waktu lokal klien) ----
function tickClock() { $('h-clock').textContent = new Date().toLocaleTimeString('id-ID', { hour12: false }); }
setInterval(tickClock, 1000); tickClock();

// ---- SSE link pill ----
function setLink(state) {
    const map = {
        ok: ['bg-green/10 border-green/30', 'text-green', 'bg-green', 'LIVE'],
        warn: ['bg-amber/10 border-amber/30', 'text-amber', 'bg-amber', 'RETRY'],
        bad: ['bg-red/10 border-red/30', 'text-red', 'bg-red', 'DOWN'],
    }[state];
    $('h-health').className = `flex items-center gap-2 px-3 py-1.5 rounded-full ${map[0]}`;
    $('h-health-text').className = `text-xs font-semibold ${map[1]}`;
    $('h-health-text').textContent = map[3];
    $('h-health-dot').className = `link-dot w-2 h-2 rounded-full ${map[2]}`;
}

// ---- peak tracker (dipertahankan antar sample) ----
let cpuPeak = 0;
let sampleCount = 0;

// ---- render satu sample SSE ----
function render(d) {
    sampleCount++;

    const cpu = d.cpu?.usage || 0;
    const cores = d.cpu?.cores || 0;
    const ramPct = d.ram?.pct || 0;
    const diskPct = d.storage?.pct || 0;
    const zramPct = d.zram?.pct || 0;
    const net = d.network || {};

    // header
    $('h-host').textContent = d.server?.hostname || '--';

    // hero + health score
    const hs = healthScore(cpu, ramPct, diskPct, zramPct);
    $('hero-score').textContent = hs;
    setRing($('hero-ring'), hs);
    const lvl = hs >= 80 ? ['All systems operational', 'text-green']
        : hs >= 50 ? ['Under elevated load', 'text-amber']
            : ['Under heavy load', 'text-red'];
    const title = $('hero-title');
    title.textContent = lvl[0];
    title.className = `text-2xl font-bold ${lvl[1]}`;
    $('hero-sub').textContent = `${d.server?.os || '--'} · ${d.server?.cpu_model || '--'}`;
    $('hero-cpu').textContent = cpu.toFixed(0) + '%';
    $('hero-mem').textContent = ramPct.toFixed(0) + '%';
    $('hero-disk').textContent = diskPct.toFixed(0) + '%';
    $('hero-load').textContent = (d.server?.load1 || 0).toFixed(2);
    $('hero-uptime').textContent = fmtUp(d.server?.uptime || 0);
    $('hero-checked').textContent = sampleCount;

    // CPU gauge
    $('kpi-cpu-val').textContent = cpu.toFixed(0) + '%';
    setRing($('kpi-cpu-ring'), cpu);
    cpuPeak = Math.max(cpuPeak, cpu);
    $('kpi-cpu-peak').textContent = cpuPeak.toFixed(0) + '%';
    $('kpi-cpu-cores').textContent = cores + ' cores';

    // Memory gauge
    $('kpi-mem-val').textContent = ramPct.toFixed(0) + '%';
    setRing($('kpi-mem-ring'), ramPct);
    $('kpi-mem-used').textContent = fmtB(d.ram?.used);
    $('kpi-mem-free').textContent = fmtB(d.ram?.total);
    $('kpi-mem-total').textContent = fmtB(d.ram?.total);

    // Disk bullet (server kirim used/total/pct agregat)
    $('kpi-disk-val').textContent = diskPct.toFixed(0) + '%';
    setBullet($('kpi-disk-fill'), diskPct);
    $('kpi-disk-used').textContent = fmtB(d.storage?.total - d.storage?.used);
    $('kpi-disk-total').textContent = 'of ' + fmtB(d.storage?.total);

    // ZRAM bullet
    $('kpi-swap-val').textContent = zramPct.toFixed(0) + '%';
    setBullet($('kpi-swap-fill'), zramPct);
    $('kpi-swap-ratio').textContent = (d.zram?.ratio || 0).toFixed(0) + '%';
    $('kpi-swap-total').textContent = 'zram ' + fmtB(d.zram?.disk_size);

    // Network
    $('net-total').textContent = (net.net_mbps || 0).toFixed(1);
    $('net-rx').textContent = (net.rx_mbps || 0).toFixed(1) + ' Mbps';
    $('net-tx').textContent = (net.tx_mbps || 0).toFixed(1) + ' Mbps';
    $('net-rx-tot').textContent = fmtB(net.rx_bytes);
    $('net-tx-tot').textContent = fmtB(net.tx_bytes);

    // stream chart
    labels.push(new Date().toLocaleTimeString('id-ID', { hour12: false }).slice(0, 5));
    cpuSeries.push(cpu); memSeries.push(ramPct);
    if (labels.length > WIN) { labels.shift(); cpuSeries.shift(); memSeries.shift(); }
    if (streamChart) streamChart.update('none');

    renderProcs(d.processes || []);
    renderLogs(d.logs || []);
}

function renderProcs(procs) {
    const tb = $('proc-body');
    if (!procs.length) {
        const tr = document.createElement('tr');
        tr.innerHTML = '<td colspan="4" class="py-6 text-center text-faint">menunggu data proses…</td>';
        tb.replaceChildren(tr);
        return;
    }
    const frag = document.createDocumentFragment();
    procs.forEach(p => {
        const tr = document.createElement('tr');
        tr.className = 'hover:bg-white/[0.02] transition';
        const cells = [
            ['py-2 px-2 text-faint num', p.pid],
            ['py-2 px-2 text-dim font-medium', p.name],
            ['py-2 px-2 text-right text-indigoHi num', (p.cpu || 0).toFixed(1) + '%'],
            ['py-2 px-2 text-right text-violetHi num', fmtB(p.ram)],
        ];
        cells.forEach(([c, v]) => {
            const td = document.createElement('td');
            td.className = c; td.textContent = v; tr.appendChild(td);
        });
        frag.appendChild(tr);
    });
    tb.replaceChildren(frag);
}

const MAX_LOGS = 40;
const seenCursors = new Set();   // dedupe: server kirim cache saat journalctl gagal

function renderLogs(logs) {
    const box = $('event-body');
    const frag = document.createDocumentFragment();
    logs.forEach(l => {
        if (l.cursor) {
            if (seenCursors.has(l.cursor)) return;
            seenCursors.add(l.cursor);
        }
        const lvl = l.priority === '3' ? ['err', 'text-red', 'bg-red/10', 'border-red/30']
            : l.priority === '4' ? ['warn', 'text-amber', 'bg-amber/10', 'border-amber/30']
                : ['sys', 'text-indigoHi', 'bg-indigoHi/10', 'border-indigoHi/30'];
        const time = (l.time || '').slice(11) || '--:--:--';
        const row = document.createElement('div');
        row.className = 'flex items-start gap-2.5';
        // teks di-set via textContent agar aman dari injeksi
        row.innerHTML = `
            <span class="shrink-0 mt-0.5 px-1.5 py-0.5 rounded border ${lvl[3]} ${lvl[2]} ${lvl[1]} text-[9px] font-bold uppercase">${lvl[0]}</span>
            <div class="min-w-0">
                <div class="text-dim truncate"></div>
                <div class="text-[10px] text-faint num mt-0.5">${time}</div>
            </div>`;
        row.querySelector('.text-dim').textContent = (l.unit ? l.unit + ' — ' : '') + stripAnsi(l.message);
        frag.appendChild(row);
    });
    box.appendChild(frag);
    while (box.children.length > MAX_LOGS) box.removeChild(box.firstChild);
    if (seenCursors.size > MAX_LOGS * 4) {
        const keep = new Set(logs.map(l => l.cursor).filter(Boolean));
        seenCursors.forEach(c => { if (!keep.has(c)) seenCursors.delete(c); });
    }
    box.scrollTop = box.scrollHeight;
}

// ---- SSE ----
// Reset stream tiap ~1MB diterima agar buffer parser EventSource tak tumbuh tanpa batas.
const RX_LIMIT = 1_000_000;
const encoder = new TextEncoder();
let rxBytes = 0, es = null;

function startStream() {
    es = new EventSource('/health');
    es.onopen = () => setLink('ok');
    es.onmessage = e => {
        try {
            const d = JSON.parse(e.data);
            render(d);
            rxBytes += encoder.encode(e.data).length;
            if (rxBytes >= RX_LIMIT) resetStream();
        } catch (err) { console.error('render error:', err); }
    };
    es.onerror = () => setLink('warn');
}
function resetStream() {
    if (es) es.close();
    rxBytes = 0;
    labels.length = cpuSeries.length = memSeries.length = 0;
    seenCursors.clear();
    startStream();
}

// script di akhir <body> — DOM siap, init langsung
initStream();
startStream();
