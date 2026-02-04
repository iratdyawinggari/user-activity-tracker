-- Main database schema
CREATE DATABASE IF NOT EXISTS activity_tracker;
USE activity_tracker;

-- Clients table
CREATE TABLE IF NOT EXISTS clients (
    id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    client_id VARCHAR(100) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    api_key VARCHAR(255) NOT NULL UNIQUE,
    ip_whitelist TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_api_key (api_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- API hits table with partitioning by date
CREATE TABLE IF NOT EXISTS api_logs (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    client_id VARCHAR(100) NOT NULL,
    endpoint VARCHAR(500) NOT NULL,
    ip_address VARCHAR(45) NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_client_timestamp (client_id, timestamp),
    INDEX idx_timestamp (timestamp),
    CONSTRAINT fk_logs_client
        FOREIGN KEY (client_id)
        REFERENCES clients(client_id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
PARTITION BY RANGE (UNIX_TIMESTAMP(timestamp)) (
    PARTITION p2024 VALUES LESS THAN (TO_DAYS('2025-01-01')),
    PARTITION p2025 VALUES LESS THAN (TO_DAYS('2026-01-01')),
    PARTITION p2026 VALUES LESS THAN (TO_DAYS('2027-01-01')),
    PARTITION p_future VALUES LESS THAN MAXVALUE
);

-- Daily usage aggregation table
CREATE TABLE IF NOT EXISTS daily_usage (
    id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    client_id VARCHAR(100) NOT NULL,
    date DATE NOT NULL,
    request_count BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY idx_client_date (client_id, date),
    INDEX idx_date (date),
    CONSTRAINT fk_usage_client
        FOREIGN KEY (client_id)
        REFERENCES clients(client_id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- JWT blacklist for logout functionality
CREATE TABLE IF NOT EXISTS jwt_blacklist (
    id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    token TEXT NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_expires_at (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Shard mapping table for horizontal partitioning
CREATE TABLE IF NOT EXISTS shard_mapping (
    id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    client_id VARCHAR(100) NOT NULL UNIQUE,
    shard_id INT UNSIGNED NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_shard_id (shard_id),
    CONSTRAINT fk_shard_client
        FOREIGN KEY (client_id)
        REFERENCES clients(client_id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Create read replica user
CREATE USER IF NOT EXISTS 'replica_user'@'%' IDENTIFIED BY 'replica_password';
GRANT SELECT ON activity_tracker.* TO 'replica_user'@'%';

-- Create stored procedure for daily aggregation
DELIMITER //
CREATE PROCEDURE AggregateDailyUsage()
BEGIN
    INSERT INTO daily_usage (client_id, date, request_count)
    SELECT 
        client_id,
        DATE(timestamp) as date,
        COUNT(*) as request_count
    FROM api_logs
    WHERE timestamp >= DATE_SUB(CURDATE(), INTERVAL 1 DAY)
    AND timestamp < CURDATE()
    GROUP BY client_id, DATE(timestamp)
    ON DUPLICATE KEY UPDATE 
        request_count = VALUES(request_count),
        updated_at = CURRENT_TIMESTAMP;
END //
DELIMITER ;

-- Create event scheduler for daily aggregation
SET GLOBAL event_scheduler = ON;
CREATE EVENT IF NOT EXISTS daily_aggregation
ON SCHEDULE EVERY 1 DAY
STARTS TIMESTAMP(CURRENT_DATE, '23:59:59')
DO
    CALL AggregateDailyUsage();