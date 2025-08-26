import { useState, useEffect, useCallback, useRef } from 'react';
import api, { toggleEstimateEnabled } from '../services/api';
import { message } from 'antd';

/**
 * 全局价格预估数据管理 Hook
 * 定时获取所有价格预估数据，提供给整个应用使用
 */
const useEstimates = () => {
  const [estimates, setEstimates] = useState([]);
  const [symbolEstimates, setSymbolEstimates] = useState({}); // 按交易对统计
  const [loading, setLoading] = useState(true);
  const [lastUpdate, setLastUpdate] = useState(null);
  const [error, setError] = useState(null);
  
  // 使用ref来避免useEffect的依赖问题
  const intervalRef = useRef(null);

  // 获取所有价格预估数据
  const fetchEstimatesData = useCallback(async () => {
    try {
      setError(null);
      const response = await api.get('/estimates');
      const estimatesData = response.data.data || [];
      
      // 按时间倒序排列
      const sortedEstimates = estimatesData.sort((a, b) => {
        return new Date(b.created_at) - new Date(a.created_at);
      });
      
      // 按交易对统计监听数量（只统计listening状态的）
      const estimatesMap = {};
      estimatesData.forEach(estimate => {
        if (estimate.status === 'listening') {
          estimatesMap[estimate.symbol] = (estimatesMap[estimate.symbol] || 0) + 1;
        }
      });
      
      setEstimates(sortedEstimates);
      setSymbolEstimates(estimatesMap);
      setLastUpdate(new Date());
      setLoading(false);
      
      console.log(`[价格预估更新] 获取到 ${estimatesData.length} 条价格预估数据`);
    } catch (err) {
      console.error('获取价格预估数据失败:', err);
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
    fetchEstimatesData();
    
    // 设置定时器，每秒更新一次
    intervalRef.current = setInterval(fetchEstimatesData, 1000);
    
    console.log('[价格预估管理] 价格预估数据定时更新已启动 (1秒间隔)');
  }, [fetchEstimatesData]);
  
  // 停止定时器
  const stopPolling = useCallback(() => {
    if (intervalRef.current) {
      clearInterval(intervalRef.current);
      intervalRef.current = null;
      console.log('[价格预估管理] 价格预估数据定时更新已停止');
    }
  }, []);

  // 手动刷新
  const refreshEstimatesData = useCallback(() => {
    setLoading(true);
    fetchEstimatesData();
  }, [fetchEstimatesData]);

  // 获取价格预估列表（根据symbol过滤）
  const fetchEstimates = useCallback(async (symbol = '') => {
    // 从内存中过滤数据，不再发起新的API请求
    if (symbol) {
      const filtered = estimates.filter(estimate => estimate.symbol === symbol);
      return filtered;
    }
    return estimates;
  }, [estimates]);

  // 获取各交易对的监听数量统计
  const fetchSymbolEstimates = useCallback(async () => {
    // 从内存中返回已计算好的统计数据
    return symbolEstimates;
  }, [symbolEstimates]);

  // 删除价格预估
  const deleteEstimate = useCallback(async (id) => {
    try {
      await api.delete(`/estimates/${id}`);
      message.success('删除成功');
      // 立即刷新数据而不是等待下次定时更新
      await fetchEstimatesData();
      return true;
    } catch (error) {
      message.error('删除失败');
      return false;
    }
  }, [fetchEstimatesData]);

  // 获取指定交易对的监听（从内存中过滤）
  const getEstimatesBySymbol = useCallback((symbol, status = null) => {
    let filtered = estimates;
    
    if (symbol) {
      filtered = filtered.filter(estimate => estimate.symbol === symbol);
    }
    
    if (status) {
      filtered = filtered.filter(estimate => estimate.status === status);
    } else {
      // 如果没有指定状态，返回listening状态的数据
      filtered = filtered.filter(estimate => estimate.status === 'listening');
    }
    
    return filtered;
  }, [estimates]);

  // 检查交易对是否有监听
  const hasAnyEstimate = useCallback((symbol) => {
    return symbolEstimates[symbol] > 0;
  }, [symbolEstimates]);

  // 切换价格监听状态
  const toggleEstimate = useCallback(async (id, enabled) => {
    try {
      await toggleEstimateEnabled(id, enabled);
      message.success(`监听已${enabled ? '开启' : '关闭'}`);
      
      // 立即刷新数据
      await fetchEstimatesData();
      return true;
    } catch (error) {
      console.error('切换监听状态失败:', error);
      const errorMsg = error.response?.data?.error || error.message || '切换监听状态失败';
      message.error(`切换失败: ${errorMsg}`);
      return false;
    }
  }, [fetchEstimatesData]);

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
    estimates,           // 所有价格预估数据
    symbolEstimates,     // 按交易对统计的监听数量
    loading,             // 加载状态
    lastUpdate,          // 最后更新时间
    error,               // 错误信息
    
    // 方法
    refreshEstimatesData, // 手动刷新
    startPolling,        // 启动定时器
    stopPolling,         // 停止定时器
    deleteEstimate,      // 删除价格预估
    getEstimatesBySymbol, // 获取指定交易对的监听（内存过滤）
    hasAnyEstimate,      // 检查交易对是否有监听
    toggleEstimate,      // 切换价格监听状态
    
    // 为了兼容现有代码，保留旧的方法名
    fetchEstimates,      // 兼容旧代码（内存过滤）
    fetchSymbolEstimates, // 兼容旧代码（内存过滤）
    
    // 统计信息
    estimateCount: estimates.length,
    listeningCount: estimates.filter(e => e.status === 'listening').length,
    triggeredCount: estimates.filter(e => e.status === 'triggered').length,
    failedCount: estimates.filter(e => e.status === 'failed').length
  };
};

export default useEstimates;
