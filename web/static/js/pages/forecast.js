/**
 * KOPMA Express - Forecast & Restock Page Script
 * Data source: /api/v1/forecast endpoints.
 */

(() => {
  const state = {
    status: '',
    query: '',
    sort: 'restock_qty',
    page: 1,
    limit: 20,
    total: 0,
    rows: [],
    selectedCode: '',
    chart: null,
    searchTimer: null,
    productSearchTimer: null,   // + baru
    productSearchId: 0,         // + baru
    productOptions: [],         // + baru
    activeIndex: -1,            // + baru
    selectedLabel: '',
    listRequestId: 0,
    detailRequestId: 0,
  };

  const $ = (id) => document.getElementById(id);
  const STATUS_LABELS = {
    CRITICAL: 'CRITICAL',
    OVERSTOCK_WARNING: 'OVERSTOCK WARNING',
    SAFE: 'SAFE',
    INSUFFICIENT_DATA: 'INSUFFICIENT DATA',
  };
  const STATUS_STYLES = {
    CRITICAL: {
      badge: 'bg-status-critical-bg text-status-critical-text',
      dot: 'bg-status-critical-text',
      metric: 'bg-status-critical-bg text-status-critical-text',
    },
    OVERSTOCK_WARNING: {
      badge: 'bg-status-warning-bg text-status-warning-text',
      dot: 'bg-status-warning-text',
      metric: 'bg-status-warning-bg text-status-warning-text',
    },
    SAFE: {
      badge: 'bg-status-success-bg text-status-success-text',
      dot: 'bg-status-success-text',
      metric: 'bg-status-success-bg text-status-success-text',
    },
    INSUFFICIENT_DATA: {
      badge: 'bg-surface-container text-on-surface-variant',
      dot: 'bg-outline',
      metric: 'bg-surface-container text-on-surface-variant',
    },
  };
  const MONTHS_ID = [
    'Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni',
    'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember',
  ];

  function number(value, digits = 0) {
    const parsed = Number(value);
    if (!Number.isFinite(parsed)) return '—';
    return new Intl.NumberFormat('id-ID', {
      minimumFractionDigits: digits,
      maximumFractionDigits: digits,
    }).format(parsed);
  }

  function monthName(value, short = false) {
    if (!/^\d{4}-\d{2}$/.test(String(value || ''))) return value || '—';
    const [year, month] = value.split('-').map(Number);
    const name = MONTHS_ID[month - 1] || value;
    return `${short ? name.slice(0, 3) : name} ${year}`;
  }

  function monthFromHistory(item) {
    const year = Number(item.tahun);
    const month = Number(item.bulan);
    if (!year || month < 1 || month > 12) return 'Periode';
    return `${MONTHS_ID[month - 1].slice(0, 3)} ${String(year).slice(-2)}`;
  }

  function notify(type, title, message) {
    if (typeof window.showToast === 'function') {
      window.showToast(type, title, message || '');
    } else if (type === 'error') {
      console.error(`[Forecast] ${title}: ${message || ''}`);
    }
  }

  function setText(id, value) {
    const element = $(id);
    if (element) element.textContent = value == null ? '—' : String(value);
  }

  function formatTimestamp(value) {
    if (!value) return 'Belum tersedia';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return 'Belum tersedia';
    return new Intl.DateTimeFormat('id-ID', {
      dateStyle: 'medium',
      timeStyle: 'short',
      timeZone: 'Asia/Jakarta',
    }).format(date) + ' WIB';
  }

  function renderSummary(summary) {
    setText('forecastCriticalCount', number(summary.CRITICAL));
    setText('forecastOverstockCount', number(summary.OVERSTOCK_WARNING));
    setText('forecastSafeCount', number(summary.SAFE));
    setText('forecastInsufficientCount', number(summary.INSUFFICIENT_DATA));
    setText('forecastCriticalNavBadge', number(summary.CRITICAL));
  }

  function createCell(className, text) {
    const cell = document.createElement('td');
    cell.className = className;
    cell.textContent = text;
    return cell;
  }

  function createStatusBadge(status) {
    const style = STATUS_STYLES[status] || STATUS_STYLES.INSUFFICIENT_DATA;
    const badge = document.createElement('span');
    badge.className = `inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full font-label-sm text-label-sm font-semibold ${style.badge}`;

    const dot = document.createElement('span');
    dot.className = `w-1.5 h-1.5 rounded-full ${style.dot}`;
    badge.append(dot, document.createTextNode(STATUS_LABELS[status] || status || 'UNKNOWN'));
    return badge;
  }

  function createForecastRow(item) {
    const row = document.createElement('tr');
    row.className = 'forecast-data-row hover:bg-surface-bright transition-colors cursor-pointer focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary';
    row.dataset.status = item.status || '';
    row.dataset.forecastCode = item.kode_barang || '';
    row.tabIndex = 0;
    row.setAttribute('aria-label', `Lihat detail forecast ${item.nama_barang || item.kode_barang}`);

    row.append(
      createCell('py-3.5 px-space-md font-mono text-label-md text-primary font-semibold whitespace-nowrap', item.kode_barang || '—'),
    );

    const productCell = document.createElement('td');
    productCell.className = 'py-3.5 px-space-md min-w-[180px]';
    const productWrap = document.createElement('div');
    productWrap.className = 'flex flex-col';
    const productName = document.createElement('span');
    productName.className = 'font-title-md text-title-md text-on-surface';
    productName.textContent = item.nama_barang || 'Nama produk tidak tersedia';
    const productMeta = document.createElement('span');
    productMeta.className = 'font-body-sm text-body-sm text-outline';
    productMeta.textContent = `${number(item.jumlah_data_bulan)} periode historis · Target ${monthName(item.bulan_target)}`;
    productWrap.append(productName, productMeta);
    productCell.append(productWrap);
    row.append(productCell);

    row.append(
      createCell('py-3.5 px-space-md text-right text-on-surface', number(item.stok_saat_ini)),
      createCell('py-3.5 px-space-md text-right text-on-surface-variant', number(item.wma, 1)),
      createCell('py-3.5 px-space-md text-right text-on-surface-variant', number(item.regresi_linear, 1)),
      createCell('py-3.5 px-space-md text-right font-semibold text-primary', number(item.prediksi_bulan_depan)),
    );

    const restockCell = document.createElement('td');
    restockCell.className = 'py-3.5 px-space-md text-right';
    const restock = document.createElement('span');
    const restockStyle = STATUS_STYLES[item.status] || STATUS_STYLES.INSUFFICIENT_DATA;
    restock.className = `px-2.5 py-1 rounded-lg font-title-md text-title-md ${restockStyle.metric}`;
    restock.textContent = number(item.rekomendasi_restock);
    restockCell.append(restock);
    row.append(restockCell);

    const statusCell = document.createElement('td');
    statusCell.className = 'py-3.5 px-space-md text-center';
    statusCell.append(createStatusBadge(item.status));
    row.append(statusCell);
    return row;
  }

  function renderTable() {
    const tbody = document.querySelector('#forecastTable tbody');
    if (!tbody) return;
    tbody.replaceChildren();

    if (!state.rows.length) {
      const row = document.createElement('tr');
      const cell = createCell('py-10 px-space-md text-center text-outline', 'Tidak ada hasil forecast untuk filter ini.');
      cell.colSpan = 8;
      row.append(cell);
      tbody.append(row);
    } else {
      state.rows.forEach((item) => tbody.append(createForecastRow(item)));
    }

    const start = state.total === 0 ? 0 : ((state.page - 1) * state.limit) + 1;
    const end = Math.min(state.page * state.limit, state.total);
    setText('forecastTableMeta', `Menampilkan ${number(start)}–${number(end)} dari ${number(state.total)} hasil forecast`);

    const previous = $('forecastPrevPage');
    const next = $('forecastNextPage');
    const pageLabel = $('forecastPageLabel');
    const pageCount = Math.max(1, Math.ceil(state.total / state.limit));
    if (pageLabel) pageLabel.textContent = `${state.page} / ${pageCount}`;
    if (previous) previous.disabled = state.page <= 1;
    if (next) next.disabled = state.page >= pageCount;
  }

  function renderProductOptions() {
    const input = $('forecastProductSearch');
    if (!input) return;
    input.disabled = false;

    if (!state.selectedCode && state.rows.length) {
      state.selectedCode = state.rows[0].kode_barang; // default: produk teratas
    }
    if (!state.selectedCode) {
      input.value = '';
      state.selectedLabel = '';
      input.placeholder = 'Tidak ada produk';
    } else {
      input.placeholder = 'Cari kode / nama barang...';
    }
  }

  function setProductInput(detail) {
    const input = $('forecastProductSearch');
    state.selectedLabel = `${detail.nama_barang || detail.kode_barang} (${detail.kode_barang})`;
    if (input && document.activeElement !== input) input.value = state.selectedLabel;
  }

  function openProductList(open) {
    const list = $('forecastProductList');
    const input = $('forecastProductSearch');
    if (list) list.hidden = !open;
    if (input) input.setAttribute('aria-expanded', String(open));
  }

  function closeProductList(restore = true) {
    openProductList(false);
    state.activeIndex = -1;
    const input = $('forecastProductSearch');
    if (restore && input) input.value = state.selectedLabel;
  }

  function setActiveOption(index) {
    const list = $('forecastProductList');
    const input = $('forecastProductSearch');
    if (!list || !state.productOptions.length) return;
    const count = state.productOptions.length;
    state.activeIndex = (index + count) % count;
    list.querySelectorAll('li[data-code]').forEach((li, i) => {
      const active = i === state.activeIndex;
      li.classList.toggle('bg-surface-container-low', active);
      li.setAttribute('aria-selected', String(active));
      if (active) {
        li.scrollIntoView({ block: 'nearest' });
        input?.setAttribute('aria-activedescendant', li.id);
      }
    });
  }

  function renderProductList(items, message) {
    const list = $('forecastProductList');
    if (!list) return;
    list.replaceChildren();
    state.productOptions = items;
    state.activeIndex = -1;

    if (message || !items.length) {
      const li = document.createElement('li');
      li.className = 'px-space-md py-2 text-sm text-outline';
      li.textContent = message || 'Produk tidak ditemukan';
      list.append(li);
    } else {
      items.forEach((item, index) => {
        const li = document.createElement('li');
        li.id = `forecastProductOption-${index}`;
        li.setAttribute('role', 'option');
        li.dataset.code = item.kode_barang;
        li.className = 'px-space-md py-2 cursor-pointer hover:bg-surface-container-low flex flex-col';
        const name = document.createElement('span');
        name.className = 'font-title-md text-title-md text-on-surface';
        name.textContent = item.nama_barang || item.kode_barang;
        const code = document.createElement('span');
        code.className = 'font-mono text-label-md text-outline';
        code.textContent = item.kode_barang;
        li.append(name, code);
        list.append(li);
      });
    }
    openProductList(true);
  }

  async function searchProducts(q) {
    if (!window.apiClient) return;
    const requestId = ++state.productSearchId;
    renderProductList([], 'Mencari...');
    try {
      const params = { page: 1, limit: 10, sort: 'restock_qty' };
      if (state.status) params.status = state.status;   // ikut filter card status
      if (q) params.q = q;
      const response = await window.apiClient.get('/forecast', params);
      if (requestId !== state.productSearchId) return;
      renderProductList(Array.isArray(response.data) ? response.data : []);
    } catch (error) {
      if (requestId !== state.productSearchId) return;
      renderProductList([], 'Gagal memuat produk');
    }
  }

  function selectProduct(code) {
    if (!code) return;
    state.selectedCode = code;
    closeProductList(false);
    $('forecastProductSearch')?.blur();
    loadDetail(code);
  }

  function renderPaginationMeta(meta) {
    setText('forecastSyncAt', formatTimestamp(meta?.calculated_at));
    setText('forecastTargetMonthHeader', monthName(meta?.target_month));
  }

  function renderDetailMetrics(detail) {
    setProductInput(detail);
    setText('forecastProductName', detail.nama_barang || detail.kode_barang);
    setText('forecastProductStatusBadge', STATUS_LABELS[detail.status] || detail.status);
    setText('forecastMetricWma', number(detail.wma, 1));
    setText('forecastMetricSlope', `${Number(detail.slope) > 0 ? '+' : ''}${number(detail.slope, 2)}`);
    const slopeElement = $('forecastMetricSlope');
    if (slopeElement) {
      slopeElement.classList.remove('text-status-success-text', 'text-status-critical-text');
      slopeElement.classList.add(Number(detail.slope) < 0 ? 'text-status-critical-text' : 'text-status-success-text');
    }
    setText('forecastMetricDemand', number(detail.prediksi_bulan_depan));
    setText('forecastMetricStock', number(detail.stok_saat_ini));
    setText('forecastMetricRestock', number(detail.rekomendasi_restock));
    setText('forecastMetricLinear', `OLS ${number(detail.regresi_linear, 1)}`);
    setText('forecastR2Value', number(detail.r_squared, 3));
    setText('forecastHistoryCount', `${number(detail.jumlah_data_bulan)} periode data`);
    setText('forecastTargetMonth', `Target suplai ${monthName(detail.bulan_target)}`);

    const badge = $('forecastProductStatusBadge');
    if (badge) {
      Object.values(STATUS_STYLES).forEach((style) => {
        style.badge.split(' ').forEach((className) => badge.classList.remove(className));
      });
      const selectedStyle = STATUS_STYLES[detail.status] || STATUS_STYLES.INSUFFICIENT_DATA;
      selectedStyle.badge.split(' ').forEach((className) => badge.classList.add(className));
    }

    const restockCard = $('forecastMetricRestockCard');
    if (restockCard) {
      ['bg-status-critical-bg', 'bg-status-warning-bg', 'bg-status-success-bg', 'bg-surface-container'].forEach((className) => {
        restockCard.classList.remove(className);
      });
      const statusStyle = STATUS_STYLES[detail.status] || STATUS_STYLES.INSUFFICIENT_DATA;
      const backgroundClass = statusStyle.metric.split(' ').find((className) => className.startsWith('bg-'));
      if (backgroundClass) restockCard.classList.add(backgroundClass);

      const statusColors = {
        CRITICAL: { main: 'text-status-critical-text', soft: 'text-status-critical-text' },
        OVERSTOCK_WARNING: { main: 'text-status-warning-text', soft: 'text-status-warning-text' },
        SAFE: { main: 'text-status-success-text', soft: 'text-status-success-text' },
        INSUFFICIENT_DATA: { main: 'text-on-surface-variant', soft: 'text-on-surface-variant' },
      };
      const colors = statusColors[detail.status] || statusColors.INSUFFICIENT_DATA;
      restockCard.querySelectorAll('[data-forecast-status-color]').forEach((element) => {
        ['text-status-critical-text', 'text-status-warning-text', 'text-status-success-text', 'text-on-surface-variant',
          'text-status-critical-text/80'].forEach((className) => {
            element.classList.remove(className);
          });
        element.classList.add(colors[element.dataset.forecastStatusColor] || colors.main);
      });
    }

    renderChart(detail);
  }

  function renderChart(detail) {
    const canvas = $('forecastTrendChart');
    const fallback = $('forecastChartFallback');
    if (!canvas) return;

    if (typeof window.Chart !== 'function') {
      if (fallback) {
        fallback.hidden = false;
        fallback.textContent = 'Grafik tidak dapat dimuat karena pustaka Chart.js tidak tersedia.';
      }
      return;
    }
    if (fallback) fallback.hidden = true;

    const history = Array.isArray(detail.riwayat_bulanan) ? [...detail.riwayat_bulanan] : [];
    history.sort((a, b) => (Number(a.tahun) * 100 + Number(a.bulan)) - (Number(b.tahun) * 100 + Number(b.bulan)));

    const [targetYear, targetMonthNumber] = String(detail.bulan_target || '').split('-').map(Number);
    const dataPoints = Math.max(0, Math.min(12, Number(detail.jumlah_data_bulan) || history.length));
    const historyMap = new Map();
    history.forEach((item) => {
      const key = `${Number(item.tahun)}-${String(Number(item.bulan)).padStart(2, '0')}`;
      historyMap.set(key, (historyMap.get(key) || 0) + (Number(item.qty_terjual) || 0));
    });

    // Recreate the monthly axis used by the backend (up to 12 periods), filling
    // absent months with zero just as the forecasting series preparation does.
    const periods = [];
    if (targetYear && targetMonthNumber >= 1 && targetMonthNumber <= 12 && dataPoints > 0) {
      const startDate = new Date(Date.UTC(targetYear, targetMonthNumber - 1 - dataPoints, 1));
      for (let index = 0; index < dataPoints; index += 1) {
        const date = new Date(Date.UTC(startDate.getUTCFullYear(), startDate.getUTCMonth() + index, 1));
        const key = `${date.getUTCFullYear()}-${String(date.getUTCMonth() + 1).padStart(2, '0')}`;
        periods.push({ label: `${MONTHS_ID[date.getUTCMonth()].slice(0, 3)} ${String(date.getUTCFullYear()).slice(-2)}`, value: historyMap.get(key) || 0 });
      }
    } else {
      history.forEach((item) => periods.push({ label: monthFromHistory(item), value: Number(item.qty_terjual) || 0 }));
    }

    const labels = periods.map((period) => period.label);
    labels.push(monthName(detail.bulan_target, true));
    const actual = periods.map((period) => period.value);
    actual.push(null);
    const predicted = periods.map(() => null);
    predicted.push(Number(detail.prediksi_bulan_depan) || 0);

    // Anchor the OLS line on the backend-provided next-period linear prediction
    // and extend it backwards using the backend-provided slope. Forecast itself
    // remains owned by the backend; the browser only visualizes these values.
    const targetIndex = periods.length + 1;
    const linearTarget = Number(detail.regresi_linear) || 0;
    const slope = Number(detail.slope) || 0;
    const ols = periods.map((_, index) => Number((linearTarget - slope * (targetIndex - (index + 1))).toFixed(2)));
    ols.push(Number(linearTarget.toFixed(2)));

    if (state.chart) {
      state.chart.destroy();
      state.chart = null;
    }

    state.chart = new window.Chart(canvas.getContext('2d'), {
      type: 'line',
      data: {
        labels,
        datasets: [
          {
            type: 'line',
            label: 'Penjualan historis',
            data: actual,
            borderColor: '#16877b',
            backgroundColor: 'rgba(22, 135, 123, 0.10)',
            borderWidth: 2.5,
            pointRadius: 3,
            pointBackgroundColor: '#16877b',
            pointBorderColor: '#ffffff',
            pointBorderWidth: 1.5,
            tension: 0.25,
            spanGaps: false,
          },
          {
            type: 'line',
            label: 'Tren regresi OLS',
            data: ols,
            borderColor: '#536b68',
            backgroundColor: 'transparent',
            borderWidth: 2,
            borderDash: [6, 5],
            pointRadius: 0,
            pointHoverRadius: 3,
            tension: 0,
          },
          {
            type: 'line',
            label: 'Prediksi bulan depan',
            data: predicted,
            showLine: false,
            borderColor: '#d94841',
            backgroundColor: '#d94841',
            pointRadius: 6,
            pointHoverRadius: 8,
            pointBorderColor: '#ffffff',
            pointBorderWidth: 2,
          },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        interaction: { mode: 'index', intersect: false },
        plugins: {
          // The legend is rendered as an aligned HTML panel beside the plot.
          legend: { display: false },
          tooltip: {
            callbacks: {
              label(context) {
                return `${context.dataset.label}: ${number(context.parsed.y)}`;
              },
            },
          },
        },
        scales: {
          x: {
            grid: { display: false },
            ticks: { color: '#6e7977', font: { family: 'Inter', size: 11 } },
          },
          y: {
            beginAtZero: true,
            grid: { color: '#eff4f6', borderDash: [4, 4] },
            ticks: { color: '#6e7977', precision: 0 },
          },
        },
      },
    });

    const chartNote = $('forecastChartNote');
    if (chartNote) {
      chartNote.textContent = `${number(periods.length)} periode historis digunakan untuk memperkirakan kebutuhan ${monthName(detail.bulan_target)}.`;
    }
  }

  function renderEmptyDetail() {
    setText('forecastProductName', 'Belum ada data produk');
    setText('forecastProductStatusBadge', 'NO DATA');
    setText('forecastMetricWma', '—');
    setText('forecastMetricSlope', '—');
    const slopeElement = $('forecastMetricSlope');
    if (slopeElement) {
      slopeElement.classList.remove('text-status-success-text', 'text-status-critical-text');
      slopeElement.classList.add('text-status-success-text');
    }
    setText('forecastMetricDemand', '—');
    setText('forecastMetricStock', '—');
    setText('forecastMetricRestock', '—');
    setText('forecastMetricLinear', 'OLS —');
    setText('forecastR2Value', '—');
    setText('forecastHistoryCount', '0 periode data');
    setText('forecastTargetMonth', 'Target suplai —');
    const badge = $('forecastProductStatusBadge');
    if (badge) {
      Object.values(STATUS_STYLES).forEach((style) => style.badge.split(' ').forEach((name) => badge.classList.remove(name)));
      STATUS_STYLES.INSUFFICIENT_DATA.badge.split(' ').forEach((name) => badge.classList.add(name));
    }
    const restockCard = $('forecastMetricRestockCard');
    if (restockCard) {
      ['bg-status-critical-bg', 'bg-status-warning-bg', 'bg-status-success-bg'].forEach((name) => restockCard.classList.remove(name));
      restockCard.classList.add('bg-surface-container');
      restockCard.querySelectorAll('[data-forecast-status-color]').forEach((element) => {
        ['text-status-critical-text', 'text-status-warning-text', 'text-status-success-text', 'text-on-surface-variant',
          'text-status-critical-text/80'].forEach((name) => element.classList.remove(name));
        element.classList.add(element.dataset.forecastStatusColor === 'soft' ? 'text-on-surface-variant' : 'text-on-surface-variant');
      });
    }
    if (state.chart) {
      state.chart.destroy();
      state.chart = null;
    }
    const chartNote = $('forecastChartNote');
    if (chartNote) chartNote.textContent = '';
    const fallback = $('forecastChartFallback');
    if (fallback) {
      fallback.hidden = false;
      fallback.textContent = 'Belum ada hasil forecast untuk ditampilkan.';
    }
  }

  async function loadDetail(code) {
    if (!code || !window.apiClient) {
      renderEmptyDetail();
      return;
    }
    const requestId = ++state.detailRequestId;
    try {
      const response = await window.apiClient.get(`/forecast/${encodeURIComponent(code)}`);
      if (requestId !== state.detailRequestId || code !== state.selectedCode) return;
      if (!response.data) throw new Error('Detail forecast tidak ditemukan.');
      renderDetailMetrics(response.data);
    } catch (error) {
      if (requestId !== state.detailRequestId) return;
      renderEmptyDetail();
      notify('error', 'Gagal memuat detail produk', error.message);
    }
  }

  async function loadDashboard() {
    if (!window.apiClient) {
      notify('error', 'API Client tidak tersedia', 'Periksa pemuatan /static/js/components/api.js.');
      return false;
    }

    const requestId = ++state.listRequestId;
    try {
      const params = {
        page: state.page,
        limit: state.limit,
        sort: state.sort,
      };
      if (state.status) params.status = state.status;
      if (state.query) params.q = state.query;

      const [summaryResponse, listResponse] = await Promise.all([
        window.apiClient.get('/forecast/summary'),
        window.apiClient.get('/forecast', params),
      ]);
      if (requestId !== state.listRequestId) return false;

      renderSummary(summaryResponse.data || {});
      state.rows = Array.isArray(listResponse.data) ? listResponse.data : [];
      state.total = Number(listResponse.meta?.total) || 0;
      renderTable();
      renderPaginationMeta(listResponse.meta);
      renderProductOptions();

      if (state.selectedCode) {
        await loadDetail(state.selectedCode);
      } else {
        renderEmptyDetail();
      }
      return true;
    } catch (error) {
      if (requestId !== state.listRequestId) return false;
      console.error('[Forecast] Dashboard load failed:', error);
      notify('error', 'Gagal memuat data forecast', error.message);
      const tbody = document.querySelector('#forecastTable tbody');
      if (tbody) {
        tbody.replaceChildren();
        const row = document.createElement('tr');
        const cell = createCell('py-10 px-space-md text-center text-error', 'Data gagal dimuat. Periksa koneksi server lalu coba lagi.');
        cell.colSpan = 8;
        row.append(cell);
        tbody.append(row);
      }
      return false;
    }
  }

  function updateFilter(status) {
    state.status = status === 'ALL' ? '' : status;
    state.page = 1;
    state.selectedCode = '';   // pilih ulang produk teratas sesuai status baru
    state.selectedLabel = '';
    loadDashboard();
  }

  function bindForecastSidebar() {
    const toggle = $('forecastMenuToggle');
    const overlay = $('forecastSidebarOverlay');
    const sidebar = $('forecastSidebar');
    if (!toggle || !overlay || !sidebar) return;

    const setOpen = (open) => {
      document.body.classList.toggle('forecast-sidebar-open', open);
      toggle.setAttribute('aria-expanded', String(open));
      toggle.setAttribute('aria-label', open ? 'Tutup menu navigasi' : 'Buka menu navigasi');
      overlay.tabIndex = open ? 0 : -1;
      sidebar.setAttribute('aria-hidden', String(!open && window.innerWidth <= 1100));
      sidebar.inert = !open && window.innerWidth <= 1100;
    };

    toggle.addEventListener('click', () => {
      setOpen(!document.body.classList.contains('forecast-sidebar-open'));
    });
    overlay.addEventListener('click', () => setOpen(false));
    sidebar.querySelectorAll('a').forEach((link) => {
      link.addEventListener('click', () => setOpen(false));
    });
    document.addEventListener('keydown', (event) => {
      if (event.key === 'Escape') setOpen(false);
    });
    window.addEventListener('resize', () => {
      if (window.innerWidth > 1100) setOpen(false);
    });
  }

  function bindEvents() {
    bindForecastSidebar();
    const button = $('btnRecalculate');
    const spinIcon = $('spinIcon');
    if (button) {
      button.addEventListener('click', async () => {
        if (button.disabled) return;
        button.disabled = true;
        if (spinIcon) spinIcon.classList.add('rotate-[720deg]');
        button.classList.add('opacity-90');
        try {
          await window.apiClient.post('/forecast/recalculate', {});
          state.page = 1;
          state.selectedCode = '';
          const loaded = await loadDashboard();
          if (loaded) notify('success', 'Kalkulasi selesai', 'Hasil forecast dan ringkasan berhasil diperbarui.');
        } catch (error) {
          notify('error', 'Kalkulasi gagal', error.message);
        } finally {
          button.disabled = false;
          button.classList.remove('opacity-90');
          if (spinIcon) spinIcon.classList.remove('rotate-[720deg]');
        }
      });
    }

    const search = $('tableSearch');
    if (search) {
      search.addEventListener('input', (event) => {
        window.clearTimeout(state.searchTimer);
        state.searchTimer = window.setTimeout(() => {
          state.query = event.target.value.trim();
          state.page = 1;
          loadDashboard();
        }, 300);
      });
    }

    const clearFilter = $('forecastClearFilter');
    if (clearFilter) clearFilter.addEventListener('click', () => updateFilter('ALL'));

    const sort = $('forecastSort');
    if (sort) {
      sort.addEventListener('change', () => {
        state.sort = sort.value || 'restock_qty';
        state.page = 1;
        loadDashboard();
      });
    }

    const productInput = $('forecastProductSearch');
    const productList = $('forecastProductList');
    if (productInput && productList) {
      productInput.addEventListener('focus', () => {
        productInput.select();
        searchProducts('');
      });
      productInput.addEventListener('input', () => {
        window.clearTimeout(state.productSearchTimer);
        state.productSearchTimer = window.setTimeout(() => {
          searchProducts(productInput.value.trim());
        }, 250);
      });
      productInput.addEventListener('keydown', (event) => {
        if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
          event.preventDefault();
          if (productList.hidden) { searchProducts(''); return; }
          setActiveOption(state.activeIndex + (event.key === 'ArrowDown' ? 1 : -1));
        } else if (event.key === 'Enter') {
          event.preventDefault();
          const options = state.productOptions;
          const item = options[state.activeIndex] || (options.length === 1 ? options[0] : null);
          if (item) selectProduct(item.kode_barang);
        } else if (event.key === 'Escape') {
          closeProductList();
        }
      });
      productInput.addEventListener('blur', () => closeProductList());
      // mousedown (bukan click) + preventDefault supaya input tidak blur sebelum item terpilih
      productList.addEventListener('mousedown', (event) => {
        event.preventDefault();
        const li = event.target.closest('li[data-code]');
        if (li) selectProduct(li.dataset.code);
      });
    }

    const tbody = document.querySelector('#forecastTable tbody');
    if (tbody) {
      const selectRow = (row) => {
        const code = row?.dataset.forecastCode;
        if (!code) return;
        state.selectedCode = code;
        loadDetail(code);
        $('forecastProductName')?.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
      };
      tbody.addEventListener('click', (event) => {
        const row = event.target.closest('tr[data-forecast-code]');
        if (row) selectRow(row);
      });
      tbody.addEventListener('keydown', (event) => {
        if (event.key !== 'Enter' && event.key !== ' ') return;
        const row = event.target.closest('tr[data-forecast-code]');
        if (!row) return;
        event.preventDefault();
        selectRow(row);
      });
    }

    const previous = $('forecastPrevPage');
    const next = $('forecastNextPage');
    if (previous) {
      previous.addEventListener('click', () => {
        if (state.page > 1) {
          state.page -= 1;
          loadDashboard();
        }
      });
    }
    if (next) {
      next.addEventListener('click', () => {
        if (state.page * state.limit < state.total) {
          state.page += 1;
          loadDashboard();
        }
      });
    }

  }

  window.filterStatus = updateFilter;

  document.addEventListener('DOMContentLoaded', () => {
    bindEvents();
    loadDashboard();
  });
})();