import { getToken } from './auth.js';

class WebSocketManager {
  constructor() {
    this.ws = null;
    this.isConnected = false;
    this.isConnecting = false;
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = 5;
    this.subscriptions = new Map(); // dataType -> Set of callbacks
    this.heartbeatInterval = null;
    this.heartbeatTimer = 30000; // 30秒
    
    // 消息队列（连接断开时缓存消息）
    this.messageQueue = [];
    
    // 事件订阅者
    this.connectionListeners = new Set();
    this.disconnectionListeners = new Set();
    this.errorListeners = new Set();
    this.reconnectListeners = new Set();
  }

  // 连接WebSocket
  connect() {
    if (this.isConnected) {

      return Promise.resolve();
    }

    if (this.isConnecting) {

      return Promise.reject(new Error('Already connecting'));
    }

    this.isConnecting = true;
    return new Promise((resolve, reject) => {
      try {
        // 构建WebSocket URL
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        
        // 获取token
        const token = getToken();
        if (!token) {
          console.warn('[WebSocket] 未找到认证token，跳过连接');
          reject(new Error('No authentication token found'));
          return;
        }
        
        // 在开发环境中，WebSocket需要连接到后端服务器
        let wsUrl;
        if (process.env.NODE_ENV === 'development') {
          // 开发环境：连接到后端服务器 (localhost:8080)
          wsUrl = `${protocol}//localhost:8080/ws?token=${encodeURIComponent(token)}`;
        } else {
          // 生产环境：使用当前host
          const host = window.location.host;
          wsUrl = `${protocol}//${host}/ws?token=${encodeURIComponent(token)}`;
        }

        this.ws = new WebSocket(wsUrl);

        this.ws.onopen = () => {

          this.isConnected = true;
          this.isConnecting = false;
          this.reconnectAttempts = 0;
          
          // 开始心跳
          this.startHeartbeat();
          
          // 重新订阅所有数据类型
          this.resubscribeAll();
          
          // 处理消息队列
          this.processMessageQueue();
          
          // 通知所有连接监听者
          this.connectionListeners.forEach(callback => {
            try {
              callback();
            } catch (error) {
              console.error('[WebSocket] 连接回调执行失败:', error);
            }
          });
          
          resolve();
        };

        this.ws.onmessage = (event) => {
          this.handleMessage(event.data);
        };

        this.ws.onclose = (event) => {
  
          this.isConnected = false;
          this.stopHeartbeat();
          
          // 通知所有断连监听者
          this.disconnectionListeners.forEach(callback => {
            try {
              callback(event);
            } catch (error) {
              console.error('[WebSocket] 断连回调执行失败:', error);
            }
          });
          
          // 自动重连
          if (this.reconnectAttempts < this.maxReconnectAttempts) {
            this.scheduleReconnect();
          } else {
            console.error('[WebSocket] 达到最大重连次数，停止重连');
          }
        };

        this.ws.onerror = (error) => {
          console.error('[WebSocket] 连接错误:', error);
          this.isConnecting = false;
          // 通知所有错误监听者
          this.errorListeners.forEach(callback => {
            try {
              callback(error);
            } catch (err) {
              console.error('[WebSocket] 错误回调执行失败:', err);
            }
          });
          reject(error);
        };

      } catch (error) {
        console.error('[WebSocket] 创建连接失败:', error);
        this.isConnecting = false;
        reject(error);
      }
    });
  }

  // 断开连接
  disconnect() {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.isConnected = false;
    this.stopHeartbeat();

  }

  // 发送消息
  send(message) {
    if (this.isConnected && this.ws) {
      try {
        this.ws.send(JSON.stringify(message));
        return true;
      } catch (error) {
        console.error('[WebSocket] 发送消息失败:', error);
        // 添加到消息队列，等待重连后发送
        this.messageQueue.push(message);
        return false;
      }
    } else {
      // 添加到消息队列
      this.messageQueue.push(message);
      return false;
    }
  }

  // 订阅数据类型
  subscribe(dataType, callback) {
    if (!this.subscriptions.has(dataType)) {
      this.subscriptions.set(dataType, new Set());
    }
    this.subscriptions.get(dataType).add(callback);

    // 如果没有连接，尝试连接
    if (!this.isConnected && !this.isConnecting) {
      this.connect().catch(error => {
        console.warn('[WebSocket] 连接失败，跳过订阅:', error.message);
      });
    }

    // 发送订阅消息到服务器
    this.send({
      type: 'subscribe',
      dataType: dataType,
      timestamp: Date.now()
    });


  }

  // 取消订阅数据类型
  unsubscribe(dataType, callback) {
    if (this.subscriptions.has(dataType)) {
      this.subscriptions.get(dataType).delete(callback);
      
      // 如果没有更多回调，从服务器取消订阅
      if (this.subscriptions.get(dataType).size === 0) {
        this.subscriptions.delete(dataType);
        this.send({
          type: 'unsubscribe',
          dataType: dataType,
          timestamp: Date.now()
        });

      }
    }
  }

  // 处理收到的消息
  handleMessage(data) {
    // 处理可能连接在一起的多个JSON消息
    const messages = this.splitJSONMessages(data);
    
    messages.forEach(messageStr => {
      if (!messageStr.trim()) return;
      
      try {
        const message = JSON.parse(messageStr);
        
        if (message.type === 'message' && message.dataType) {
          // 数据消息，分发给订阅者
          const callbacks = this.subscriptions.get(message.dataType);
          if (callbacks) {
            callbacks.forEach(callback => {
              try {
                callback(message.data, message);
              } catch (error) {
                console.error(`[WebSocket] 回调执行失败 (${message.dataType}):`, error);
              }
            });
          }
        } else if (message.type === 'pong') {
          // 心跳响应
          console.debug('[WebSocket] 收到pong');
        } else if (message.type === 'error') {
          // 错误消息
          console.error('[WebSocket] 服务器错误:', message.data);
        }
      } catch (error) {
        console.error('[WebSocket] 解析消息失败:', error, messageStr);
      }
    });
  }

  // 分割可能连接在一起的JSON消息
  splitJSONMessages(data) {
    const messages = [];
    let braceCount = 0;
    let currentMessage = '';
    let inString = false;
    let escapeNext = false;

    for (let i = 0; i < data.length; i++) {
      const char = data[i];
      currentMessage += char;

      if (escapeNext) {
        escapeNext = false;
        continue;
      }

      if (char === '\\') {
        escapeNext = true;
        continue;
      }

      if (char === '"') {
        inString = !inString;
        continue;
      }

      if (!inString) {
        if (char === '{') {
          braceCount++;
        } else if (char === '}') {
          braceCount--;
          if (braceCount === 0) {
            // 完整的JSON消息
            messages.push(currentMessage.trim());
            currentMessage = '';
          }
        }
      }
    }

    // 如果还有剩余内容，也加入消息列表
    if (currentMessage.trim()) {
      messages.push(currentMessage.trim());
    }

    return messages;
  }

  // 重新订阅所有数据类型
  resubscribeAll() {
    this.subscriptions.forEach((callbacks, dataType) => {
      this.send({
        type: 'subscribe',
        dataType: dataType,
        timestamp: Date.now()
      });
    });

  }

  // 处理消息队列
  processMessageQueue() {
    if (this.messageQueue.length > 0) {

      const queue = [...this.messageQueue];
      this.messageQueue = [];
      
      queue.forEach(message => {
        this.send(message);
      });
    }
  }

  // 开始心跳
  startHeartbeat() {
    this.stopHeartbeat();
    this.heartbeatInterval = setInterval(() => {
      if (this.isConnected) {
        this.send({
          type: 'ping',
          timestamp: Date.now()
        });
      }
    }, this.heartbeatTimer);
  }

  // 停止心跳
  stopHeartbeat() {
    if (this.heartbeatInterval) {
      clearInterval(this.heartbeatInterval);
      this.heartbeatInterval = null;
    }
  }

  // 安排重连
  scheduleReconnect() {
    this.reconnectAttempts++;
    const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts - 1), 30000); // 指数退避，最大30秒
    setTimeout(() => {
      if (!this.isConnected) {
        this.connect().then(() => {
          // 通知所有重连监听者
          this.reconnectListeners.forEach(callback => {
            try {
              callback(this.reconnectAttempts);
            } catch (error) {
              console.error('[WebSocket] 重连回调执行失败:', error);
            }
          });
        }).catch(error => {
          console.error(`[WebSocket] 第 ${this.reconnectAttempts} 次重连失败:`, error);
        });
      }
    }, delay);
  }

  // 获取连接状态
  getStatus() {
    return {
      isConnected: this.isConnected,
      isConnecting: this.isConnecting,
      reconnectAttempts: this.reconnectAttempts,
      subscriptions: Array.from(this.subscriptions.keys()),
      queuedMessages: this.messageQueue.length
    };
  }

  // 添加连接状态监听器
  addConnectionListener(callback) {
    this.connectionListeners.add(callback);
  }

  // 移除连接状态监听器
  removeConnectionListener(callback) {
    this.connectionListeners.delete(callback);
  }

  // 添加断连状态监听器
  addDisconnectionListener(callback) {
    this.disconnectionListeners.add(callback);
  }

  // 移除断连状态监听器
  removeDisconnectionListener(callback) {
    this.disconnectionListeners.delete(callback);
  }
}

// 创建全局WebSocket管理器实例
const globalWebSocketManager = new WebSocketManager();

export default globalWebSocketManager;
