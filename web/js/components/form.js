/**
 * Vigil Dashboard - Form Component (Task 4.3)
 *
 * Renders manifest form fields with full support for:
 *  - Field types: text, number, select, checkbox, hidden, range
 *  - visible_when: conditional visibility ("field=value" or "field!=value")
 *  - depends_on: cascading select — re-fetches options when parent changes
 *  - live_calculation: safe arithmetic formulas evaluated on every input
 *  - security_gate: multi-step flow — Step 1: Password → Step 2: Path Confirm → Step 3: OK
 */

const FormComponent = {
    _forms: {},  // keyed by componentId → { fields, addonId, action, gateStep }

    /**
     * @param {string} compId  - Manifest component ID
     * @param {Object} config  - { fields: FormField[], action?: string }
     * @param {number} addonId - Parent add-on ID
     * @returns {string} HTML
     */
    render(compId, config, addonId) {
        const fields = config.fields || [];
        if (fields.length === 0) {
            return '<p class="form-empty">No fields configured</p>';
        }

        this._forms[compId] = {
            fields, addonId,
            action: config.action || '',
            gateStep: 0  // 0=idle, 1=password, 2=path confirm, 3=done
        };

        const fieldHtml = fields.map(f => this._renderField(compId, f)).join('');

        // Kick off initial visibility + calc evaluation after DOM insertion
        setTimeout(() => {
            this._evaluateVisibility(compId);
            this._evaluateCalculations(compId);
        }, 0);

        return `
            <form class="addon-form" id="form-${compId}" onsubmit="FormComponent.submit(event, '${compId}')">
                ${fieldHtml}
                <div class="addon-form-actions">
                    <button type="submit" class="btn btn-primary">Submit</button>
                </div>
                <div class="addon-form-error" id="form-error-${compId}"></div>
            </form>
        `;
    },

    // ─── Field Rendering ──────────────────────────────────────────────────

    _renderField(compId, field) {
        const id = `field-${compId}-${field.name}`;
        const required = field.required ? 'required' : '';
        const hidden = field.visible_when ? 'style="display:none"' : '';
        const vw = field.visible_when ? `data-visible-when="${this._escape(field.visible_when)}"` : '';
        const dep = field.depends_on ? `data-depends-on="${this._escape(field.depends_on)}"` : '';
        const calc = field.live_calculation ? `data-calc="${this._escape(field.live_calculation)}"` : '';

        if (field.type === 'hidden') {
            return `<input type="hidden" id="${id}" name="${this._escape(field.name)}" value="">`;
        }

        const input = this._inputForType(id, field, required, compId);

        const calcDisplay = field.live_calculation
            ? `<span class="form-calc-result" id="calc-${id}"></span>`
            : '';

        return `
            <div class="form-group addon-form-group" id="fg-${id}" ${hidden} ${vw} ${dep} ${calc}>
                ${field.type !== 'checkbox' ? `<label for="${id}">${this._escape(field.label || field.name)}</label>` : ''}
                ${input}
                ${calcDisplay}
            </div>
        `;
    },

    _inputForType(id, field, required, compId) {
        const ev = `FormComponent._onInput('${compId}', '${this._escapeJS(field.name)}')`;
        const name = this._escape(field.name);

        switch (field.type) {
            case 'select':
                return this._selectInput(id, field, required, ev, name);
            case 'checkbox':
                return this._checkboxInput(id, field, ev, name);
            case 'range':
                return this._rangeInput(id, field, ev, name);
            case 'number':
                return `<input type="number" id="${id}" name="${name}" class="form-input" ${required}
                            ${field.min != null ? `min="${field.min}"` : ''}
                            ${field.max != null ? `max="${field.max}"` : ''}
                            ${field.step != null ? `step="${field.step}"` : ''}
                            oninput="${ev}">`;
            default: // text
                return `<input type="text" id="${id}" name="${name}" class="form-input" ${required}
                            oninput="${ev}">`;
        }
    },

    _selectInput(id, field, required, ev, name) {
        const options = (field.options || [])
            .map(o => `<option value="${this._escape(o.value)}">${this._escape(o.label)}</option>`)
            .join('');

        return `<select id="${id}" name="${name}" class="form-input" ${required}
                    onchange="${ev}">
                    <option value="">Select...</option>
                    ${options}
                </select>`;
    },

    _checkboxInput(id, field, ev, name) {
        return `<label class="addon-checkbox">
                    <input type="checkbox" id="${id}" name="${name}" onchange="${ev}">
                    ${this._escape(field.label || field.name)}
                </label>`;
    },

    _rangeInput(id, field, ev, name) {
        const min = field.min ?? 0;
        const max = field.max ?? 100;
        const step = field.step ?? 1;
        const unit = field.unit || '';
        const initial = field.default ?? min;

        return `<div class="form-range-wrap">
                    <input type="range" id="${id}" name="${name}" class="form-range"
                        min="${min}" max="${max}" step="${step}" value="${initial}"
                        oninput="${ev}; FormComponent._updateRangeDisplay('${id}', '${this._escapeJS(unit)}')">
                    <div class="form-range-labels">
                        <span>${min}${this._escape(unit)}</span>
                        <span class="form-range-value" id="rv-${id}">${initial}${this._escape(unit)}</span>
                        <span>${max}${this._escape(unit)}</span>
                    </div>
                </div>`;
    },

    _updateRangeDisplay(id, unit) {
        const input = document.getElementById(id);
        const display = document.getElementById(`rv-${id}`);
        if (input && display) {
            display.textContent = input.value + unit;
        }
    },

    // ─── Reactivity ───────────────────────────────────────────────────────

    _onInput(compId, fieldName) {
        this._evaluateVisibility(compId);
        this._evaluateCalculations(compId);
        this._evaluateDependsOn(compId, fieldName);
    },

    /** Show/hide fields based on visible_when expressions. */
    _evaluateVisibility(compId) {
        const form = document.getElementById(`form-${compId}`);
        if (!form) return;

        form.querySelectorAll('[data-visible-when]').forEach(group => {
            const expr = group.dataset.visibleWhen;
            const visible = this._evalCondition(compId, expr);
            group.style.display = visible ? '' : 'none';
        });
    },

    /**
     * Evaluate a visibility condition.
     * Supports: "field=value", "field!=value", "field>value", "field<value"
     */
    _evalCondition(compId, expr) {
        let op = '=';
        let parts;

        for (const candidate of ['!=', '>=', '<=', '>', '<', '=']) {
            if (expr.includes(candidate)) {
                op = candidate;
                parts = expr.split(candidate);
                break;
            }
        }

        if (!parts || parts.length !== 2) return true;
        const [refName, expected] = parts.map(s => s.trim());
        const value = this._getFieldValue(compId, refName);

        switch (op) {
            case '!=': return value !== expected;
            case '>=': return parseFloat(value) >= parseFloat(expected);
            case '<=': return parseFloat(value) <= parseFloat(expected);
            case '>':  return parseFloat(value) > parseFloat(expected);
            case '<':  return parseFloat(value) < parseFloat(expected);
            default:   return value === expected;
        }
    },

    /**
     * Cascading depends_on: when a parent field changes, update dependent selects.
     * The add-on must expose a GET /api/addons/{id}/options?field={name}&parent_value={val}
     * endpoint to supply dynamic options.
     */
    _evaluateDependsOn(compId, changedField) {
        const form = document.getElementById(`form-${compId}`);
        const meta = this._forms[compId];
        if (!form || !meta) return;

        form.querySelectorAll('[data-depends-on]').forEach(group => {
            const parentName = group.dataset.dependsOn;
            if (parentName !== changedField) return;

            // Find the select inside this group
            const select = group.querySelector('select');
            if (!select) return;

            const parentValue = this._getFieldValue(compId, parentName);
            const fieldName = select.name;

            // Clear and set loading state
            select.innerHTML = '<option value="">Loading...</option>';
            select.disabled = true;

            // Fetch new options from the addon
            this._fetchDependentOptions(meta.addonId, fieldName, parentValue)
                .then(options => {
                    select.innerHTML = '<option value="">Select...</option>' +
                        options.map(o => `<option value="${this._escape(o.value)}">${this._escape(o.label)}</option>`).join('');
                    select.disabled = false;
                })
                .catch(() => {
                    select.innerHTML = '<option value="">Failed to load options</option>';
                    select.disabled = false;
                });
        });
    },

    async _fetchDependentOptions(addonId, fieldName, parentValue) {
        const resp = await API.get(`/api/addons/${addonId}/options?field=${encodeURIComponent(fieldName)}&parent_value=${encodeURIComponent(parentValue)}`);
        if (!resp.ok) return [];
        const data = await resp.json();
        return Array.isArray(data) ? data : (data.options || []);
    },

    /** Re-evaluate all live_calculation fields. */
    _evaluateCalculations(compId) {
        const form = document.getElementById(`form-${compId}`);
        if (!form) return;

        form.querySelectorAll('[data-calc]').forEach(group => {
            const formula = group.dataset.calc;
            const result = this._evalFormula(compId, formula);
            const display = group.querySelector('.form-calc-result');
            if (display) {
                display.textContent = result !== null ? this._formatCalcResult(result) : '--';
            }
            // Also set the field value to the calculated result
            const input = group.querySelector('input:not([type="range"]), select');
            if (input && result !== null) {
                input.value = this._formatCalcResult(result);
            }
        });
    },

    _formatCalcResult(val) {
        // Show up to 2 decimal places, but trim trailing zeros
        return parseFloat(val.toFixed(4)).toString();
    },

    /**
     * Safe formula evaluation — arithmetic only, no eval().
     * Supports: +, -, *, /, %, parentheses, field references, numbers.
     */
    _evalFormula(compId, formula) {
        try {
            const vars = {};
            const meta = this._forms[compId];
            if (meta) {
                for (const field of meta.fields) {
                    const val = parseFloat(this._getFieldValue(compId, field.name));
                    vars[field.name] = isNaN(val) ? 0 : val;
                }
            }
            return this._parseExpr(formula, vars);
        } catch {
            return null;
        }
    },

    /** Recursive-descent parser for basic arithmetic (mirrors Go EvalFormula). */
    _parseExpr(input, vars) {
        let pos = 0;

        const skipSpaces = () => { while (pos < input.length && input[pos] === ' ') pos++; };

        const parseFactor = () => {
            skipSpaces();
            if (pos >= input.length) return 0;

            if (input[pos] === '(') {
                pos++;
                const val = parseExpression();
                skipSpaces();
                if (pos < input.length && input[pos] === ')') pos++;
                return val;
            }

            if ((input[pos] >= '0' && input[pos] <= '9') || input[pos] === '.') {
                const start = pos;
                while (pos < input.length && ((input[pos] >= '0' && input[pos] <= '9') || input[pos] === '.')) pos++;
                return parseFloat(input.substring(start, pos));
            }

            if (/[a-zA-Z_]/.test(input[pos])) {
                const start = pos;
                while (pos < input.length && /[a-zA-Z0-9_]/.test(input[pos])) pos++;
                const name = input.substring(start, pos);
                return vars[name] || 0;
            }

            return 0;
        };

        const parseTerm = () => {
            let left = parseFactor();
            while (true) {
                skipSpaces();
                if (pos >= input.length) break;
                const op = input[pos];
                if (op !== '*' && op !== '/' && op !== '%') break;
                pos++;
                const right = parseFactor();
                if (op === '*') left *= right;
                else if (op === '%') left = right !== 0 ? left % right : 0;
                else left = right !== 0 ? left / right : 0;
            }
            return left;
        };

        const parseExpression = () => {
            let left = parseTerm();
            while (true) {
                skipSpaces();
                if (pos >= input.length) break;
                const op = input[pos];
                if (op !== '+' && op !== '-') break;
                pos++;
                const right = parseTerm();
                left = op === '+' ? left + right : left - right;
            }
            return left;
        };

        return parseExpression();
    },

    _getFieldValue(compId, fieldName) {
        const el = document.getElementById(`field-${compId}-${fieldName}`);
        if (!el) return '';
        if (el.type === 'checkbox') return el.checked ? 'true' : 'false';
        return el.value;
    },

    // ─── Submission & Security Gate ───────────────────────────────────────

    async submit(event, compId) {
        event.preventDefault();
        const meta = this._forms[compId];
        if (!meta) return;

        const errorEl = document.getElementById(`form-error-${compId}`);
        if (errorEl) { errorEl.textContent = ''; errorEl.className = 'addon-form-error'; }

        // Check for security_gate fields — triggers multi-step modal
        const hasGate = meta.fields.some(f => f.security_gate);
        if (hasGate) {
            meta.gateStep = 1;
            this._showGateStep1(compId);
            return;
        }

        await this._doSubmit(compId);
    },

    /** Step 1: Password confirmation. */
    _showGateStep1(compId) {
        Modals.create(`
            <div class="modal">
                <div class="modal-header">
                    <h3>Security Confirmation — Step 1 of 2</h3>
                    <button class="modal-close" onclick="Modals.close(this)">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
                        </svg>
                    </button>
                </div>
                <div class="modal-body">
                    <p>This action requires elevated confirmation. Enter your password to continue.</p>
                    <div class="gate-steps">
                        <div class="gate-step active">1. Password</div>
                        <div class="gate-step">2. Confirm Path</div>
                    </div>
                    <div class="form-group">
                        <label>Password</label>
                        <input type="password" id="gate-pw-${compId}" class="form-input" autofocus>
                    </div>
                    <div id="gate-error-${compId}" class="form-error"></div>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-secondary" onclick="Modals.close(this)">Cancel</button>
                    <button class="btn btn-primary" onclick="FormComponent._gateStep1Submit('${compId}')">Next</button>
                </div>
            </div>
        `);
        setTimeout(() => document.getElementById(`gate-pw-${compId}`)?.focus(), 50);
    },

    /** Step 1 submit: verify password locally (non-empty) then proceed. */
    _gateStep1Submit(compId) {
        const pw = document.getElementById(`gate-pw-${compId}`)?.value;
        if (!pw) {
            const err = document.getElementById(`gate-error-${compId}`);
            if (err) err.textContent = 'Password is required';
            return;
        }

        // Store password for final submission
        const meta = this._forms[compId];
        if (meta) meta._gatePassword = pw;

        // Close step 1, open step 2
        document.querySelector('.modal-overlay')?.remove();
        this._showGateStep2(compId);
    },

    /** Step 2: Path/action confirmation. */
    _showGateStep2(compId) {
        const meta = this._forms[compId];
        const action = meta?.action || 'this action';

        // Collect key field values for summary
        const summary = this._gateFieldSummary(compId);

        Modals.create(`
            <div class="modal">
                <div class="modal-header">
                    <h3>Security Confirmation — Step 2 of 2</h3>
                    <button class="modal-close" onclick="Modals.close(this)">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
                        </svg>
                    </button>
                </div>
                <div class="modal-body">
                    <div class="gate-steps">
                        <div class="gate-step done">1. Password</div>
                        <div class="gate-step active">2. Confirm Path</div>
                    </div>
                    <p>Please review and confirm the following action:</p>
                    <div class="gate-summary">
                        <div class="gate-summary-action">${this._escape(action)}</div>
                        ${summary}
                    </div>
                    <div class="form-group">
                        <label>Type <strong>CONFIRM</strong> to proceed</label>
                        <input type="text" id="gate-confirm-${compId}" class="form-input form-input-mono"
                               placeholder="CONFIRM" autocomplete="off">
                    </div>
                    <div id="gate-error2-${compId}" class="form-error"></div>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-secondary" onclick="Modals.close(this)">Cancel</button>
                    <button class="btn btn-primary btn-danger" onclick="FormComponent._gateStep2Submit('${compId}')">Execute</button>
                </div>
            </div>
        `);
        setTimeout(() => document.getElementById(`gate-confirm-${compId}`)?.focus(), 50);
    },

    _gateFieldSummary(compId) {
        const meta = this._forms[compId];
        if (!meta) return '';

        const items = meta.fields
            .filter(f => f.type !== 'hidden' && !f.security_gate)
            .map(f => {
                const val = this._getFieldValue(compId, f.name);
                if (!val) return '';
                return `<div class="gate-summary-item">
                    <span class="gate-label">${this._escape(f.label || f.name)}:</span>
                    <span class="gate-value">${this._escape(val)}</span>
                </div>`;
            })
            .filter(Boolean)
            .join('');

        return items ? `<div class="gate-summary-fields">${items}</div>` : '';
    },

    /** Step 2 submit: verify CONFIRM text then fire the actual request. */
    async _gateStep2Submit(compId) {
        const confirmText = document.getElementById(`gate-confirm-${compId}`)?.value;
        if (confirmText !== 'CONFIRM') {
            const err = document.getElementById(`gate-error2-${compId}`);
            if (err) err.textContent = 'Please type CONFIRM exactly';
            return;
        }

        document.querySelector('.modal-overlay')?.remove();

        const meta = this._forms[compId];
        await this._doSubmit(compId, meta?._gatePassword);
        if (meta) delete meta._gatePassword;
    },

    async _doSubmit(compId, password) {
        const meta = this._forms[compId];
        if (!meta) return;

        const formData = {};
        for (const field of meta.fields) {
            formData[field.name] = this._getFieldValue(compId, field.name);
        }
        if (password) formData._password = password;

        const errorEl = document.getElementById(`form-error-${compId}`);

        try {
            const resp = await API.post(`/api/addons/${meta.addonId}/action`, {
                component_id: compId,
                action: meta.action,
                data: formData
            });

            if (resp.ok) {
                if (errorEl) {
                    errorEl.textContent = 'Submitted successfully';
                    errorEl.className = 'addon-form-error success';
                    setTimeout(() => { errorEl.textContent = ''; errorEl.className = 'addon-form-error'; }, 3000);
                }
            } else {
                const data = await resp.json().catch(() => ({}));
                if (errorEl) errorEl.textContent = data.error || 'Submission failed';
            }
        } catch {
            if (errorEl) errorEl.textContent = 'Connection error';
        }
    },

    // ─── Helpers ──────────────────────────────────────────────────────────

    _escape(str) {
        if (!str) return '';
        const div = document.createElement('div');
        div.textContent = String(str);
        return div.innerHTML;
    },

    _escapeJS(str) {
        if (!str) return '';
        return String(str).replace(/\\/g, '\\\\').replace(/'/g, "\\'");
    }
};
