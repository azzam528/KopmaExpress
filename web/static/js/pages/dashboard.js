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
    selectedProduct: null,
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
      ensureDefaultTrendProduct();
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
            ticks: { color: '#64748b', precision: 0, font: { family: 'Inter', size: 11 } },
          },
          y: {
            grid: { display: false },
            ticks: {
              color: '#0b1c30',
              font: { family: 'Inter', size: 11 },
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

    ensureDefaultTrendProduct();
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
      ensureDefaultTrendProduct();
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
            ticks: { color: '#64748b', precision: 0, font: { family: 'Inter', size: 11 } },
          },
          y: {
            grid: { display: false },
            ticks: {
              color: '#0b1c30',
              font: { family: 'Inter', size: 11 },
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

    ensureDefaultTrendProduct();
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

  let trendSearchTimer = null;
  let activeSuggestIndex = -1;

  function showTrendSuggestions() {
    const suggest = el('trendProductSuggest');
    const input = el('trendProductSearch');
    if (suggest) {
      suggest.classList.remove('hidden');
      input?.setAttribute('aria-expanded', 'true');
    }
  }

  function hideTrendSuggestions() {
    const suggest = el('trendProductSuggest');
    const input = el('trendProductSearch');
    if (suggest) {
      suggest.classList.add('hidden');
      input?.setAttribute('aria-expanded', 'false');
    }
  }

  function selectTrendProduct(code, name) {
    if (!code) return;
    const displayName = name || code;
    state.selectedProduct = { code, name: displayName };

    const searchInput = el('trendProductSearch');
    if (searchInput) {
      searchInput.value = `${displayName} (${code})`;
    }
    const clearBtn = el('trendProductClear');
    if (clearBtn) {
      clearBtn.classList.remove('hidden');
    }
    hideTrendSuggestions();
    loadMonthlyTrend(code, displayName);
  }

  async function ensureDefaultTrendProduct() {
    if (state.selectedProduct) return;
    const candidate = state.topSelling[0] || state.topStock[0];
    if (candidate && candidate.kode_barang) {
      selectTrendProduct(candidate.kode_barang, candidate.nama_barang);
      return;
    }
    try {
      const res = await window.apiClient.get('/products', { limit: 1 });
      const first = res.data && res.data[0];
      if (first && first.kode_barang && !state.selectedProduct) {
        selectTrendProduct(first.kode_barang, first.nama_barang);
      } else if (!state.selectedProduct) {
        setStateMessage('monthlyTrendState', 'Pilih atau cari produk untuk melihat tren penjualan.');
      }
    } catch (err) {
      if (!state.selectedProduct) {
        setStateMessage('monthlyTrendState', 'Pilih atau cari produk untuk melihat tren penjualan.');
      }
    }
  }

  async function searchTrendProducts(query) {
    const suggest = el('trendProductSuggest');
    if (!suggest) return;

    showTrendSuggestions();
    suggest.innerHTML = '<li class="px-3 py-3 text-center text-outline font-body-sm text-body-sm">Mencari produk...</li>';
    activeSuggestIndex = -1;

    try {
      const res = await window.apiClient.get('/products', { q: query || '', limit: 10 });
      const items = Array.isArray(res.data) ? res.data : [];
      renderTrendSuggestions(items, query);
    } catch (err) {
      console.warn('[Dashboard] /products search failed, using local items:', err);
      const seen = new Set();
      const local = [...state.topSelling, ...state.topStock].filter((it) => {
        if (!it.kode_barang || seen.has(it.kode_barang)) return false;
        seen.add(it.kode_barang);
        if (!query) return true;
        const q = query.toLowerCase();
        return (it.nama_barang && it.nama_barang.toLowerCase().includes(q)) ||
               it.kode_barang.toLowerCase().includes(q);
      });
      renderTrendSuggestions(local, query);
    }
  }

  function renderTrendSuggestions(items, query) {
    const suggest = el('trendProductSuggest');
    if (!suggest) return;

    if (!items.length) {
      suggest.innerHTML = `<li class="px-3 py-3 text-center text-outline font-body-sm text-body-sm">
        ${query ? `Produk "${escapeHTML(query)}" tidak ditemukan.` : 'Belum ada data produk.'}
      </li>`;
      return;
    }

    suggest.innerHTML = items.map((p, idx) => {
      const code = p.kode_barang || '';
      const name = p.nama_barang || code;
      const category = p.kategori ? ` · ${escapeHTML(p.kategori)}` : '';
      const stock = p.stok != null ? `<span class="text-xs text-outline shrink-0 font-body-sm">Stok ${formatNumber(p.stok)}</span>` : '';
      return `
        <li class="px-3 py-2 cursor-pointer hover:bg-canvas-bg transition-colors flex items-center justify-between gap-2"
            data-index="${idx}" data-code="${escapeHTML(code)}" data-name="${escapeHTML(name)}" role="option">
          <div class="min-w-0">
            <div class="font-medium text-on-surface truncate font-body-sm text-body-sm">${escapeHTML(name)}</div>
            <div class="text-xs text-outline font-body-sm">${escapeHTML(code)}${category}</div>
          </div>
          ${stock}
        </li>
      `;
    }).join('');

    suggest.querySelectorAll('li[data-code]').forEach((li) => {
      li.addEventListener('mousedown', (e) => {
        e.preventDefault();
        const code = li.getAttribute('data-code');
        const name = li.getAttribute('data-name');
        selectTrendProduct(code, name);
      });
    });
  }

  function updateActiveSuggestion(options) {
    options.forEach((opt, idx) => {
      if (idx === activeSuggestIndex) {
        opt.classList.add('is-selected');
        opt.scrollIntoView({ block: 'nearest' });
      } else {
        opt.classList.remove('is-selected');
      }
    });
  }

  function setupTrendProductSearch() {
    const searchInput = el('trendProductSearch');
    const clearBtn = el('trendProductClear');
    const suggest = el('trendProductSuggest');
    if (!searchInput) return;

    searchInput.addEventListener('input', (e) => {
      const query = e.target.value.trim();
      if (clearBtn) {
        clearBtn.classList.toggle('hidden', !e.target.value);
      }
      clearTimeout(trendSearchTimer);
      trendSearchTimer = setTimeout(() => {
        searchTrendProducts(query);
      }, 200);
    });

    searchInput.addEventListener('focus', () => {
      searchInput.select();
      searchTrendProducts('');
    });

    searchInput.addEventListener('keydown', (e) => {
      if (!suggest || suggest.classList.contains('hidden')) {
        if (e.key === 'ArrowDown') {
          showTrendSuggestions();
          searchTrendProducts('');
        }
        return;
      }

      const options = suggest.querySelectorAll('li[data-code]');
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        if (!options.length) return;
        activeSuggestIndex = (activeSuggestIndex + 1) % options.length;
        updateActiveSuggestion(options);
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        if (!options.length) return;
        activeSuggestIndex = (activeSuggestIndex - 1 + options.length) % options.length;
        updateActiveSuggestion(options);
      } else if (e.key === 'Enter') {
        e.preventDefault();
        if (activeSuggestIndex >= 0 && options[activeSuggestIndex]) {
          const li = options[activeSuggestIndex];
          selectTrendProduct(li.getAttribute('data-code'), li.getAttribute('data-name'));
        } else if (options.length > 0) {
          const li = options[0];
          selectTrendProduct(li.getAttribute('data-code'), li.getAttribute('data-name'));
        }
      } else if (e.key === 'Escape') {
        e.preventDefault();
        hideTrendSuggestions();
        if (state.selectedProduct) {
          searchInput.value = `${state.selectedProduct.name} (${state.selectedProduct.code})`;
          clearBtn?.classList.remove('hidden');
        }
      }
    });

    if (clearBtn) {
      clearBtn.addEventListener('click', (e) => {
        e.preventDefault();
        searchInput.value = '';
        clearBtn.classList.add('hidden');
        searchInput.focus();
        searchTrendProducts('');
      });
    }

    document.addEventListener('click', (e) => {
      const container = el('trendSearchContainer');
      if (!container || container.contains(e.target)) return;
      hideTrendSuggestions();
      if (state.selectedProduct && searchInput.value !== `${state.selectedProduct.name} (${state.selectedProduct.code})`) {
        searchInput.value = `${state.selectedProduct.name} (${state.selectedProduct.code})`;
        clearBtn?.classList.remove('hidden');
      }
    });
  }

  async function loadMonthlyTrend(productCode, productName) {
    if (!productCode) return;
    setStateMessage('monthlyTrendState', 'Memuat tren penjualan...', true);
    try {
      const response = await window.apiClient.get('/analytics/monthly-trend', { code: productCode });
      const rows = Array.isArray(response.data) ? response.data : [];

      if (!rows.length) {
        const displayName = productName || productCode;
        setStateMessage('monthlyTrendState', `Belum ada riwayat penjualan untuk ${displayName}.`);
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
            x: { grid: { display: false }, ticks: { color: '#64748b', font: { family: 'Inter', size: 11 } } },
            y: {
              beginAtZero: true,
              grid: { color: '#edf2f0' },
              ticks: { color: '#64748b', precision: 0, font: { family: 'Inter', size: 11 } },
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

  setupTrendProductSearch();

  const refreshButton = el('refreshDashboard');
  if (refreshButton) refreshButton.addEventListener('click', refreshDashboard);

  refreshDashboard();
});
