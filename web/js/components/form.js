/**
 * Vigil Dashboard - Form Component (Task 4.3)
 *
 * Renders manifest form fields with support for:
 *  - Field types: text, number, select, checkbox, hidden
 *  - visible_when: conditional visibility based on another field's value
 *  - depends_on: cascading select / value dependency
 *  - live_calculation: safe arithmetic formulas evaluated on input change
 *  - security_gate: requires password confirmation before submission
 */

const FormComponent = {
    _forms: {},  // keyed by componentId

    /**
     * Render a form component from manifest config.
     * @param {string} compId - Manifest component ID
     * @param {Object} config - { fields: FormField[], action?: string }
     * @param {number} addonId - Parent add-on ID
     * @returns {string} HTML
     */
    render(compId, config, addonId) {
        const fields = config.fields || [];
        if (fields.length === 0) {
            return '<p class="form-empty">No fields configured</p>';
        }

        this._forms[compId] = { fields, addonId, action: config.action || '' };

        const fieldHtml = fields.map(f => this._renderField(compId, f)).join('');

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

    _renderField(compId, field) {
        const id = `field-${compId}-${field.name}`;
        const required = field.required ? 'required' : '';
        const visibleStyle = field.visible_when ? 'style="display:none"' : '';
        const visibleAttr = field.visible_when ? `data-visible-when="${this._escape(field.visible_when)}"` : '';
        const dependsAttr = field.depends_on ? `data-depends-on="${this._escape(field.depends_on)}"` : '';
        const calcAttr = field.live_calculation ? `data-calc="${this._escape(field.live_calculation)}"` : '';

        if (field.type === 'hidden') {
            return `<input type="hidden" id="${id}" name="${this._escape(field.name)}" value="">`;
        }

        let input;
        switch (field.type) {
            case 'select':
                input = this._renderSelect(id, field, required, compId);
                break;
            case 'checkbox':
                input = this._renderCheckbox(id, field, compId);
                break;
            case 'number':
                input = `<input type="number" id="${id}" name="${this._escape(field.name)}"
                            class="form-input" ${required}
                            oninput="FormComponent._onInput('${compId}', '${this._escapeJS(field.name)}')">`;
                break;
            default: // text
                input = `<input type="text" id="${id}" name="${this._escape(field.name)}"
                            class="form-input" ${required}
                            oninput="FormComponent._onInput('${compId}', '${this._escapeJS(field.name)}')">`;
        }

        const calcDisplay = field.live_calculation
            ? `<span class="form-calc-result" id="calc-${id}"></span>`
            : '';

        return `
            <div class="form-group addon-form-group" id="fg-${id}" ${visibleStyle} ${visibleAttr} ${dependsAttr} ${calcAttr}>
                <label for="${id}">${this._escape(field.label || field.name)}</label>
                ${input}
                ${calcDisplay}
            </div>
        `;
    },

    _renderSelect(id, field, required, compId) {
        const options = (field.options || [])
            .map(o => `<option value="${this._escape(o.value)}">${this._escape(o.label)}</option>`)
            .join('');

        return `<select id="${id}" name="${this._escape(field.name)}" class="form-input" ${required}
                    onchange="FormComponent._onInput('${compId}', '${this._escapeJS(field.name)}')">
                    <option value="">Select...</option>
                    ${options}
                </select>`;
    },

    _renderCheckbox(id, field, compId) {
        return `<label class="addon-checkbox">
                    <input type="checkbox" id="${id}" name="${this._escape(field.name)}"
                        onchange="FormComponent._onInput('${compId}', '${this._escapeJS(field.name)}')">
                    ${this._escape(field.label || field.name)}
                </label>`;
    },

    // ─── Reactivity ───────────────────────────────────────────────────────

    _onInput(compId, fieldName) {
        this._evaluateVisibility(compId);
        this._evaluateCalculations(compId);
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
     * Format: "field_name=value" or "field_name!=value"
     */
    _evalCondition(compId, expr) {
        let negate = false;
        let parts;

        if (expr.includes('!=')) {
            negate = true;
            parts = expr.split('!=');
        } else {
            parts = expr.split('=');
        }

        if (parts.length !== 2) return true;
        const [refName, expected] = parts.map(s => s.trim());
        const value = this._getFieldValue(compId, refName);

        const match = value === expected;
        return negate ? !match : match;
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
                display.textContent = result !== null ? result.toFixed(2) : '--';
            }
            // Also set the field value to the calculated result
            const input = group.querySelector('input, select');
            if (input && result !== null) {
                input.value = result.toFixed(2);
            }
        });
    },

    /**
     * Safe formula evaluation — arithmetic only, no eval().
     * Supports: +, -, *, /, parentheses, field references, numbers.
     */
    _evalFormula(compId, formula) {
        try {
            // Collect variable values from form fields
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
        const parser = { input, vars, pos: 0, err: null };

        const skipSpaces = () => { while (parser.pos < input.length && input[parser.pos] === ' ') parser.pos++; };

        const parseFactor = () => {
            skipSpaces();
            if (parser.pos >= input.length) return 0;

            if (input[parser.pos] === '(') {
                parser.pos++;
                const val = parseExpression();
                skipSpaces();
                if (parser.pos < input.length && input[parser.pos] === ')') parser.pos++;
                return val;
            }

            if ((input[parser.pos] >= '0' && input[parser.pos] <= '9') || input[parser.pos] === '.') {
                const start = parser.pos;
                while (parser.pos < input.length && ((input[parser.pos] >= '0' && input[parser.pos] <= '9') || input[parser.pos] === '.')) parser.pos++;
                return parseFloat(input.substring(start, parser.pos));
            }

            if (/[a-zA-Z_]/.test(input[parser.pos])) {
                const start = parser.pos;
                while (parser.pos < input.length && /[a-zA-Z0-9_]/.test(input[parser.pos])) parser.pos++;
                const name = input.substring(start, parser.pos);
                return vars[name] || 0;
            }

            return 0;
        };

        const parseTerm = () => {
            let left = parseFactor();
            while (true) {
                skipSpaces();
                if (parser.pos >= input.length) break;
                const op = input[parser.pos];
                if (op !== '*' && op !== '/') break;
                parser.pos++;
                const right = parseFactor();
                left = op === '*' ? left * right : (right !== 0 ? left / right : 0);
            }
            return left;
        };

        const parseExpression = () => {
            let left = parseTerm();
            while (true) {
                skipSpaces();
                if (parser.pos >= input.length) break;
                const op = input[parser.pos];
                if (op !== '+' && op !== '-') break;
                parser.pos++;
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

    // ─── Submission ───────────────────────────────────────────────────────

    async submit(event, compId) {
        event.preventDefault();
        const meta = this._forms[compId];
        if (!meta) return;

        const errorEl = document.getElementById(`form-error-${compId}`);
        if (errorEl) errorEl.textContent = '';

        // Check for security_gate fields
        const hasGate = meta.fields.some(f => f.security_gate);
        if (hasGate) {
            this._showSecurityGate(compId);
            return;
        }

        await this._doSubmit(compId);
    },

    _showSecurityGate(compId) {
        const modal = Modals.create(`
            <div class="modal">
                <div class="modal-header">
                    <h3>Confirm Action</h3>
                    <button class="modal-close" onclick="Modals.close(this)">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <line x1="18" y1="6" x2="6" y2="18"/>
                            <line x1="6" y1="6" x2="18" y2="18"/>
                        </svg>
                    </button>
                </div>
                <div class="modal-body">
                    <p>This action requires password confirmation.</p>
                    <div class="form-group">
                        <label>Password</label>
                        <input type="password" id="gate-password-${compId}" class="form-input">
                    </div>
                    <div id="gate-error-${compId}" class="form-error"></div>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-secondary" onclick="Modals.close(this)">Cancel</button>
                    <button class="btn btn-primary" onclick="FormComponent._confirmGate('${compId}')">Confirm</button>
                </div>
            </div>
        `);
        document.getElementById(`gate-password-${compId}`)?.focus();
    },

    async _confirmGate(compId) {
        const pw = document.getElementById(`gate-password-${compId}`)?.value;
        if (!pw) {
            const err = document.getElementById(`gate-error-${compId}`);
            if (err) err.textContent = 'Password is required';
            return;
        }

        document.querySelector('.modal-overlay')?.remove();
        await this._doSubmit(compId, pw);
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
                    errorEl.classList.add('success');
                    setTimeout(() => { errorEl.textContent = ''; errorEl.classList.remove('success'); }, 3000);
                }
            } else {
                const data = await resp.json().catch(() => ({}));
                if (errorEl) errorEl.textContent = data.error || 'Submission failed';
            }
        } catch (e) {
            if (errorEl) errorEl.textContent = 'Connection error';
        }
    },

    // ─── Helpers ──────────────────────────────────────────────────────────

    _escape(str) {
        if (!str) return '';
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    },

    _escapeJS(str) {
        if (!str) return '';
        return str.replace(/\\/g, '\\\\').replace(/'/g, "\\'");
    }
};
