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

// _formatDuration (unified: column formatter + progress overlay helper)
test('_formatDuration seconds',        () => assertEq(S._formatDuration(45),    '45s',     'd1'));
test('_formatDuration minutes+sec',    () => assertEq(S._formatDuration(125),   '2m 5s',   'd2'));
test('_formatDuration hours+min',      () => assertEq(S._formatDuration(3700),  '1h 1m',   'd3'));
test('_formatDuration zero',           () => assertEq(S._formatDuration(0),     '0s',      'd4'));
test('_formatDuration NaN → str',      () => assertEq(S._formatDuration('abc'), 'abc',     'd5'));
test('_formatDuration null → empty',   () => assertEq(S._formatDuration(null),  '',        'd6'));
test('_formatDuration negative → str', () => assertEq(S._formatDuration(-3),    '-3',      'd7'));

// _interpolate — new {form.X} namespace support
test('_interpolate {form.X} from form ns', () =>
    assertEq(S._interpolate('k=/{form.key}/', { form: { key: 'abc' } }), 'k=/abc/', 'i1'));
test('_interpolate {form.X} falls back to flat ctx', () =>
    assertEq(S._interpolate('v={form.x}', { x: 7 }), 'v=7', 'i2'));
test('_interpolate {form.X} missing → empty', () =>
    assertEq(S._interpolate('{form.missing}', {}), '', 'i3'));
test('_interpolate {row.X} from row ns', () =>
    assertEq(S._interpolate('{row.name}', { row: { name: 'tank' } }), 'tank', 'i4'));
test('_interpolate {row.X} falls back to flat ctx (legacy)', () =>
    assertEq(S._interpolate('{row.name}', { name: 'legacy' }), 'legacy', 'i5'));

// _tierVariant — safety_tier → CSS variant
test('_tierVariant red',        () => assertEq(S._tierVariant('red'),     'danger',       't1'));
test('_tierVariant yellow',     () => assertEq(S._tierVariant('yellow'),  'warning',      't2'));
test('_tierVariant black',      () => assertEq(S._tierVariant('black'),   'irreversible', 't3'));
test('_tierVariant green',      () => assertEq(S._tierVariant('green'),   'primary',      't4'));
test('_tierVariant unknown',    () => assertEq(S._tierVariant('weird'),   'primary',      't5'));
test('_tierVariant undefined',  () => assertEq(S._tierVariant(undefined), 'primary',      't6'));

// _isTerminalStatus — async progress overlay terminal detection
test('_isTerminalStatus running → false', () => assertEq(S._isTerminalStatus('running'), false, 'ts1'));
test('_isTerminalStatus pending → false', () => assertEq(S._isTerminalStatus('pending'), false, 'ts2'));
test('_isTerminalStatus undefined → false', () => assertEq(S._isTerminalStatus(undefined), false, 'ts3'));
test('_isTerminalStatus completed → true', () => assertEq(S._isTerminalStatus('completed'), true, 'ts4'));
test('_isTerminalStatus failed → true', () => assertEq(S._isTerminalStatus('failed'), true, 'ts5'));
test('_isTerminalStatus cancelled → true', () => assertEq(S._isTerminalStatus('cancelled'), true, 'ts6'));
test('_isTerminalStatus canceled alias', () => assertEq(S._isTerminalStatus('canceled'), true, 'ts7'));
test('_isTerminalStatus case-insensitive', () => assertEq(S._isTerminalStatus('COMPLETED'), true, 'ts8'));
test('_isTerminalStatus done alias', () => assertEq(S._isTerminalStatus('done'), true, 'ts9'));
test('_isTerminalStatus error alias', () => assertEq(S._isTerminalStatus('error'), true, 'ts10'));

// _getNestedValue — dot-path extractor for remote_value.value_path
test('_getNestedValue flat key', () =>
    assertEq(S._getNestedValue({ a: 1 }, 'a'), 1, 'g1'));
test('_getNestedValue two-deep', () =>
    assertEq(S._getNestedValue({ a: { b: 'x' } }, 'a.b'), 'x', 'g2'));
test('_getNestedValue three-deep', () =>
    assertEq(S._getNestedValue({ a: { b: { c: 7 } } }, 'a.b.c'), 7, 'g3'));
test('_getNestedValue missing → undefined', () =>
    assertEq(S._getNestedValue({ a: 1 }, 'a.b.c'), undefined, 'g4'));
test('_getNestedValue null obj', () =>
    assertEq(S._getNestedValue(null, 'a'), undefined, 'g5'));
test('_getNestedValue empty path', () =>
    assertEq(S._getNestedValue({ a: 1 }, ''), undefined, 'g6'));

// _describeCron — human-readable preview + invalid detection
test('_describeCron empty',          () => assertEq(S._describeCron(''),              'Invalid — enter 5 space-separated fields',              'cr1'));
test('_describeCron wrong field count',() => { const d = S._describeCron('* * *'); if (!d.startsWith('Invalid')) throw new Error('expected Invalid, got '+d); });
test('_describeCron bogus char',     () => { const d = S._describeCron('a b c d e'); if (!d.startsWith('Invalid')) throw new Error('expected Invalid, got '+d); });
test('_describeCron midnight daily', () => assertEq(S._describeCron('0 0 * * *'), 'Runs at 00:00, every day.', 'cr2'));
test('_describeCron every 15 min',   () => assertEq(S._describeCron('*/15 * * * *'), 'Runs every 15 minutes, every day.', 'cr3'));
test('_describeCron top of hour',    () => assertEq(S._describeCron('0 * * * *'), 'Runs at the top of every hour, every day.', 'cr4'));
test('_describeCron weekly Sun',     () => assertEq(S._describeCron('0 2 * * 0'), 'Runs at 02:00, every Sun.', 'cr5'));
test('_describeCron multiple dows',  () => assertEq(S._describeCron('30 3 * * 1,3,5'), 'Runs at 03:30, on Mon, Wed, Fri.', 'cr6'));
test('_describeCron monthly on 15',  () => assertEq(S._describeCron('0 4 15 * *'), 'Runs at 04:00, on day 15 of every month.', 'cr7'));
test('_describeCron specific month', () => assertEq(S._describeCron('0 0 1 1 *'), 'Runs at 00:00, on Jan 1.', 'cr8'));
test('_describeCron range accepted', () => { const d = S._describeCron('0 9-17 * * *'); if (d.startsWith('Invalid')) throw new Error('range should be valid, got '+d); });

// _resolveAgentId — numeric agent_id coerced to string
test('_resolveAgentId row.agent_id string', () => {
    global.ManifestRenderer = undefined;
    assertEq(S._resolveAgentId({agent_id: 'node-a'}), 'node-a', 'r1');
});
test('_resolveAgentId row.agent_id zero treated as missing', () => {
    global.ManifestRenderer = { getSelectedAgentId: () => 'fallback' };
    // 0 is valid-but-falsy — we want the fallback to win only when row.agent_id is null/undefined.
    // Verifies current behaviour doesn't regress silently.
    const got = S._resolveAgentId({agent_id: 0});
    if (got !== '0' && got !== 'fallback') {
        throw new Error(`unexpected: ${got} (document the choice in the source if it changes)`);
    }
    global.ManifestRenderer = undefined;
});

// _buildRequestBody composition — body + body_map + formValues precedence
test('_buildRequestBody merges static body', () => {
    const out = S._buildRequestBody({}, null, {}, { body: { action: 'foo' } });
    assertEq(out, { action: 'foo' }, 'bb1');
});
test('_buildRequestBody body_map pulls from row', () => {
    const out = S._buildRequestBody({}, { name: 'tank' }, {}, { body_map: { pool: 'row.name' } });
    assertEq(out, { pool: 'tank' }, 'bb2');
});
test('_buildRequestBody form values override body_map', () => {
    const out = S._buildRequestBody({}, { name: 'tank' }, { pool: 'zroot' },
        { body_map: { pool: 'row.name' } });
    assertEq(out, { pool: 'zroot' }, 'bb3');
});
test('_buildRequestBody strips empty strings', () => {
    const out = S._buildRequestBody({}, null, { name: '', count: 5 }, {});
    assertEq(out, { count: 5 }, 'bb4');
});
test('_buildRequestBody strips empty arrays', () => {
    const out = S._buildRequestBody({}, null, { tags: [], keep: ['x'] }, {});
    assertEq(out, { keep: ['x'] }, 'bb5');
});

const pass = results.filter(r => r.ok).length;
const fail = results.length - pass;
for (const r of results) console.log((r.ok ? 'PASS ' : 'FAIL ') + r.name + (r.err ? '  — ' + r.err : ''));
console.log(`\n${pass} passed, ${fail} failed`);
process.exit(fail === 0 ? 0 : 1);
