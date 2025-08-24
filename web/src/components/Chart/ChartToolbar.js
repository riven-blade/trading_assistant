import React from 'react';
import { 
  Select,
  Badge,
  Dropdown,
  Typography,
  Switch
} from 'antd';
import {
  BarChartOutlined,
  LineChartOutlined,
  ReloadOutlined,
  FullscreenOutlined,
  FullscreenExitOutlined,
  SettingOutlined,
  ApartmentOutlined
} from '@ant-design/icons';

const { Option } = Select;

// 时间周期配置
const TIME_INTERVALS = [
  { value: '1m', label: '1m' },
  { value: '5m', label: '5m' },
  { value: '15m', label: '15m' },
  { value: '30m', label: '30m' },
  { value: '1h', label: '1h' },
  { value: '2h', label: '2h' },
  { value: '4h', label: '4h' },
  { value: '8h', label: '8h' },
  { value: '12h', label: '12h' },
  { value: '1d', label: '1d' },
  { value: '1w', label: '1w' }
];

const { Text } = Typography;

const ChartToolbar = ({
  selectedCoin,
  coins = [],
  onCoinChange,
  selectedInterval,
  onIntervalChange,
  onRefresh,
  onOpenIndicatorPanel,
  selectedIndicators = [],
  loading = false,
  isFullscreen = false,
  onToggleFullscreen,
  onToggleVolume,
  priceEstimatesCount = 0,
  hasAnyEstimate = false,
  enabledEstimatesCount = 0,
  disabledEstimatesCount = 0,
  currentSymbolEstimates = [],
  onToggleEstimate,
  positions = [],
  positionsLoading = false,
  isMobile = false
}) => {
  
  // 检查是否只选择了成交量指标
  const isOnlyVolumeSelected = selectedIndicators.length === 1 && selectedIndicators.includes('VOL');
  
  // 获取操作类型文字和颜色
  const getEstimateDisplayInfo = (estimate) => {
    const { action_type, created_by } = estimate;
    
    if (action_type === 'close') {
      if (created_by === 'take_profit') {
        return { text: '止盈', color: 'blue' };
      } else if (created_by === 'stop_loss') {
        return { text: '止损', color: 'red' };
      } else {
        return { text: '平仓', color: 'orange' };
      }
    }
    
    const actionMap = {
      'take_profit': { text: '止盈', color: 'blue' },
      'stop_loss': { text: '止损', color: 'red' },
      'open': { text: '开仓', color: 'orange' },
      'add': { text: '加仓', color: 'orange' }
    };
    
    return actionMap[action_type] || { text: action_type, color: 'orange' };
  };
  
  return (
    <div style={{
      display: 'flex',
      flexDirection: isMobile ? 'column' : 'row',
      alignItems: isMobile ? 'stretch' : 'center',
      justifyContent: isMobile ? 'flex-start' : 'space-between',
      padding: isMobile ? '6px 8px' : '10px 20px',
      background: 'transparent',
      minHeight: isMobile ? 'auto' : '44px',
              gap: isMobile ? '6px' : '0',
      border: 'none',
      outline: 'none',
      boxSizing: 'border-box'
    }}>
      {/* 上部或左侧：币种和周期 */}
      <div style={{ 
        display: 'flex', 
        alignItems: 'center', 
        gap: isMobile ? '8px' : '16px',
        flexWrap: isMobile ? 'wrap' : 'nowrap',
        border: 'none',
        outline: 'none',
        minHeight: isMobile ? '32px' : '40px'
      }}>
        {/* 币种选择 */}
        <Select
          value={selectedCoin}
          onChange={onCoinChange}
          style={{ 
            width: isMobile ? 120 : 130,
            fontSize: isMobile ? '13px' : '14px',
            height: isMobile ? '32px' : '36px',
            display: 'flex',
            alignItems: 'center'
          }}
          size={isMobile ? "small" : "middle"}
          variant="borderless"
          showSearch
          placeholder="币种"
          optionFilterProp="children"
          filterOption={(input, option) =>
            (option?.children ?? '').toLowerCase().includes(input.toLowerCase())
          }
        >
          {coins.map(coin => (
            <Option key={coin.symbol} value={coin.symbol}>
              {coin.symbol}
            </Option>
          ))}
        </Select>

        {/* 价格监听标识 */}
        {hasAnyEstimate && (
          <div style={{ 
            display: 'flex', 
            alignItems: 'center', 
            gap: isMobile ? '4px' : '8px',
            height: isMobile ? '32px' : '36px'
          }}>
            <div style={{ 
              display: 'flex', 
              alignItems: 'center', 
              gap: isMobile ? '3px' : '6px',
              padding: isMobile ? '2px 6px' : '2px 8px',
              background: '#f6ffed',
              border: '1px solid #b7eb8f',
              borderRadius: '4px',
              fontSize: isMobile ? '11px' : '12px',
              color: '#52c41a'
            }}>
              <span style={{ fontSize: isMobile ? '8px' : '9px' }}>📈</span>
              <span style={{ 
                fontSize: isMobile ? '11px' : '12px',
                fontWeight: '500'
              }}>
                {isMobile ? priceEstimatesCount : `${priceEstimatesCount}个监听`}
              </span>
              {enabledEstimatesCount > 0 && (
                <span style={{ 
                  background: '#52c41a', 
                  color: 'white', 
                  padding: isMobile ? '2px 4px' : '1px 3px', 
                  borderRadius: '2px',
                  fontSize: isMobile ? '10px' : '10px',
                  fontWeight: '500'
                }}>
                  {enabledEstimatesCount}{isMobile ? '' : '启用'}
                </span>
              )}
              {disabledEstimatesCount > 0 && !isMobile && (
                <span style={{ 
                  background: '#d9d9d9', 
                  color: '#666', 
                  padding: '1px 4px', 
                  borderRadius: '2px',
                  fontSize: '10px',
                  fontWeight: '500'
                }}>
                  {disabledEstimatesCount}待启用
                </span>
              )}
            </div>
            
            {/* 价格监听管理下拉菜单 */}
            {currentSymbolEstimates.length > 0 && (
              <Dropdown
                menu={{
                  style: { 
                    maxHeight: isMobile ? '280px' : '300px', 
                    overflowY: 'auto',
                    width: isMobile ? 'calc(100vw - 24px)' : '350px',
                    maxWidth: isMobile ? '380px' : '350px',
                    padding: isMobile ? '12px 0' : '8px 0'
                  },
                  items: [
                    {
                      key: 'header',
                      disabled: true,
                      label: (
                        <Text strong style={{ color: '#1890ff', fontSize: isMobile ? '12px' : '14px' }}>
                          {selectedCoin} {isMobile ? '监听' : '价格监听管理'}
                        </Text>
                      )
                    },
                    {
                      type: 'divider'
                    },
                    ...currentSymbolEstimates.map((estimate) => ({
                      key: estimate.id,
                      style: { 
                        padding: isMobile ? '10px 12px' : '8px 12px',
                        height: 'auto',
                        lineHeight: 'normal'
                      },
                      onClick: () => {
                        // 空函数，阻止默认行为
                      },
                      label: (
                        <div 
                          style={{ 
                            display: 'flex', 
                            justifyContent: 'space-between', 
                            alignItems: 'center',
                            gap: isMobile ? '10px' : '12px'
                          }}
                          onClick={(e) => {
                            e.stopPropagation();
                          }}
                        >
                          <div style={{ flex: 1 }}>
                            <div style={{ 
                              display: 'flex', 
                              alignItems: 'center', 
                              gap: isMobile ? '8px' : '6px',
                              marginBottom: isMobile ? '4px' : '2px'
                            }}>
                              <Badge 
                                status={estimate.enabled ? 'success' : 'default'} 
                              />
                              <Text strong style={{ fontSize: isMobile ? '13px' : '12px' }}>
                                ${estimate.target_price.toFixed(4)}
                              </Text>
                              <Badge 
                                color={estimate.side === 'long' ? 'green' : 'red'}
                                text={estimate.side === 'long' ? '多' : '空'}
                              />
                              <Badge 
                                color={getEstimateDisplayInfo(estimate).color}
                                text={getEstimateDisplayInfo(estimate).text}
                              />
                            </div>
                            <Text type="secondary" style={{ fontSize: isMobile ? '11px' : '10px' }}>
                              {isMobile ? 
                                new Date(estimate.created_at).toLocaleDateString() : 
                                `创建于 ${new Date(estimate.created_at).toLocaleString()}`
                              }
                            </Text>
                          </div>
                          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: isMobile ? '2px' : '4px' }}>
                            <Switch
                              size="small"
                              checked={estimate.enabled}
                              onChange={(checked) => {
                                if (onToggleEstimate && estimate.id) {
                                  onToggleEstimate(estimate.id, checked);
                                }
                              }}
                              onClick={(e) => {
                                if (e && e.stopPropagation) {
                                  e.stopPropagation();
                                }
                              }}
                              checkedChildren={isMobile ? "" : "启用"}
                              unCheckedChildren={isMobile ? "" : "关闭"}
                            />
                            {!isMobile && (
                              <Text type="secondary" style={{ fontSize: '9px' }}>
                                {estimate.enabled ? '已启用' : '未启用'}
                              </Text>
                            )}
                          </div>
                        </div>
                      )
                    }))
                  ]
                }}
                trigger={['click']}
                placement={isMobile ? "bottom" : "bottomRight"}
              >
                <button
                  style={{
                    border: 'none',
                    background: 'rgba(24, 144, 255, 0.1)',
                    color: '#1890ff',
                    borderRadius: '50%',
                    width: isMobile ? '32px' : '24px',
                    height: isMobile ? '32px' : '24px',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    cursor: 'pointer',
                    transition: 'all 0.2s ease',
                    fontSize: isMobile ? '14px' : '12px'
                  }}
                  onMouseEnter={(e) => {
                    e.target.style.background = 'rgba(24, 144, 255, 0.2)';
                  }}
                  onMouseLeave={(e) => {
                    e.target.style.background = 'rgba(24, 144, 255, 0.1)';
                  }}
                >
                  <SettingOutlined />
                </button>
              </Dropdown>
            )}
          </div>
        )}

        {/* 时间周期 */}
        <div style={{ 
          display: 'flex', 
          gap: isMobile ? '1px' : '2px', 
          flexWrap: 'wrap',
          width: isMobile ? '100%' : 'auto',
          alignItems: 'center',
          minHeight: isMobile ? '32px' : '36px'
        }}>
          {TIME_INTERVALS.map(interval => (
            <button
              key={interval.value}
              onClick={() => onIntervalChange(interval.value)}
              style={{
                padding: isMobile ? '6px 10px' : '4px 8px',
                fontSize: isMobile ? '13px' : '12px',
                border: 'none',
                borderRadius: '4px',
                backgroundColor: selectedInterval === interval.value ? '#1890ff' : 'transparent',
                color: selectedInterval === interval.value ? '#ffffff' : '#666',
                cursor: 'pointer',
                transition: 'all 0.15s',
                fontWeight: selectedInterval === interval.value ? '500' : '400',
                minWidth: isMobile ? '32px' : '30px',
                minHeight: isMobile ? '32px' : '24px',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center'
              }}
              onMouseEnter={(e) => {
                if (selectedInterval !== interval.value) {
                  e.target.style.backgroundColor = 'rgba(0,0,0,0.04)';
                }
              }}
              onMouseLeave={(e) => {
                if (selectedInterval !== interval.value) {
                  e.target.style.backgroundColor = 'transparent';
                }
              }}
            >
              {interval.label}
            </button>
          ))}
        </div>
      </div>

      {/* 下部或右侧：功能按钮 */}
      <div style={{ 
        display: 'flex', 
        alignItems: 'center', 
        gap: isMobile ? '6px' : '8px',
        flexWrap: isMobile ? 'wrap' : 'nowrap',
        justifyContent: isMobile ? 'space-between' : 'flex-end',
        minHeight: isMobile ? '32px' : '40px'
      }}>
        {/* 指标按钮 */}
        <button
          onClick={onOpenIndicatorPanel}
          style={{
            padding: isMobile ? '8px 12px' : '6px 10px',
            fontSize: isMobile ? '14px' : '13px',
            border: 'none',
            borderRadius: '4px',
            backgroundColor: selectedIndicators.length > 0 ? 'rgba(24, 144, 255, 0.1)' : 'transparent',
            color: selectedIndicators.length > 0 ? '#1890ff' : '#666',
            cursor: 'pointer',
            transition: 'all 0.15s',
            display: 'flex',
            alignItems: 'center',
            gap: isMobile ? '4px' : '4px',
            minHeight: isMobile ? '40px' : '32px',
            minWidth: isMobile ? '40px' : 'auto'
          }}
          onMouseEnter={(e) => {
            if (selectedIndicators.length === 0) {
              e.target.style.backgroundColor = 'rgba(0,0,0,0.04)';
            }
          }}
          onMouseLeave={(e) => {
            if (selectedIndicators.length === 0) {
              e.target.style.backgroundColor = 'transparent';
            }
          }}
        >
          <BarChartOutlined style={{ fontSize: isMobile ? '16px' : '13px' }} />
          {!isMobile && '指标'}
          {selectedIndicators.length > 0 && (
            <Badge 
              count={selectedIndicators.length} 
              size="small"
              style={{ 
                fontSize: '10px',
                height: '16px',
                minWidth: '16px',
                lineHeight: '16px'
              }}
            />
          )}
        </button>
        
        {/* 成交量快捷按钮 */}
        <button
          onClick={onToggleVolume}
          style={{
            padding: isMobile ? '8px 12px' : '6px 10px',
            fontSize: isMobile ? '14px' : '13px',
            border: 'none',
            borderRadius: '4px',
            backgroundColor: isOnlyVolumeSelected ? 'rgba(250, 140, 22, 0.1)' : 'transparent',
            color: isOnlyVolumeSelected ? '#fa8c16' : '#666',
            cursor: 'pointer',
            transition: 'all 0.15s',
            display: 'flex',
            alignItems: 'center',
            gap: isMobile ? '4px' : '4px',
            minHeight: isMobile ? '40px' : '32px',
            minWidth: isMobile ? '40px' : 'auto'
          }}
          onMouseEnter={(e) => {
            if (!isOnlyVolumeSelected) {
              e.target.style.backgroundColor = 'rgba(0,0,0,0.04)';
            }
          }}
          onMouseLeave={(e) => {
            if (!isOnlyVolumeSelected) {
              e.target.style.backgroundColor = 'transparent';
            }
          }}
        >
          <LineChartOutlined style={{ fontSize: isMobile ? '16px' : '13px' }} />
          {!isMobile && '成交量'}
        </button>

        {/* 仓位信息显示 */}
        {positions.length > 0 && (
          <div
            style={{
              padding: isMobile ? '8px 12px' : '6px 10px',
              fontSize: isMobile ? '14px' : '13px',
              border: 'none',
              borderRadius: '4px',
              backgroundColor: 'rgba(16, 185, 129, 0.1)',
              color: '#10b981',
              display: 'flex',
              alignItems: 'center',
              gap: isMobile ? '4px' : '4px',
              minHeight: isMobile ? '40px' : '32px',
              minWidth: isMobile ? '40px' : 'auto'
            }}
          >
            <ApartmentOutlined style={{ fontSize: isMobile ? '16px' : '13px' }} />
            {!isMobile && '持仓'}
            <Badge 
              count={positions.length} 
              size="small"
              style={{ 
                fontSize: isMobile ? '9px' : '10px',
                height: isMobile ? '14px' : '16px',
                minWidth: isMobile ? '14px' : '16px',
                lineHeight: isMobile ? '14px' : '16px'
              }}
            />
          </div>
        )}
        
        {/* 刷新按钮 */}
        <button
          onClick={onRefresh}
          disabled={loading}
          style={{
            padding: isMobile ? '8px' : '6px',
            border: 'none',
            borderRadius: '4px',
            backgroundColor: 'transparent',
            color: '#666',
            cursor: loading ? 'not-allowed' : 'pointer',
            transition: 'all 0.15s',
            minHeight: isMobile ? '40px' : '32px',
            minWidth: isMobile ? '40px' : '32px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center'
          }}
          onMouseEnter={(e) => {
            if (!loading) {
              e.target.style.backgroundColor = 'rgba(0,0,0,0.04)';
            }
          }}
          onMouseLeave={(e) => {
            if (!loading) {
              e.target.style.backgroundColor = 'transparent';
            }
          }}
        >
          <ReloadOutlined spin={loading} style={{ fontSize: isMobile ? '16px' : '13px' }} />
        </button>
        
        {/* 全屏按钮 */}
        <button
          onClick={onToggleFullscreen}
          style={{
            padding: isMobile ? '8px' : '6px',
            border: 'none',
            borderRadius: '4px',
            backgroundColor: isFullscreen ? 'rgba(24, 144, 255, 0.1)' : 'transparent',
            color: isFullscreen ? '#1890ff' : '#666',
            cursor: 'pointer',
            transition: 'all 0.15s',
            minHeight: isMobile ? '40px' : '32px',
            minWidth: isMobile ? '40px' : '32px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center'
          }}
          onMouseEnter={(e) => {
            if (!isFullscreen) {
              e.target.style.backgroundColor = 'rgba(0,0,0,0.04)';
            }
          }}
          onMouseLeave={(e) => {
            if (!isFullscreen) {
              e.target.style.backgroundColor = 'transparent';
            }
          }}
        >
          {isFullscreen ? 
            <FullscreenExitOutlined style={{ fontSize: isMobile ? '16px' : '13px' }} /> : 
            <FullscreenOutlined style={{ fontSize: isMobile ? '16px' : '13px' }} />
          }
        </button>
      </div>
    </div>
  );
};

export default ChartToolbar;