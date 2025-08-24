import { useState, useEffect, useCallback } from 'react';
import api from '../services/api';

/**
 * 账户数据管理 Hook
 * 统一管理账户余额、持仓等信息的获取和更新
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

  // 获取账户价值信息
  const fetchAccountInfo = useCallback(async () => {
    try {
      const response = await api.get('/monitor/balances');
      if (response.data?.data) {
        setAccountValue(response.data.data);
      }
    } catch (error) {
      console.error('获取账户价值信息失败:', error);
      // 重置为默认值
      setAccountValue({
        total_value: 0,
        usdt_total: 0,
        usdt_free: 0,
        other_assets_value: 0,
        total_pnl: 0,
        net_value: 0
      });
    }
  }, []);

  // 获取持仓信息
  const fetchPositions = useCallback(async () => {
    try {
      const response = await api.get('/monitor/positions');
      const positionsData = response.data.data || [];
      setPositions(positionsData);
      
      // 将持仓信息转换为按交易对和方向索引的对象
      const positionsMap = {};
      positionsData.forEach(position => {
        const key = `${position.symbol}_${position.side.toLowerCase()}`;
        positionsMap[key] = position;
      });
      setPositionsMap(positionsMap);
    } catch (error) {
      console.error('获取持仓信息失败:', error);
      setPositions([]);
      setPositionsMap({});
    } finally {
      setLoading(false);
    }
  }, []);

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

  // 初始化
  useEffect(() => {
    fetchAccountInfo();
    fetchPositions();
  }, [fetchAccountInfo, fetchPositions]);

  return {
    accountValue,
    positions,
    positionsMap,
    loading,
    fetchAccountInfo,
    fetchPositions,
    hasPosition,
    hasAnyPosition
  };
};

export default useAccountData;
