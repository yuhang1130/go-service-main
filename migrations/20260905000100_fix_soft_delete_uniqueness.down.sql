-- This rollback restores the former uniqueness rules. Run it only before duplicate
-- deleted business keys have been created; otherwise preserve the data and roll
-- forward instead of deleting or renaming history to force this migration through.

ALTER TABLE sys_config
    DROP INDEX uk_sys_config_active_key,
    DROP COLUMN active_config_key,
    ADD UNIQUE KEY uk_sys_config_key_deleted (config_key, is_deleted);

ALTER TABLE sys_dict
    DROP INDEX uk_sys_dict_active_code,
    DROP COLUMN active_dict_code,
    ADD UNIQUE KEY uk_sys_dict_code_deleted (dict_code, is_deleted);

ALTER TABLE sys_user
    DROP INDEX uk_sys_user_active_username,
    DROP COLUMN active_username,
    ADD UNIQUE KEY uk_sys_user_username (username);

ALTER TABLE sys_role
    DROP INDEX uk_sys_role_active_name,
    DROP INDEX uk_sys_role_active_code,
    DROP COLUMN active_name,
    DROP COLUMN active_code,
    ADD UNIQUE KEY uk_sys_role_name (name),
    ADD UNIQUE KEY uk_sys_role_code (code);

ALTER TABLE sys_dept
    DROP INDEX uk_sys_dept_active_code,
    DROP COLUMN active_code,
    ADD UNIQUE KEY uk_sys_dept_code (code);
