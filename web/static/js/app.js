/**
 * Main application coordinator for GoDB
 */
const App = {
  activeDB: 'default',
  schema: { tables: [], relations: [] },
  activeTab: 'visualizer',
  selectedTable: null,

  async init() {
    this.bindEvents();
    await this.loadDatabases();
    await this.refreshSchema();
    this.switchTab('visualizer');
  },

  bindEvents() {
    // Navigation Tabs
    document.querySelectorAll('.nav-tab-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        const tab = e.currentTarget.dataset.tab;
        this.switchTab(tab);
      });
    });

    // DB Switcher
    const dbSelect = document.getElementById('db-select');
    if (dbSelect) {
      dbSelect.addEventListener('change', async (e) => {
        const targetDB = e.target.value;
        if (targetDB === '__new__') {
          dbSelect.value = this.activeDB;
          this.openModal('modal-create-db');
          return;
        }
        try {
          await API.switchDatabase(targetDB);
          this.activeDB = targetDB;
          this.showToast(`Switched to database '${targetDB}'`, 'success');
          await this.refreshSchema();
        } catch (err) {
          this.showToast(err.message, 'error');
          dbSelect.value = this.activeDB;
        }
      });
    }

    // Modal Close buttons
    document.querySelectorAll('.modal-close-btn, .modal-cancel-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        const modal = e.target.closest('.modal-overlay');
        if (modal) modal.classList.remove('open');
      });
    });

    // Close on overlay backdrop click
    document.querySelectorAll('.modal-overlay').forEach(overlay => {
      overlay.addEventListener('click', (e) => {
        if (e.target === overlay) overlay.classList.remove('open');
      });
    });

    // Create DB Form
    const createDbForm = document.getElementById('form-create-db');
    if (createDbForm) {
      createDbForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const input = document.getElementById('new-db-name');
        const dbName = input.value.trim();
        if (!dbName) return;

        try {
          await API.createDatabase(dbName);
          this.closeModal('modal-create-db');
          input.value = '';
          this.showToast(`Database '${dbName}' created`, 'success');
          await this.loadDatabases();
          await this.refreshSchema();
        } catch (err) {
          this.showToast(err.message, 'error');
        }
      });
    }

    // Load Sample Button
    const btnLoadSample = document.getElementById('btn-load-sample');
    if (btnLoadSample) {
      btnLoadSample.addEventListener('click', async () => {
        if (!confirm(`Load sample E-Commerce tables into database '${this.activeDB}'? (Existing sample tables will be re-created)`)) {
          return;
        }
        try {
          await API.loadSample();
          this.showToast('Sample E-Commerce database loaded successfully!', 'success');
          await this.refreshSchema();
        } catch (err) {
          this.showToast(err.message, 'error');
        }
      });
    }
  },

  async loadDatabases() {
    try {
      const res = await API.listDatabases();
      const select = document.getElementById('db-select');
      if (!select) return;

      select.innerHTML = '';
      const dbs = res.data.databases || [];
      this.activeDB = res.data.active || (dbs[0] ? dbs[0].name : 'default');

      dbs.forEach(db => {
        const opt = document.createElement('option');
        opt.value = db.name;
        opt.textContent = `${db.name} (${this.formatBytes(db.size)})`;
        if (db.name === this.activeDB) opt.selected = true;
        select.appendChild(opt);
      });

      // Add "New Database..." option at bottom
      const newOpt = document.createElement('option');
      newOpt.value = '__new__';
      newOpt.textContent = '+ Create New Database...';
      select.appendChild(newOpt);
    } catch (err) {
      console.error('Failed to load databases:', err);
    }
  },

  async refreshSchema() {
    try {
      const res = await API.getSchema();
      this.schema = res.data.schema;
      this.activeDB = res.data.database;

      // Update badge
      const activeDbBadge = document.getElementById('active-db-badge');
      if (activeDbBadge) activeDbBadge.textContent = this.activeDB;

      // Notify modules
      if (window.Visualizer && typeof Visualizer.render === 'function') {
        Visualizer.render(this.schema);
      }
      if (window.DataEntry && typeof DataEntry.updateTables === 'function') {
        DataEntry.updateTables(this.schema.tables);
      }
      if (window.QueryBuilder && typeof QueryBuilder.updateSchema === 'function') {
        QueryBuilder.updateSchema(this.schema);
      }
    } catch (err) {
      this.showToast('Failed to load schema: ' + err.message, 'error');
    }
  },

  switchTab(tabName) {
    this.activeTab = tabName;
    document.querySelectorAll('.nav-tab-btn').forEach(btn => {
      btn.classList.toggle('active', btn.dataset.tab === tabName);
    });
    document.querySelectorAll('.tab-pane').forEach(pane => {
      pane.classList.toggle('active', pane.id === `tab-${tabName}`);
    });

    if (tabName === 'visualizer' && window.Visualizer) {
      Visualizer.onTabActivate();
    } else if (tabName === 'data' && window.DataEntry) {
      DataEntry.onTabActivate();
    } else if (tabName === 'query' && window.QueryBuilder) {
      QueryBuilder.onTabActivate();
    }
  },

  openModal(modalId) {
    const el = document.getElementById(modalId);
    if (el) el.classList.add('open');
  },

  closeModal(modalId) {
    const el = document.getElementById(modalId);
    if (el) el.classList.remove('open');
  },

  showToast(message, type = 'info') {
    const container = document.getElementById('toast-container');
    if (!container) return;

    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    
    let icon = 'ℹ️';
    if (type === 'success') icon = '✅';
    if (type === 'error') icon = '⚠️';

    toast.innerHTML = `<span>${icon}</span><span style="flex:1">${escapeHTML(message)}</span>`;
    container.appendChild(toast);

    setTimeout(() => {
      toast.style.opacity = '0';
      toast.style.transform = 'translateX(100%)';
      toast.style.transition = 'all 0.25s ease';
      setTimeout(() => toast.remove(), 250);
    }, 4000);
  },

  formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  }
};

function escapeHTML(str) {
  if (typeof str !== 'string') return String(str);
  return str.replace(/[&<>'"]/g, 
    tag => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' }[tag] || tag)
  );
}

document.addEventListener('DOMContentLoaded', () => {
  App.init();
});
