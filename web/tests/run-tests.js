#!/usr/bin/env node
// Node runner for smart-table pure-function tests — exit 0 on success.
// Usage: node web/tests/run-tests.js

const fs = require('fs');
const path = require('path');

global.window = global;
global.document = {
    getElementById: () => null,
    querySelector: () => null,
    createElement: () => ({
        classList: { add() {}, remove() {}, toggle() {} },
        appendChild() {}, style: {}, setAttribute() {}, addEventListener() {}
    }),
    body: { appendChild() {} }
};
global.Utils = { escapeHtml: s => String(s), toast() {}, reconcileChildren() {} };

const repoRoot = path.resolve(__dirname, '..');
const src = fs.readFileSync(path.join(repoRoot, 'js/components/smart-table.js'), 'utf8');
(new Function(src + '; global.SmartTableComponent = SmartTableComponent;'))();

const results = [];
function test(name, fn) {
    try { fn(); results.push({ name, ok: true }); }
    catch (e) { results.push({ name, ok: false, err: e.message }); }
}
function assertEq(a, e, label) {
    const A = JSON.stringify(a), E = JSON.stringify(e);
    if (A !== E) throw new Error(`${label}: got ${A}, expected ${E}`);
}

const S = SmartTableComponent;

test('_interpolate row.key', () => assertEq(S._interpolate('Delete {row.name}?', {name:'tank'}), 'Delete tank?', 'a'));
test('_interpolate bare {key}', () => assertEq(S._interpolate('{count} items', {count:7}), '7 items', 'b'));
test('_interpolate missing bare stays literal', () => assertEq(S._interpolate('{x}', {}), '{x}', 'c'));
test('_interpolate empty template', () => assertEq(S._interpolate('', {x:1}), '', 'c2'));

test('_sortRows asc', () => assertEq(S._sortRows([{a:3},{a:1},{a:2}], {key:'a',dir:'asc'}).map(r=>r.a), [1,2,3], 'd'));
test('_sortRows desc', () => assertEq(S._sortRows([{a:1},{a:3},{a:2}], {key:'a',dir:'desc'}).map(r=>r.a), [3,2,1], 'd2'));
test('_sortRows case-insensitive strings', () => assertEq(S._sortRows([{a:'banana'},{a:'Apple'}], {key:'a',dir:'asc'}).map(r=>r.a), ['Apple','banana'], 'd3'));
test('_sortRows null sort returns same ref', () => { const r=[{a:1}]; if (S._sortRows(r,null) !== r) throw new Error('not same ref'); });

test('_resolveAgentId row wins', () => { global.ManifestRenderer = undefined; assertEq(S._resolveAgentId({agent_id:42}), '42', 'f'); });
test('_resolveAgentId no-agent empty', () => { global.ManifestRenderer = undefined; assertEq(S._resolveAgentId({}), '', 'g'); });
test('_resolveAgentId selector fallback', () => { global.ManifestRenderer = {getSelectedAgentId: () => 'p-7'}; assertEq(S._resolveAgentId({}), 'p-7', 'h'); global.ManifestRenderer = undefined; });

test('_appendAgentIdToPath first query', () => assertEq(S._appendAgentIdToPath('/a', {agent_id:3}), '/a?agent_id=3', 'i'));
test('_appendAgentIdToPath second query', () => assertEq(S._appendAgentIdToPath('/a?x=1', {agent_id:3}), '/a?x=1&agent_id=3', 'j'));
test('_appendAgentIdToPath no agent returns original', () => { global.ManifestRenderer = undefined; assertEq(S._appendAgentIdToPath('/a', null), '/a', 'j2'); });

test('_setNestedValue flat', () => { const o={}; S._setNestedValue(o,'k','v'); assertEq(o,{k:'v'},'k1'); });
test('_setNestedValue nested', () => { const o={}; S._setNestedValue(o,'a.b.c',7); assertEq(o,{a:{b:{c:7}}},'k2'); });
test('_setNestedValue preserves siblings', () => { const o={a:{other:1}}; S._setNestedValue(o,'a.b',2); assertEq(o,{a:{other:1,b:2}},'k3'); });

test('_formatBytes 0', () => assertEq(S._formatBytes(0), '0 B', 'l'));
test('_formatBytes 1024', () => assertEq(S._formatBytes(1024), '1.0 KB', 'm'));
test('_formatBytes null', () => assertEq(S._formatBytes(null), '0 B', 'n'));

const pass = results.filter(r => r.ok).length;
const fail = results.length - pass;
for (const r of results) console.log((r.ok ? 'PASS ' : 'FAIL ') + r.name + (r.err ? '  — ' + r.err : ''));
console.log(`\n${pass} passed, ${fail} failed`);
process.exit(fail === 0 ? 0 : 1);
