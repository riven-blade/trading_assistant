import { useState, useEffect, useCallback } from 'react';
import WebSocketManager from '../utils/websocket';

/**
 * 账户数据管理Hook
 * 通过WebSocket实时获取账户余额和持仓数据
 */
const useAccountData = () => {
  // 余额相关状态
  const [accountValue, setAccountValue] = useState({
    usdt_free: 0,
    usdt_margin: 0,
    usdt_total: 0,
    net_value: 0,
    total_pnl: 0
  });
  
  // 资产详情状态
  const [assetDetails, setAssetDetails] = useState([]);
  
  // 持仓相关状态
  const [positions, setPositions] = useState([]);
  const [totalPnl, setTotalPnl] = useState(0);
  const [positionCount, setPositionCount] = useState(0);
  
  // 通用状态
  const [loading, setLoading] = useState(true);
  const [lastUpdate, setLastUpdate] = useState(null);
  const [error, setError] = useState(null);
  const [wsConnected, setWsConnected] = useState(false);

  // WebSocket账户数据消息处理器
  const handleAccountMessage = useCallback((data) => {
    try {
      if (data.positions) {
        setPositions(data.positions);
        setPositionCount(data.positionCount || data.positions.length);
      }
      
      if (data.totalPnl !== undefined) {
        setTotalPnl(data.totalPnl);
      }
      
      setLastUpdate(new Date(data.lastUpdate * 1000));
      setLoading(false);
      setError(null);
    } catch (err) {
      console.error('处理WebSocket账户数据失败:', err);
      setError(err.message);
    }
  }, []);

  // WebSocket余额数据消息处理器
  const handleBalanceMessage = useCallback((data) => {
    try {
      // 处理账户总值数据
      setAccountValue(data);
      
      // 处理资产详情数据
      if (data.asset_details) {
        setAssetDetails(data.asset_details);
      }
      
      setLastUpdate(new Date());
      setLoading(false);
      setError(null);
    } catch (err) {
      console.error('处理WebSocket余额数据失败:', err);
      setError(err.message);
    }
  }, []);
  
  // 获取指定仓位
  const getPositionBySymbol = useCallback((symbol) => {
    return positions.find(pos => pos.symbol === symbol) || null;
  }, [positions]);
  
  // 检查是否有仓位
  const hasPosition = useCallback((symbol, side) => {
    const position = positions.find(pos => 
      pos.symbol === symbol && 
      pos.side.toLowerCase() === side.toLowerCase() && 
      Math.abs(pos.size) > 0
    );
    return !!position;
  }, [positions]);

  // 检查是否有任何方向的仓位
  const hasAnyPosition = useCallback((symbol) => {
    const symbolPositions = positions.filter(pos => 
      pos.symbol === symbol && Math.abs(pos.size) > 0
    );
    return symbolPositions.length > 0;
  }, [positions]);
  
  // 获取账户总价值
  const getTotalAccountValue = useCallback(() => {
    return accountValue.net_value || 0;
  }, [accountValue]);

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

      // 订阅账户和余额数据
      WebSocketManager.subscribe('account', handleAccountMessage);
      WebSocketManager.subscribe('balances', handleBalanceMessage);

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
      WebSocketManager.unsubscribe('account', handleAccountMessage);
      WebSocketManager.unsubscribe('balances', handleBalanceMessage);
      WebSocketManager.removeConnectionListener(handleConnect);
      WebSocketManager.removeDisconnectionListener(handleDisconnect);
    };
  }, [handleAccountMessage, handleBalanceMessage]);
  
  return {
    // 余额数据
    accountValue,        // 账户余额信息
    assetDetails,        // 资产详情列表
    
    // 持仓数据
    positions,           // 持仓列表
    totalPnl,            // 总盈亏
    positionCount,       // 持仓数量
    
    // 通用状态
    loading,             // 加载状态
    lastUpdate,          // 最后更新时间
    error,               // 错误信息
    
    // 连接状态
    wsConnected,         // WebSocket连接状态
    connectionMode: 'websocket', // 连接模式（固定为WebSocket）
    
    // 方法
    getPositionBySymbol, // 获取指定仓位
    hasPosition,         // 检查是否有仓位
    hasAnyPosition,      // 检查是否有任何方向的仓位
    getTotalAccountValue, // 获取账户总价值
  };
};

export default useAccountData;