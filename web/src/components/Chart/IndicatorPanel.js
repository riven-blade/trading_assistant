import React, { useState, useMemo } from 'react';
import {
  Drawer, List, Card, Typography, Input, Switch, Space
} from 'antd';
import {
  SearchOutlined, LineChartOutlined, BarChartOutlined
} from '@ant-design/icons';
import { getAvailableIndicators } from '../../utils/indicatorUtils';
import '../../styles/indicator-panel.css';

const { Text } = Typography;
const { Search } = Input;

const IndicatorPanel = ({ 
  visible, 
  onClose, 
  selectedIndicators = [],
  onSelectedIndicatorsChange
}) => {
  const [searchText, setSearchText] = useState('');
  const [activeCategory, setActiveCategory] = useState('all');

  // 获取可用指标列表
  const availableIndicators = useMemo(() => getAvailableIndicators(), []);

  // 分类列表
  const categories = [
    { key: 'all', label: '全部', count: availableIndicators.length },
    { key: 'trend', label: '趋势', count: availableIndicators.filter(i => i.categoryInfo.category === 'trend').length },
    { key: 'momentum', label: '动量', count: availableIndicators.filter(i => i.categoryInfo.category === 'momentum').length },
    { key: 'volatility', label: '波动率', count: availableIndicators.filter(i => i.categoryInfo.category === 'volatility').length },
    { key: 'volume', label: '成交量', count: availableIndicators.filter(i => i.categoryInfo.category === 'volume').length }
  ];

  // 过滤指标
  const filteredIndicators = useMemo(() => {
    let filtered = availableIndicators;
    
    // 按分类过滤
    if (activeCategory !== 'all') {
      filtered = filtered.filter(indicator => 
        indicator.categoryInfo.category === activeCategory
      );
    }
    
    // 按搜索文本过滤
    if (searchText) {
      filtered = filtered.filter(indicator =>
        indicator.name.toLowerCase().includes(searchText.toLowerCase()) ||
        indicator.key.toLowerCase().includes(searchText.toLowerCase()) ||
        (indicator.description && indicator.description.toLowerCase().includes(searchText.toLowerCase()))
      );
    }
    
    return filtered;
  }, [availableIndicators, activeCategory, searchText]);

  // 切换指标状态
  const toggleIndicator = (indicatorKey) => {
    const newSelectedIndicators = selectedIndicators.includes(indicatorKey)
      ? selectedIndicators.filter(key => key !== indicatorKey)
      : [...selectedIndicators, indicatorKey];
    
    onSelectedIndicatorsChange(newSelectedIndicators);
  };

  // 获取指标图标
  const getIndicatorIcon = (type) => {
    return type === 'overlay' ? <LineChartOutlined /> : <BarChartOutlined />;
  };

  // 获取指标类型颜色
  const getIndicatorTypeColor = (type) => {
    return type === 'overlay' ? 'blue' : 'purple';
  };

  return (
    <Drawer
      title="技术指标"
      placement="right"
      onClose={onClose}
      open={visible}
      width={400}
      styles={{
        body: { padding: '16px', height: '100%', display: 'flex', flexDirection: 'column' }
      }}
    >
      <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
        {/* 搜索框 */}
        <Search
          placeholder="搜索指标..."
          prefix={<SearchOutlined />}
          value={searchText}
          onChange={(e) => setSearchText(e.target.value)}
          style={{ marginBottom: '16px' }}
          allowClear
        />

        {/* 分类标签 */}
        <div style={{ marginBottom: '16px' }}>
          <Space wrap>
            {categories.map(category => (
              <button
                key={category.key}
                onClick={() => setActiveCategory(category.key)}
                style={{
                  padding: '4px 12px',
                  fontSize: '12px',
                  border: '1px solid #d9d9d9',
                  borderRadius: '16px',
                  backgroundColor: activeCategory === category.key ? '#1890ff' : '#ffffff',
                  color: activeCategory === category.key ? '#ffffff' : '#666',
                  cursor: 'pointer',
                  transition: 'all 0.2s'
                }}
              >
                {category.label} ({category.count})
              </button>
            ))}
          </Space>
        </div>

        {/* 已选择指标统计 */}
        {selectedIndicators.length > 0 && (
          <Card 
            size="small" 
            style={{ marginBottom: '16px', backgroundColor: '#f0f7ff' }}
          >
            <Text style={{ fontSize: '13px', color: '#1890ff' }}>
              已选择 {selectedIndicators.length} 个指标
            </Text>
          </Card>
        )}

        {/* 指标列表 - 可滚动区域 */}
        <div style={{ 
          flex: 1, 
          overflow: 'auto',
          marginTop: '8px',
          paddingRight: '4px',
          maxHeight: 'calc(100vh - 300px)', // 确保在小屏幕上也能滚动
          scrollbarWidth: 'thin', // Firefox
          msOverflowStyle: 'auto', // IE
        }}
        className="custom-scrollbar"
        >
          <List
            dataSource={filteredIndicators}
            renderItem={(indicator) => (
            <List.Item style={{ padding: '8px 0', border: 'none' }}>
              <Card 
                size="small" 
                style={{ 
                  width: '100%',
                  border: selectedIndicators.includes(indicator.key) ? '1px solid #1890ff' : '1px solid #f0f0f0',
                  backgroundColor: selectedIndicators.includes(indicator.key) ? '#f0f7ff' : '#ffffff'
                }}
              >
                <div style={{ 
                  display: 'flex', 
                  justifyContent: 'space-between', 
                  alignItems: 'center' 
                }}>
                  <div style={{ flex: 1 }}>
                    <div style={{ 
                      display: 'flex', 
                      alignItems: 'center', 
                      gap: '8px', 
                      marginBottom: '4px' 
                    }}>
                      {getIndicatorIcon(indicator.categoryInfo?.type)}
                      <Text strong style={{ fontSize: '14px' }}>
                        {indicator.name}
                      </Text>
                      <span style={{
                        fontSize: '10px',
                        padding: '1px 6px',
                        borderRadius: '8px',
                        backgroundColor: getIndicatorTypeColor(indicator.categoryInfo?.type) === 'blue' ? '#e6f7ff' : '#f9f0ff',
                        color: getIndicatorTypeColor(indicator.categoryInfo?.type) === 'blue' ? '#1890ff' : '#722ed1'
                      }}>
                        {indicator.categoryInfo?.type === 'overlay' ? '主图' : '副图'}
                      </span>
                    </div>
                    
                    {indicator.description && (
                      <Text type="secondary" style={{ fontSize: '12px' }}>
                        {indicator.description}
                      </Text>
                    )}
                  </div>
                  
                  <Switch
                    size="small"
                    checked={selectedIndicators.includes(indicator.key)}
                    onChange={() => toggleIndicator(indicator.key)}
                  />
                </div>
              </Card>
            </List.Item>
          )}
          />

          {filteredIndicators.length === 0 && (
            <div style={{ 
              textAlign: 'center', 
              padding: '40px 20px',
              color: '#999'
            }}>
              <Text type="secondary">没有找到匹配的指标</Text>
            </div>
          )}
        </div>
      </div>
    </Drawer>
  );
};

export default IndicatorPanel;