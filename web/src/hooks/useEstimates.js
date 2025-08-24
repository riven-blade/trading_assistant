import { useState, useEffect, useCallback } from 'react';
import api, { toggleEstimateEnabled } from '../services/api';
import { message } from 'antd';

/**
 * 价格监听管理 Hook
 * 统一管理价格预估/监听的获取、删除等操作
 */
const useEstimates = () => {
  const [estimates, setEstimates] = useState([]);
  const [symbolEstimates, setSymbolEstimates] = useState({}); // 按交易对统计
  const [loading, setLoading] = useState(true);

  // 获取价格预估列表
  const fetchEstimates = useCallback(async (symbol = '') => {
    try {
      setLoading(true);
      const url = symbol ? `/estimates?symbol=${symbol}` : '/estimates';
      const response = await api.get(url);
      const estimatesData = response.data.data || [];
      
      // 按时间倒序排列
      const sortedEstimates = estimatesData.sort((a, b) => {
        return new Date(b.created_at) - new Date(a.created_at);
      });
      
      setEstimates(sortedEstimates);
      return sortedEstimates;
    } catch (error) {
      message.error('获取价格预估失败');
      return [];
    } finally {
      setLoading(false);
    }
  }, []);

  // 获取各交易对的监听数量统计
  const fetchSymbolEstimates = useCallback(async () => {
    try {
      const response = await api.get('/estimates');
      const estimatesData = response.data.data || [];
      
      // 按交易对统计监听数量（只统计listening状态的）
      const estimatesMap = {};
      estimatesData.forEach(estimate => {
        if (estimate.status === 'listening') {
          estimatesMap[estimate.symbol] = (estimatesMap[estimate.symbol] || 0) + 1;
        }
      });
      
      setSymbolEstimates(estimatesMap);
      return estimatesMap;
    } catch (error) {
      console.error('获取监听信息失败:', error);
      setSymbolEstimates({});
      return {};
    }
  }, []);

  // 删除价格预估
  const deleteEstimate = useCallback(async (id) => {
    try {
      await api.delete(`/estimates/${id}`);
      message.success('删除成功');
      await fetchEstimates(); // 重新获取列表
      await fetchSymbolEstimates(); // 更新统计
      return true;
    } catch (error) {
      message.error('删除失败');
      return false;
    }
  }, [fetchEstimates, fetchSymbolEstimates]);

  // 获取指定交易对的监听
  const getEstimatesBySymbol = useCallback(async (symbol, status = null) => {
    try {
      const url = symbol ? `/estimates?symbol=${symbol}` : '/estimates';
      const response = await api.get(url);
      const data = response.data.data || [];
      
      // 如果没有指定状态，返回listening状态的数据
      if (!status) {
        return data.filter(estimate => 
          estimate.status === 'listening'
        );
      }
      
      return data.filter(estimate => estimate.status === status);
    } catch (error) {
      console.error('获取交易对监听失败:', error);
      return [];
    }
  }, []);

  // 检查交易对是否有监听
  const hasAnyEstimate = useCallback((symbol) => {
    return symbolEstimates[symbol] > 0;
  }, [symbolEstimates]);

  // 切换价格监听状态
  const toggleEstimate = useCallback(async (id, enabled) => {
    try {

      await toggleEstimateEnabled(id, enabled);
      message.success(`监听已${enabled ? '开启' : '关闭'}`);
      
      // 刷新数据
      await fetchEstimates();
      await fetchSymbolEstimates();
      return true;
    } catch (error) {
      console.error('切换监听状态失败:', error);
      const errorMsg = error.response?.data?.error || error.message || '切换监听状态失败';
      message.error(`切换失败: ${errorMsg}`);
      return false;
    }
  }, [fetchEstimates, fetchSymbolEstimates]);

  // 初始化
  useEffect(() => {
    fetchEstimates();
    fetchSymbolEstimates();
  }, [fetchEstimates, fetchSymbolEstimates]);

  return {
    estimates,
    symbolEstimates,
    loading,
    fetchEstimates,
    fetchSymbolEstimates,
    deleteEstimate,
    getEstimatesBySymbol,
    hasAnyEstimate,
    toggleEstimate
  };
};

export default useEstimates;
