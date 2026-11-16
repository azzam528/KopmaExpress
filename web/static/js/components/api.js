/**
 * KOPMA Express - API Client Helper & Utilities
 * Compliant with PRD §6 and §8.6
 */

const API_BASE = '/api/v1';

const apiClient = {
  async request(endpoint, options = {}) {
    const url = endpoint.startsWith('http') ? endpoint : `${API_BASE}${endpoint}`;
    const defaultHeaders = {
      'Accept': 'application/json',
    };

    if (!(options.body instanceof FormData)) {
      defaultHeaders['Content-Type'] = 'application/json';
    }

    const config = {
      ...options,
      headers: {
        ...defaultHeaders,
        ...(options.headers || {}),
      },
    };

    try {
      const response = await fetch(url, config);
      const json = await response.json();

      if (!response.ok || !json.success) {
        const error = new Error(json.error?.message || 'Permintaan gagal');
        error.status = response.status;
        error.code = json.error?.code || 'UNKNOWN_ERROR';
        error.details = json.error?.details || null;
        throw error;
      }

      return json;
    } catch (err) {
      console.error('[API Error]', err);
      throw err;
    }
  },

  get(endpoint, params = {}) {
    const query = new URLSearchParams(params).toString();
    const url = query ? `${endpoint}?${query}` : endpoint;
    return this.request(url, { method: 'GET' });
  },

  post(endpoint, data) {
    return this.request(endpoint, {
      method: 'POST',
      body: data instanceof FormData ? data : JSON.stringify(data),
    });
  },

  put(endpoint, data) {
    return this.request(endpoint, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  },

  delete(endpoint) {
    return this.request(endpoint, { method: 'DELETE' });
  },
};

// Utilities for formatting
const formatCurrency = (amount) => {
  return new Intl.NumberFormat('id-ID', {
    style: 'currency',
    currency: 'IDR',
    maximumFractionDigits: 0,
  }).format(amount || 0);
};

const formatNumber = (val) => {
  return new Intl.NumberFormat('id-ID').format(val || 0);
};

// Attach to global window
window.apiClient = apiClient;
window.formatCurrency = formatCurrency;
window.formatNumber = formatNumber;
