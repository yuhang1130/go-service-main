CREATE TABLE sys_dict (
    id BIGINT NOT NULL AUTO_INCREMENT,
    dict_code VARCHAR(50) NOT NULL,
    name VARCHAR(50) NOT NULL,
    status TINYINT NOT NULL DEFAULT 1,
    remark VARCHAR(255) NOT NULL DEFAULT '',
    create_by BIGINT NULL,
    create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    update_by BIGINT NULL,
    update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    is_deleted TINYINT NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    UNIQUE KEY uk_sys_dict_code_deleted (dict_code, is_deleted),
    KEY idx_sys_dict_status (status, is_deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE sys_dict_item (
    id BIGINT NOT NULL AUTO_INCREMENT,
    dict_code VARCHAR(50) NOT NULL,
    value VARCHAR(50) NOT NULL,
    label VARCHAR(100) NOT NULL,
    tag_type CHAR(1) NOT NULL DEFAULT 'N',
    status TINYINT NOT NULL DEFAULT 1,
    sort INT NOT NULL DEFAULT 0,
    remark VARCHAR(255) NOT NULL DEFAULT '',
    create_by BIGINT NULL,
    create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    update_by BIGINT NULL,
    update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_sys_dict_item_code_value (dict_code, value),
    KEY idx_sys_dict_item_code_status_sort (dict_code, status, sort)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE sys_config (
    id BIGINT NOT NULL AUTO_INCREMENT,
    config_name VARCHAR(100) NOT NULL,
    config_key VARCHAR(100) NOT NULL,
    config_value TEXT NOT NULL,
    remark VARCHAR(500) NOT NULL DEFAULT '',
    create_by BIGINT NULL,
    create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    update_by BIGINT NULL,
    update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    is_deleted TINYINT NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    UNIQUE KEY uk_sys_config_key_deleted (config_key, is_deleted),
    KEY idx_sys_config_deleted (is_deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE sys_notice (
    id BIGINT NOT NULL AUTO_INCREMENT,
    title VARCHAR(200) NOT NULL,
    content TEXT NOT NULL,
    type TINYINT NOT NULL,
    level CHAR(1) NOT NULL DEFAULT 'L',
    target_type TINYINT NOT NULL DEFAULT 1,
    target_user_ids JSON NOT NULL,
    publisher_id BIGINT NULL,
    publish_status TINYINT NOT NULL DEFAULT 0,
    publish_time DATETIME(3) NULL,
    revoke_time DATETIME(3) NULL,
    create_by BIGINT NOT NULL,
    create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    update_by BIGINT NULL,
    update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    is_deleted TINYINT NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    KEY idx_sys_notice_status_time (publish_status, publish_time, is_deleted),
    KEY idx_sys_notice_type_time (type, create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE sys_user_notice (
    id BIGINT NOT NULL AUTO_INCREMENT,
    notice_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    is_read TINYINT NOT NULL DEFAULT 0,
    read_time DATETIME(3) NULL,
    create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_sys_user_notice_notice_user (notice_id, user_id),
    KEY idx_sys_user_notice_user_read (user_id, is_read)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE sys_log (
    id BIGINT NOT NULL AUTO_INCREMENT,
    module VARCHAR(50) NOT NULL,
    action_type VARCHAR(30) NOT NULL,
    title VARCHAR(100) NOT NULL,
    content TEXT NOT NULL,
    operator_id BIGINT NULL,
    operator_name VARCHAR(64) NOT NULL DEFAULT '',
    request_uri VARCHAR(255) NOT NULL,
    request_method VARCHAR(10) NOT NULL,
    ip VARCHAR(45) NOT NULL DEFAULT '',
    region VARCHAR(100) NOT NULL DEFAULT '',
    device VARCHAR(100) NOT NULL DEFAULT '',
    os VARCHAR(100) NOT NULL DEFAULT '',
    browser VARCHAR(100) NOT NULL DEFAULT '',
    status TINYINT NOT NULL DEFAULT 1,
    error_msg VARCHAR(255) NOT NULL DEFAULT '',
    execution_time BIGINT NOT NULL DEFAULT 0,
    create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_sys_log_module_action_time (module, action_type, create_time),
    KEY idx_sys_log_operator_time (operator_id, create_time),
    KEY idx_sys_log_time (create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO sys_dict (id, dict_code, name, status, remark, create_by, update_by)
VALUES
    (1, 'gender', '性别', 1, '', 1, 1),
    (2, 'notice_type', '通知类型', 1, '', 1, 1),
    (3, 'notice_level', '通知级别', 1, '', 1, 1);

INSERT INTO sys_dict_item (id, dict_code, value, label, tag_type, status, sort, remark, create_by, update_by)
VALUES
    (1, 'gender', '1', '男', 'P', 1, 1, '', 1, 1),
    (2, 'gender', '2', '女', 'D', 1, 2, '', 1, 1),
    (3, 'gender', '0', '保密', 'I', 1, 3, '', 1, 1),
    (4, 'notice_type', '1', '系统升级', 'S', 1, 1, '', 1, 1),
    (5, 'notice_type', '2', '系统维护', 'P', 1, 2, '', 1, 1),
    (6, 'notice_type', '3', '安全警告', 'D', 1, 3, '', 1, 1),
    (7, 'notice_type', '4', '假期通知', 'S', 1, 4, '', 1, 1),
    (8, 'notice_type', '5', '公司新闻', 'P', 1, 5, '', 1, 1),
    (9, 'notice_type', '99', '其他', 'I', 1, 99, '', 1, 1),
    (10, 'notice_level', 'L', '低', 'I', 1, 1, '', 1, 1),
    (11, 'notice_level', 'M', '中', 'W', 1, 2, '', 1, 1),
    (12, 'notice_level', 'H', '高', 'D', 1, 3, '', 1, 1);

INSERT INTO sys_menu (id, parent_id, tree_path, name, type, route_name, route_path, component, perm, always_show, keep_alive, visible, sort, icon, redirect)
VALUES
    (250, 1, '0,1', '字典管理', 'M', 'Dict', 'dict', 'system/dict/index', NULL, 0, 1, 1, 5, 'dict', NULL),
    (2501, 250, '0,1,250', '字典查询', 'B', NULL, '', NULL, 'sys:dict:list', 0, 0, 1, 1, '', NULL),
    (2502, 250, '0,1,250', '字典新增', 'B', NULL, '', NULL, 'sys:dict:create', 0, 0, 1, 2, '', NULL),
    (2503, 250, '0,1,250', '字典编辑', 'B', NULL, '', NULL, 'sys:dict:update', 0, 0, 1, 3, '', NULL),
    (2504, 250, '0,1,250', '字典删除', 'B', NULL, '', NULL, 'sys:dict:delete', 0, 0, 1, 4, '', NULL),
    (251, 1, '0,1', '字典项', 'M', 'DictItem', 'dict-item', 'system/dict/dict-item', NULL, 0, 1, 0, 6, '', NULL),
    (2511, 251, '0,1,251', '字典项查询', 'B', NULL, '', NULL, 'sys:dict-item:list', 0, 0, 1, 1, '', NULL),
    (2512, 251, '0,1,251', '字典项新增', 'B', NULL, '', NULL, 'sys:dict-item:create', 0, 0, 1, 2, '', NULL),
    (2513, 251, '0,1,251', '字典项编辑', 'B', NULL, '', NULL, 'sys:dict-item:update', 0, 0, 1, 3, '', NULL),
    (2514, 251, '0,1,251', '字典项删除', 'B', NULL, '', NULL, 'sys:dict-item:delete', 0, 0, 1, 4, '', NULL),
    (260, 1, '0,1', '系统日志', 'M', 'Log', 'log', 'system/log/index', NULL, 0, 1, 1, 7, 'document', NULL),
    (2601, 260, '0,1,260', '日志查询', 'B', NULL, '', NULL, 'sys:log:list', 0, 0, 1, 1, '', NULL),
    (270, 1, '0,1', '系统配置', 'M', 'Config', 'config', 'system/config/index', NULL, 0, 1, 1, 8, 'setting', NULL),
    (2701, 270, '0,1,270', '系统配置查询', 'B', NULL, '', NULL, 'sys:config:list', 0, 0, 1, 1, '', NULL),
    (2702, 270, '0,1,270', '系统配置新增', 'B', NULL, '', NULL, 'sys:config:create', 0, 0, 1, 2, '', NULL),
    (2703, 270, '0,1,270', '系统配置修改', 'B', NULL, '', NULL, 'sys:config:update', 0, 0, 1, 3, '', NULL),
    (2704, 270, '0,1,270', '系统配置删除', 'B', NULL, '', NULL, 'sys:config:delete', 0, 0, 1, 4, '', NULL),
    (280, 1, '0,1', '通知公告', 'M', 'Notice', 'notice', 'system/notice/index', NULL, 0, 1, 1, 9, 'bell', NULL),
    (2801, 280, '0,1,280', '通知查询', 'B', NULL, '', NULL, 'sys:notice:list', 0, 0, 1, 1, '', NULL),
    (2802, 280, '0,1,280', '通知新增', 'B', NULL, '', NULL, 'sys:notice:create', 0, 0, 1, 2, '', NULL),
    (2803, 280, '0,1,280', '通知编辑', 'B', NULL, '', NULL, 'sys:notice:update', 0, 0, 1, 3, '', NULL),
    (2804, 280, '0,1,280', '通知删除', 'B', NULL, '', NULL, 'sys:notice:delete', 0, 0, 1, 4, '', NULL),
    (2805, 280, '0,1,280', '通知发布撤回', 'B', NULL, '', NULL, 'sys:notice:publish', 0, 0, 1, 5, '', NULL),
    (2106, 210, '0,1,210', '用户导入', 'B', NULL, '', NULL, 'sys:user:import', 0, 0, 1, 6, '', NULL),
    (2107, 210, '0,1,210', '用户导出', 'B', NULL, '', NULL, 'sys:user:export', 0, 0, 1, 7, '', NULL),
    (2108, 210, '0,1,210', '文件上传', 'B', NULL, '', NULL, 'sys:file:create', 0, 0, 1, 8, '', NULL),
    (2109, 210, '0,1,210', '文件删除', 'B', NULL, '', NULL, 'sys:file:delete', 0, 0, 1, 9, '', NULL);

INSERT INTO sys_role_menu (role_id, menu_id)
SELECT 2, id FROM sys_menu WHERE id IN (250,2501,2502,2503,2504,251,2511,2512,2513,2514,260,2601,270,2701,2702,2703,2704,280,2801,2802,2803,2804,2805,2106,2107,2108,2109);
