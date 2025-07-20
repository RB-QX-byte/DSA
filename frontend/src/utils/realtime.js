/**
 * Real-time utilities for handling SSE connections and real-time updates
 */

export class RealtimeConnection {
  constructor(options = {}) {
    this.eventSource = null;
    this.isConnected = false;
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = options.maxReconnectAttempts || 5;
    this.reconnectDelay = options.reconnectDelay || 1000;
    this.baseUrl = options.baseUrl || '/api/v1/realtime';
    this.contestId = options.contestId;
    this.authToken = options.authToken || this.getAuthToken();
    this.listeners = new Map();
    this.onConnectionChange = options.onConnectionChange;
    this.onError = options.onError;
  }

  getAuthToken() {
    return localStorage.getItem('token');
  }

  connect() {
    if (this.eventSource) {
      this.disconnect();
    }

    if (!this.authToken) {
      console.warn('No authentication token available');
      this.handleError('No authentication token');
      return;
    }

    let url = this.baseUrl + '/sse';
    if (this.contestId) {
      url = `${this.baseUrl}/contests/${this.contestId}/sse`;
    }

    // Add auth token as query parameter
    url += `?token=${encodeURIComponent(this.authToken)}`;

    this.eventSource = new EventSource(url);
    this.setupEventHandlers();
  }

  setupEventHandlers() {
    this.eventSource.onopen = () => {
      console.log('Real-time connection established');
      this.isConnected = true;
      this.reconnectAttempts = 0;
      this.notifyConnectionChange(true);
    };

    this.eventSource.onerror = () => {
      console.error('Real-time connection error');
      this.isConnected = false;
      this.notifyConnectionChange(false);
      this.handleReconnection();
    };

    this.eventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        this.handleMessage('message', data);
      } catch (error) {
        console.error('Error parsing real-time message:', error);
      }
    };

    // Register specific event listeners
    this.eventSource.addEventListener('connected', (event) => {
      console.log('SSE connection confirmed');
    });

    this.eventSource.addEventListener('submission_update', (event) => {
      try {
        const data = JSON.parse(event.data);
        this.handleMessage('submission_update', data);
      } catch (error) {
        console.error('Error parsing submission update:', error);
      }
    });

    this.eventSource.addEventListener('contest_submission_update', (event) => {
      try {
        const data = JSON.parse(event.data);
        this.handleMessage('contest_submission_update', data);
      } catch (error) {
        console.error('Error parsing contest submission update:', error);
      }
    });

    this.eventSource.addEventListener('leaderboard_update', (event) => {
      try {
        const data = JSON.parse(event.data);
        this.handleMessage('leaderboard_update', data);
      } catch (error) {
        console.error('Error parsing leaderboard update:', error);
      }
    });

    this.eventSource.addEventListener('contest_update', (event) => {
      try {
        const data = JSON.parse(event.data);
        this.handleMessage('contest_update', data);
      } catch (error) {
        console.error('Error parsing contest update:', error);
      }
    });

    this.eventSource.addEventListener('system_notification', (event) => {
      try {
        const data = JSON.parse(event.data);
        this.handleMessage('system_notification', data);
      } catch (error) {
        console.error('Error parsing system notification:', error);
      }
    });
  }

  handleMessage(type, data) {
    const listeners = this.listeners.get(type);
    if (listeners) {
      listeners.forEach(listener => {
        try {
          listener(data);
        } catch (error) {
          console.error(`Error in ${type} listener:`, error);
        }
      });
    }
  }

  handleReconnection() {
    if (this.reconnectAttempts < this.maxReconnectAttempts) {
      this.reconnectAttempts++;
      const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1);
      
      console.log(`Attempting reconnection ${this.reconnectAttempts}/${this.maxReconnectAttempts} in ${delay}ms`);
      
      setTimeout(() => {
        this.connect();
      }, delay);
    } else {
      console.error('Max reconnection attempts reached');
      this.handleError('Connection failed after maximum retry attempts');
    }
  }

  handleError(message) {
    if (this.onError) {
      this.onError(message);
    }
  }

  notifyConnectionChange(connected) {
    if (this.onConnectionChange) {
      this.onConnectionChange(connected);
    }
  }

  addEventListener(type, listener) {
    if (!this.listeners.has(type)) {
      this.listeners.set(type, new Set());
    }
    this.listeners.get(type).add(listener);
  }

  removeEventListener(type, listener) {
    const listeners = this.listeners.get(type);
    if (listeners) {
      listeners.delete(listener);
      if (listeners.size === 0) {
        this.listeners.delete(type);
      }
    }
  }

  disconnect() {
    if (this.eventSource) {
      this.eventSource.close();
      this.eventSource = null;
    }
    this.isConnected = false;
    this.notifyConnectionChange(false);
  }

  isConnectionActive() {
    return this.isConnected && this.eventSource && this.eventSource.readyState === EventSource.OPEN;
  }

  getConnectionStats() {
    return {
      isConnected: this.isConnected,
      reconnectAttempts: this.reconnectAttempts,
      readyState: this.eventSource ? this.eventSource.readyState : null,
      url: this.eventSource ? this.eventSource.url : null
    };
  }
}

/**
 * Submission status tracker for real-time submission updates
 */
export class SubmissionTracker {
  constructor(options = {}) {
    this.connection = new RealtimeConnection({
      ...options,
      onConnectionChange: (connected) => this.handleConnectionChange(connected),
      onError: (error) => this.handleError(error)
    });
    
    this.submissions = new Map();
    this.listeners = new Set();
    this.maxSubmissions = options.maxSubmissions || 50;
    
    this.setupListeners();
  }

  setupListeners() {
    this.connection.addEventListener('submission_update', (data) => {
      this.handleSubmissionUpdate(data);
    });

    this.connection.addEventListener('contest_submission_update', (data) => {
      this.handleSubmissionUpdate(data);
    });
  }

  handleSubmissionUpdate(data) {
    const submissionId = data.submission_id;
    
    // Update or add submission
    this.submissions.set(submissionId, {
      ...this.submissions.get(submissionId),
      ...data,
      lastUpdated: new Date()
    });
    
    // Limit number of tracked submissions
    if (this.submissions.size > this.maxSubmissions) {
      const oldest = [...this.submissions.entries()]
        .sort((a, b) => a[1].lastUpdated - b[1].lastUpdated)[0];
      this.submissions.delete(oldest[0]);
    }
    
    // Notify listeners
    this.notifyListeners('update', data);
  }

  handleConnectionChange(connected) {
    this.notifyListeners('connection', { connected });
  }

  handleError(error) {
    this.notifyListeners('error', { error });
  }

  notifyListeners(type, data) {
    this.listeners.forEach(listener => {
      try {
        listener(type, data);
      } catch (error) {
        console.error('Error in submission tracker listener:', error);
      }
    });
  }

  addListener(listener) {
    this.listeners.add(listener);
  }

  removeListener(listener) {
    this.listeners.delete(listener);
  }

  getSubmission(submissionId) {
    return this.submissions.get(submissionId);
  }

  getAllSubmissions() {
    return [...this.submissions.values()].sort((a, b) => 
      new Date(b.timestamp) - new Date(a.timestamp)
    );
  }

  getSubmissionsByStatus(status) {
    return this.getAllSubmissions().filter(s => s.status === status);
  }

  connect() {
    this.connection.connect();
  }

  disconnect() {
    this.connection.disconnect();
  }

  isConnected() {
    return this.connection.isConnectionActive();
  }
}

/**
 * Leaderboard tracker for real-time leaderboard updates
 */
export class LeaderboardTracker {
  constructor(contestId, options = {}) {
    this.contestId = contestId;
    this.connection = new RealtimeConnection({
      ...options,
      contestId,
      onConnectionChange: (connected) => this.handleConnectionChange(connected),
      onError: (error) => this.handleError(error)
    });
    
    this.leaderboard = [];
    this.listeners = new Set();
    this.lastUpdate = null;
    
    this.setupListeners();
  }

  setupListeners() {
    this.connection.addEventListener('leaderboard_update', (data) => {
      if (data.contest_id === this.contestId) {
        this.handleLeaderboardUpdate(data);
      }
    });
  }

  handleLeaderboardUpdate(data) {
    this.leaderboard = data.rankings || [];
    this.lastUpdate = new Date(data.timestamp);
    
    this.notifyListeners('update', {
      leaderboard: this.leaderboard,
      updateType: data.update_type,
      timestamp: this.lastUpdate
    });
  }

  handleConnectionChange(connected) {
    this.notifyListeners('connection', { connected });
  }

  handleError(error) {
    this.notifyListeners('error', { error });
  }

  notifyListeners(type, data) {
    this.listeners.forEach(listener => {
      try {
        listener(type, data);
      } catch (error) {
        console.error('Error in leaderboard tracker listener:', error);
      }
    });
  }

  addListener(listener) {
    this.listeners.add(listener);
  }

  removeListener(listener) {
    this.listeners.delete(listener);
  }

  getLeaderboard() {
    return this.leaderboard;
  }

  getUserRank(userId) {
    const entry = this.leaderboard.find(entry => entry.user_id === userId);
    return entry ? entry.rank : null;
  }

  getTopUsers(count = 10) {
    return this.leaderboard.slice(0, count);
  }

  connect() {
    this.connection.connect();
  }

  disconnect() {
    this.connection.disconnect();
  }

  isConnected() {
    return this.connection.isConnectionActive();
  }
}

/**
 * Connection status indicator component
 */
export function createConnectionIndicator(container, connection) {
  const indicator = document.createElement('div');
  indicator.className = 'connection-indicator';
  indicator.innerHTML = `
    <div class="flex items-center space-x-2 text-sm">
      <div class="connection-dot w-2 h-2 rounded-full"></div>
      <span class="connection-text">Connecting...</span>
    </div>
  `;
  
  // Add styles
  const style = document.createElement('style');
  style.textContent = `
    .connection-indicator .connection-dot.connected {
      background-color: #10b981;
    }
    .connection-indicator .connection-dot.disconnected {
      background-color: #ef4444;
    }
    .connection-indicator .connection-dot.connecting {
      background-color: #f59e0b;
      animation: pulse 2s infinite;
    }
    @keyframes pulse {
      0%, 100% { opacity: 1; }
      50% { opacity: 0.5; }
    }
  `;
  document.head.appendChild(style);
  
  const dot = indicator.querySelector('.connection-dot');
  const text = indicator.querySelector('.connection-text');
  
  function updateStatus(connected) {
    if (connected) {
      dot.className = 'connection-dot w-2 h-2 rounded-full connected';
      text.textContent = 'Connected';
    } else {
      dot.className = 'connection-dot w-2 h-2 rounded-full disconnected';
      text.textContent = 'Disconnected';
    }
  }
  
  // Initial state
  dot.className = 'connection-dot w-2 h-2 rounded-full connecting';
  
  // Listen to connection changes
  if (connection.onConnectionChange) {
    const originalCallback = connection.onConnectionChange;
    connection.onConnectionChange = (connected) => {
      originalCallback(connected);
      updateStatus(connected);
    };
  } else {
    connection.onConnectionChange = updateStatus;
  }
  
  container.appendChild(indicator);
  return indicator;
}

// Utility functions
export function formatTimeAgo(timestamp) {
  if (!timestamp) return '';
  
  const now = new Date();
  const time = new Date(timestamp);
  const diffMs = now - time;
  const diffSec = Math.floor(diffMs / 1000);
  const diffMin = Math.floor(diffSec / 60);
  const diffHour = Math.floor(diffMin / 60);
  const diffDay = Math.floor(diffHour / 24);
  
  if (diffSec < 60) return `${diffSec}s ago`;
  if (diffMin < 60) return `${diffMin}m ago`;
  if (diffHour < 24) return `${diffHour}h ago`;
  if (diffDay < 30) return `${diffDay}d ago`;
  return time.toLocaleDateString();
}

export function getSubmissionStatusInfo(status, verdict) {
  const statusMap = {
    'PE': { text: 'Pending', class: 'pending', color: 'yellow' },
    'QU': { text: 'Queued', class: 'pending', color: 'yellow' },
    'RU': { text: 'Running', class: 'running', color: 'blue' },
    'CO': { text: 'Compiling', class: 'running', color: 'blue' },
    'AC': { text: 'Accepted', class: 'accepted', color: 'green' },
    'WA': { text: 'Wrong Answer', class: 'rejected', color: 'red' },
    'TLE': { text: 'Time Limit Exceeded', class: 'rejected', color: 'red' },
    'MLE': { text: 'Memory Limit Exceeded', class: 'rejected', color: 'red' },
    'RE': { text: 'Runtime Error', class: 'rejected', color: 'red' },
    'CE': { text: 'Compilation Error', class: 'rejected', color: 'red' }
  };
  
  const info = statusMap[status] || { text: status, class: 'pending', color: 'gray' };
  
  return {
    ...info,
    text: verdict || info.text
  };
}