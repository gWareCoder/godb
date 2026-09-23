/**
 * API client module for GoDB
 */
window.API = {
  async request(endpoint, options = {}) {
    const defaultHeaders = {
      'Content-Type': 'application/json',
    };

    options.headers = {
      ...defaultHeaders,
      ...options.headers,
    };

    try {
      const response = await fetch(endpoint, options);
      const data = await response.json();
      
      if (!response.ok || !data.success) {
        const errorMsg = data.error || data.message || `HTTP ${response.status}`;
        throw new Error(errorMsg);
      }
      return data;
    } catch (err) {
      console.error(`API Error on [${options.method || 'GET'}] ${endpoint}:`, err);
      throw err;
    }
  },

  // Databases
  async listDatabases() {
    return this.request('/api/databases');
  },
  async createDatabase(name) {
    return this.request('/api/databases', {
      method: 'POST',
      body: JSON.stringify({ name }),
    });
  },
  async switchDatabase(name) {
    return this.request('/api/databases/switch', {
      method: 'POST',
      body: JSON.stringify({ name }),
    });
  },
  async deleteDatabase(name) {
    return this.request(`/api/databases/${encodeURIComponent(name)}`, {
      method: 'DELETE',
    });
  },
  async loadSample() {
    return this.request('/api/databases/sample', {
      method: 'POST',
    });
  },

  // Schema & Tables
  async getSchema() {
    return this.request('/api/schema');
  },
  async createTable(tableDef) {
    return this.request('/api/tables', {
      method: 'POST',
      body: JSON.stringify(tableDef),
    });
  },
  async dropTable(tableName) {
    return this.request(`/api/tables/${encodeURIComponent(tableName)}`, {
      method: 'DELETE',
    });
  },
  async addColumn(tableName, colDef) {
    return this.request(`/api/tables/${encodeURIComponent(tableName)}/columns`, {
      method: 'POST',
      body: JSON.stringify(colDef),
    });
  },
  async linkTables(linkReq) {
    return this.request('/api/tables/link', {
      method: 'POST',
      body: JSON.stringify(linkReq),
    });
  },

  // Data & CRUD
  async getRows(table, { page = 1, pageSize = 25, search = '', sortBy = '', sortOrder = 'asc' } = {}) {
    const params = new URLSearchParams({
      page: page.toString(),
      page_size: pageSize.toString(),
      search,
      sort_by: sortBy,
      sort_order: sortOrder,
    });
    return this.request(`/api/data/${encodeURIComponent(table)}?${params.toString()}`);
  },
  async insertRow(table, rowData) {
    return this.request(`/api/data/${encodeURIComponent(table)}/row`, {
      method: 'POST',
      body: JSON.stringify({ data: rowData }),
    });
  },
  async updateRow(table, pk, rowData) {
    return this.request(`/api/data/${encodeURIComponent(table)}/row`, {
      method: 'PUT',
      body: JSON.stringify({ pk, data: rowData }),
    });
  },
  async deleteRow(table, pk) {
    return this.request(`/api/data/${encodeURIComponent(table)}/row`, {
      method: 'DELETE',
      body: JSON.stringify({ pk }),
    });
  },
  async getFKOptions(table, refTable, refColumn) {
    const params = new URLSearchParams({
      ref_table: refTable,
      ref_column: refColumn,
    });
    return this.request(`/api/data/${encodeURIComponent(table)}/fk-options?${params.toString()}`);
  },

  // Queries
  async previewQuery(queryDef) {
    return this.request('/api/query/preview', {
      method: 'POST',
      body: JSON.stringify(queryDef),
    });
  },
  async buildAndExecuteQuery(queryDef) {
    return this.request('/api/query/build', {
      method: 'POST',
      body: JSON.stringify(queryDef),
    });
  },
  async executeRawSQL(query) {
    return this.request('/api/query/execute', {
      method: 'POST',
      body: JSON.stringify({ query }),
    });
  },
};
