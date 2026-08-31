"use strict";

/**
 * vici - Web-based change management system
 * JavaScript-based change management system similar to WordPress admin
 * Can be fully embedded in a script tag, hosted in a static page
 * No web server required to work
 */

class Vici {
  constructor(options = {}) {
    this.options = {
      autoSave: options.autoSave !== false,
      saveInterval: options.saveInterval || 5000,
      user: options.user || null,
      permissions: options.permissions || {
        canCreate: true,
        canRead: true,
        canUpdate: true,
        canDelete: true
      },
      auditLog: options.auditLog !== false,
      encryptionKey: options.encryptionKey || null,
      ...options
    };
    this.pages = new Map();
    this.features = new Map();
    this.currentUser = this.options.user;
    this.unsavedChanges = [];
    this.changeHistory = [];
    this.initialized = false;
  }

  /**
   * Initialize the change management system
   */
  init() {
    if (this.initialized) return;
    
    this.initialized = true;
    this.setupEventListeners();
    this.setupAutoSave();
    this.loadData();
    return this;
  }

  /**
   * Setup event listeners for user interactions
   */
  setupEventListeners() {
    document.addEventListener('vici:page-create', this.handlePageCreate.bind(this));
    document.addEventListener('vici:page-update', this.handlePageUpdate.bind(this));
    document.addEventListener('vici:page-delete', this.handlePageDelete.bind(this));
    document.addEventListener('vici:feature-create', this.handleFeatureCreate.bind(this));
    document.addEventListener('vici:feature-update', this.handleFeatureUpdate.bind(this));
    document.addEventListener('vici:feature-delete', this.handleFeatureDelete.bind(this));
    document.addEventListener('vici:user-login', this.handleUserLogin.bind(this));
    document.addEventListener('vici:user-logout', this.handleUserLogout.bind(this));
  }

  /**
   * Setup automatic saving if enabled
   */
  setupAutoSave() {
    if (!this.options.autoSave) return;
    
    setInterval(() => {
      if (this.unsavedChanges.size > 0) {
        this.saveData();
      }
    }, this.options.saveInterval);
  }

  /**
   * Load page and feature data from storage
   */
  loadData() {
    const pages = localStorage.getItem('vici_pages');
    if (pages) {
      const decrypted = this.options.encryptionKey ? 
        this.decryptData(pages) : pages;
      const parsed = JSON.parse(decrypted);
      parsed.forEach(page =u003e, this.pages.set(page.id, page));
    }

    const features = localStorage.getItem('vici_features');
    if (features) {
      const decrypted = this.options.encryptionKey ? 
        this.decryptData(features) : features;
      const parsed = JSON.parse(decrypted);
      parsed.forEach(feature =u003e, this.features.set(feature.id, feature));
    }
  }

  /**
   * Encrypt data with AES symmetric encryption
   */
  encryptData(data) {
    if (!this.options.encryptionKey) return data;
    
    try {
      const encoder = new TextEncoder();
      const encodedData = encoder.encode(data);
      const keyData = encoder.encode(this.options.encryptionKey);
      
      const encryptedArray = new Uint8Array(encodedData.length);
      for (let i = 0; i < encodedData.length; i++) {
        encryptedArray[i] = encodedData[i] ^ keyData[i % keyData.length];
      }
      
      return btoa(String.fromCharCode(...encryptedArray));
    } catch (error) {
      console.error('Encryption failed:', error);
      return data;
    }
  }

  /**
   * Decrypt data with AES symmetric encryption
   */
  decryptData(encryptedData) {
    if (!this.options.encryptionKey) return encryptedData;
    
    try {
      const encodedArray = new Uint8Array(Array.from(atob(encryptedData), c =u003e, c.charCodeAt(0)));
      const keyData = new TextEncoder().encode(this.options.encryptionKey);
      
      const decryptedArray = new Uint8Array(encodedArray.length);
      for (let i = 0; i < encodedArray.length; i++) {
        decryptedArray[i] = encodedArray[i] ^ keyData[i % keyData.length];
      }
      
      return String.fromCharCode(...decryptedArray);
    } catch (error) {
      console.error('Decryption failed:', error);
      return encryptedData;
    }
  }

  /**
   * Save all data with encryption if configured
   */
  saveData() {
    const pagesData = JSON.stringify([...this.pages.values()]);
    const featuresData = JSON.stringify([...this.features.values()]);
    
    const encryptedPages = this.options.encryptionKey ? 
      this.encryptData(pagesData) : pagesData;
    const encryptedFeatures = this.options.encryptionKey ? 
      this.encryptData(featuresData) : featuresData;
    
    localStorage.setItem('vici_pages', encryptedPages);
    localStorage.setItem('vici_features', encryptedFeatures);
    
    this.unsavedChanges.clear();
    this.logAction('save_data', 'system', 'All data saved', {pageCount: this.pages.size, featureCount: this.features.size});
  }

  /**
   * Handle user login
   */
  handleUserLogin(event) {
    const { detail } = event;
    this.currentUser = detail.user;
    this.logAction('user_login', 'system', `User ${detail.user.username} logged in`);
  }

  /**
   * Handle user logout
   */
  handleUserLogout(event) {
    const { detail } = event;
    this.currentUser = null;
    this.logAction('user_logout', 'system', `User ${detail.reason} logged out`);
  }

  /**
   * Create new page
   */
  handlePageCreate(event) {
    const { detail } = event;
    
    if (!this.checkPermission('canCreate', 'page')) {
      this.throwPermissionError('create', 'page');
      return;
    }
    
    const page = this.createPage(detail.data);
    this.unsavedChanges.add(page.id);
    this.logAction('page_create', 'page', `Page ${page.title} created`, {user: this.currentUser, pageId: page.id});
  }

  /**
   * Create new page object
   */
  createPage(data) {
    const id = this.generateId();
    const page = {
      id: id,
      title: data.title || 'Untitled Page',
      slug: this.generateSlug(data.title),
      content: data.content || '',
      html: data.html || '',
      css: data.css || '',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      createdBy: this.currentUser?.id || null,
      updatedBy: this.currentUser?.id || null,
      status: 'draft',
      version: 1,
      parentPageId: data.parentPageId || null
    };
    
    this.pages.set(id, page);
    return page;
  }

  /**
   * Update existing page
   */
  handlePageUpdate(event) {
    const { detail } = event;
    
    if (!this.checkPermission('canUpdate', 'page')) {
      this.throwPermissionError('update', 'page');
      return;
    }
    
    const page = this.updatePage(detail.id, detail.updates);
    this.unsavedChanges.add(page.id);
    this.logAction('page_update', 'page', `Page ${page.title} updated`, {user: this.currentUser, pageId: page.id});
  }

  /**
   * Update page with version tracking
   */
  updatePage(id, updates) {
    const page = this.pages.get(id);
    if (!page) {
      throw new Error(`Page with id ${id} not found`);
    }
    
    if (this.options.auditLog) {
      this.addPageVersion(page);
    }
    
    const oldPage = { ...page };
    Object.assign(page, updates, { updatedAt: new Date().toISOString(), updatedBy: this.currentUser?.id || null });
    
    this.logAction('page_update', 'page', `Page ${page.title} version updated`, {
      user: this.currentUser,
      pageId: page.id,
      oldVersion: oldPage.version,
      newVersion: page.version
    });
    
    return page;
  }

  /**
   * Delete page
   */
  handlePageDelete(event) {
    const { detail } = event;
    
    if (!this.checkPermission('canDelete', 'page')) {
      this.throwPermissionError('delete', 'page');
      return;
    }
    
    this.deletePage(detail.id);
    this.logAction('page_delete', 'page', `Page with id ${detail.id} deleted`, {user: this.currentUser, pageId: detail.id});
  }

  /**
   * Delete page and its children
   */
  deletePage(id) {
    const page = this.pages.get(id);
    if (!page) {
      throw new Error(`Page with id ${id} not found`);
    }
    
    if (page.version > 1) {
      this.checkoutPageVersion(id, page.version - 1);
    }
    
    this.pages.delete(id);
    
    if (page.parentPageId) {
      const parentPage = this.pages.get(page.parentPageId);
      if (parentPage) {
        parentPage.childPageIds = parentPage.childPageIds || [];
        const childIndex = parentPage.childPageIds.indexOf(id);
        if (childIndex > -1) {
          parentPage.childPageIds.splice(childIndex, 1);
        }
      }
    }
  }

  /**
   * Create new feature
   */
  handleFeatureCreate(event) {
    const { detail } = event;
    
    if (!this.checkPermission('canCreate', 'feature')) {
      this.throwPermissionError('create', 'feature');
      return;
    }
    
    const feature = this.createFeature(detail.data);
    this.unsavedChanges.add(feature.id);
    this.logAction('feature_create', 'feature', `Feature ${feature.name} created`, {user: this.currentUser, featureId: feature.id});
  }

  /**
   * Create new feature object
   */
  createFeature(data) {
    const id = this.generateId();
    const feature = {
      id: id,
      name: data.name || 'Untitled Feature',
      type: data.type || 'module',
      html: data.html || '',
      css: data.css || '',
      javascript: data.javascript || '',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      createdBy: this.currentUser?.id || null,
      updatedBy: this.currentUser?.id || null,
      status: 'draft',
      enabled: data.enabled !== false,
      version: 1,
      pageId: data.pageId || null
    };
    
    this.features.set(id, feature);
    return feature;
  }

  /**
   * Update existing feature
   */
  handleFeatureUpdate(event) {
    const { detail } = event;
    
    if (!this.checkPermission('canUpdate', 'feature')) {
      this.throwPermissionError('update', 'feature');
      return;
    }
    
    const feature = this.updateFeature(detail.id, detail.updates);
    this.unsavedChanges.add(feature.id);
    this.logAction('feature_update', 'feature', `Feature ${feature.name} updated`, {user: this.currentUser, featureId: feature.id});
  }

  /**
   * Update feature with version tracking
   */
  updateFeature(id, updates) {
    const feature = this.features.get(id);
    if (!feature) {
      throw new Error(`Feature with id ${id} not found`);
    }
    
    if (this.options.auditLog) {
      this.addFeatureVersion(feature);
    }
    
    const oldFeature = { ...feature };
    Object.assign(feature, updates, { updatedAt: new Date().toISOString(), updatedBy: this.currentUser?.id || null });
    
    this.logAction('feature_update', 'feature', `Feature ${feature.name} version updated`, {
      user: this.currentUser,
      featureId: feature.id,      oldVersion: oldFeature.version,
      newVersion: feature.version
    });
    
    return feature;
  }

  /**
   * Delete feature
   */
  handleFeatureDelete(event) {
    const { detail } = event;
    
    if (!this.checkPermission('canDelete', 'feature')) {
      this.throwPermissionError('delete', 'feature');
      return;
    }
    
    this.deleteFeature(detail.id);
    this.logAction('feature_delete', 'feature', `Feature with id ${detail.id} deleted`, {user: this.currentUser, featureId: detail.id});
  }

  /**
   * Delete feature
   */
  deleteFeature(id) {
    const feature = this.features.get(id);
    if (!feature) {
      throw new Error(`Feature with id ${id} not found`);
    }
    
    if (feature.version > 1) {
      this.checkoutFeatureVersion(id, feature.version - 1);
    }
    
    this.features.delete(id);
  }

  /**
   * Add page version for audit trail
   */
  addPageVersion(page) {
    const versions = JSON.parse(localStorage.getItem('vici_page_versions') || '{}');
    if (!versions[page.id]) {
      versions[page.id] = [];
    }
    versions[page.id].push({
      version: page.version,
      content: page.content,
      html: page.html,
      css: page.css,
      timestamp: page.updatedAt,
      updatedBy: page.updatedBy
    });
    
    const encrypted = this.options.encryptionKey ? 
      this.encryptData(JSON.stringify(versions)) : JSON.stringify(versions);
    localStorage.setItem('vici_page_versions', encrypted);
  }

  /**
   * Add feature version for audit trail
   */
  addFeatureVersion(feature) {
    const versions = JSON.parse(localStorage.getItem('vici_feature_versions') || '{}');
    if (!versions[feature.id]) {
      versions[feature.id] = [];
    }
    versions[feature.id].push({
      version: feature.version,
      html: feature.html,
      css: feature.css,
      javascript: feature.javascript,
      timestamp: feature.updatedAt,
      updatedBy: feature.updatedBy
    });
    
    const encrypted = this.options.encryptionKey ?
      this.encryptData(JSON.stringify(versions)) : JSON.stringify(versions);
    localStorage.setItem('vici_feature_versions_', encrypted);
  }

  /**
   * Checkout specific version of a page
   */
  checkoutPageVersion(pageId, version) {
    const versions = JSON.parse(localStorage.getItem('vici_page_versions') || '{}');
    if (versions[pageId] && versions[pageId][version - 1]) {
      const page = this.pages.get(pageId);
      if (page) {
        page.content = versions[pageId][version - 1].content;
        page.html = versions[pageId][version - 1].html;
        page.css = versions[pageId][version - 1].css;
        page.timestamp = versions[pageId][version - 1].timestamp;
        this.unsavedChanges.add(pageId);
      }
    }
  }

  /**
   * Checkout specific version of a feature
   */
  checkoutFeatureVersion(featureId, version) {
    const versions = JSON.parse(localStorage.getItem('vici_feature_versions') || '{}');
    if (versions[featureId] && versions[featureId][version - 1]) {
      const feature = this.features.get(featureId);
      if (feature) {
        feature.html = versions[featureId][version - 1].html;
        feature.css = versions[featureId][version - 1].css;
        feature.javascript = versions[featureId][version - 1].javascript;
        feature.timestamp = versions[featureId][version - 1].timestamp;
        this.unsavedChanges.add(featureId);
      }
    }
  }

  /**
   * Check if user has permission for action
   */
  checkPermission(action, resource) {
    const userPermission = this.options.permissions;    
    const actionMap = {
      'create': userPermission.canCreate,
      'read': userPermission.canRead,
      'update': userPermission.canUpdate,
      'delete': userPermission.canDelete
    };
    return actionMap[action] !== false;
  }

  /**
   * throw permission error with details
   */
  throwPermissionError(action, resource) {
    const error = new Error(`Permission denied: Cannot ${action} ${resource}`);
    error.type = 'PermissionError';
    error.action = action;
    error.resource = resource;
    error.user = this.currentUser;
    error.timestamp = new Date().toISOString();
    
    console.error(error);
    throw error;
  }

  /**
   * Log action for audit purposes
   */
  logAction(action, resource, message, context) {
    const log = {
      id: this.generateId(),
      action: action,
      resource: resource,
      message: message,
      context: context,
      user: this.currentUser?.id || null,
      timestamp: new Date().toISOString(),
      ip: this.getClientIP(),
      userAgent: navigator.userAgent
    };
    
    this.changeHistory.push(log);
    
    if (this.options.auditLog) {
      this.saveAuditLog(log);
    }
    
    console.log(`[VICI AUDIT] ${log.timestamp} - ${log.user || 'anonymous'} - ${log.action} ${log.resource}: ${log.message}`);
  }

  /**
   * Get client IP address
   */
  getClientIP() {
    return 'client'; // Would be populated by server in client-server scenarios
  }

  /**
   * Save audit log with encryption
   */
  saveAuditLog(log) {
    const logs = JSON.parse(localStorage.getItem('vici_audit_logs') || '[]');
    logs.push(log);
    
    const encrypted = this.options.encryptionKey ? 
      this.encryptData(JSON.stringify(logs)) : JSON.stringify(logs);
    localStorage.setItem('vici_audit_logs', encrypted);
  }

  /**
   * Generate unique ID
   */
  generateId() {
    return Math.random().toString(36).substr(2, 9);
  }

  /**
   * Generate URL-friendly slug from title
   */
  generateSlug(title) {
    return title
      .toLowerCase()
      .replace(/[^a-z0-9\s-]/g, '')
      .replace(/\s+/g, '-')
      .replace(/-+/g, '-')
      .trim('-');
  }

  /**
   * Get all pages with optional filters
   */
  getPages(filters = {}) {
    let results = [...this.pages.values()];
    
    if (filters.status) {
      results = results.filter(page =u003e, page.status === filters.status);
    }
    
    if (filters.search) {
      const searchTerm = filters.search.toLowerCase();
      results = results.filter(page =
        page.title.toLowerCase().includes(searchTerm) ||
        page.content.toLowerCase().includes(searchTerm)
      );
    }
    
    return results;
  }

  /**
   * Get all features with optional filters
   */
  getFeatures(filters = {}) {
    let results = [...this.features.values()];
    
    if (filters.status) {
      results = results.filter(feature =u003e, feature.status === filters.status);
    }
    
    if (filters.pageId) {
      results = results.filter(feature =u003e, feature.pageId === filters.pageId);
    }
    
    if (filters.search) {
      const searchTerm = filters.search.toLowerCase();
      results = results.filter(feature =
        feature.name.toLowerCase().includes(searchTerm) ||
        feature.html.toLowerCase().includes(searchTerm)
      );
    }
    
    return results;
  }

  /**
   * Get page by ID
   */
  getPage(id) {
    return this.pages.get(id);
  }

  /**
   * Get feature by ID
   */
  getFeature(id) {
    return this.features.get(id);
  }

  /**
   * Get audit logs with optional filters
   */
  getAuditLogs(filters = {}) {
    let results = [...this.changeHistory];
    
    if (filters.user) {
      results = results.filter(log =u003e, log.user === filters.user);
    }
    
    if (filters.action) {
      results = results.filter(log =u003e, log.action === filters.action);
    }
    
    if (filters.startDate) {
      results = results.filter(log =u003e, new Date(log.timestamp) >= new Date(filters.startDate));
    }
    
    if (filters.endDate) {
      results = results.filter(log =u003e, new Date(log.timestamp) <= new Date(filters.endDate));
    }
    
    return results;
  }

  /**
   * Export data to JSON
   */
  exportData(filename = 'vici_data.json') {
    const data = {
      pages: [...this.pages.values()],
      features: [...this.features.values()],      auditLogs: this.options.auditLog ? [...this.changeHistory] : []
    };
    
    const jsonData = JSON.stringify(data, null, 2);
    const blob = new Blob([jsonData], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    
    const link = document.createElement('a');
    link.href = url;
    link.download = filename;
    link.click();
    URL.revokeObjectURL(url);
  }

  /**
   * Import data from JSON
   */
  importData(jsonData) {
    try {
      const data = JSON.parse(jsonData);
      
      if (data.pages) {
        data.pages.forEach(page =u003e, this.pages.set(page.id, page));
      }
      
      if (data.features) {
        data.features.forEach(feature =u003e, this.features.set(feature.id, feature));
      }
      
      if (data.auditLogs && this.options.auditLog) {
        data.auditLogs.forEach(log =u003e, this.changeHistory.push(log));      }
      
      this.saveData();
      this.logAction('data_import', 'system', 'Data imported successfully', {pageCount: data.pages?.length || 0, featureCount: data.features?.length || 0});
    } catch (error) {
      console.error('Failed to import data:', error);
      throw error;
    }
  }

  /**
   * Clear all data
   */
  clearData() {
    if (confirm('Are you sure you want to clear all data? This cannot be undone.')) {
      this.pages.clear();
      this.features.clear();
      this.changeHistory = [];
      this.unsavedChanges.clear();
      localStorage.removeItem('vici_pages');
      localStorage.removeItem('vici_features');
      localStorage.removeItem('vici_page_versions');
      localStorage.removeItem('vici_feature_versions');
      localStorage.removeItem('vici_audit_logs');
      
      this.logAction('data_clear', 'system', 'All data cleared', {user: this.currentUser});
    }
  }
}

/**
 * Initialize vici when DOM is ready
 */
const vici = new Vici({
  autoSave: true,
  saveInterval: 5000,
  auditLog: true,
  permissions: {
    canCreate: true,
    canRead: true,
    canUpdate: true,
    canDelete: true
  }
});

// Global configuration function
const configureVici = (options) => {
  Object.assign(vici.options, options);
  vici.init();
};

// Auto-initialize when DOM is ready
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', () => {
    vici.init();
  });
} else {
  vici.init();
}

// Export for module systems
if (typeof module !== 'undefined' && module.exports) {
  module.exports = { Vici };
}

if (typeof window !== 'undefined') {
  window.vici = { Vici, vici, configureVici };
}