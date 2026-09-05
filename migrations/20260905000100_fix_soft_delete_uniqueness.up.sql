ALTER TABLE sys_dept
    DROP INDEX uk_sys_dept_code,
    ADD COLUMN active_code VARCHAR(50)
        GENERATED ALWAYS AS (CASE WHEN is_deleted = 0 THEN code ELSE NULL END) STORED,
    ADD UNIQUE KEY uk_sys_dept_active_code (active_code);

ALTER TABLE sys_role
    DROP INDEX uk_sys_role_name,
    DROP INDEX uk_sys_role_code,
    ADD COLUMN active_name VARCHAR(50)
        GENERATED ALWAYS AS (CASE WHEN is_deleted = 0 THEN name ELSE NULL END) STORED,
    ADD COLUMN active_code VARCHAR(50)
        GENERATED ALWAYS AS (CASE WHEN is_deleted = 0 THEN code ELSE NULL END) STORED,
    ADD UNIQUE KEY uk_sys_role_active_name (active_name),
    ADD UNIQUE KEY uk_sys_role_active_code (active_code);

ALTER TABLE sys_user
    DROP INDEX uk_sys_user_username,
    ADD COLUMN active_username VARCHAR(64)
        GENERATED ALWAYS AS (CASE WHEN is_deleted = 0 THEN username ELSE NULL END) STORED,
    ADD UNIQUE KEY uk_sys_user_active_username (active_username);

ALTER TABLE sys_dict
    DROP INDEX uk_sys_dict_code_deleted,
    ADD COLUMN active_dict_code VARCHAR(50)
        GENERATED ALWAYS AS (CASE WHEN is_deleted = 0 THEN dict_code ELSE NULL END) STORED,
    ADD UNIQUE KEY uk_sys_dict_active_code (active_dict_code);

ALTER TABLE sys_config
    DROP INDEX uk_sys_config_key_deleted,
    ADD COLUMN active_config_key VARCHAR(100)
        GENERATED ALWAYS AS (CASE WHEN is_deleted = 0 THEN config_key ELSE NULL END) STORED,
    ADD UNIQUE KEY uk_sys_config_active_key (active_config_key);
