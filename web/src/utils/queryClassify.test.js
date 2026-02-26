/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Query Classifier Tests
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { describe, it, expect } from 'vitest';
import { isWriteQuery } from './queryClassify';

describe('isWriteQuery', () => {
    describe('read queries return false', () => {
        it('classifies SELECT as read', () => {
            expect(isWriteQuery('SELECT * FROM users')).toBe(false);
        });

        it('classifies WITH (CTE) as read', () => {
            expect(isWriteQuery('WITH cte AS (SELECT 1) SELECT * FROM cte')).toBe(false);
        });

        it('classifies TABLE as read', () => {
            expect(isWriteQuery('TABLE users')).toBe(false);
        });

        it('classifies VALUES as read', () => {
            expect(isWriteQuery('VALUES (1, 2, 3)')).toBe(false);
        });

        it('classifies EXPLAIN as read', () => {
            expect(isWriteQuery('EXPLAIN SELECT * FROM users')).toBe(false);
        });

        it('classifies SHOW as read', () => {
            expect(isWriteQuery('SHOW search_path')).toBe(false);
        });
    });

    describe('DDL write queries return true', () => {
        it('classifies CREATE as write', () => {
            expect(isWriteQuery('CREATE TABLE users (id int)')).toBe(true);
        });

        it('classifies DROP as write', () => {
            expect(isWriteQuery('DROP TABLE users')).toBe(true);
        });

        it('classifies ALTER as write', () => {
            expect(isWriteQuery('ALTER TABLE users ADD COLUMN name text')).toBe(true);
        });

        it('classifies TRUNCATE as write', () => {
            expect(isWriteQuery('TRUNCATE TABLE users')).toBe(true);
        });
    });

    describe('DML write queries return true', () => {
        it('classifies INSERT as write', () => {
            expect(isWriteQuery('INSERT INTO users (name) VALUES (\'test\')')).toBe(true);
        });

        it('classifies UPDATE as write', () => {
            expect(isWriteQuery('UPDATE users SET name = \'test\'')).toBe(true);
        });

        it('classifies DELETE as write', () => {
            expect(isWriteQuery('DELETE FROM users WHERE id = 1')).toBe(true);
        });
    });

    describe('case insensitivity', () => {
        it('handles lowercase SELECT', () => {
            expect(isWriteQuery('select * from users')).toBe(false);
        });

        it('handles mixed case SELECT', () => {
            expect(isWriteQuery('Select * From users')).toBe(false);
        });

        it('handles lowercase INSERT', () => {
            expect(isWriteQuery('insert into users values (1)')).toBe(true);
        });

        it('handles mixed case CREATE', () => {
            expect(isWriteQuery('Create Table test (id int)')).toBe(true);
        });
    });

    describe('leading whitespace', () => {
        it('handles leading spaces before SELECT', () => {
            expect(isWriteQuery('   SELECT * FROM users')).toBe(false);
        });

        it('handles leading tabs before INSERT', () => {
            expect(isWriteQuery('\tINSERT INTO users VALUES (1)')).toBe(true);
        });

        it('handles leading newlines before DELETE', () => {
            expect(isWriteQuery('\n  DELETE FROM users')).toBe(true);
        });
    });

    describe('unknown queries treated as write', () => {
        it('classifies GRANT as write', () => {
            expect(isWriteQuery('GRANT SELECT ON users TO role')).toBe(true);
        });

        it('classifies REVOKE as write', () => {
            expect(isWriteQuery('REVOKE ALL ON users FROM role')).toBe(true);
        });

        it('classifies VACUUM as write', () => {
            expect(isWriteQuery('VACUUM users')).toBe(true);
        });

        it('classifies REINDEX as write', () => {
            expect(isWriteQuery('REINDEX TABLE users')).toBe(true);
        });
    });

    describe('edge cases', () => {
        it('returns false for null', () => {
            expect(isWriteQuery(null)).toBe(false);
        });

        it('returns false for undefined', () => {
            expect(isWriteQuery(undefined)).toBe(false);
        });

        it('returns false for empty string', () => {
            expect(isWriteQuery('')).toBe(false);
        });

        it('returns false for non-string (number)', () => {
            expect(isWriteQuery(42)).toBe(false);
        });

        it('returns false for non-string (object)', () => {
            expect(isWriteQuery({})).toBe(false);
        });

        it('returns false for non-string (array)', () => {
            expect(isWriteQuery([])).toBe(false);
        });

        it('returns false for non-string (boolean)', () => {
            expect(isWriteQuery(true)).toBe(false);
        });
    });
});
