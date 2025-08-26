import { useState, useEffect, useCallback, useRef } from 'react';
import api from '../services/api';

/**
 * 全局账户数据管理 Hook
 * 定时获取账户余额、持仓等信息，提供给整个应用使用
 */
const useAccountData = () => {
  const [accountValue, setAccountValue] = useState({
    total_value: 0,
    usdt_total: 0,
    usdt_free: 0,
    other_assets_value: 0,
    total_pnl: 0,
    net_value: 0
  });

  const [positions, setPositions] = useState([]);
  const [positionsMap, setPositionsMap] = useState({});
  const [loading, setLoading] = useState(true);
  const [lastUpdate, setLastUpdate] = useState(null);
  const [error, setError] = useState(null);
  
  // 使用ref来避免useEffect的依赖问题
  const intervalRef = useRef(null);

  // 获取账户数据（余额+持仓）
  const fetchAccountData = useCallback(async () => {
    try {
      setError(null);
      
      // 并行获取账户余额和持仓信息
      const [balanceResponse, positionsResponse] = await Promise.all([
        api.get('/monitor/balances'),
        api.get('/monitor/positions')
      ]);
      
      // 更新账户价值信息
      if (balanceResponse.data?.data) {
        setAccountValue(balanceResponse.data.data);
      }
      
      // 更新持仓信息
      const positionsData = positionsResponse.data.data || [];
      setPositions(positionsData);
      
      // 将持仓信息转换为按交易对和方向索引的对象
      const positionsMap = {};
      positionsData.forEach(position => {
        const key = `${position.symbol}_${position.side.toLowerCase()}`;
        positionsMap[key] = position;
      });
      setPositionsMap(positionsMap);
      
      setLastUpdate(new Date());
      setLoading(false);
      
      console.log(`[账户更新] 余额更新完成，获取到 ${positionsData.length} 个持仓`);
    } catch (err) {
      console.error('获取账户数据失败:', err);
      setError(err.message);
      setLoading(false);
      
      // 出错时重置为默认值
      setAccountValue({
        total_value: 0,
        usdt_total: 0,
        usdt_free: 0,
        other_assets_value: 0,
        total_pnl: 0,
        net_value: 0
      });
      setPositions([]);
      setPositionsMap({});
    }
  }, []);
  
  // 启动定时器
  const startPolling = useCallback(() => {
    // 清除现有定时器
    if (intervalRef.current) {
      clearInterval(intervalRef.current);
    }
    
    // 立即获取一次数据
    fetchAccountData();
    
    // 设置定时器，每秒更新一次
    intervalRef.current = setInterval(fetchAccountData, 1000);
    
    console.log('[账户管理] 账户数据定时更新已启动 (1秒间隔)');
  }, [fetchAccountData]);
  
  // 停止定时器
  const stopPolling = useCallback(() => {
    if (intervalRef.current) {
      clearInterval(intervalRef.current);
      intervalRef.current = null;
      console.log('[账户管理] 账户数据定时更新已停止');
    }
  }, []);
  
  // 手动刷新
  const refreshAccountData = useCallback(() => {
    setLoading(true);
    fetchAccountData();
  }, [fetchAccountData]);
  
  // 为了兼容现有代码，保留单独的获取方法
  const fetchAccountInfo = useCallback(() => refreshAccountData(), [refreshAccountData]);
  const fetchPositions = useCallback(() => refreshAccountData(), [refreshAccountData]);

  // 检查是否有指定方向的仓位
  const hasPosition = useCallback((symbol, side) => {
    const key = `${symbol}_${side.toLowerCase()}`;
    return positionsMap[key] && Math.abs(positionsMap[key].size) > 0;
  }, [positionsMap]);

  // 检查交易对是否有任何仓位
  const hasAnyPosition = useCallback((symbol) => {
    const longKey = `${symbol}_long`;
    const shortKey = `${symbol}_short`;
    return (positionsMap[longKey] && Math.abs(positionsMap[longKey].size) > 0) ||
           (positionsMap[shortKey] && Math.abs(positionsMap[shortKey].size) > 0);
  }, [positionsMap]);

  // 组件挂载时启动，卸载时清理
  useEffect(() => {
    startPolling();
    
    return () => {
      stopPolling();
    };
  }, [startPolling, stopPolling]);
  
  // 处理页面可见性变化
  useEffect(() => {
    const handleVisibilityChange = () => {
      if (document.hidden) {
        // 页面隐藏时停止轮询
        stopPolling();
      } else {
        // 页面可见时重新开始轮询
        startPolling();
      }
    };
    
    document.addEventListener('visibilitychange', handleVisibilityChange);
    
    return () => {
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
  }, [startPolling, stopPolling]);

  return {
    // 数据
    accountValue,        // 账户价值信息
    positions,           // 持仓列表
    positionsMap,        // 持仓映射 {symbol_side: position}
    loading,             // 加载状态
    lastUpdate,          // 最后更新时间
    error,               // 错误信息
    
    // 方法
    refreshAccountData,  // 手动刷新
    startPolling,        // 启动定时器
    stopPolling,         // 停止定时器
    hasPosition,         // 检查是否有指定方向的仓位
    hasAnyPosition,      // 检查交易对是否有任何仓位
    
    // 为了兼容现有代码，保留旧的方法名
    fetchAccountInfo,    // 兼容旧代码
    fetchPositions,      // 兼容旧代码
    
    // 统计信息
    positionCount: positions.length,
    activePositionCount: positions.filter(p => Math.abs(p.size) > 0).length
  };
};

export default useAccountData;
