import { useState, useEffect, useCallback, useRef } from 'react';
import { getAllSelectedCoinsPrices } from '../services/api';

/**
 * 全局价格数据管理Hook
 * 定时获取所有选中币种的markPrice数据，提供给整个应用使用
 */
const usePriceData = () => {
  const [priceData, setPriceData] = useState({});
  const [loading, setLoading] = useState(true);
  const [lastUpdate, setLastUpdate] = useState(null);
  const [error, setError] = useState(null);
  
  // 使用ref来避免useEffect的依赖问题
  const intervalRef = useRef(null);
  
  // 获取价格数据
  const fetchPriceData = useCallback(async () => {
    try {
      setError(null);
      const data = await getAllSelectedCoinsPrices();
      
      // 转换数据格式为前端需要的格式
      const newPriceData = {};
      data.forEach(item => {
        newPriceData[item.symbol] = {
          symbol: item.symbol,
          markPrice: item.mark_price,
          indexPrice: item.index_price,
          fundingRate: item.funding_rate,
          fundingTime: item.funding_time,
          updateTime: item.update_time,
          priceChange: item.price_change,
          priceChangePercent: item.price_change_percent,
          // 为了兼容现有代码，也提供这些字段
          currentPrice: item.mark_price,
          bestBid: item.mark_price * 0.9999, // 模拟买一价
          bestAsk: item.mark_price * 1.0001, // 模拟卖一价
          lastUpdate: new Date(item.update_time),
          hasValidData: item.mark_price > 0
        };
      });
      
      setPriceData(newPriceData);
      setLastUpdate(new Date());
      setLoading(false);
      
      console.log(`[价格更新] 获取到 ${data.length} 个币种的价格数据`);
    } catch (err) {
      console.error('获取价格数据失败:', err);
      setError(err.message);
      setLoading(false);
    }
  }, []);
  
  // 启动定时器
  const startPolling = useCallback(() => {
    // 清除现有定时器
    if (intervalRef.current) {
      clearInterval(intervalRef.current);
    }
    
    // 立即获取一次数据
    fetchPriceData();
    
    // 设置定时器，每秒更新一次
    intervalRef.current = setInterval(fetchPriceData, 1000);
    
    console.log('[价格管理] 价格数据定时更新已启动 (1秒间隔)');
  }, [fetchPriceData]);
  
  // 停止定时器
  const stopPolling = useCallback(() => {
    if (intervalRef.current) {
      clearInterval(intervalRef.current);
      intervalRef.current = null;
      console.log('[价格管理] 价格数据定时更新已停止');
    }
  }, []);
  
  // 手动刷新
  const refreshPriceData = useCallback(() => {
    setLoading(true);
    fetchPriceData();
  }, [fetchPriceData]);
  
  // 获取单个币种价格
  const getPriceBySymbol = useCallback((symbol) => {
    return priceData[symbol] || null;
  }, [priceData]);
  
  // 检查是否有价格数据
  const hasPriceData = useCallback((symbol) => {
    return priceData[symbol] && priceData[symbol].hasValidData;
  }, [priceData]);
  
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
    priceData,           // 所有价格数据 {symbol: priceInfo}
    loading,             // 加载状态
    lastUpdate,          // 最后更新时间
    error,               // 错误信息
    
    // 方法
    refreshPriceData,    // 手动刷新
    getPriceBySymbol,    // 获取单个币种价格
    hasPriceData,        // 检查是否有价格数据
    startPolling,        // 启动定时器
    stopPolling,         // 停止定时器
    
    // 统计信息
    priceCount: Object.keys(priceData).length,
    validPriceCount: Object.values(priceData).filter(p => p.hasValidData).length
  };
};

export default usePriceData;
