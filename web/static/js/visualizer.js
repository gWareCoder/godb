/**
 * Visualizer Module: Interactive ER Diagram, Draggable Tables, SVG Connectors, and Visual Schema Creation
 */
const Visualizer = {
  canvas: null,
  svgLayer: null,
  schema: null,
  tablePositions: {}, // { [tableName]: { x, y } }
  zoom: 1.0,
  panX: 0,
  panY: 0,
  isPanning: false,
  startPan: { x: 0, y: 0 },
  draggingCard: null,
  dragOffset: { x: 0, y: 0 },

  init() {
    this.canvas = document.getElementById('visualizer-canvas');
    this.svgLayer = document.getElementById('visualizer-svg');
    this.bindEvents();
  },

  bindEvents() {
    const wrapper = document.getElementById('visualizer-wrapper');
    if (!wrapper) return;

    // Pan canvas with middle click or space+drag
    wrapper.addEventListener('mousedown', (e) => {
      if (e.target === wrapper || e.target === this.canvas || e.target === this.svgLayer) {
        this.isPanning = true;
        this.startPan = { x: e.clientX - this.panX, y: e.clientY - this.panY };
        wrapper.style.cursor = 'grabbing';
      }
    });

    window.addEventListener('mousemove', (e) => {
      if (this.isPanning) {
        this.panX = e.clientX - this.startPan.x;
        this.panY = e.clientY - this.startPan.y;
        this.applyTransform();
      } else if (this.draggingCard) {
        const x = (e.clientX - this.dragOffset.x - this.panX) / this.zoom;
        const y = (e.clientY - this.dragOffset.y - this.panY) / this.zoom;
        const tableName = this.draggingCard.dataset.table;
        this.tablePositions[tableName] = { x: Math.max(20, x), y: Math.max(20, y) };
        this.draggingCard.style.left = `${this.tablePositions[tableName].x}px`;
        this.draggingCard.style.top = `${this.tablePositions[tableName].y}px`;
        this.drawConnections();
      }
    });

    window.addEventListener('mouseup', () => {
      if (this.isPanning) {
        this.isPanning = false;
        wrapper.style.cursor = 'default';
      }
      if (this.draggingCard) {
        this.draggingCard.classList.remove('dragging');
        this.draggingCard = null;
      }
    });

    // Zoom Buttons
    document.getElementById('btn-zoom-in')?.addEventListener('click', () => this.setZoom(this.zoom + 0.15));
    document.getElementById('btn-zoom-out')?.addEventListener('click', () => this.setZoom(this.zoom - 0.15));
    document.getElementById('btn-zoom-reset')?.addEventListener('click', () => {
      this.zoom = 1.0;
      this.panX = 40;
      this.panY = 40;
      this.applyTransform();
    });
    document.getElementById('btn-auto-layout')?.addEventListener('click', () => this.autoLayout());

    // Create Table Modal Trigger
    document.getElementById('btn-modal-create-table')?.addEventListener('click', () => {
      this.openCreateTableModal();
    });

    // Link Tables Modal Trigger
    document.getElementById('btn-modal-link-tables')?.addEventListener('click', () => {
      this.openLinkTablesModal();
    });

    // Create Table Form
    this.bindCreateTableForm();
    this.bindLinkTablesForm();
  },

  setZoom(val) {
    this.zoom = Math.min(Math.max(val, 0.4), 2.0);
    const indicator = document.getElementById('zoom-val');
    if (indicator) indicator.textContent = `${Math.round(this.zoom * 100)}%`;
    this.applyTransform();
  },

  applyTransform() {
    if (!this.canvas) return;
    this.canvas.style.transform = `translate(${this.panX}px, ${this.panY}px) scale(${this.zoom})`;
  },

  onTabActivate() {
    this.drawConnections();
  },

  render(schema) {
    this.schema = schema;
    if (!this.canvas) this.init();
    if (!this.canvas) return;

    // Clear existing table cards
    const existingCards = this.canvas.querySelectorAll('.table-card');
    existingCards.forEach(c => c.remove());

    if (!schema.tables || schema.tables.length === 0) {
      this.drawConnections();
      return;
    }

    // Auto layout if positions not set
    let cols = Math.ceil(Math.sqrt(schema.tables.length));
    if (cols < 2) cols = 2;
    const cardWidth = 280;
    const cardGapX = 140;
    const cardGapY = 60;

    schema.tables.forEach((tbl, idx) => {
      if (!this.tablePositions[tbl.name]) {
        const colIdx = idx % cols;
        const rowIdx = Math.floor(idx / cols);
        this.tablePositions[tbl.name] = {
          x: 40 + colIdx * (cardWidth + cardGapX),
          y: 40 + rowIdx * 340,
        };
      }
      this.renderTableCard(tbl);
    });

    // Draw lines after cards are rendered
    setTimeout(() => this.drawConnections(), 50);
  },

  autoLayout() {
    if (!this.schema || !this.schema.tables) return;
    const cols = Math.max(2, Math.ceil(Math.sqrt(this.schema.tables.length)));
    this.schema.tables.forEach((tbl, idx) => {
      const colIdx = idx % cols;
      const rowIdx = Math.floor(idx / cols);
      this.tablePositions[tbl.name] = {
        x: 60 + colIdx * 420,
        y: 60 + rowIdx * 340,
      };
      const card = this.canvas.querySelector(`.table-card[data-table="${tbl.name}"]`);
      if (card) {
        card.style.left = `${this.tablePositions[tbl.name].x}px`;
        card.style.top = `${this.tablePositions[tbl.name].y}px`;
      }
    });
    this.panX = 40;
    this.panY = 40;
    this.zoom = 1.0;
    this.applyTransform();
    this.drawConnections();
  },

  renderTableCard(tbl) {
    const card = document.createElement('div');
    card.className = 'table-card';
    card.dataset.table = tbl.name;
    const pos = this.tablePositions[tbl.name] || { x: 50, y: 50 };
    card.style.left = `${pos.x}px`;
    card.style.top = `${pos.y}px`;

    // Make Card Draggable
    card.addEventListener('mousedown', (e) => {
      // Don't drag if clicking buttons or dropdowns
      if (e.target.closest('button') || e.target.closest('select') || e.target.closest('.dropdown')) {
        return;
      }
      this.draggingCard = card;
      card.classList.add('dragging');
      this.dragOffset = {
        x: e.clientX - pos.x * this.zoom - this.panX,
        y: e.clientY - pos.y * this.zoom - this.panY,
      };
      e.stopPropagation();
    });

    // Header
    const header = document.createElement('div');
    header.className = 'table-card-header';
    header.innerHTML = `
      <div class="table-card-title">
        <span>📋</span>
        <span>${escapeHTML(tbl.name)}</span>
        <span class="table-row-count">${tbl.row_count} rows</span>
      </div>
      <div class="table-card-actions">
        <button class="btn btn-secondary btn-sm btn-icon" title="View & Edit Data" data-action="data">📝</button>
        <button class="btn btn-danger btn-sm btn-icon" title="Drop Table" data-action="drop">🗑️</button>
      </div>
    `;

    header.querySelector('[data-action="data"]').addEventListener('click', () => {
      App.switchTab('data');
      if (window.DataEntry) DataEntry.selectTable(tbl.name);
    });

    header.querySelector('[data-action="drop"]').addEventListener('click', async () => {
      if (confirm(`Are you sure you want to drop table '${tbl.name}' and all its data?`)) {
        try {
          await API.dropTable(tbl.name);
          delete this.tablePositions[tbl.name];
          App.showToast(`Table '${tbl.name}' dropped`, 'success');
          await App.refreshSchema();
        } catch (err) {
          App.showToast(err.message, 'error');
        }
      }
    });

    card.appendChild(header);

    // Columns list
    const colList = document.createElement('ul');
    colList.className = 'table-card-columns';

    tbl.columns.forEach(col => {
      const row = document.createElement('li');
      row.className = 'table-col-row';
      row.dataset.col = col.name;

      // Check if FK
      const fk = (tbl.foreign_keys || []).find(f => f.column === col.name);

      let icon = '🔹';
      if (col.primary_key) icon = '🔑';
      else if (fk) icon = '🔗';

      row.innerHTML = `
        <div class="col-info">
          <span class="col-icon" title="${col.primary_key ? 'Primary Key' : (fk ? `References ${fk.ref_table}.${fk.ref_column}` : '')}">${icon}</span>
          <span class="col-name" style="${col.primary_key ? 'font-weight:700;color:#fbbf24' : ''}">${escapeHTML(col.name)}</span>
        </div>
        <div class="col-meta">
          ${fk ? `<span class="badge badge-fk" title="FK to ${fk.ref_table}.${fk.ref_column}">FK</span>` : ''}
          <span class="badge badge-type">${escapeHTML(col.type)}</span>
        </div>
      `;
      colList.appendChild(row);
    });

    card.appendChild(colList);
    this.canvas.appendChild(card);
  },

  drawConnections() {
    if (!this.svgLayer || !this.schema) return;
    this.svgLayer.innerHTML = `
      <defs>
        <marker id="arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
          <path d="M 0 1 L 10 5 L 0 9 z" fill="#6366f1" />
        </marker>
        <marker id="arrow-hover" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
          <path d="M 0 1 L 10 5 L 0 9 z" fill="#38bdf8" />
        </marker>
      </defs>
    `;

    const relations = this.schema.relations || [];

    relations.forEach(rel => {
      const fromCard = this.canvas.querySelector(`.table-card[data-table="${rel.from_table}"]`);
      const toCard = this.canvas.querySelector(`.table-card[data-table="${rel.to_table}"]`);
      if (!fromCard || !toCard) return;

      const fromRow = fromCard.querySelector(`.table-col-row[data-col="${rel.from_column}"]`);
      const toRow = toCard.querySelector(`.table-col-row[data-col="${rel.to_column}"]`);

      const fromCardPos = this.tablePositions[rel.from_table];
      const toCardPos = this.tablePositions[rel.to_table];
      if (!fromCardPos || !toCardPos) return;

      const fromCardWidth = fromCard.offsetWidth || 280;
      const toCardWidth = toCard.offsetWidth || 280;

      // Vertical offset within card
      let fromYOffset = 60;
      if (fromRow) fromYOffset = fromRow.offsetTop + fromRow.offsetHeight / 2;
      let toYOffset = 60;
      if (toRow) toYOffset = toRow.offsetTop + toRow.offsetHeight / 2;

      // Determine left/right connector anchor based on relative positions
      const isFromOnLeft = fromCardPos.x + fromCardWidth / 2 < toCardPos.x + toCardWidth / 2;

      let x1, y1, x2, y2, cx1, cy1, cx2, cy2;

      if (isFromOnLeft) {
        x1 = fromCardPos.x + fromCardWidth;
        y1 = fromCardPos.y + fromYOffset;
        x2 = toCardPos.x;
        y2 = toCardPos.y + toYOffset;
        const dx = Math.max(60, Math.abs(x2 - x1) * 0.5);
        cx1 = x1 + dx;
        cy1 = y1;
        cx2 = x2 - dx;
        cy2 = y2;
      } else {
        x1 = fromCardPos.x;
        y1 = fromCardPos.y + fromYOffset;
        x2 = toCardPos.x + toCardWidth;
        y2 = toCardPos.y + toYOffset;
        const dx = Math.max(60, Math.abs(x2 - x1) * 0.5);
        cx1 = x1 - dx;
        cy1 = y1;
        cx2 = x2 + dx;
        cy2 = y2;
      }

      const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
      path.setAttribute('d', `M ${x1} ${y1} C ${cx1} ${cy1}, ${cx2} ${cy2}, ${x2} ${y2}`);
      path.setAttribute('class', 'relation-path');
      path.setAttribute('marker-end', 'url(#arrow)');

      // Tooltip title
      const title = document.createElementNS('http://www.w3.org/2000/svg', 'title');
      title.textContent = `${rel.from_table}.${rel.from_column} -> ${rel.to_table}.${rel.to_column} (ON DELETE ${rel.on_delete})`;
      path.appendChild(title);

      path.addEventListener('mouseenter', () => {
        path.setAttribute('marker-end', 'url(#arrow-hover)');
      });
      path.addEventListener('mouseleave', () => {
        path.setAttribute('marker-end', 'url(#arrow)');
      });

      this.svgLayer.appendChild(path);
    });
  },

  /* Table Creation Form & Modal */
  bindCreateTableForm() {
    const btnAddCol = document.getElementById('btn-add-table-column');
    const colsContainer = document.getElementById('table-columns-container');
    const btnAddFK = document.getElementById('btn-add-table-fk');
    const fksContainer = document.getElementById('table-fks-container');
    const form = document.getElementById('form-create-table');

    btnAddCol?.addEventListener('click', () => {
      this.appendColumnRow(colsContainer);
    });

    btnAddFK?.addEventListener('click', () => {
      this.appendFKRow(fksContainer);
    });

    form?.addEventListener('submit', async (e) => {
      e.preventDefault();
      const tableName = document.getElementById('create-table-name').value.trim();
      if (!tableName) {
        App.showToast('Table name is required', 'error');
        return;
      }

      // Collect Columns
      const colRows = colsContainer.querySelectorAll('.column-builder-row');
      const columns = [];
      colRows.forEach(row => {
        const name = row.querySelector('.col-name-input').value.trim();
        if (!name) return;
        const type = row.querySelector('.col-type-select').value;
        const isPK = row.querySelector('.col-pk-check').checked;
        const isAI = row.querySelector('.col-ai-check').checked;
        const isNN = row.querySelector('.col-nn-check').checked;
        const isUQ = row.querySelector('.col-uq-check').checked;
        const dflt = row.querySelector('.col-default-input').value.trim();

        columns.push({
          name,
          type,
          primary_key: isPK,
          auto_increment: isAI,
          not_null: isNN,
          unique: isUQ,
          default_value: dflt ? dflt : null,
        });
      });

      if (columns.length === 0) {
        App.showToast('Please add at least one column', 'error');
        return;
      }

      // Collect FKs
      const fkRows = fksContainer.querySelectorAll('.fk-builder-row');
      const foreignKeys = [];
      fkRows.forEach(row => {
        const col = row.querySelector('.fk-col-input').value.trim();
        const refTable = row.querySelector('.fk-ref-table-select').value;
        const refCol = row.querySelector('.fk-ref-col-input').value.trim();
        const onDel = row.querySelector('.fk-on-delete-select').value;
        if (col && refTable && refCol) {
          foreignKeys.push({
            column: col,
            ref_table: refTable,
            ref_column: refCol,
            on_delete: onDel,
          });
        }
      });

      try {
        await API.createTable({
          name: tableName,
          columns,
          foreign_keys: foreignKeys,
        });
        App.closeModal('modal-create-table');
        App.showToast(`Table '${tableName}' created successfully!`, 'success');
        // Auto position near center
        this.tablePositions[tableName] = {
          x: 100 + (Math.random() * 200),
          y: 100 + (Math.random() * 200),
        };
        await App.refreshSchema();
      } catch (err) {
        App.showToast(err.message, 'error');
      }
    });
  },

  openCreateTableModal() {
    const colsContainer = document.getElementById('table-columns-container');
    const fksContainer = document.getElementById('table-fks-container');
    const tableNameInput = document.getElementById('create-table-name');
    if (!colsContainer || !tableNameInput) return;

    tableNameInput.value = '';
    colsContainer.innerHTML = '';
    fksContainer.innerHTML = '';

    // Add default primary key column 'id'
    this.appendColumnRow(colsContainer, { name: 'id', type: 'INTEGER', pk: true, ai: true, nn: true });
    // Add default column 'name'
    this.appendColumnRow(colsContainer, { name: 'name', type: 'TEXT', nn: true });

    App.openModal('modal-create-table');
  },

  appendColumnRow(container, initial = {}) {
    const row = document.createElement('div');
    row.className = 'column-builder-row form-row';
    row.style.marginBottom = '0.75rem';
    row.style.alignItems = 'center';

    row.innerHTML = `
      <input type="text" class="form-control col-name-input" placeholder="col_name" value="${initial.name || ''}" style="flex: 2;" required>
      <select class="form-control col-type-select" style="flex: 1.5;">
        <option value="INTEGER" ${initial.type === 'INTEGER' ? 'selected' : ''}>INTEGER</option>
        <option value="TEXT" ${initial.type === 'TEXT' ? 'selected' : ''}>TEXT</option>
        <option value="REAL" ${initial.type === 'REAL' ? 'selected' : ''}>REAL</option>
        <option value="BOOLEAN" ${initial.type === 'BOOLEAN' ? 'selected' : ''}>BOOLEAN</option>
        <option value="DATETIME" ${initial.type === 'DATETIME' ? 'selected' : ''}>DATETIME</option>
        <option value="BLOB" ${initial.type === 'BLOB' ? 'selected' : ''}>BLOB</option>
      </select>
      <label class="form-check" title="Primary Key" style="font-size:0.75rem;">
        <input type="checkbox" class="col-pk-check" ${initial.pk ? 'checked' : ''}> PK
      </label>
      <label class="form-check" title="Auto Increment" style="font-size:0.75rem;">
        <input type="checkbox" class="col-ai-check" ${initial.ai ? 'checked' : ''}> AI
      </label>
      <label class="form-check" title="Not Null" style="font-size:0.75rem;">
        <input type="checkbox" class="col-nn-check" ${initial.nn ? 'checked' : ''}> NN
      </label>
      <label class="form-check" title="Unique" style="font-size:0.75rem;">
        <input type="checkbox" class="col-uq-check" ${initial.uq ? 'checked' : ''}> UQ
      </label>
      <input type="text" class="form-control col-default-input" placeholder="Default" style="flex: 1.5; font-size: 0.8rem;">
      <button type="button" class="btn btn-danger btn-sm btn-icon btn-remove-col" title="Remove Column">✕</button>
    `;

    row.querySelector('.btn-remove-col').addEventListener('click', () => row.remove());
    container.appendChild(row);
  },

  appendFKRow(container) {
    const row = document.createElement('div');
    row.className = 'fk-builder-row form-row';
    row.style.marginBottom = '0.75rem';
    row.style.alignItems = 'center';

    let tableOptions = '<option value="">Select target table...</option>';
    (this.schema?.tables || []).forEach(tbl => {
      tableOptions += `<option value="${escapeHTML(tbl.name)}">${escapeHTML(tbl.name)}</option>`;
    });

    row.innerHTML = `
      <input type="text" class="form-control fk-col-input" placeholder="Column in this table" style="flex: 2;" required>
      <span style="color:var(--text-dim);">➜</span>
      <select class="form-control fk-ref-table-select" style="flex: 2;">
        ${tableOptions}
      </select>
      <input type="text" class="form-control fk-ref-col-input" placeholder="Target Col (e.g. id)" value="id" style="flex: 1.5;" required>
      <select class="form-control fk-on-delete-select" style="flex: 1.5;">
        <option value="CASCADE">CASCADE</option>
        <option value="SET NULL">SET NULL</option>
        <option value="RESTRICT">RESTRICT</option>
        <option value="NO ACTION">NO ACTION</option>
      </select>
      <button type="button" class="btn btn-danger btn-sm btn-icon btn-remove-fk" title="Remove Link">✕</button>
    `;

    row.querySelector('.btn-remove-fk').addEventListener('click', () => row.remove());
    container.appendChild(row);
  },

  /* Link Tables Form & Modal */
  bindLinkTablesForm() {
    const form = document.getElementById('form-link-tables');
    const srcTableSelect = document.getElementById('link-src-table');
    const srcColSelect = document.getElementById('link-src-col');
    const tgtTableSelect = document.getElementById('link-tgt-table');
    const tgtColSelect = document.getElementById('link-tgt-col');

    srcTableSelect?.addEventListener('change', () => {
      const tbl = (this.schema?.tables || []).find(t => t.name === srcTableSelect.value);
      srcColSelect.innerHTML = '<option value="">Select column...</option>';
      if (tbl) {
        tbl.columns.forEach(c => {
          srcColSelect.innerHTML += `<option value="${escapeHTML(c.name)}">${escapeHTML(c.name)} (${c.type})</option>`;
        });
      }
    });

    tgtTableSelect?.addEventListener('change', () => {
      const tbl = (this.schema?.tables || []).find(t => t.name === tgtTableSelect.value);
      tgtColSelect.innerHTML = '<option value="">Select column...</option>';
      if (tbl) {
        tbl.columns.forEach(c => {
          tgtColSelect.innerHTML += `<option value="${escapeHTML(c.name)}" ${c.primary_key ? 'selected' : ''}>${escapeHTML(c.name)} (${c.type})${c.primary_key ? ' [PK]' : ''}</option>`;
        });
      }
    });

    form?.addEventListener('submit', async (e) => {
      e.preventDefault();
      const sourceTable = srcTableSelect.value;
      const sourceColumn = srcColSelect.value;
      const targetTable = tgtTableSelect.value;
      const targetColumn = tgtColSelect.value;
      const onDelete = document.getElementById('link-on-delete').value;

      if (!sourceTable || !sourceColumn || !targetTable || !targetColumn) {
        App.showToast('All source and target table/column selections are required', 'error');
        return;
      }

      try {
        await API.linkTables({
          source_table: sourceTable,
          source_column: sourceColumn,
          target_table: targetTable,
          target_column: targetColumn,
          on_delete: onDelete,
        });
        App.closeModal('modal-link-tables');
        App.showToast(`Linked ${sourceTable}.${sourceColumn} ➜ ${targetTable}.${targetColumn}!`, 'success');
        await App.refreshSchema();
      } catch (err) {
        App.showToast(err.message, 'error');
      }
    });
  },

  openLinkTablesModal() {
    const srcTableSelect = document.getElementById('link-src-table');
    const tgtTableSelect = document.getElementById('link-tgt-table');
    const srcColSelect = document.getElementById('link-src-col');
    const tgtColSelect = document.getElementById('link-tgt-col');
    if (!srcTableSelect || !tgtTableSelect) return;

    let options = '<option value="">Select table...</option>';
    (this.schema?.tables || []).forEach(tbl => {
      options += `<option value="${escapeHTML(tbl.name)}">${escapeHTML(tbl.name)}</option>`;
    });

    srcTableSelect.innerHTML = options;
    tgtTableSelect.innerHTML = options;
    srcColSelect.innerHTML = '<option value="">Select source table first</option>';
    tgtColSelect.innerHTML = '<option value="">Select target table first</option>';

    App.openModal('modal-link-tables');
  }
};
