-- DevOps Toolkit Database Schema
-- Initialize database for device management

CREATE DATABASE IF NOT EXISTS devops_toolkit;
USE devops_toolkit;

-- Devices table
CREATE TABLE IF NOT EXISTS devices (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    status ENUM('pending', 'active', 'inactive', 'error') DEFAULT 'pending',
    business_unit VARCHAR(100),
    compute_cluster VARCHAR(100),
    labels JSON,
    config JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    last_seen_at TIMESTAMP NULL,
    INDEX idx_type (type),
    INDEX idx_status (status),
    INDEX idx_business_unit (business_unit)
);

-- Device events audit log
CREATE TABLE IF NOT EXISTS device_events (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    device_id VARCHAR(36) NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    event_data JSON,
    ip_address VARCHAR(45),
    user_agent VARCHAR(500),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_device_id (device_id),
    INDEX idx_event_type (event_type),
    INDEX idx_created_at (created_at)
);

-- Device configurations history
CREATE TABLE IF NOT EXISTS device_config_history (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    device_id VARCHAR(36) NOT NULL,
    config JSON NOT NULL,
    version VARCHAR(10) NOT NULL,
    changed_by VARCHAR(100),
    changed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_device_id (device_id),
    INDEX idx_changed_at (changed_at)
);

-- Insert sample devices for testing
INSERT INTO devices (id, name, type, status, business_unit, labels) VALUES
    ('d1000001-0000-0000-0000-000000000001', 'server-web-01', 'container', 'active', 'frontend', '{"env":"development","device_type":"web","device_group":"frontend"}'),
    ('d1000001-0000-0000-0000-000000000002', 'server-web-02', 'container', 'active', 'frontend', '{"env":"development","device_type":"web","device_group":"frontend"}'),
    ('d1000001-0000-0000-0000-000000000003', 'server-app-01', 'container', 'active', 'backend', '{"env":"development","device_type":"app","device_group":"backend"}'),
    ('d1000001-0000-0000-0000-000000000004', 'loadbalancer-devops', 'network_device', 'active', 'infrastructure', '{"env":"development","device_type":"loadbalancer"}'),
    ('d1000001-0000-0000-0000-000000000005', 'devops-node-exporter', 'container', 'active', 'monitoring', '{"env":"development","device_type":"exporter"}');
