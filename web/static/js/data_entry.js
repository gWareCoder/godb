/**
 * Data Entry & Management Screen: Dynamic Forms, Smart FK Dropdowns, Data Grid, and CRUD
 */
window.DataEntry = {
  currentTable: null,
  tableSchema: null,
  rows: [],
  columns: [],
  page: 1,
  pageSize: 25,
  totalPages: 1,
  totalRows: 0,
  search: '',
  sortBy: '',
  sortOrder: 'asc',
  searchTimeout: null,
  editingPK: null, // If set, modal is in Edit mode

  init() {
    this.bindEvents();
  },

  bindEvents() {
    // Search input with debounce
    const searchInput = document.getElementById('data-search-input');
    searchInput?.addEventListener('input', (e) => {
      clearTimeout(this.searchTimeout);
      this.searchTimeout = setTimeout(() => {
        this.search = e.target.value.trim();
        this.page = 1;
        this.loadData();
      }, 300);
    });

    // Add Record Button
    document.getElementById('btn-add-record')?.addEventListener('click', () => {
      this.openRecordModal(null);
    });

    // Refresh Data Button
    document.getElementById('btn-refresh-data')?.addEventListener('click', () => {
      this.loadData();
    });

    // Export CSV Button
    document.getElementById('btn-export-csv')?.addEventListener('click', () => {
      this.exportCSV();
    });

    // Pagination
    document.getElementById('btn-prev-page')?.addEventListener('click', () => {
      if (this.page > 1) {
        this.page--;
        this.loadData();
      }
    });

    document.getElementById('btn-next-page')?.addEventListener('click', () => {
      if (this.page < this.totalPages) {
        this.page++;
        this.loadData();
      }
    });

    document.getElementById('page-size-select')?.addEventListener('change', (e) => {
      this.pageSize = parseInt(e.target.value, 10);
      this.page = 1;
      this.loadData();
    });

    // Record Form Submit
    document.getElementById('form-record')?.addEventListener('submit', async (e) => {
      e.preventDefault();
      await this.saveRecord();
    });
  },

  onTabActivate() {
    if (!this.currentTable && App.schema?.tables?.length > 0) {
      this.selectTable(App.schema.tables[0].name);
    } else if (this.currentTable) {
      this.loadData();
    }
  },

  updateTables(tables) {
    const list = document.getElementById('data-tables-list');
    if (!list) return;

    list.innerHTML = '';
    if (!tables || tables.length === 0) {
      list.innerHTML = '<li style="padding:1rem;color:var(--text-dim);font-size:0.8rem;">No tables yet</li>';
      return;
    }

    tables.forEach(tbl => {
      const li = document.createElement('li');
      li.className = `table-list-item ${tbl.name === this.currentTable ? 'active' : ''}`;
      li.dataset.table = tbl.name;
      li.innerHTML = `
        <span style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap;">📋 ${escapeHTML(tbl.name)}</span>
        <span class="table-row-count">${tbl.row_count}</span>
      `;
      li.addEventListener('click', () => this.selectTable(tbl.name));
      list.appendChild(li);
    });

    if (!this.currentTable && tables.length > 0) {
      this.selectTable(tables[0].name);
    }
  },

  selectTable(tableName) {
    this.currentTable = tableName;
    this.page = 1;
    this.search = '';
    this.sortBy = '';
    this.sortOrder = 'asc';

    const searchInput = document.getElementById('data-search-input');
    if (searchInput) searchInput.value = '';

    // Update active highlight in sidebar
    document.querySelectorAll('#data-tables-list .table-list-item').forEach(li => {
      li.classList.toggle('active', li.dataset.table === tableName);
    });

    // Update Header
    const titleEl = document.getElementById('current-table-title');
    if (titleEl) titleEl.textContent = tableName;

    this.tableSchema = (App.schema?.tables || []).find(t => t.name === tableName);
    this.loadData();
  },

  async loadData() {
    if (!this.currentTable) return;
    const tbody = document.getElementById('data-table-body');
    const thead = document.getElementById('data-table-head');
    if (!tbody || !thead) return;

    tbody.innerHTML = `<tr><td colspan="100" style="text-align:center;padding:2rem;color:var(--text-dim);">Loading records...</td></tr>`;

    try {
      const res = await API.getRows(this.currentTable, {
        page: this.page,
        pageSize: this.pageSize,
        search: this.search,
        sortBy: this.sortBy,
        sortOrder: this.sortOrder,
      });

      const data = res.data;
      this.columns = data.columns || [];
      this.rows = data.rows || [];
      this.totalRows = data.total_rows;
      this.totalPages = data.total_pages;

      this.renderTableHeaders(thead);
      this.renderTableRows(tbody);
      this.updatePaginationUI();
    } catch (err) {
      tbody.innerHTML = `<tr><td colspan="100" style="text-align:center;padding:2rem;color:var(--danger);">Error: ${escapeHTML(err.message)}</td></tr>`;
      App.showToast(err.message, 'error');
    }
  },

  renderTableHeaders(thead) {
    thead.innerHTML = '';
    const tr = document.createElement('tr');

    this.columns.forEach(col => {
      const th = document.createElement('th');
      th.className = 'sortable';
      
      let sortIcon = '↕️';
      if (this.sortBy === col.name) {
        sortIcon = this.sortOrder === 'asc' ? '🔼' : '🔽';
      }

      const fk = (this.tableSchema?.foreign_keys || []).find(f => f.column === col.name);

      th.innerHTML = `
        <div style="display:flex;align-items:center;justify-content:space-between;gap:0.5rem;">
          <span>
            ${col.primary_key ? '🔑 ' : (fk ? '🔗 ' : '')}${escapeHTML(col.name)}
          </span>
          <span style="font-size:0.65rem;opacity:0.7;">${sortIcon}</span>
        </div>
      `;

      th.addEventListener('click', () => {
        if (this.sortBy === col.name) {
          this.sortOrder = this.sortOrder === 'asc' ? 'desc' : 'asc';
        } else {
          this.sortBy = col.name;
          this.sortOrder = 'asc';
        }
        this.loadData();
      });

      tr.appendChild(th);
    });

    // Actions Header
    const thActions = document.createElement('th');
    thActions.textContent = 'Actions';
    thActions.style.width = '100px';
    thActions.style.textAlign = 'right';
    tr.appendChild(thActions);

    thead.appendChild(tr);
  },

  renderTableRows(tbody) {
    tbody.innerHTML = '';

    if (this.rows.length === 0) {
      tbody.innerHTML = `<tr><td colspan="${this.columns.length + 1}" style="text-align:center;padding:3rem;color:var(--text-dim);">No records found. Click "+ Add Record" to create one.</td></tr>`;
      return;
    }

    this.rows.forEach(row => {
      const tr = document.createElement('tr');

      this.columns.forEach(col => {
        const td = document.createElement('td');
        const val = row[col.name];

        // Format display value
        if (val === null || val === undefined) {
          td.innerHTML = '<span style="color:var(--text-dim);font-style:italic;">NULL</span>';
        } else if (typeof val === 'boolean' || col.type === 'BOOLEAN') {
          const isTrue = val === true || val === 1 || val === '1';
          td.innerHTML = `<span class="badge ${isTrue ? 'badge-pk' : 'badge-type'}">${isTrue ? 'TRUE' : 'FALSE'}</span>`;
        } else {
          const fk = (this.tableSchema?.foreign_keys || []).find(f => f.column === col.name);
          if (fk) {
            td.innerHTML = `<span class="badge badge-fk" title="References ${fk.ref_table}.${fk.ref_column}">#${escapeHTML(String(val))}</span>`;
          } else {
            td.textContent = String(val);
          }
        }

        tr.appendChild(td);
      });

      // Actions Cell
      const tdActions = document.createElement('td');
      tdActions.className = 'actions-cell';
      tdActions.innerHTML = `
        <button class="btn btn-secondary btn-sm btn-icon btn-edit" title="Edit Record">✏️</button>
        <button class="btn btn-danger btn-sm btn-icon btn-delete" title="Delete Record">🗑️</button>
      `;

      tdActions.querySelector('.btn-edit').addEventListener('click', () => {
        this.openRecordModal(row);
      });

      tdActions.querySelector('.btn-delete').addEventListener('click', async () => {
        const pk = this.extractPK(row);
        if (confirm(`Are you sure you want to delete this record?`)) {
          try {
            await API.deleteRow(this.currentTable, pk);
            App.showToast('Record deleted successfully', 'success');
            await this.loadData();
            await App.refreshSchema();
          } catch (err) {
            App.showToast(err.message, 'error');
          }
        }
      });

      tr.appendChild(tdActions);
      tbody.appendChild(tr);
    });
  },

  updatePaginationUI() {
    const pageIndicator = document.getElementById('data-pagination-info');
    if (pageIndicator) {
      const start = this.totalRows === 0 ? 0 : (this.page - 1) * this.pageSize + 1;
      const end = Math.min(this.page * this.pageSize, this.totalRows);
      pageIndicator.textContent = `Showing ${start}-${end} of ${this.totalRows} records (Page ${this.page} of ${this.totalPages})`;
    }

    const prevBtn = document.getElementById('btn-prev-page');
    const nextBtn = document.getElementById('btn-next-page');
    if (prevBtn) prevBtn.disabled = this.page <= 1;
    if (nextBtn) nextBtn.disabled = this.page >= this.totalPages;
  },

  extractPK(row) {
    const pk = {};
    const pkCols = (this.tableSchema?.primary_keys && this.tableSchema.primary_keys.length > 0)
      ? this.tableSchema.primary_keys
      : (this.columns.length > 0 ? [this.columns[0].name] : []);

    pkCols.forEach(col => {
      pk[col] = row[col];
    });
    return pk;
  },

  /**
   * Opens the Dynamic Form Modal for Add or Edit
   */
  async openRecordModal(existingRow = null) {
    const modalTitle = document.getElementById('record-modal-title');
    const formFields = document.getElementById('record-form-fields');
    if (!formFields) return;

    this.editingPK = existingRow ? this.extractPK(existingRow) : null;
    modalTitle.textContent = existingRow
      ? `Edit Record in '${this.currentTable}'`
      : `Add Record to '${this.currentTable}'`;

    formFields.innerHTML = '<div style="text-align:center;padding:1rem;color:var(--text-dim);">Preparing form...</div>';
    App.openModal('modal-record');

    const fieldsHTML = [];

    // Pre-fetch FK options for all foreign key columns in parallel
    const fks = this.tableSchema?.foreign_keys || [];
    const fkOptionsMap = {};

    for (const fk of fks) {
      try {
        const res = await API.getFKOptions(this.currentTable, fk.ref_table, fk.ref_column);
        fkOptionsMap[fk.column] = res.data || [];
      } catch (err) {
        console.error(`Failed to load FK options for ${fk.column}:`, err);
        fkOptionsMap[fk.column] = [];
      }
    }

    formFields.innerHTML = '';

    for (const col of this.columns) {
      const isPK = col.primary_key;
      const isAI = col.auto_increment;
      const isFK = fks.some(f => f.column === col.name);
      const curVal = existingRow ? existingRow[col.name] : '';

      // Skip autoincrement primary key when adding new row
      if (!existingRow && isPK && isAI) {
        const div = document.createElement('div');
        div.className = 'form-group';
        div.innerHTML = `
          <label>${escapeHTML(col.name)} (Primary Key)</label>
          <input type="text" class="form-control" value="Auto-generated by database" disabled style="opacity:0.6;font-style:italic;">
        `;
        formFields.appendChild(div);
        continue;
      }

      const formGroup = document.createElement('div');
      formGroup.className = 'form-group';

      const labelText = `${escapeHTML(col.name)} ${col.not_null ? '<span style="color:var(--danger)">*</span>' : ''}`;
      
      if (isFK) {
        // Render Smart Foreign Key Dropdown
        const options = fkOptionsMap[col.name] || [];
        let optionsHTML = `<option value="">${col.not_null ? '-- Select linked record --' : '-- None (NULL) --'}</option>`;
        options.forEach(opt => {
          const selected = existingRow && String(curVal) === String(opt.value) ? 'selected' : '';
          optionsHTML += `<option value="${escapeHTML(String(opt.value))}" ${selected}>${escapeHTML(opt.label)}</option>`;
        });

        formGroup.innerHTML = `
          <label>${labelText} <span class="badge badge-fk">🔗 References ${fks.find(f => f.column === col.name).ref_table}</span></label>
          <select name="${escapeHTML(col.name)}" class="form-control" ${col.not_null ? 'required' : ''}>
            ${optionsHTML}
          </select>
        `;
      } else if (col.type === 'BOOLEAN') {
        const isChecked = existingRow ? (curVal === true || curVal === 1 || curVal === '1') : false;
        formGroup.innerHTML = `
          <label class="form-check" style="margin-top:0.5rem;">
            <input type="checkbox" name="${escapeHTML(col.name)}" ${isChecked ? 'checked' : ''}>
            <span style="font-weight:600;">${labelText}</span>
          </label>
        `;
      } else if (col.type === 'DATETIME' || col.type === 'DATE') {
        let valFormatted = curVal || '';
        // If ISO string, convert for datetime-local
        if (valFormatted && valFormatted.includes('T')) {
          valFormatted = valFormatted.substring(0, 16);
        }
        formGroup.innerHTML = `
          <label>${labelText} <span class="badge badge-type">${col.type}</span></label>
          <input type="${col.type === 'DATE' ? 'date' : 'datetime-local'}" name="${escapeHTML(col.name)}" class="form-control" value="${escapeHTML(String(valFormatted))}" ${col.not_null && !existingRow ? 'required' : ''}>
        `;
      } else if (col.type === 'INTEGER' || col.type === 'REAL') {
        const step = col.type === 'REAL' ? 'any' : '1';
        formGroup.innerHTML = `
          <label>${labelText} <span class="badge badge-type">${col.type}</span></label>
          <input type="number" step="${step}" name="${escapeHTML(col.name)}" class="form-control" value="${curVal !== null && curVal !== undefined ? escapeHTML(String(curVal)) : ''}" ${col.not_null && !existingRow ? 'required' : ''} ${isPK && existingRow ? 'readonly style="opacity:0.7;"' : ''}>
        `;
      } else if (col.type === 'TEXT' && (col.name.includes('description') || col.name.includes('address') || col.name.includes('bio'))) {
        formGroup.innerHTML = `
          <label>${labelText} <span class="badge badge-type">TEXT</span></label>
          <textarea name="${escapeHTML(col.name)}" class="form-control" rows="3" ${col.not_null && !existingRow ? 'required' : ''}>${escapeHTML(String(curVal || ''))}</textarea>
        `;
      } else {
        // Standard text
        formGroup.innerHTML = `
          <label>${labelText} <span class="badge badge-type">${escapeHTML(col.type)}</span></label>
          <input type="text" name="${escapeHTML(col.name)}" class="form-control" value="${curVal !== null && curVal !== undefined ? escapeHTML(String(curVal)) : ''}" ${col.not_null && !existingRow ? 'required' : ''} ${isPK && existingRow ? 'readonly style="opacity:0.7;"' : ''}>
        `;
      }

      formFields.appendChild(formGroup);
    }
  },

  async saveRecord() {
    const form = document.getElementById('form-record');
    const formData = new FormData(form);
    const rowData = {};

    this.columns.forEach(col => {
      // Check if boolean checkbox
      if (col.type === 'BOOLEAN') {
        const input = form.querySelector(`[name="${col.name}"]`);
        if (input && input.type === 'checkbox') {
          rowData[col.name] = input.checked ? 1 : 0;
          return;
        }
      }

      const val = formData.get(col.name);
      if (val === null || val === '') {
        if (!col.not_null) {
          rowData[col.name] = null;
        } else {
          rowData[col.name] = '';
        }
      } else if (col.type === 'INTEGER') {
        rowData[col.name] = parseInt(val, 10);
      } else if (col.type === 'REAL') {
        rowData[col.name] = parseFloat(val);
      } else {
        rowData[col.name] = val;
      }
    });

    try {
      if (this.editingPK) {
        await API.updateRow(this.currentTable, this.editingPK, rowData);
        App.showToast('Record updated successfully', 'success');
      } else {
        await API.insertRow(this.currentTable, rowData);
        App.showToast('Record added successfully', 'success');
      }
      App.closeModal('modal-record');
      await this.loadData();
      await App.refreshSchema();
    } catch (err) {
      App.showToast(err.message, 'error');
    }
  },

  exportCSV() {
    if (this.rows.length === 0) {
      App.showToast('No data to export', 'error');
      return;
    }

    const headers = this.columns.map(c => `"${c.name.replace(/"/g, '""')}"`).join(',');
    const rowsCSV = this.rows.map(row => {
      return this.columns.map(c => {
        const val = row[c.name];
        if (val === null || val === undefined) return '';
        return `"${String(val).replace(/"/g, '""')}"`;
      }).join(',');
    });

    const csvContent = 'data:text/csv;charset=utf-8,' + [headers, ...rowsCSV].join('\n');
    const encodedUri = encodeURI(csvContent);
    const link = document.createElement('a');
    link.setAttribute('href', encodedUri);
    link.setAttribute('download', `${this.currentTable}_export.csv`);
    document.body.appendChild(link);
    link.click();
    link.remove();
  }
};
