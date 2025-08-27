import { useState, useEffect, useCallback } from 'react';
import WebSocketManager from '../utils/websocket';

/**
 * 全局价格数据管理Hook
 * 通过WebSocket实时获取所有选中币种的markPrice数据
 */
const usePriceData = () => {
  const [priceData, setPriceData] = useState({});
  const [loading, setLoading] = useState(true);
  const [lastUpdate, setLastUpdate] = useState(null);
  const [error, setError] = useState(null);
  const [wsConnected, setWsConnected] = useState(false);
  
  // WebSocket消息处理器
  const handlePriceMessage = useCallback((data) => {
    try {
      // 增量更新：合并新数据到现有数据
      setPriceData(prevData => ({
        ...prevData,
        ...data
      }));
      
      setLastUpdate(new Date());
      setLoading(false);
      setError(null);
    } catch (err) {
      console.error('处理WebSocket价格数据失败:', err);
      setError(err.message);
    }
  }, []);
  
  // 获取单个币种价格
  const getPriceBySymbol = useCallback((symbol) => {
    return priceData[symbol] || null;
  }, [priceData]);
  
  // 检查是否有价格数据
  const hasPriceData = useCallback((symbol) => {
    return priceData[symbol] && priceData[symbol].markPrice > 0;
  }, [priceData]);
  
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

      // 订阅价格数据
      WebSocketManager.subscribe('prices', handlePriceMessage);

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
      WebSocketManager.unsubscribe('prices', handlePriceMessage);
      WebSocketManager.removeConnectionListener(handleConnect);
      WebSocketManager.removeDisconnectionListener(handleDisconnect);
    };
  }, [handlePriceMessage]);
  
  return {
    // 数据
    priceData,           // 所有价格数据 {symbol: priceInfo}
    loading,             // 加载状态
    lastUpdate,          // 最后更新时间
    error,               // 错误信息
    
    // 连接状态
    wsConnected,         // WebSocket连接状态
    connectionMode: 'websocket', // 连接模式（固定为WebSocket）
    
    // 方法
    getPriceBySymbol,    // 获取单个币种价格
    hasPriceData,        // 检查是否有价格数据
    
    // 统计信息
    priceCount: Object.keys(priceData).length,
    validPriceCount: Object.values(priceData).filter(p => p.markPrice > 0).length
  };
};

export default usePriceData;