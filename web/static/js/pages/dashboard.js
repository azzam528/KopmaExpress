/**
 * KOPMA Express - Dashboard Analytics
 * Data source: Analytics API and Forecast API.
 */
document.addEventListener('DOMContentLoaded', () => {
  const state = {
    period: '3m',
    summary: null,
    topSelling: [],
    topStock: [],
    forecastSummary: {},
    criticalItems: [],
    charts: {
      topSelling: null,
      topStock: null,
      monthlyTrend: null,
      forecastStatus: null,
    },
  };

  const STATUS_META = [
    { key: 'CRITICAL', label: 'Critical', color: '#c6534d' },
    { key: 'OVERSTOCK_WARNING', label: 'Overstock Warning', color: '#d49a35' },
    { key: 'SAFE', label: 'Safe', color: '#27865f' },
    { key: 'INSUFFICIENT_DATA', label: 'Insufficient Data', color: '#87948f' },
  ];

  const el = (id) => document.getElementById(id);
  const toNumber = (value) => Number.isFinite(Number(value)) ? Number(value) : 0;
  const formatNumber = (value) => window.formatNumber
    ? window.formatNumber(toNumber(value))
    : new Intl.NumberFormat('id-ID').format(toNumber(value));
  const formatCurrency = (value) => window.formatCurrency
    ? window.formatCurrency(toNumber(value))
    : new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', maximumFractionDigits: 0 }).format(toNumber(value));

  function escapeHTML(value) {
    return String(value ?? '').replace(/[&<>"']/g, (char) => ({
      '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
    }[char]));
  }

  function setText(id, value) {
    const node = el(id);
    if (node) node.textContent = value;
  }

  function setStateMessage(id, message, visible = true) {
    const node = el(id);
    if (!node) return;
    node.textContent = message || '';
    node.classList.toggle('is-hidden', !visible);
  }

  function setDashboardError(messages) {
    const node = el('dashboardError');
    if (!node) return;
    const validMessages = messages.filter(Boolean);
    node.hidden = validMessages.length === 0;
    node.textContent = validMessages.length
      ? `Sebagian data dashboard gagal dimuat: ${validMessages.join('; ')}. Coba tekan Perbarui.`
      : '';
  }

  function monthName(year, month) {
    const date = new Date(Number(year), Number(month) - 1, 1);
    return date.toLocaleDateString('id-ID', { month: 'short', year: 'numeric' });
  }

  function renderSummary(summary = {}) {
    state.summary = summary;
    setText('kpiTotalProduk', formatNumber(summary.total_produk));
    setText('kpiTotalAnggota', formatNumber(summary.total_anggota));
    setText('kpiTotalPesanan', formatNumber(summary.total_pesanan_bulan_ini));
    setText('kpiOmzet', formatCurrency(summary.omzet_bulan_ini));
    setText('kpiValuasi', formatCurrency(summary.total_valuasi_gudang));

    const now = new Date();
    const month = now.toLocaleDateString('id-ID', { month: 'long', year: 'numeric' });
    setText('currentMonthOrdersCaption', `Pesanan selesai · ${month}`);
  }

  function createOrUpdateChart(key, canvasId, config, stateId = `${key}ChartState`) {
    if (typeof Chart === 'undefined') {
      setStateMessage(stateId, 'Chart.js tidak berhasil dimuat.');
      return;
    }
    const canvas = el(canvasId);
    if (!canvas) return;

    if (state.charts[key]) {
      state.charts[key].data = config.data;
      state.charts[key].options = config.options;
      state.charts[key].update();
    } else {
      state.charts[key] = new Chart(canvas, config);
    }
  }

  function renderTopSelling(rows = []) {
    state.topSelling = Array.isArray(rows) ? rows : [];
    const chartStateId = 'topSellingChartState';

    if (!state.topSelling.length) {
      setStateMessage(chartStateId, 'Belum ada data penjualan pada periode ini.');
      if (state.charts.topSelling) {
        state.charts.topSelling.destroy();
        state.charts.topSelling = null;
      }
      setText('topSellingRows', '');
      el('topSellingRows').innerHTML = '<tr><td colspan="3" class="dashboard-empty-cell">Belum ada data penjualan.</td></tr>';
      updateTrendProductOptions();
      return;
    }

    setStateMessage(chartStateId, '', false);
    createOrUpdateChart('topSelling', 'topSellingChart', {
      type: 'bar',
      data: {
        labels: state.topSelling.map((item) => item.nama_barang || item.kode_barang),
        datasets: [{
          label: 'Qty Terjual',
          data: state.topSelling.map((item) => toNumber(item.total_qty)),
          backgroundColor: '#1b8378',
          hoverBackgroundColor: '#11675e',
          borderRadius: 5,
          borderSkipped: false,
          barThickness: 13,
        }],
      },
      options: {
        indexAxis: 'y',
        responsive: true,
        maintainAspectRatio: false,
        animation: { duration: 250 },
        plugins: {
          legend: { display: false },
          tooltip: {
            callbacks: {
              label: (context) => ` ${formatNumber(context.raw)} unit terjual`,
            },
          },
        },
        scales: {
          x: {
            beginAtZero: true,
            grid: { color: '#edf2f0' },
            ticks: { color: '#81908c', precision: 0, font: { size: 9 } },
          },
          y: {
            grid: { display: false },
            ticks: {
              color: '#4f625d',
              font: { size: 9 },
              callback(value) {
                const label = this.getLabelForValue(value);
                return label.length > 23 ? `${label.slice(0, 21)}…` : label;
              },
            },
          },
        },
      },
    });

    el('topSellingRows').innerHTML = state.topSelling.map((item, index) => `
      <tr>
        <td>
          <span class="product-name">${index + 1}. ${escapeHTML(item.nama_barang || 'Nama tidak tersedia')}</span>
          <span class="product-meta">${escapeHTML(item.kode_barang || '')}${item.kategori ? ` · ${escapeHTML(item.kategori)}` : ''}</span>
        </td>
        <td class="align-right">${formatNumber(item.total_qty)}</td>
        <td class="align-right">${formatCurrency(item.total_omzet)}</td>
      </tr>
    `).join('');

    updateTrendProductOptions();
  }

  function renderTopStock(rows = [], meta = {}) {
    state.topStock = Array.isArray(rows) ? rows : [];
    const chartStateId = 'topStockChartState';

    setText('topStockValuationNote', `Total valuasi gudang: ${formatCurrency(meta.total_valuasi_gudang ?? state.summary?.total_valuasi_gudang)}`);

    if (!state.topStock.length) {
      setStateMessage(chartStateId, 'Belum ada data stok barang aktif.');
      if (state.charts.topStock) {
        state.charts.topStock.destroy();
        state.charts.topStock = null;
      }
      el('topStockRows').innerHTML = '<tr><td colspan="3" class="dashboard-empty-cell">Belum ada data stok.</td></tr>';
      updateTrendProductOptions();
      return;
    }

    setStateMessage(chartStateId, '', false);
    createOrUpdateChart('topStock', 'topStockChart', {
      type: 'bar',
      data: {
        labels: state.topStock.map((item) => item.nama_barang || item.kode_barang),
        datasets: [{
          label: 'Stok',
          data: state.topStock.map((item) => toNumber(item.stok)),
          backgroundColor: '#d7a13e',
          hoverBackgroundColor: '#bd8527',
          borderRadius: 5,
          borderSkipped: false,
          barThickness: 13,
        }],
      },
      options: {
        indexAxis: 'y',
        responsive: true,
        maintainAspectRatio: false,
        animation: { duration: 250 },
        plugins: {
          legend: { display: false },
          tooltip: {
            callbacks: {
              label: (context) => ` ${formatNumber(context.raw)} unit tersedia`,
            },
          },
        },
        scales: {
          x: {
            beginAtZero: true,
            grid: { color: '#edf2f0' },
            ticks: { color: '#81908c', precision: 0, font: { size: 9 } },
          },
          y: {
            grid: { display: false },
            ticks: {
              color: '#4f625d',
              font: { size: 9 },
              callback(value) {
                const label = this.getLabelForValue(value);
                return label.length > 23 ? `${label.slice(0, 21)}…` : label;
              },
            },
          },
        },
      },
    });

    el('topStockRows').innerHTML = state.topStock.map((item, index) => `
      <tr>
        <td>
          <span class="product-name">${index + 1}. ${escapeHTML(item.nama_barang || 'Nama tidak tersedia')}</span>
          <span class="product-meta">${escapeHTML(item.kode_barang || '')}${item.kategori ? ` · ${escapeHTML(item.kategori)}` : ''}</span>
        </td>
        <td class="align-right">${formatNumber(item.stok)}</td>
        <td class="align-right">${formatCurrency(item.valuasi_aset)}</td>
      </tr>
    `).join('');

    updateTrendProductOptions();
  }

  function renderForecastStatus(summary = {}) {
    state.forecastSummary = summary || {};
    const counts = STATUS_META.map((item) => toNumber(state.forecastSummary[item.key]));
    const total = counts.reduce((sum, value) => sum + value, 0);

    setText('criticalCount', formatNumber(state.forecastSummary.CRITICAL));
    const legend = el('forecastStatusLegend');
    if (legend) {
      legend.innerHTML = STATUS_META.map((item) => `
        <div class="dashboard-legend-row">
          <span class="dashboard-legend-label">
            <span class="dashboard-legend-dot" style="background:${item.color}"></span>
            ${item.label}
          </span>
          <strong>${formatNumber(state.forecastSummary[item.key])}</strong>
        </div>
      `).join('');
    }

    if (total === 0) {
      setStateMessage('forecastStatusState', 'Belum ada hasil forecasting.');
      if (state.charts.forecastStatus) {
        state.charts.forecastStatus.destroy();
        state.charts.forecastStatus = null;
      }
      return;
    }

    setStateMessage('forecastStatusState', '', false);
    createOrUpdateChart('forecastStatus', 'forecastStatusChart', {
      type: 'doughnut',
      data: {
        labels: STATUS_META.map((item) => item.label),
        datasets: [{
          data: counts,
          backgroundColor: STATUS_META.map((item) => item.color),
          borderColor: '#ffffff',
          borderWidth: 3,
          hoverOffset: 4,
        }],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        cutout: '69%',
        plugins: {
          legend: { display: false },
          tooltip: {
            callbacks: {
              label: (context) => ` ${context.label}: ${formatNumber(context.raw)} SKU`,
            },
          },
        },
      },
    }, 'forecastStatusState');
  }

  function renderCriticalItems(rows = []) {
    state.criticalItems = Array.isArray(rows) ? rows.slice() : [];
    state.criticalItems.sort((a, b) => toNumber(b.rekomendasi_restock) - toNumber(a.rekomendasi_restock));

    const tbody = el('criticalRows');
    if (!tbody) return;

    if (!state.criticalItems.length) {
      tbody.innerHTML = '<tr><td colspan="5" class="dashboard-empty-cell">Tidak ada produk berstatus Critical saat ini.</td></tr>';
      setText('criticalListNote', 'Tidak ada prioritas restock yang dikembalikan oleh Forecast API.');
      return;
    }

    tbody.innerHTML = state.criticalItems.map((item) => `
      <tr>
        <td>
          <span class="product-name">${escapeHTML(item.nama_barang || 'Nama tidak tersedia')}</span>
          <span class="product-meta">${escapeHTML(item.kode_barang || '')}</span>
        </td>
        <td class="align-right">${formatNumber(item.stok_saat_ini)}</td>
        <td class="align-right">${formatNumber(item.prediksi_bulan_depan)}</td>
        <td class="align-right"><strong>${formatNumber(item.rekomendasi_restock)}</strong></td>
        <td><span class="dashboard-status-pill"><span class="dashboard-legend-dot" style="background:#c6534d"></span>CRITICAL</span></td>
      </tr>
    `).join('');

    setText('criticalListNote', `Menampilkan ${state.criticalItems.length} produk Critical dengan rekomendasi restock terbesar.`);
  }

  function updateTrendProductOptions() {
    const select = el('trendProductSelect');
    if (!select) return;

    const products = new Map();
    [...state.topSelling, ...state.topStock].forEach((item) => {
      if (item.kode_barang) {
        products.set(item.kode_barang, item.nama_barang || item.kode_barang);
      }
    });

    const options = [...products.entries()];
    const previousValue = select.value;
    select.innerHTML = options.length
      ? options.map(([code, name]) => `<option value="${escapeHTML(code)}">${escapeHTML(name)} (${escapeHTML(code)})</option>`).join('')
      : '<option value="">Belum ada produk dengan data analitik</option>';
    select.disabled = options.length === 0;

    const stillExists = options.some(([code]) => code === previousValue);
    if (stillExists) {
      select.value = previousValue;
    } else if (options.length) {
      select.value = options[0][0];
      loadMonthlyTrend(select.value);
    } else {
      setStateMessage('monthlyTrendState', 'Pilih periode dengan data penjualan untuk melihat tren.');
      if (state.charts.monthlyTrend) {
        state.charts.monthlyTrend.destroy();
        state.charts.monthlyTrend = null;
      }
    }
  }

  async function loadMonthlyTrend(productCode) {
    if (!productCode) return;
    setStateMessage('monthlyTrendState', 'Memuat tren penjualan...', true);
    try {
      const response = await window.apiClient.get('/analytics/monthly-trend', { code: productCode });
      const rows = Array.isArray(response.data) ? response.data : [];

      if (!rows.length) {
        setStateMessage('monthlyTrendState', 'Belum ada riwayat penjualan untuk produk ini.');
        if (state.charts.monthlyTrend) {
          state.charts.monthlyTrend.destroy();
          state.charts.monthlyTrend = null;
        }
        return;
      }

      setStateMessage('monthlyTrendState', '', false);
      createOrUpdateChart('monthlyTrend', 'monthlyTrendChart', {
        type: 'line',
        data: {
          labels: rows.map((row) => monthName(row.tahun, row.bulan)),
          datasets: [{
            label: 'Unit Terjual',
            data: rows.map((row) => toNumber(row.qty_terjual)),
            borderColor: '#16776d',
            backgroundColor: 'rgba(22,119,109,.10)',
            fill: true,
            tension: .28,
            borderWidth: 2,
            pointRadius: 3,
            pointHoverRadius: 5,
          }],
        },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          plugins: {
            legend: { display: false },
            tooltip: {
              callbacks: { label: (context) => ` ${formatNumber(context.raw)} unit terjual` },
            },
          },
          scales: {
            x: { grid: { display: false }, ticks: { color: '#81908c', font: { size: 9 } } },
            y: {
              beginAtZero: true,
              grid: { color: '#edf2f0' },
              ticks: { color: '#81908c', precision: 0, font: { size: 9 } },
            },
          },
        },
      });
    } catch (error) {
      console.error('[Dashboard] Monthly trend:', error);
      setStateMessage('monthlyTrendState', error.message || 'Gagal memuat tren penjualan.');
    }
  }

  async function refreshDashboard() {
    const button = el('refreshDashboard');
    const errors = [];
    if (button) button.disabled = true;

    const period = state.period;
    const jobs = [
      ['summary', () => window.apiClient.get('/analytics/summary')],
      ['topSelling', () => window.apiClient.get('/analytics/top-selling', { period, limit: 10 })],
      ['topStock', () => window.apiClient.get('/analytics/top-stock', { limit: 10 })],
      ['forecastSummary', () => window.apiClient.get('/forecast/summary')],
      ['criticalItems', () => window.apiClient.get('/forecast', { status: 'CRITICAL', limit: 5, page: 1, sort: 'restock_qty' })],
      ['health', () => window.apiClient.get('/health')],
    ];

    const results = await Promise.allSettled(jobs.map(([, request]) => request()));
    results.forEach((result, index) => {
      const name = jobs[index][0];
      if (result.status === 'rejected') {
        errors.push(`${name}: ${result.reason?.message || 'request gagal'}`);
        console.warn(`[Dashboard] ${name}:`, result.reason);
        return;
      }

      const response = result.value;
      switch (name) {
        case 'summary':
          renderSummary(response.data || {});
          break;
        case 'topSelling':
          renderTopSelling(response.data || []);
          setText('topSellingPeriodNote', `Periode: ${period === 'all' ? 'seluruh histori' : `${period.replace('m', ' bulan')} terakhir`}.`);
          break;
        case 'topStock':
          renderTopStock(response.data || [], response.meta || {});
          break;
        case 'forecastSummary':
          renderForecastStatus(response.data || {});
          break;
        case 'criticalItems':
          renderCriticalItems(response.data || []);
          break;
        case 'health': {
          const status = response.data?.database === 'CONNECTED' ? 'connected' : 'error';
          const statusNode = el('backendStatus');
          if (statusNode) statusNode.dataset.state = status;
          setText('backendStatusText', status === 'connected' ? 'Database terhubung' : `Database ${response.data?.database || 'tidak diketahui'}`);
          break;
        }
      }
    });

    setDashboardError(errors);
    const updated = new Date();
    setText('lastUpdated', `Diperbarui ${updated.toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' })}`);

    if (button) button.disabled = false;
  }

  document.querySelectorAll('[data-period]').forEach((button) => {
    button.addEventListener('click', async () => {
      const period = button.dataset.period;
      if (!period || period === state.period) return;

      document.querySelectorAll('[data-period]').forEach((item) => {
        item.setAttribute('aria-pressed', String(item === button));
      });

      state.period = period;
      setStateMessage('topSellingChartState', 'Memuat data barang terlaris...', true);
      try {
        const response = await window.apiClient.get('/analytics/top-selling', { period, limit: 10 });
        renderTopSelling(response.data || []);
        setText('topSellingPeriodNote', `Periode: ${period === 'all' ? 'seluruh histori' : `${period.replace('m', ' bulan')} terakhir`}.`);
      } catch (error) {
        console.error('[Dashboard] Top selling period:', error);
        setStateMessage('topSellingChartState', error.message || 'Gagal memuat barang terlaris.');
      }
    });
  });

  const trendSelect = el('trendProductSelect');
  if (trendSelect) {
    trendSelect.addEventListener('change', () => loadMonthlyTrend(trendSelect.value));
  }

  const refreshButton = el('refreshDashboard');
  if (refreshButton) refreshButton.addEventListener('click', refreshDashboard);

  refreshDashboard();
});
