CREATE TABLE sys_dept (
    id BIGINT NOT NULL AUTO_INCREMENT,
    name VARCHAR(100) NOT NULL,
    code VARCHAR(100) NOT NULL,
    parent_id BIGINT NOT NULL DEFAULT 0,
    tree_path VARCHAR(255) NOT NULL,
    sort SMALLINT NOT NULL DEFAULT 0,
    status TINYINT NOT NULL DEFAULT 1,
    create_by BIGINT NULL,
    create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    update_by BIGINT NULL,
    update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    is_deleted TINYINT NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    UNIQUE KEY uk_sys_dept_code (code),
    KEY idx_sys_dept_parent (parent_id, is_deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE sys_menu (
    id BIGINT NOT NULL AUTO_INCREMENT,
    parent_id BIGINT NOT NULL DEFAULT 0,
    tree_path VARCHAR(255) NOT NULL DEFAULT '0',
    name VARCHAR(64) NOT NULL,
    type CHAR(1) NOT NULL,
    route_name VARCHAR(255) NULL,
    route_path VARCHAR(128) NULL,
    component VARCHAR(128) NULL,
    external_url VARCHAR(512) NULL,
    perm VARCHAR(128) NULL,
    always_show TINYINT NOT NULL DEFAULT 0,
    keep_alive TINYINT NOT NULL DEFAULT 0,
    visible TINYINT NOT NULL DEFAULT 1,
    sort INT NOT NULL DEFAULT 0,
    icon VARCHAR(64) NULL,
    redirect VARCHAR(128) NULL,
    params JSON NULL,
    create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_sys_menu_parent (parent_id),
    KEY idx_sys_menu_perm (perm)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE sys_role (
    id BIGINT NOT NULL AUTO_INCREMENT,
    name VARCHAR(64) NOT NULL,
    code VARCHAR(32) NOT NULL,
    sort INT NOT NULL DEFAULT 0,
    status TINYINT NOT NULL DEFAULT 1,
    data_scope TINYINT NOT NULL DEFAULT 4,
    create_by BIGINT NULL,
    create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    update_by BIGINT NULL,
    update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    is_deleted TINYINT NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    UNIQUE KEY uk_sys_role_name (name),
    UNIQUE KEY uk_sys_role_code (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE sys_role_menu (
    role_id BIGINT NOT NULL,
    menu_id BIGINT NOT NULL,
    PRIMARY KEY (role_id, menu_id),
    KEY idx_sys_role_menu_menu (menu_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE sys_role_dept (
    role_id BIGINT NOT NULL,
    dept_id BIGINT NOT NULL,
    PRIMARY KEY (role_id, dept_id),
    KEY idx_sys_role_dept_dept (dept_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE sys_user (
    id BIGINT NOT NULL AUTO_INCREMENT,
    username VARCHAR(64) NOT NULL,
    nickname VARCHAR(64) NOT NULL,
    gender TINYINT NOT NULL DEFAULT 0,
    password VARCHAR(100) NOT NULL,
    dept_id BIGINT NULL,
    avatar VARCHAR(255) NULL,
    mobile VARCHAR(20) NULL,
    status TINYINT NOT NULL DEFAULT 1,
    email VARCHAR(128) NULL,
    create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    create_by BIGINT NULL,
    update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    update_by BIGINT NULL,
    is_deleted TINYINT NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    UNIQUE KEY uk_sys_user_username (username),
    KEY idx_sys_user_dept (dept_id, is_deleted),
    KEY idx_sys_user_mobile (mobile),
    KEY idx_sys_user_email (email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE sys_user_role (
    user_id BIGINT NOT NULL,
    role_id BIGINT NOT NULL,
    PRIMARY KEY (user_id, role_id),
    KEY idx_sys_user_role_role (role_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO sys_dept (id, name, code, parent_id, tree_path, sort, status, create_by, update_by)
VALUES (1, '默认组织', 'DEFAULT', 0, '0', 1, 1, 1, 1);

INSERT INTO sys_role (id, name, code, sort, status, data_scope, create_by, update_by)
VALUES
    (1, '超级管理员', 'ROOT', 1, 1, 1, 1, 1),
    (2, '系统管理员', 'ADMIN', 2, 1, 1, 1, 1);

INSERT INTO sys_menu (id, parent_id, tree_path, name, type, route_name, route_path, component, perm, always_show, keep_alive, visible, sort, icon, redirect)
VALUES
    (1, 0, '0', '系统管理', 'C', '', '/system', 'Layout', NULL, 1, 0, 1, 1, 'system', '/system/user'),
    (210, 1, '0,1', '用户管理', 'M', 'User', 'user', 'system/user/index', NULL, 0, 1, 1, 1, 'el-icon-User', NULL),
    (2101, 210, '0,1,210', '用户查询', 'B', NULL, '', NULL, 'sys:user:list', 0, 0, 1, 1, '', NULL),
    (2102, 210, '0,1,210', '用户新增', 'B', NULL, '', NULL, 'sys:user:create', 0, 0, 1, 2, '', NULL),
    (2103, 210, '0,1,210', '用户编辑', 'B', NULL, '', NULL, 'sys:user:update', 0, 0, 1, 3, '', NULL),
    (2104, 210, '0,1,210', '用户删除', 'B', NULL, '', NULL, 'sys:user:delete', 0, 0, 1, 4, '', NULL),
    (2105, 210, '0,1,210', '重置密码', 'B', NULL, '', NULL, 'sys:user:reset-password', 0, 0, 1, 5, '', NULL),
    (220, 1, '0,1', '角色管理', 'M', 'Role', 'role', 'system/role/index', NULL, 0, 1, 1, 2, 'role', NULL),
    (2201, 220, '0,1,220', '角色查询', 'B', NULL, '', NULL, 'sys:role:list', 0, 0, 1, 1, '', NULL),
    (2202, 220, '0,1,220', '角色新增', 'B', NULL, '', NULL, 'sys:role:create', 0, 0, 1, 2, '', NULL),
    (2203, 220, '0,1,220', '角色编辑', 'B', NULL, '', NULL, 'sys:role:update', 0, 0, 1, 3, '', NULL),
    (2204, 220, '0,1,220', '角色删除', 'B', NULL, '', NULL, 'sys:role:delete', 0, 0, 1, 4, '', NULL),
    (2205, 220, '0,1,220', '角色分配权限', 'B', NULL, '', NULL, 'sys:role:assign', 0, 0, 1, 5, '', NULL),
    (230, 1, '0,1', '菜单管理', 'M', 'SysMenu', 'menu', 'system/menu/index', NULL, 0, 1, 1, 3, 'menu', NULL),
    (2301, 230, '0,1,230', '菜单查询', 'B', NULL, '', NULL, 'sys:menu:list', 0, 0, 1, 1, '', NULL),
    (2302, 230, '0,1,230', '菜单新增', 'B', NULL, '', NULL, 'sys:menu:create', 0, 0, 1, 2, '', NULL),
    (2303, 230, '0,1,230', '菜单编辑', 'B', NULL, '', NULL, 'sys:menu:update', 0, 0, 1, 3, '', NULL),
    (2304, 230, '0,1,230', '菜单删除', 'B', NULL, '', NULL, 'sys:menu:delete', 0, 0, 1, 4, '', NULL),
    (240, 1, '0,1', '部门管理', 'M', 'Dept', 'dept', 'system/dept/index', NULL, 0, 1, 1, 4, 'tree', NULL),
    (2401, 240, '0,1,240', '部门查询', 'B', NULL, '', NULL, 'sys:dept:list', 0, 0, 1, 1, '', NULL),
    (2402, 240, '0,1,240', '部门新增', 'B', NULL, '', NULL, 'sys:dept:create', 0, 0, 1, 2, '', NULL),
    (2403, 240, '0,1,240', '部门编辑', 'B', NULL, '', NULL, 'sys:dept:update', 0, 0, 1, 3, '', NULL),
    (2404, 240, '0,1,240', '部门删除', 'B', NULL, '', NULL, 'sys:dept:delete', 0, 0, 1, 4, '', NULL);

INSERT INTO sys_role_menu (role_id, menu_id)
SELECT 2, id FROM sys_menu;
