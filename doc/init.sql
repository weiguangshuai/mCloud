CREATE TABLE users (
    id INT PRIMARY KEY AUTO_INCREMENT,
    username VARCHAR(50) UNIQUE NOT NULL,
    password VARCHAR(255) NOT NULL,  -- bcrypt 加密
    nickname VARCHAR(100),
    avatar VARCHAR(255),
    storage_quota BIGINT DEFAULT 10737418240 COMMENT '存储配额(字节)，默认10GB',
    storage_used BIGINT DEFAULT 0 COMMENT '已使用存储空间(字节)',
    deleted_at TIMESTAMP NULL DEFAULT NULL COMMENT '软删除时间',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE folders (
    id INT PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(255) NOT NULL,
    parent_id INT NULL,  -- NULL 表示根目录
    user_id INT NOT NULL,
    is_root TINYINT(1) NULL DEFAULT NULL, -- 仅根目录为1，其余为NULL
    path VARCHAR(1000) NOT NULL,  -- 以 / 开头，不以 / 结尾
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    INDEX idx_user_id (user_id),
    INDEX idx_parent_id (parent_id),
    UNIQUE KEY uk_user_root (user_id, is_root),
    UNIQUE KEY uk_sibling_name (user_id, parent_id, name, deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE files (
    id INT PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(255) NOT NULL,
    original_name VARCHAR(255) NOT NULL,
    folder_id INT NOT NULL,                -- 指向真实文件夹(root为用户root id)
    user_id INT NOT NULL,
    file_object_id INT NOT NULL,           -- 物理文件对象
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    deleted_by INT NULL,
    INDEX idx_user_id (user_id),
    INDEX idx_folder_id (folder_id),
    INDEX idx_file_object_id (file_object_id),
    INDEX idx_deleted_at (deleted_at),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE file_objects (
    id INT PRIMARY KEY AUTO_INCREMENT,
    file_path VARCHAR(1000) NOT NULL,      -- 实际文件存储路径
    thumbnail_path VARCHAR(1000),          -- 缩略图路径（仅图片）
    file_size BIGINT NOT NULL,
    mime_type VARCHAR(100),
    is_image TINYINT(1) DEFAULT 0,
    width INT,                             -- 图片宽度
    height INT,                            -- 图片高度
    file_md5 VARCHAR(32) NOT NULL,         -- 用于秒传与完整性验证
    ref_count INT DEFAULT 1,               -- 引用计数
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_md5 (file_md5),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE upload_tasks (
    id INT PRIMARY KEY AUTO_INCREMENT,
    upload_id VARCHAR(36) UNIQUE NOT NULL COMMENT 'UUID上传任务ID',
    user_id INT NOT NULL,
    folder_id INT NOT NULL COMMENT '目标文件夹ID(根目录为root id)',
    file_name VARCHAR(255) NOT NULL,
    file_size BIGINT NOT NULL,
    file_md5 VARCHAR(32) NOT NULL,
    total_chunks INT NOT NULL,
    uploaded_chunks TEXT COMMENT 'JSON数组，已上传分片索引',
    status ENUM('pending', 'uploading', 'completed', 'failed') DEFAULT 'pending',
    temp_dir VARCHAR(500) COMMENT '临时文件存储目录',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL COMMENT '过期时间，24小时后',
    INDEX idx_upload_id (upload_id),
    INDEX idx_user_id (user_id),
    INDEX idx_expires_at (expires_at),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE recycle_bin (
    id INT PRIMARY KEY AUTO_INCREMENT,
    user_id INT NOT NULL,
    original_id INT NOT NULL COMMENT '原文件或文件夹ID',
    original_type ENUM('file', 'folder') NOT NULL,
    original_name VARCHAR(255) NOT NULL,
    original_path VARCHAR(1000) COMMENT '删除前的完整路径',
    original_full_path VARCHAR(1000) COMMENT '删除前完整路径(用于冲突恢复判定)',
    original_folder_id INT COMMENT '删除前所在文件夹ID',
    file_object_id INT NULL COMMENT '物理文件对象ID(仅文件类型)',
    file_size BIGINT COMMENT '文件大小（仅文件类型）',
    deleted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '删除时间',
    expires_at TIMESTAMP NOT NULL COMMENT '回收站过期时间，默认30天',
    metadata JSON COMMENT '额外元数据（缩略图路径、MIME类型等）',
    INDEX idx_user_id (user_id),
    INDEX idx_deleted_at (deleted_at),
    INDEX idx_expires_at (expires_at),
    INDEX idx_original_type (original_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE thumbnail_tasks (
    id INT PRIMARY KEY AUTO_INCREMENT,
    file_id INT NOT NULL,
    status ENUM('pending', 'processing', 'completed', 'failed') DEFAULT 'pending',
    retry_count INT DEFAULT 0,
    max_retries INT DEFAULT 3,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    completed_at TIMESTAMP NULL,
    INDEX idx_file_id (file_id),
    INDEX idx_status (status),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 登录 MySQL
mysql -u root -p

-- 创建数据库
CREATE DATABASE mcloud CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- 创建用户（可选，建议使用独立用户）
CREATE USER 'mcloud'@'localhost' IDENTIFIED BY 'your_password';
GRANT ALL PRIVILEGES ON mcloud.* TO 'mcloud'@'localhost';
FLUSH PRIVILEGES;

-- 使用数据库
USE mcloud;
