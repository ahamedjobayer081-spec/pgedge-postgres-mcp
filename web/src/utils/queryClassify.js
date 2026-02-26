/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

const WRITE_PREFIXES = [
    'CREATE', 'DROP', 'ALTER', 'TRUNCATE',
    'INSERT', 'UPDATE', 'DELETE',
];

const READ_PREFIXES = [
    'SELECT', 'WITH', 'TABLE', 'VALUES',
    'EXPLAIN', 'SHOW',
];

/**
 * Classifies whether a SQL query is a write (DDL/DML) operation.
 * Read queries (SELECT, WITH, etc.) return false.
 * Write queries (CREATE, DROP, INSERT, etc.) return true.
 * Unknown query types are treated as potentially destructive.
 *
 * @param {string} sql - The SQL query to classify
 * @returns {boolean} - True if the query is a write operation
 */
export function isWriteQuery(sql) {
    if (!sql || typeof sql !== 'string') return false;
    const upper = sql.trim().toUpperCase();
    if (READ_PREFIXES.some(p => upper.startsWith(p))) return false;
    if (WRITE_PREFIXES.some(p => upper.startsWith(p))) return true;
    // Unknown query types are treated as potentially destructive
    return true;
}
