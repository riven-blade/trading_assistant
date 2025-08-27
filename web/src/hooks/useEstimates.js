import { useState, useEffect, useCallback } from 'react';
import WebSocketManager from '../utils/websocket';
import { toggleEstimateEnabled, deleteEstimate as deleteEstimateAPI } from '../services/api';

/**
 * 价格预估数据管理Hook
 * 通过WebSocket实时获取价格预估数据
 */
const useEstimates = () => {
  const [symbolEstimates, setSymbolEstimates] = useState({});
  const [loading, setLoading] = useState(true);
  const [lastUpdate, setLastUpdate] = useState(null);
  const [error, setError] = useState(null);
  const [wsConnected, setWsConnected] = useState(false);

  // WebSocket价格预估消息处理器
  const handleEstimatesMessage = useCallback((data) => {
    try {
      // 直接更新symbolEstimates数据
      if (data.symbolEstimates) {
        setSymbolEstimates(data.symbolEstimates);
      }
      
      setLastUpdate(new Date(data.lastUpdate * 1000));
      setLoading(false);
      setError(null);
    } catch (err) {
      console.error('处理WebSocket价格预估数据失败:', err);
      setError(err.message);
    }
  }, []);
  
  // 根据币种获取预估
  const getEstimatesBySymbol = useCallback((symbol, status = null) => {
    // 从symbolEstimates中获取指定币种的预估列表
    const symbolEstimateList = symbolEstimates[symbol] || [];
    
    // 由于现在都是listening状态，status参数主要用于未来扩展
    if (status && status !== 'listening') {
      return []; // 只有listening状态的数据
    }
    
    return symbolEstimateList;
  }, [symbolEstimates]);
  
  // 获取正在监听的预估
  const getListeningEstimates = useCallback((symbol = null) => {
    if (symbol) {
      return symbolEstimates[symbol] || [];
    }
    // 返回所有监听预估的平铺数组
    return Object.values(symbolEstimates).flat();
  }, [symbolEstimates]);
  
  // 检查是否有监听中的预估
  const hasListeningEstimates = useCallback((symbol = null) => {
    const listening = getListeningEstimates(symbol);
    return listening.length > 0;
  }, [getListeningEstimates]);

  // 检查是否有任何预估
  const hasAnyEstimate = useCallback((symbol) => {
    if (!symbol) return false;
    // 检查是否有该symbol的监听预估
    return symbolEstimates[symbol] && symbolEstimates[symbol].length > 0;
  }, [symbolEstimates]);
  
  // 获取预估统计
  const getEstimateStats = useCallback(() => {
    const allEstimates = Object.values(symbolEstimates).flat();
    return {
      listening: allEstimates.length,
      symbolCount: Object.keys(symbolEstimates).length,
      symbolEstimates: symbolEstimates,
    };
  }, [symbolEstimates]);

  // 切换预估启用/禁用状态
  const toggleEstimate = useCallback(async (id, enabled) => {
    try {
      await toggleEstimateEnabled(id, enabled);
      // WebSocket会自动推送更新的数据
      return true;
    } catch (error) {
      console.error('切换预估状态失败:', error);
      return false;
    }
  }, []);

  // 删除预估
  const deleteEstimate = useCallback(async (id) => {
    try {
      await deleteEstimateAPI(id);
      // WebSocket会自动推送更新的数据
      return true;
    } catch (error) {
      console.error('删除预估失败:', error);
      throw error;
    }
  }, []);


  // 初始化WebSocket连接
  useEffect(() => {
    let mounted = true;

    // 定义连接状态回调函数
    const handleConnect = () => {
      if (mounted) {
        setWsConnected(true);
        setError(null);
      }
    };

    const handleDisconnect = () => {
      if (mounted) {
        setWsConnected(false);
        setError('WebSocket连接断开');
      }
    };

    const initializeWebSocket = () => {
      // 添加连接状态监听器
      WebSocketManager.addConnectionListener(handleConnect);
      WebSocketManager.addDisconnectionListener(handleDisconnect);

      // 订阅价格预估数据
      WebSocketManager.subscribe('estimates', handleEstimatesMessage);

      // 检查初始连接状态
      const status = WebSocketManager.getStatus();
      if (mounted) {
        setWsConnected(status.isConnected);
        if (!status.isConnected) {
          setError('WebSocket未连接');
        }
      }
    };

    initializeWebSocket();

    // 清理函数
    return () => {
      mounted = false;
      WebSocketManager.unsubscribe('estimates', handleEstimatesMessage);
      WebSocketManager.removeConnectionListener(handleConnect);
      WebSocketManager.removeDisconnectionListener(handleDisconnect);
    };
  }, [handleEstimatesMessage]);
  
  return {
    // 数据
    symbolEstimates,          // 按币种分组的预估数据 {symbol: [estimates]}
    loading,                  // 加载状态
    lastUpdate,               // 最后更新时间
    error,                    // 错误信息
    
    // 连接状态
    wsConnected,              // WebSocket连接状态
    connectionMode: 'websocket', // 连接模式
    
    // 方法
    getEstimatesBySymbol,     // 根据币种获取预估（仅监听状态）
    getListeningEstimates,    // 获取正在监听的预估
    hasListeningEstimates,    // 检查是否有监听中的预估
    hasAnyEstimate,           // 检查是否有任何预估
    getEstimateStats,         // 获取预估统计
    toggleEstimate,           // 切换预估启用/禁用状态
    deleteEstimate,           // 删除预估
    
    // 统计信息
    listeningCount: Object.values(symbolEstimates).flat().length
  };
};

export default useEstimates;