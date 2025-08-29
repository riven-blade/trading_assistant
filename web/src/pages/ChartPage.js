import React, { useState, useEffect, useCallback, useRef } from 'react';
import { 
  message, 
  Spin
} from 'antd';
import { useLocation, useSearchParams } from 'react-router-dom';
import { init, dispose, registerOverlay } from 'klinecharts';

import ChartToolbar from '../components/Chart/ChartToolbar';
import IndicatorPanel from '../components/Chart/IndicatorPanel';
import { getIndicatorInfo } from '../utils/indicatorUtils';
import useEstimates from '../hooks/useEstimates';
import useAccountData from '../hooks/useAccountData';
import api from '../services/api';
import '../styles/chart-hide-toolbar.css';

// 注册自定义价格线覆盖物，支持文字标签显示
registerOverlay({
  name: 'priceLineWithText',
  totalStep: 1,
  needDefaultPointFigure: false,
  needDefaultXAxisFigure: false,
  needDefaultYAxisFigure: true,
  createPointFigures: ({ coordinates, bounding }) => {
    if (coordinates.length > 0) {
      return [{
        type: 'line',
        attrs: {
          coordinates: [
            { x: 0, y: coordinates[0].y },
            { x: bounding.width, y: coordinates[0].y }
          ]
        },
        ignoreEvent: true
      }];
    }
    return [];
  },
  createYAxisFigures: ({ overlay, coordinates, bounding, yAxis }) => {
    if (coordinates.length === 0) return null;
    
    const isFromZero = yAxis?.isFromZero() ?? false;
    let textAlign = 'left';
    let x = 0;
    
    if (isFromZero) {
      textAlign = 'left';
      x = 0;
    } else {
      textAlign = 'right';
      x = bounding.width;
    }
    
    // 从extendData中获取显示文字
    const text = overlay.extendData || '';
    
    return {
      type: 'text',
      attrs: {
        x,
        y: coordinates[0].y,
        text,
        align: textAlign,
        baseline: 'middle'
      },
      ignoreEvent: true
    };
  }
});

// 注册仓位线覆盖物，使用不同的样式区分
registerOverlay({
  name: 'positionLineWithText',
  totalStep: 1,
  needDefaultPointFigure: false,
  needDefaultXAxisFigure: false,
  needDefaultYAxisFigure: true,
  createPointFigures: ({ coordinates, bounding }) => {
    if (coordinates.length > 0) {
      return [{
        type: 'line',
        attrs: {
          coordinates: [
            { x: 0, y: coordinates[0].y },
            { x: bounding.width, y: coordinates[0].y }
          ]
        },
        ignoreEvent: true
      }];
    }
    return [];
  },
  createYAxisFigures: ({ overlay, coordinates, bounding, yAxis }) => {
    if (coordinates.length === 0) return null;
    
    const isFromZero = yAxis?.isFromZero() ?? false;
    let textAlign = 'left';
    let x = 0;
    
    if (isFromZero) {
      textAlign = 'left';
      x = 0;
    } else {
      textAlign = 'right';
      x = bounding.width;
    }
    
    // 从extendData中获取显示文字
    const text = overlay.extendData || '';
    
    return {
      type: 'text',
      attrs: {
        x,
        y: coordinates[0].y,
        text,
        align: textAlign,
        baseline: 'middle'
      },
      ignoreEvent: true
    };
  }
});

const ChartPage = () => {
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  
  // 解析URL参数的辅助函数
  const parseUrlParams = (search) => {
    const searchParams = new URLSearchParams(search);
    return {
      symbol: searchParams.get('symbol') || 'BTCUSDT',
      interval: searchParams.get('interval') || '5m'
    };
  };
  
  // 基础状态管理
  const [selectedCoin, setSelectedCoin] = useState(() => parseUrlParams(location.search).symbol);
  const [selectedInterval, setSelectedInterval] = useState(() => parseUrlParams(location.search).interval);

  // 同步更新币种状态和URL
  const handleCoinChange = useCallback((newCoin) => {
    if (newCoin !== selectedCoin) {
      setSelectedCoin(newCoin);
      const newParams = new URLSearchParams(searchParams);
      newParams.set('symbol', newCoin);
      setSearchParams(newParams, { replace: true });
    }
  }, [selectedCoin, searchParams, setSearchParams]);

  // 同步更新周期状态和URL
  const handleIntervalChange = useCallback(async (newInterval) => {
    if (newInterval !== selectedInterval) {
      setSelectedInterval(newInterval);
      const newParams = new URLSearchParams(searchParams);
      newParams.set('interval', newInterval);
      setSearchParams(newParams, { replace: true });
      
      // 立即加载新周期的K线数据，避免异步状态更新导致的延迟
      if (selectedCoin && newInterval) {
        setLoading(true);
        try {
          const response = await api.get('/klines', {
            params: {
              symbol: selectedCoin,
              interval: newInterval, // 直接使用新的interval值
              limit: 1000
            }
          });
          
          const data = response.data.data || [];
          
          if (!isMountedRef.current) return;
          
          setKlineData(data);
          
          if (data.length === 0) {
            message.warning('暂无K线数据');
          }
        } catch (error) {
          console.error('加载K线数据失败:', error);
          if (isMountedRef.current) {
            message.error(`加载K线数据失败: ${error.response?.data?.error || error.message}`);
            setKlineData([]);
          }
        }
        setLoading(false);
      }
    }
  }, [selectedInterval, searchParams, setSearchParams, selectedCoin]);
  
  // 解析URL参数的回调函数
  const getUrlParams = useCallback(() => {
    return parseUrlParams(location.search);
  }, [location.search]);
  const [coins, setCoins] = useState([]);
  const [loading, setLoading] = useState(false);
  const [klineData, setKlineData] = useState([]);
  
  // 指标相关状态 - 默认选中成交量和布林带指标
  const [selectedIndicators, setSelectedIndicators] = useState(['VOL', 'BOLL']);
  
  // UI状态管理
  const [indicatorPanelVisible, setIndicatorPanelVisible] = useState(false);
  const [isMobile, setIsMobile] = useState(false);
  
  // 图表引用
  const chartRef = useRef(null);
  const chartInstanceRef = useRef(null);

  const indicatorApplying = useRef(false); // 防止重复应用指标
  const isMountedRef = useRef(true); // 组件挂载状态
  const [chartInitialized, setChartInitialized] = useState(false);

  // 价格监听功能
  const { symbolEstimates, hasAnyEstimate, toggleEstimate } = useEstimates();
  
  // 账户数据（包含持仓信息）
  const { positions, loading: positionsLoading } = useAccountData();
  const [currentSymbolEstimates, setCurrentSymbolEstimates] = useState([]);
  const [currentSymbolPositions, setCurrentSymbolPositions] = useState([]);
  const priceLineIds = useRef(new Set()); // 跟踪已绘制的价格线
  const positionLineIds = useRef(new Set()); // 跟踪已绘制的仓位线



  // 监听URL参数变化
  useEffect(() => {
    const params = getUrlParams();
    if (params.symbol !== selectedCoin) {
      setSelectedCoin(params.symbol);
    }
    if (params.interval !== selectedInterval) {
      setSelectedInterval(params.interval);
    }
  }, [getUrlParams, selectedCoin, selectedInterval]);

  // 移动端检测和窗口大小变化监听
  useEffect(() => {
    const checkIsMobile = () => {
      setIsMobile(window.innerWidth < 768);
    };

    const handleResize = () => {
      checkIsMobile();
      // 窗口大小变化时重新调整面板高度
      if (chartInitialized && chartInstanceRef.current) {
        setTimeout(() => {
          adjustPaneHeights();
        }, 200);
      }
    };

    checkIsMobile();
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, [chartInitialized]);

  // 获取币种列表
  const fetchSelectedCoins = useCallback(async () => {
    try {
      const response = await api.get('/coins/selected');
      const coinData = response.data.data || [];
      setCoins(coinData);
    } catch (error) {
      setCoins([]);
    }
  }, []);



  // 获取当前币种的仓位数据
  const fetchCurrentSymbolPositions = useCallback(async () => {
    if (!selectedCoin || !positions.length) {
      setCurrentSymbolPositions([]);
      return;
    }
    
    try {
      // 筛选当前交易对的仓位，只显示有实际持仓的
      const symbolPositions = positions.filter(position => 
        position.symbol === selectedCoin && Math.abs(position.size) > 0
      );
      
      if (!isMountedRef.current) return;
      
      setCurrentSymbolPositions(symbolPositions);
    } catch (error) {
      setCurrentSymbolPositions([]);
    }
  }, [selectedCoin, positions]);

  // 获取当前币种的价格监听数据（只获取listening状态的监听）
  const fetchCurrentSymbolEstimates = useCallback(() => {
    if (!selectedCoin) {
      setCurrentSymbolEstimates([]);
      return;
    }
    
    try {
      // 直接从symbolEstimates中获取当前币种的预估数据
      const symbolEstimateList = symbolEstimates[selectedCoin] || [];
      
      // 过滤掉无效的测试数据
      const validEstimates = symbolEstimateList.filter(estimate => 
        estimate.id && 
        estimate.id.length > 10 && // 确保ID不是简单的测试ID
        !estimate.id.startsWith('test-')
      );
      
      if (!isMountedRef.current) return;
      
      setCurrentSymbolEstimates(validEstimates);
    } catch (error) {
      setCurrentSymbolEstimates([]);
    }
  }, [selectedCoin, symbolEstimates]);

  // 监听全局symbolEstimates数据变化
  useEffect(() => {
    fetchCurrentSymbolEstimates();
  }, [symbolEstimates, fetchCurrentSymbolEstimates]);

  // 手动刷新K线数据
  const handleRefresh = useCallback(async () => {
    if (!selectedCoin || !selectedInterval) {
      return;
    }
    
    setLoading(true);
    try {
      // 同时刷新K线数据和仓位数据
      const [klineResponse] = await Promise.all([
        api.get('/klines', {
          params: {
            symbol: selectedCoin,
            interval: selectedInterval,
            limit: 1000
          }
        })
      ]);
      
      const data = klineResponse.data.data || [];
      
      // 检查组件是否已卸载
      if (!isMountedRef.current) return;
      
      setKlineData(data);
      
      // 刷新价格监听数据
      fetchCurrentSymbolEstimates();
      
      if (data.length === 0) {
        message.warning('暂无K线数据');
      } else {
        message.success('刷新成功');
      }
    } catch (error) {
      if (isMountedRef.current) {
        message.error(`刷新K线数据失败: ${error.response?.data?.error || error.message}`);
      }
    }
    setLoading(false);
  }, [selectedCoin, selectedInterval, fetchCurrentSymbolEstimates]);

  // 统一绘制所有覆盖层（价格线和仓位线）
  const drawAllOverlays = useCallback(() => {
    if (!chartInstanceRef.current) {
      return;
    }

    // 清除所有现有覆盖层
    priceLineIds.current.forEach(lineId => {
      try {
        chartInstanceRef.current.removeOverlay(lineId);
      } catch (error) {
        // 忽略移除错误
      }
    });
    priceLineIds.current.clear();

    positionLineIds.current.forEach(lineId => {
      try {
        chartInstanceRef.current.removeOverlay(lineId);
      } catch (error) {
        // 忽略移除错误
      }
    });
    positionLineIds.current.clear();

    // 收集所有要绘制的覆盖层
    const overlaysToCreate = [];

    // 1. 收集价格监听线
    currentSymbolEstimates.forEach((estimate) => {
      const lineId = `price_line_${estimate.id}`;
      
      // 内联颜色逻辑
      let color = '#1890ff';
      const { action_type, created_by, side } = estimate;
      if (action_type === 'take_profit' || created_by === 'take_profit') {
        color = side === 'long' ? '#00ff00' : '#ff6600';
      } else if (action_type === 'stop_loss' || created_by === 'stop_loss') {
        color = side === 'long' ? '#ff0000' : '#9932cc';
      } else if (action_type === 'open' || action_type === 'add') {
        color = side === 'long' ? '#0066ff' : '#ff8c00';
      } else if (action_type === 'close') {
        if (created_by === 'take_profit') {
          color = side === 'long' ? '#00ff00' : '#ff6600';
        } else if (created_by === 'stop_loss') {
          color = side === 'long' ? '#ff0000' : '#9932cc';
        } else {
          color = side === 'long' ? '#00ff00' : '#ff8c00';
        }
      }
      
      // 内联动作文字逻辑
      let actionText = action_type;
      if (action_type === 'close') {
        if (created_by === 'take_profit') {
          actionText = '止盈';
        } else if (created_by === 'stop_loss') {
          actionText = '止损';
        } else {
          actionText = '平仓';
        }
      } else {
        const actionMap = {
          'take_profit': '止盈',
          'stop_loss': '止损',
          'open': '开仓',
          'close': '平仓',
          'add': '加仓',
          'addition': '加仓'
        };
        actionText = actionMap[action_type] || action_type;
      }
      
      const lineStyle = estimate.enabled ? 'solid' : 'dashed';
      const sideText = estimate.side === 'long' ? '多' : '空';
      const displayText = `${sideText}${actionText}`;

      overlaysToCreate.push({
        type: 'price',
        overlay: {
          name: 'priceLineWithText',
          id: lineId,
          points: [{ value: estimate.target_price }],
          extendData: displayText,
          styles: {
            line: {
              style: lineStyle,
              color: color,
              size: estimate.enabled ? 2 : 1,
            },
            text: {
              color: color,
              size: 12,
              weight: 'bold',
              backgroundColor: 'rgba(255, 255, 255, 0.9)',
              borderColor: color,
              borderSize: 1,
              borderRadius: 3,
              paddingLeft: 6,
              paddingRight: 6,
              paddingTop: 3,
              paddingBottom: 3
            }
          },
          lock: true
        },
        lineId
      });
    });

    // 2. 收集仓位线
    currentSymbolPositions.forEach((position) => {
      const lineId = `position_line_${position.symbol}_${position.side}`;
      const color = position.side === 'LONG' ? '#1890ff' : '#ff4d4f';
      const sideText = position.side === 'LONG' ? '多头' : '空头';
      const displayText = `${sideText}仓`;

      overlaysToCreate.push({
        type: 'position',
        overlay: {
          name: 'positionLineWithText',
          id: lineId,
          points: [{ value: position.entry_price }],
          extendData: displayText,
          styles: {
            line: {
              style: 'dashed',
              color: color,
              size: 4,
            },
            text: {
              color: color,
              size: 13,
              weight: 'bold',
              backgroundColor: 'rgba(255, 255, 255, 0.98)',
              borderColor: color,
              borderSize: 3,
              borderRadius: 5,
              paddingLeft: 10,
              paddingRight: 10,
              paddingTop: 5,
              paddingBottom: 5
            }
          },
          lock: true
        },
        lineId
      });
    });

    // 3. 统一绘制所有覆盖层
    overlaysToCreate.forEach(({ type, overlay, lineId }) => {
      try {
        chartInstanceRef.current.createOverlay(overlay);
        
        if (type === 'price') {
          priceLineIds.current.add(lineId);
        } else {
          positionLineIds.current.add(lineId);
        }
              } catch (error) {
          // 绘制失败，忽略错误
        }
    });
  }, [currentSymbolEstimates, currentSymbolPositions]);

  // 初始化图表
  useEffect(() => {
    const currentChartRef = chartRef.current;
    
    const initChart = () => {
      const actualRef = chartRef.current; // 每次都重新获取最新的ref
      
      if (!actualRef) {
        setTimeout(initChart, 500);
        return;
      }
      
      if (actualRef && !chartInstanceRef.current) {
        // 检查容器尺寸，确保有有效的尺寸才初始化
        const { offsetWidth, offsetHeight } = actualRef;
        
        // 放宽尺寸检查条件，只要容器存在就尝试初始化
        if (offsetWidth === 0 && offsetHeight === 0) {
          setTimeout(initChart, 100);
          return;
        }
        
        try {
          chartInstanceRef.current = init(actualRef);
          
          if (!chartInstanceRef.current) {
            message.error('图表初始化失败');
            return;
          }
          
          // 设置现代化图表样式
          chartInstanceRef.current.setStyles({
            candle: {
              margin: { top: 0.05, bottom: 0.05 },
              bar: {
                upColor: '#11B981',
                downColor: '#EF4444',
                noChangeColor: '#6B7280',
                upBorderColor: '#11B981',
                downBorderColor: '#EF4444', 
                noChangeBorderColor: '#6B7280',
                upWickColor: '#11B981',
                downWickColor: '#EF4444',
                noChangeWickColor: '#6B7280'
              },
              tooltip: {
                showRule: 'always',
                showType: 'standard',
                text: {
                  size: 12,
                  family: 'Inter, -apple-system, BlinkMacSystemFont, sans-serif',
                  color: '#374151'
                },
                rect: {
                  paddingLeft: 12,
                  paddingRight: 12,
                  paddingTop: 8,
                  paddingBottom: 8,
                  borderRadius: 8,
                  borderSize: 0,
                  color: 'rgba(255, 255, 255, 0.95)',
                  borderColor: 'transparent'
                }
              }
            },
            grid: {
              show: true,
              horizontal: {
                show: true,
                size: 1,
                color: 'rgba(148, 163, 184, 0.15)',
                style: 'solid'
              },
              vertical: {
                show: true,
                size: 1,
                color: 'rgba(148, 163, 184, 0.15)',
                style: 'solid'
              }
            },
            separator: {
              size: 1,
              color: '#e5e5e5',
              fill: true,
              activeBackgroundColor: 'rgba(230, 230, 230, 0.15)'
            },
            xAxis: {
              show: true,
              height: 36,
              axisLine: { 
                show: true, 
                color: 'rgba(107, 114, 128, 0.2)' 
              },
              tickText: { 
                show: true, 
                color: '#6B7280', 
                size: 11,
                family: 'Inter, -apple-system, BlinkMacSystemFont, sans-serif'
              }
            },
            yAxis: {
              show: true,
              width: 68,
              position: 'right',
              axisLine: { 
                show: true, 
                color: 'rgba(107, 114, 128, 0.2)' 
              },
              tickText: { 
                show: true, 
                color: '#6B7280', 
                size: 11,
                family: 'Inter, -apple-system, BlinkMacSystemFont, sans-serif'
              }
            },
            // 十字线配置
            crosshair: {
              show: true,
              horizontal: {
                show: true,
                line: {
                  show: true,
                  style: 'dash',
                  size: 1,
                  color: '#9ca3af'
                }
              },
              vertical: {
                show: true,
                line: {
                  show: true,
                  style: 'dash',
                  size: 1,
                  color: '#9ca3af'
                }
              }
            }
          });
          
          // 创建成交量指标
          try {
            chartInstanceRef.current.createIndicator('VOL', false);
          } catch (error) {
            // 创建成交量指标失败，忽略错误
          }
          


          setChartInitialized(true);
        } catch (error) {
          message.error('图表初始化失败');
        }
      }
    };

    const timer = setTimeout(initChart, 100);

    return () => {
      clearTimeout(timer);

      if (chartInstanceRef.current && currentChartRef) {
        try {
          dispose(currentChartRef);
          chartInstanceRef.current = null;
          setChartInitialized(false);
        } catch (error) {
          // 图表销毁失败，忽略错误
        }
      }
    };
  }, []); // 图表初始化只需要执行一次
  
  // 组件卸载时的清理
  useEffect(() => {
    return () => {
      isMountedRef.current = false;
    };
  }, []);

  // 清除所有指标
  const clearAllIndicators = useCallback(() => {
    try {
      chartInstanceRef.current.removeIndicator();
    } catch (error) {
      // 清除指标失败，忽略错误
    }
  }, []);

  // 添加选中的指标到图表
  const addSelectedIndicators = useCallback(() => {
    selectedIndicators.forEach((indicatorKey) => {
      try {
        // 根据指标键获取指标信息
        const indicatorInfo = getIndicatorInfo(indicatorKey);
        if (!indicatorInfo) {
          return;
        }
        
        // 判断指标类型
        const isOverlay = indicatorInfo.categoryInfo?.type === 'overlay';
        
        if (isOverlay) {
          // 叠加指标：添加到主图 - 使用正确的API参数
          chartInstanceRef.current.createIndicator(
            indicatorKey,
            true,  // isStack = true 表示叠加
            { id: 'candle_pane' }  // 指定主图面板ID
          );
        } else {
          // 振荡指标：创建新面板
          chartInstanceRef.current.createIndicator(
            indicatorKey,
            false  // isStack = false 创建新面板
          );
        }
      } catch (error) {
        message.error(`添加指标失败: ${error.message}`);
      }
    });
  }, [selectedIndicators]);

  // 更新图表数据
  useEffect(() => {
    // 如果有数据但图表还没初始化，尝试重新初始化图表
    if (klineData.length > 0 && !chartInitialized) {
      if (chartRef.current && !chartInstanceRef.current) {
        setTimeout(() => {
          if (chartRef.current && !chartInstanceRef.current) {
                          try {
                chartInstanceRef.current = init(chartRef.current);
                if (chartInstanceRef.current) {
                  // 设置基本样式，包括成交量显示
                  chartInstanceRef.current.setStyles({
                    candle: { 
                      margin: { top: 0.05, bottom: 0.05 }
                    },
                    grid: { 
                      show: true, 
                      horizontal: { show: true }, 
                      vertical: { show: true } 
                    }
                  });

                  // 创建成交量指标，确保默认显示
                  try {
                    chartInstanceRef.current.createIndicator('VOL', false);
                  } catch (volError) {
                    // 创建成交量指标失败，忽略错误
                  }

                  setChartInitialized(true);
                }
              } catch (error) {
                // 主动初始化图表失败，忽略错误
              }
          }
        }, 100);
      }
      return;
    }
    
    if (chartInitialized && chartInstanceRef.current && klineData.length > 0) {
      const formattedData = klineData.map(item => ({
        timestamp: parseInt(item.timestamp),
        open: parseFloat(item.open || 0),
        high: parseFloat(item.high || 0),
        low: parseFloat(item.low || 0),
        close: parseFloat(item.close || 0),
        volume: parseFloat(item.volume || 0),
      }));
      
      try {
        chartInstanceRef.current.applyNewData(formattedData);
        
        // 调整面板高度，确保成交量可见 - 增加延迟确保图表完全初始化
        setTimeout(() => {
          adjustPaneHeights();
        }, 300);
      } catch (error) {
        console.error('更新图表数据失败:', error);
        message.error('更新图表数据失败');
      }
    }
  }, [chartInitialized, klineData]); // chartInitialized和klineData变化时更新图表数据

  // 应用所有指标实例
  const applyAllIndicators = useCallback(() => {
    if (indicatorApplying.current) {
      return;
    }
    
    indicatorApplying.current = true;
    
    try {
      // 先清除所有指标
      clearAllIndicators();
      
      // 延迟添加新指标
      setTimeout(() => {
        try {
          addSelectedIndicators();
        } finally {
          indicatorApplying.current = false;
        }
      }, 150);
    } catch (error) {
      console.error('应用指标时出错:', error);
      indicatorApplying.current = false;
    }
  }, [clearAllIndicators, addSelectedIndicators]);

  // 监听指标选择变化，单独应用指标
  useEffect(() => {
    if (chartInitialized && chartInstanceRef.current) {
      // 延迟应用指标，确保图表准备好
      const timer = setTimeout(() => {
        if (chartInstanceRef.current) {
          applyAllIndicators();
          
          // 指标变化后重新调整面板高度，确保成交量始终可见
          setTimeout(() => {
            adjustPaneHeights();
            
            // 最终确保成交量可见
            setTimeout(() => {
              adjustPaneHeights();
            }, 200);
          }, 100);
        }
      }, 200);
      
      return () => clearTimeout(timer);
    }
  }, [selectedIndicators, chartInitialized, applyAllIndicators]);

  // 简化的面板高度设置 - 在固定容器内分配比例
  const adjustPaneHeights = () => {
    if (!chartInstanceRef.current) {
      return;
    }
    
    try {
      const isMobileDevice = window.innerWidth < 768;
      
      // 固定比例分配 
      const candleHeight = isMobileDevice ? 0.65 : 0.7;   // 主图65-70%
      const volumeHeight = isMobileDevice ? 0.35 : 0.3;   // 成交量30-35%
      
      // 直接设置比例，不设置minHeight避免冲突
      chartInstanceRef.current.setPaneOptions('candle_pane', { 
        height: candleHeight
      });
      
      // 延迟设置成交量面板
      setTimeout(() => {
        try {
          chartInstanceRef.current.setPaneOptions('volume_pane', { 
            height: volumeHeight
          });
        } catch (e) {
          console.error('❌ 设置成交量面板失败:', e);
        }
      }, 50);
      
    } catch (error) {
      console.error('❌ 面板高度调整失败:', error);
    }
  };



  // 成交量快捷切换 - 清空所有指标，只保留成交量
  const toggleVolume = useCallback(() => {
    // 不管当前什么配置，都只保留成交量指标
    const newSelectedIndicators = ['VOL'];
    setSelectedIndicators(newSelectedIndicators);
    
    // 立即调整面板高度，确保成交量可见
    setTimeout(() => {
      if (chartInitialized && chartInstanceRef.current) {
        adjustPaneHeights();
      }
    }, 200);
  }, [chartInitialized]);



  // 键盘快捷键
  useEffect(() => {
    const handleKeyPress = (e) => {
      if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') return;
      
      switch (e.key.toLowerCase()) {
        case 'i':
          if (!e.ctrlKey && !e.metaKey) {
            e.preventDefault();
            setIndicatorPanelVisible(true);
          }
          break;
        case 'escape':
          if (indicatorPanelVisible) {
            setIndicatorPanelVisible(false);
          }
          break;
        default:
          break;
      }
    };

    document.addEventListener('keydown', handleKeyPress);
    return () => document.removeEventListener('keydown', handleKeyPress);
  }, [indicatorPanelVisible]);

  // 组件初始化
  useEffect(() => {
    fetchSelectedCoins();
  }, [fetchSelectedCoins]);

  // 当选择的币种变化时，获取对应的价格监听数据和仓位数据
  useEffect(() => {
    if (selectedCoin) {
      fetchCurrentSymbolEstimates();
      fetchCurrentSymbolPositions();
    }
  }, [selectedCoin, fetchCurrentSymbolEstimates, fetchCurrentSymbolPositions]);

  // 当仓位数据变化时，更新当前币种的仓位
  useEffect(() => {
    if (selectedCoin && positions.length >= 0) {
      fetchCurrentSymbolPositions();
    }
  }, [selectedCoin, positions, fetchCurrentSymbolPositions]);

  // 当监听数据或仓位数据变化时，重新绘制覆盖层
  useEffect(() => {
    if (chartInitialized && chartInstanceRef.current) {
      // 延迟一下绘制，确保图表数据已更新
      setTimeout(() => {
        drawAllOverlays();
      }, 200);
    }
  }, [chartInitialized, currentSymbolEstimates.length, currentSymbolPositions.length, klineData.length, drawAllOverlays]);



  // 初始加载K线数据
  const initialLoadRef = useRef({ coin: null, interval: null });
  useEffect(() => {
    if (coins.length > 0 && selectedCoin && selectedInterval) {
      // 检查是否是初始加载或者币种变化，而不是周期变化
      const isInitialLoad = !initialLoadRef.current.coin;
      const isCoinChange = initialLoadRef.current.coin !== selectedCoin;
      
      // 只在初始加载或币种变化时加载数据，周期变化已在handleIntervalChange中处理
      if (isInitialLoad || isCoinChange) {
        const loadData = async () => {
          if (!selectedCoin || !selectedInterval) {
            return;
          }
          
          setLoading(true);
          try {
            const response = await api.get('/klines', {
              params: {
                symbol: selectedCoin,
                interval: selectedInterval,
                limit: 1000
              }
            });
            
            const data = response.data.data || [];
            
            // 检查组件是否已卸载
            if (!isMountedRef.current) return;
            
            setKlineData(data);
            
            if (data.length === 0) {
              message.warning('暂无K线数据');
            }
          } catch (error) {
            console.error('加载K线数据失败:', error);
            if (isMountedRef.current) {
              message.error(`加载K线数据失败: ${error.response?.data?.error || error.message}`);
              setKlineData([]); // 清空数据
            }
          }
          setLoading(false);
        };

        loadData();
      }
      
      // 更新跟踪的当前值
      initialLoadRef.current = { coin: selectedCoin, interval: selectedInterval };
    }
  }, [coins, selectedCoin, selectedInterval]);

  return (
    <div style={{ 
      height: '100%',  // 适配父容器
      background: '#fafbfc',
      display: 'flex',
      flexDirection: 'column',
      overflow: 'hidden'
    }}>
      <div style={{ 
        flex: 1,
        overflow: 'hidden',
        background: '#fafbfc',
        display: 'flex',
        flexDirection: 'column',
        minHeight: 0,
        gap: isMobile ? '4px' : '0'
      }}>
                {/* 图表工具栏 */}
        <ChartToolbar
          selectedCoin={selectedCoin}
          coins={coins}
          onCoinChange={handleCoinChange}
          selectedInterval={selectedInterval}
          onIntervalChange={handleIntervalChange}
          onRefresh={handleRefresh}
          onOpenIndicatorPanel={() => setIndicatorPanelVisible(true)}
          selectedIndicators={selectedIndicators}
          loading={loading}
          onToggleVolume={toggleVolume}
          priceEstimatesCount={currentSymbolEstimates.length}
          hasAnyEstimate={hasAnyEstimate(selectedCoin)}
          enabledEstimatesCount={currentSymbolEstimates.filter(e => e.enabled).length}
          disabledEstimatesCount={currentSymbolEstimates.filter(e => !e.enabled).length}
          currentSymbolEstimates={currentSymbolEstimates}
          onToggleEstimate={async (id, enabled) => {
            await toggleEstimate(id, enabled);
            // WebSocket会自动推送更新的数据
          }}
          positions={currentSymbolPositions}
          isMobile={isMobile}
          positionsLoading={positionsLoading}
        />
        
        {/* 图表容器 - 适配窗口高度 */}
        <div 
          className="kline-chart-container"
          style={{ 
            height: isMobile ? 'calc(100% - 0px)' : 'calc(100% - 40px)',
            position: 'relative', 
            background: '#ffffff',
            overflow: 'hidden',
            margin: isMobile ? '0 4px 4px 4px' : '0 8px 8px 8px',
            borderRadius: '8px',
            border: '1px solid #e5e7eb'
          }}
        >
          {loading && (
            <div style={{
              position: 'absolute',
              top: 0,
              left: 0,
              right: 0,
              bottom: 0,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              background: 'rgba(255, 255, 255, 0.8)',
              backdropFilter: 'blur(4px)',
              zIndex: 10
            }}>
              <Spin size="large" />
            </div>
          )}
          
          <div 
            ref={chartRef}
            style={{ 
              width: '100%', 
              height: '100%'
            }}
          />
          

        </div>
      </div>

      {/* 指标配置面板 */}
      <IndicatorPanel
        visible={indicatorPanelVisible}
        onClose={() => setIndicatorPanelVisible(false)}
        selectedIndicators={selectedIndicators}
        onSelectedIndicatorsChange={setSelectedIndicators}
        selectedCoin={selectedCoin}
        selectedInterval={selectedInterval}
      />
      

    </div>
  );
};

export default ChartPage;
