/**
 * Query Builder Module: Visual SQL Query Construction, Real-time Preview, and Raw SQL Console
 */
window.QueryBuilder = {
  schema: null,
  baseTable: null,
  selectedColumns: new Set(),
  joins: [],
  conditions: [],
  orderBy: '',
  orderDir: 'ASC',
  limit: 50,
  lastResults: null,

  init() {
    this.bindEvents();
  },

  bindEvents() {
    // Mode Switcher (Visual vs Raw SQL)
    const modeTabs = document.querySelectorAll('.qb-mode-btn');
    modeTabs.forEach(btn => {
      btn.addEventListener('click', (e) => {
        const mode = e.currentTarget.dataset.mode;
        modeTabs.forEach(b => b.classList.toggle('active', b === e.currentTarget));
        document.getElementById('qb-visual-panel').style.display = mode === 'visual' ? 'block' : 'none';
        document.getElementById('qb-raw-panel').style.display = mode === 'raw' ? 'block' : 'none';
      });
    });

    // Base Table Select
    const baseTableSelect = document.getElementById('qb-base-table');
    baseTableSelect?.addEventListener('change', (e) => {
      this.setBaseTable(e.target.value);
    });

    // Add Join Button
    document.getElementById('btn-add-join')?.addEventListener('click', () => {
      this.appendJoinClause();
    });

    // Add Condition Button
    document.getElementById('btn-add-condition')?.addEventListener('click', () => {
      this.appendConditionClause();
    });

    // Order By
    document.getElementById('qb-order-by')?.addEventListener('change', (e) => {
      this.orderBy = e.target.value;
      this.updatePreview();
    });

    document.getElementById('qb-order-dir')?.addEventListener('change', (e) => {
      this.orderDir = e.target.value;
      this.updatePreview();
    });

    // Limit
    document.getElementById('qb-limit')?.addEventListener('input', (e) => {
      this.limit = parseInt(e.target.value, 10) || 50;
      this.updatePreview();
    });

    // Run Visual Query Button
    document.getElementById('btn-run-visual-query')?.addEventListener('click', () => {
      this.runVisualQuery();
    });

    // Run Raw SQL Button
    document.getElementById('btn-run-raw-sql')?.addEventListener('click', () => {
      this.runRawSQL();
    });

    // Keyboard shortcut Ctrl+Enter or Cmd+Enter for Raw SQL
    const rawSqlArea = document.getElementById('raw-sql-input');
    rawSqlArea?.addEventListener('keydown', (e) => {
      if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
        e.preventDefault();
        this.runRawSQL();
      }
    });

    // Raw SQL Templates
    document.getElementById('sql-template-select')?.addEventListener('change', (e) => {
      const template = e.target.value;
      if (!template) return;
      this.applyTemplate(template);
      e.target.value = '';
    });

    // Export Results Buttons
    document.getElementById('btn-export-results-csv')?.addEventListener('click', () => {
      this.exportResults('csv');
    });
    document.getElementById('btn-export-results-json')?.addEventListener('click', () => {
      this.exportResults('json');
    });
  },

  onTabActivate() {
    this.updatePreview();
  },

  updateSchema(schema) {
    this.schema = schema;
    const baseSelect = document.getElementById('qb-base-table');
    if (!baseSelect) return;

    baseSelect.innerHTML = '<option value="">-- Choose Base Table --</option>';
    (schema.tables || []).forEach(tbl => {
      const opt = document.createElement('option');
      opt.value = tbl.name;
      opt.textContent = `${tbl.name} (${tbl.row_count} rows)`;
      if (tbl.name === this.baseTable) opt.selected = true;
      baseSelect.appendChild(opt);
    });

    if (!this.baseTable && schema.tables?.length > 0) {
      this.setBaseTable(schema.tables[0].name);
      baseSelect.value = schema.tables[0].name;
    } else if (this.baseTable) {
      this.refreshColumnsUI();
    }
  },

  setBaseTable(tableName) {
    this.baseTable = tableName;
    this.selectedColumns.clear();
    this.joins = [];
    this.conditions = [];

    // Clear Join & Condition UI
    const joinsContainer = document.getElementById('qb-joins-container');
    const condsContainer = document.getElementById('qb-conditions-container');
    if (joinsContainer) joinsContainer.innerHTML = '';
    if (condsContainer) condsContainer.innerHTML = '';

    // Check for suggested joins based on schema relationships
    this.renderSuggestedJoins();

    this.refreshColumnsUI();
    this.refreshOrderOptions();
    this.updatePreview();
  },

  renderSuggestedJoins() {
    const suggestedBox = document.getElementById('qb-suggested-joins');
    if (!suggestedBox) return;

    suggestedBox.innerHTML = '';
    if (!this.baseTable || !this.schema) return;

    // Find relations where baseTable is from_table or to_table
    const relevantRelations = (this.schema.relations || []).filter(
      r => r.from_table === this.baseTable || r.to_table === this.baseTable
    );

    if (relevantRelations.length === 0) return;

    const wrapper = document.createElement('div');
    wrapper.style.marginBottom = '0.75rem';
    wrapper.innerHTML = `<span style="font-size:0.75rem;font-weight:600;color:var(--text-dim);">Suggested 1-Click Joins:</span>`;

    const pillsContainer = document.createElement('div');
    pillsContainer.style.display = 'flex';
    pillsContainer.style.flexWrap = 'wrap';
    pillsContainer.style.gap = '0.4rem';
    pillsContainer.style.marginTop = '0.3rem';

    relevantRelations.forEach(r => {
      const isOutbound = r.from_table === this.baseTable;
      const targetTbl = isOutbound ? r.to_table : r.from_table;
      const fromCol = isOutbound ? `${r.from_table}.${r.from_column}` : `${r.to_table}.${r.to_column}`;
      const toCol = isOutbound ? `${r.to_table}.${r.to_column}` : `${r.from_table}.${r.from_column}`;

      const btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'btn btn-secondary btn-sm';
      btn.style.fontSize = '0.75rem';
      btn.innerHTML = `+ JOIN <strong>${escapeHTML(targetTbl)}</strong>`;
      btn.addEventListener('click', () => {
        this.appendJoinClause({
          type: 'INNER',
          table: targetTbl,
          fromColumn: fromCol,
          toColumn: toCol,
        });
        btn.remove();
      });
      pillsContainer.appendChild(btn);
    });

    wrapper.appendChild(pillsContainer);
    suggestedBox.appendChild(wrapper);
  },

  refreshColumnsUI() {
    const container = document.getElementById('qb-columns-selector');
    if (!container) return;

    container.innerHTML = '';
    if (!this.baseTable || !this.schema) return;

    const baseTbl = (this.schema.tables || []).find(t => t.name === this.baseTable);
    if (!baseTbl) return;

    const tablesToInclude = [baseTbl];

    // Also include joined tables
    this.joins.forEach(j => {
      const jTbl = (this.schema.tables || []).find(t => t.name === j.table);
      if (jTbl && !tablesToInclude.includes(jTbl)) {
        tablesToInclude.push(jTbl);
      }
    });

    tablesToInclude.forEach(tbl => {
      const group = document.createElement('div');
      group.style.marginBottom = '0.75rem';
      group.innerHTML = `<div style="font-size:0.75rem;font-weight:700;color:var(--accent);margin-bottom:0.3rem;">📋 ${escapeHTML(tbl.name)}</div>`;

      const colsGrid = document.createElement('div');
      colsGrid.style.display = 'grid';
      colsGrid.style.gridTemplateColumns = 'repeat(auto-fill, minmax(130px, 1fr))';
      colsGrid.style.gap = '0.3rem';

      tbl.columns.forEach(col => {
        const fullColName = tablesToInclude.length > 1 ? `${tbl.name}.${col.name}` : col.name;
        const isChecked = this.selectedColumns.has(fullColName) || (this.selectedColumns.size === 0 && tbl === baseTbl);

        if (this.selectedColumns.size === 0 && tbl === baseTbl) {
          this.selectedColumns.add(fullColName);
        }

        const label = document.createElement('label');
        label.className = 'form-check';
        label.style.fontSize = '0.78rem';
        label.innerHTML = `
          <input type="checkbox" value="${escapeHTML(fullColName)}" ${this.selectedColumns.has(fullColName) ? 'checked' : ''}>
          <span style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap;" title="${col.name} (${col.type})">${col.primary_key ? '🔑 ' : ''}${escapeHTML(col.name)}</span>
        `;

        label.querySelector('input').addEventListener('change', (e) => {
          if (e.target.checked) {
            this.selectedColumns.add(fullColName);
          } else {
            this.selectedColumns.delete(fullColName);
          }
          this.updatePreview();
        });

        colsGrid.appendChild(label);
      });

      group.appendChild(colsGrid);
      container.appendChild(group);
    });

    this.refreshOrderOptions();
  },

  refreshOrderOptions() {
    const orderSelect = document.getElementById('qb-order-by');
    if (!orderSelect || !this.baseTable || !this.schema) return;

    orderSelect.innerHTML = '<option value="">-- None --</option>';
    const baseTbl = (this.schema.tables || []).find(t => t.name === this.baseTable);
    if (!baseTbl) return;

    baseTbl.columns.forEach(c => {
      const opt = document.createElement('option');
      opt.value = c.name;
      opt.textContent = `${c.name} (${c.type})`;
      if (c.name === this.orderBy) opt.selected = true;
      orderSelect.appendChild(opt);
    });
  },

  appendJoinClause(initial = {}) {
    const container = document.getElementById('qb-joins-container');
    if (!container) return;

    const joinIdx = this.joins.length;
    const joinObj = {
      type: initial.type || 'INNER',
      table: initial.table || '',
      fromColumn: initial.fromColumn || '',
      toColumn: initial.toColumn || '',
      customOn: initial.customOn || '',
    };
    this.joins.push(joinObj);

    const div = document.createElement('div');
    div.className = 'clause-item';
    div.style.flexWrap = 'wrap';

    let tableOptions = '<option value="">Target Table</option>';
    (this.schema?.tables || []).forEach(tbl => {
      if (tbl.name !== this.baseTable) {
        tableOptions += `<option value="${escapeHTML(tbl.name)}" ${tbl.name === joinObj.table ? 'selected' : ''}>${escapeHTML(tbl.name)}</option>`;
      }
    });

    div.innerHTML = `
      <select class="form-control join-type-sel" style="width:90px;">
        <option value="INNER" ${joinObj.type === 'INNER' ? 'selected' : ''}>INNER</option>
        <option value="LEFT" ${joinObj.type === 'LEFT' ? 'selected' : ''}>LEFT</option>
        <option value="CROSS" ${joinObj.type === 'CROSS' ? 'selected' : ''}>CROSS</option>
      </select>
      <select class="form-control join-table-sel" style="flex:1;">
        ${tableOptions}
      </select>
      <input type="text" class="form-control join-from-col" placeholder="source.col" value="${escapeHTML(joinObj.fromColumn)}" style="flex:1.2;">
      <span>=</span>
      <input type="text" class="form-control join-to-col" placeholder="target.col" value="${escapeHTML(joinObj.toColumn)}" style="flex:1.2;">
      <button type="button" class="btn btn-danger btn-sm btn-icon btn-remove-join">✕</button>
    `;

    const updateJoin = () => {
      joinObj.type = div.querySelector('.join-type-sel').value;
      joinObj.table = div.querySelector('.join-table-sel').value;
      joinObj.fromColumn = div.querySelector('.join-from-col').value.trim();
      joinObj.toColumn = div.querySelector('.join-to-col').value.trim();
      this.refreshColumnsUI();
      this.updatePreview();
    };

    div.querySelector('.join-type-sel').addEventListener('change', updateJoin);
    div.querySelector('.join-table-sel').addEventListener('change', updateJoin);
    div.querySelector('.join-from-col').addEventListener('input', updateJoin);
    div.querySelector('.join-to-col').addEventListener('input', updateJoin);

    div.querySelector('.btn-remove-join').addEventListener('click', () => {
      div.remove();
      this.joins = this.joins.filter(j => j !== joinObj);
      this.refreshColumnsUI();
      this.updatePreview();
    });

    container.appendChild(div);
    this.refreshColumnsUI();
    this.updatePreview();
  },

  appendConditionClause(initial = {}) {
    const container = document.getElementById('qb-conditions-container');
    if (!container) return;

    const condObj = {
      column: initial.column || '',
      operator: initial.operator || '=',
      value: initial.value || '',
      combinator: initial.combinator || 'AND',
    };
    this.conditions.push(condObj);

    const div = document.createElement('div');
    div.className = 'clause-item';

    div.innerHTML = `
      <select class="form-control cond-comb-sel" style="width:75px;">
        <option value="AND" ${condObj.combinator === 'AND' ? 'selected' : ''}>AND</option>
        <option value="OR" ${condObj.combinator === 'OR' ? 'selected' : ''}>OR</option>
      </select>
      <input type="text" class="form-control cond-col-input" placeholder="column" value="${escapeHTML(condObj.column)}" style="flex:1.5;">
      <select class="form-control cond-op-sel" style="width:105px;">
        <option value="=" ${condObj.operator === '=' ? 'selected' : ''}>=</option>
        <option value="!=" ${condObj.operator === '!=' ? 'selected' : ''}>!=</option>
        <option value=">" ${condObj.operator === '>' ? 'selected' : ''}>&gt;</option>
        <option value=">=" ${condObj.operator === '>=' ? 'selected' : ''}>&gt;=</option>
        <option value="<" ${condObj.operator === '<' ? 'selected' : ''}>&lt;</option>
        <option value="<=" ${condObj.operator === '<=' ? 'selected' : ''}>&lt;=</option>
        <option value="LIKE" ${condObj.operator === 'LIKE' ? 'selected' : ''}>LIKE</option>
        <option value="NOT LIKE" ${condObj.operator === 'NOT LIKE' ? 'selected' : ''}>NOT LIKE</option>
        <option value="IS NULL" ${condObj.operator === 'IS NULL' ? 'selected' : ''}>IS NULL</option>
        <option value="IS NOT NULL" ${condObj.operator === 'IS NOT NULL' ? 'selected' : ''}>IS NOT NULL</option>
        <option value="IN" ${condObj.operator === 'IN' ? 'selected' : ''}>IN</option>
      </select>
      <input type="text" class="form-control cond-val-input" placeholder="value" value="${escapeHTML(condObj.value)}" style="flex:2;">
      <button type="button" class="btn btn-danger btn-sm btn-icon btn-remove-cond">✕</button>
    `;

    const updateCond = () => {
      condObj.combinator = div.querySelector('.cond-comb-sel').value;
      condObj.column = div.querySelector('.cond-col-input').value.trim();
      condObj.operator = div.querySelector('.cond-op-sel').value;
      condObj.value = div.querySelector('.cond-val-input').value.trim();
      this.updatePreview();
    };

    div.querySelector('.cond-comb-sel').addEventListener('change', updateCond);
    div.querySelector('.cond-col-input').addEventListener('input', updateCond);
    div.querySelector('.cond-op-sel').addEventListener('change', updateCond);
    div.querySelector('.cond-val-input').addEventListener('input', updateCond);

    div.querySelector('.btn-remove-cond').addEventListener('click', () => {
      div.remove();
      this.conditions = this.conditions.filter(c => c !== condObj);
      this.updatePreview();
    });

    container.appendChild(div);
    this.updatePreview();
  },

  async updatePreview() {
    const previewEl = document.getElementById('qb-preview-sql');
    if (!previewEl || !this.baseTable) return;

    const payload = this.getBuilderPayload();
    try {
      const res = await API.previewQuery(payload);
      previewEl.textContent = res.data.sql;
      // Also sync with raw SQL editor
      const rawInput = document.getElementById('raw-sql-input');
      if (rawInput && (!rawInput.value || rawInput.dataset.synced === 'true')) {
        rawInput.value = res.data.sql;
        rawInput.dataset.synced = 'true';
      }
    } catch (err) {
      previewEl.textContent = '-- Error: ' + err.message;
    }
  },

  getBuilderPayload() {
    return {
      base_table: this.baseTable || '',
      columns: Array.from(this.selectedColumns),
      joins: this.joins.filter(j => j.table && (j.customOn || (j.fromColumn && j.toColumn))),
      conditions: this.conditions.filter(c => c.column),
      order_by: this.orderBy,
      order_dir: this.orderDir,
      limit: this.limit,
    };
  },

  async runVisualQuery() {
    const payload = this.getBuilderPayload();
    if (!payload.base_table) {
      App.showToast('Please select a base table', 'error');
      return;
    }

    try {
      const res = await API.buildAndExecuteQuery(payload);
      this.renderQueryResults(res.data);
      App.showToast(`Query completed in ${res.data.execution_ms.toFixed(2)} ms`, 'success');
    } catch (err) {
      this.renderQueryError(err.message);
      App.showToast(err.message, 'error');
    }
  },

  async runRawSQL() {
    const rawInput = document.getElementById('raw-sql-input');
    const query = rawInput?.value.trim();
    if (!query) {
      App.showToast('Query cannot be empty', 'error');
      return;
    }

    try {
      const res = await API.executeRawSQL(query);
      this.renderQueryResults(res.data);
      App.showToast(`Executed in ${res.data.execution_ms.toFixed(2)} ms`, 'success');
    } catch (err) {
      this.renderQueryError(err.message);
      App.showToast(err.message, 'error');
    }
  },

  applyTemplate(template) {
    const rawInput = document.getElementById('raw-sql-input');
    if (!rawInput) return;
    rawInput.dataset.synced = 'false';

    const tName = this.baseTable || 'customers';
    switch (template) {
      case 'select_all':
        rawInput.value = `SELECT * FROM "${tName}" LIMIT 25;`;
        break;
      case 'count_group':
        rawInput.value = `SELECT COUNT(*) AS total, "${tName}".* FROM "${tName}" GROUP BY 1;`;
        break;
      case 'join_sample':
        rawInput.value = `SELECT orders.id, customers.name, orders.total_amount, orders.status\nFROM orders\nJOIN customers ON orders.customer_id = customers.id\nORDER BY orders.id DESC\nLIMIT 20;`;
        break;
      case 'insert_sample':
        rawInput.value = `INSERT INTO "${tName}" DEFAULT VALUES;`;
        break;
      case 'tables_list':
        rawInput.value = `SELECT name, type FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%';`;
        break;
    }
  },

  renderQueryResults(result) {
    this.lastResults = result;
    const statusEl = document.getElementById('qb-results-status');
    const thead = document.getElementById('qb-results-thead');
    const tbody = document.getElementById('qb-results-tbody');
    const exportBar = document.getElementById('qb-export-bar');

    if (statusEl) {
      statusEl.innerHTML = `
        <span class="badge badge-pk">⚡ ${result.execution_ms.toFixed(2)} ms</span>
        <span class="badge badge-fk">${result.is_select ? `${result.row_count} rows` : `${result.rows_affected} affected`}</span>
        <span style="font-size:0.8rem;color:var(--text-muted);">${escapeHTML(result.message)}</span>
      `;
    }

    if (exportBar) {
      exportBar.style.display = result.is_select && result.row_count > 0 ? 'flex' : 'none';
    }

    if (!result.is_select) {
      thead.innerHTML = '';
      tbody.innerHTML = `<tr><td style="text-align:center;padding:2rem;color:var(--success);">✅ ${escapeHTML(result.message)}</td></tr>`;
      return;
    }

    // Render Headers
    thead.innerHTML = '';
    const trHead = document.createElement('tr');
    (result.columns || []).forEach(col => {
      const th = document.createElement('th');
      th.textContent = col;
      trHead.appendChild(th);
    });
    thead.appendChild(trHead);

    // Render Rows
    tbody.innerHTML = '';
    if (!result.rows || result.rows.length === 0) {
      tbody.innerHTML = `<tr><td colspan="${result.columns.length || 1}" style="text-align:center;padding:2rem;color:var(--text-dim);">No rows returned</td></tr>`;
      return;
    }

    result.rows.forEach(row => {
      const tr = document.createElement('tr');
      result.columns.forEach(col => {
        const td = document.createElement('td');
        const val = row[col];
        if (val === null || val === undefined) {
          td.innerHTML = '<span style="color:var(--text-dim);font-style:italic;">NULL</span>';
        } else {
          td.textContent = String(val);
        }
        tr.appendChild(td);
      });
      tbody.appendChild(tr);
    });
  },

  renderQueryError(errorMsg) {
    const statusEl = document.getElementById('qb-results-status');
    const thead = document.getElementById('qb-results-thead');
    const tbody = document.getElementById('qb-results-tbody');
    const exportBar = document.getElementById('qb-export-bar');

    if (exportBar) exportBar.style.display = 'none';
    if (statusEl) {
      statusEl.innerHTML = `<span class="badge" style="background-color:var(--danger);color:white;">ERROR</span>`;
    }

    thead.innerHTML = '';
    tbody.innerHTML = `
      <tr>
        <td style="padding:2rem;color:var(--danger);background-color:rgba(239, 68, 68, 0.05);font-family:var(--font-mono);font-size:0.85rem;">
          ❌ ${escapeHTML(errorMsg)}
        </td>
      </tr>
    `;
  },

  exportResults(format) {
    if (!this.lastResults || !this.lastResults.rows || this.lastResults.rows.length === 0) {
      App.showToast('No results to export', 'error');
      return;
    }

    let mimeType = 'text/plain';
    let filename = 'query_results';
    let dataStr = '';

    if (format === 'csv') {
      mimeType = 'text/csv';
      filename += '.csv';
      const cols = this.lastResults.columns;
      const headers = cols.map(c => `"${c.replace(/"/g, '""')}"`).join(',');
      const rowsCSV = this.lastResults.rows.map(row => {
        return cols.map(c => {
          const val = row[c];
          if (val === null || val === undefined) return '';
          return `"${String(val).replace(/"/g, '""')}"`;
        }).join(',');
      });
      dataStr = [headers, ...rowsCSV].join('\n');
    } else {
      mimeType = 'application/json';
      filename += '.json';
      dataStr = JSON.stringify(this.lastResults.rows, null, 2);
    }

    const blob = new Blob([dataStr], { type: mimeType });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  }
};
