package addons

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ── Manifest schema ─────────────────────────────────────────────────────

// Manifest is the top-level add-on descriptor registered by an add-on.
type Manifest struct {
	Name        string         `json:"name"`
	Version     string         `json:"version"`
	Description string         `json:"description,omitempty"`
	Author      string         `json:"author,omitempty"`
	Pages       []ManifestPage `json:"pages"`
}

// ManifestPage represents a navigable tab inside the add-on UI.
type ManifestPage struct {
	ID         string              `json:"id"`
	Title      string              `json:"title"`
	Icon       string              `json:"icon,omitempty"`
	Components []ManifestComponent `json:"components"`
}

// ManifestComponent describes a UI widget rendered from telemetry data.
type ManifestComponent struct {
	Type   string          `json:"type"` // form, progress, chart, smart-table, log-viewer
	ID     string          `json:"id"`
	Title  string          `json:"title,omitempty"`
	Source string          `json:"source,omitempty"` // addon_agents, agent_drives, telemetry
	Config json.RawMessage `json:"config,omitempty"` // type-specific; validated per-type below
}

// FormField is a field inside a "form" component's config.
type FormField struct {
	Name            string          `json:"name"`
	Label           string          `json:"label"`
	Type            string          `json:"type"` // text, number, select, checkbox, toggle, hidden
	Required        bool            `json:"required,omitempty"`
	Options         []FormOption    `json:"options,omitempty"`
	Source          string          `json:"source,omitempty"`
	Placeholder     string          `json:"placeholder,omitempty"`
	Default         json.RawMessage `json:"default,omitempty"`
	Min             *float64        `json:"min,omitempty"`
	Max             *float64        `json:"max,omitempty"`
	DependsOn       string          `json:"depends_on,omitempty"`
	VisibleWhen     json.RawMessage `json:"visible_when,omitempty"`
	LiveCalculation string          `json:"live_calculation,omitempty"`
	SecurityGate    bool            `json:"security_gate,omitempty"`
}

// FormOption is a select option.
type FormOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// ── Limits ──────────────────────────────────────────────────────────────

const (
	maxManifestSize = 256 * 1024 // 256 KiB raw JSON
	maxPages        = 20
	maxComponents   = 50
	maxFormFields   = 100
	maxNameLen      = 128
)

// validComponentTypes is the set of recognised component types.
var validComponentTypes = map[string]bool{
	"form":           true,
	"progress":       true,
	"chart":          true,
	"smart-table":    true,
	"log-viewer":     true,
	"deploy-wizard":  true,
}

// ── Validation ──────────────────────────────────────────────────────────

// ValidateManifest parses raw JSON into a Manifest and checks all
// structural and safety constraints.  Returns the parsed manifest
// or a descriptive error.
func ValidateManifest(raw []byte) (*Manifest, error) {
	if len(raw) > maxManifestSize {
		return nil, fmt.Errorf("manifest exceeds %d byte limit", maxManifestSize)
	}

	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	if m.Name == "" {
		return nil, fmt.Errorf("manifest: name is required")
	}
	if len(m.Name) > maxNameLen {
		return nil, fmt.Errorf("manifest: name exceeds %d characters", maxNameLen)
	}
	if m.Version == "" {
		return nil, fmt.Errorf("manifest: version is required")
	}
	if len(m.Pages) == 0 {
		return nil, fmt.Errorf("manifest: at least one page is required")
	}
	if len(m.Pages) > maxPages {
		return nil, fmt.Errorf("manifest: exceeds %d page limit", maxPages)
	}

	compCount := 0
	pageIDs := make(map[string]bool)
	compIDs := make(map[string]bool)

	for pi, page := range m.Pages {
		if page.ID == "" {
			return nil, fmt.Errorf("page[%d]: id is required", pi)
		}
		if pageIDs[page.ID] {
			return nil, fmt.Errorf("page[%d]: duplicate id %q", pi, page.ID)
		}
		pageIDs[page.ID] = true

		if page.Title == "" {
			return nil, fmt.Errorf("page[%d]: title is required", pi)
		}

		for ci, comp := range page.Components {
			compCount++
			if compCount > maxComponents {
				return nil, fmt.Errorf("manifest: exceeds %d total component limit", maxComponents)
			}
			if err := validateComponent(pi, ci, comp, compIDs); err != nil {
				return nil, err
			}
		}
	}

	return &m, nil
}

func validateComponent(pi, ci int, comp ManifestComponent, ids map[string]bool) error {
	prefix := fmt.Sprintf("page[%d].component[%d]", pi, ci)

	if comp.ID == "" {
		return fmt.Errorf("%s: id is required", prefix)
	}
	if ids[comp.ID] {
		return fmt.Errorf("%s: duplicate id %q", prefix, comp.ID)
	}
	ids[comp.ID] = true

	if !validComponentTypes[comp.Type] {
		return fmt.Errorf("%s: unknown type %q", prefix, comp.Type)
	}

	if comp.Type == "form" && len(comp.Config) > 0 {
		return validateFormConfig(prefix, comp.Config)
	}
	if comp.Type == "deploy-wizard" && len(comp.Config) > 0 {
		return validateDeployWizardConfig(prefix, comp.Config)
	}
	return nil
}

func validateFormConfig(prefix string, raw json.RawMessage) error {
	var cfg struct {
		Fields []FormField `json:"fields"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("%s: invalid form config: %w", prefix, err)
	}
	if len(cfg.Fields) > maxFormFields {
		return fmt.Errorf("%s: exceeds %d field limit", prefix, maxFormFields)
	}
	for fi, f := range cfg.Fields {
		if f.Name == "" {
			return fmt.Errorf("%s.field[%d]: name is required", prefix, fi)
		}
		if f.LiveCalculation != "" {
			if err := ValidateFormula(f.LiveCalculation); err != nil {
				return fmt.Errorf("%s.field[%d]: unsafe formula: %w", prefix, fi, err)
			}
		}
	}
	return nil
}

// ── Deploy wizard config ────────────────────────────────────────────────

// DeployWizardConfig describes the configuration for a deploy-wizard component.
type DeployWizardConfig struct {
	TargetLabel     string              `json:"target_label"`
	Docker          *DockerDeployConfig `json:"docker,omitempty"`
	Binary          *BinaryDeployConfig `json:"binary,omitempty"`
	PrefillEndpoint string              `json:"prefill_endpoint,omitempty"`
}

// DockerDeployConfig describes Docker deployment options.
type DockerDeployConfig struct {
	Image         string                       `json:"image"`
	DefaultTag    string                       `json:"default_tag"`
	ContainerName string                       `json:"container_name,omitempty"`
	Ports         []string                     `json:"ports,omitempty"`
	Privileged    bool                         `json:"privileged,omitempty"`
	Volumes       []string                     `json:"volumes,omitempty"`
	Environment   map[string]DeployEnvVar      `json:"environment,omitempty"`
	Platforms     map[string]DeployPlatformDef `json:"platforms"`
}

// BinaryDeployConfig describes binary installation options.
type BinaryDeployConfig struct {
	InstallURL string                       `json:"install_url"`
	Platforms  map[string]DeployPlatformDef `json:"platforms"`
}

// DeployEnvVar describes a single environment variable in the deploy wizard.
type DeployEnvVar struct {
	Source      string `json:"source"`                // prefill, user_input, literal
	Key         string `json:"key,omitempty"`          // key in prefill response
	Label       string `json:"label,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	Hint        string `json:"hint,omitempty"`
	Value       string `json:"value,omitempty"`        // for literal source
	Readonly    bool   `json:"readonly,omitempty"`
}

// DeployPlatformDef describes a deployment platform option.
type DeployPlatformDef struct {
	Label        string   `json:"label"`
	Hint         string   `json:"hint,omitempty"`
	ExtraVolumes []string `json:"extra_volumes,omitempty"`
	PID          string   `json:"pid,omitempty"`
}

func validateDeployWizardConfig(prefix string, raw json.RawMessage) error {
	var cfg DeployWizardConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("%s: invalid deploy-wizard config: %w", prefix, err)
	}

	if cfg.Docker == nil && cfg.Binary == nil {
		return fmt.Errorf("%s: deploy-wizard requires at least one of docker or binary config", prefix)
	}

	if cfg.Docker != nil {
		if cfg.Docker.Image == "" {
			return fmt.Errorf("%s: deploy-wizard docker.image is required", prefix)
		}
		if len(cfg.Docker.Platforms) == 0 {
			return fmt.Errorf("%s: deploy-wizard docker.platforms requires at least one entry", prefix)
		}
	}

	if cfg.Binary != nil {
		if cfg.Binary.InstallURL == "" {
			return fmt.Errorf("%s: deploy-wizard binary.install_url is required", prefix)
		}
		if len(cfg.Binary.Platforms) == 0 {
			return fmt.Errorf("%s: deploy-wizard binary.platforms requires at least one entry", prefix)
		}
	}

	if cfg.PrefillEndpoint != "" && !strings.HasPrefix(cfg.PrefillEndpoint, "/") {
		return fmt.Errorf("%s: deploy-wizard prefill_endpoint must start with /", prefix)
	}

	return nil
}

// ── Safe formula evaluator ──────────────────────────────────────────────

// allowedFormulaTokens are the tokens permitted in a live_calculation expression.
// Formulas may reference field names (alphanumeric + underscore), numbers,
// and basic arithmetic operators.
var allowedFormulaChars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_+-*/().%, "

// ValidateFormula checks that a formula string contains only safe tokens.
// It does NOT evaluate the formula — the frontend handles evaluation.
func ValidateFormula(expr string) error {
	if len(expr) > 512 {
		return fmt.Errorf("formula exceeds 512 character limit")
	}
	for _, ch := range expr {
		if !strings.ContainsRune(allowedFormulaChars, ch) {
			return fmt.Errorf("disallowed character %q", ch)
		}
	}

	// Block common injection patterns even if individual chars are allowed.
	lower := strings.ToLower(expr)
	blocked := []string{"eval", "function", "import", "require", "fetch",
		"window", "document", "process", "constructor", "__proto__"}
	for _, kw := range blocked {
		if strings.Contains(lower, kw) {
			return fmt.Errorf("disallowed keyword %q", kw)
		}
	}

	return nil
}

// EvalFormula evaluates a simple arithmetic formula with the given variable
// bindings.  Only supports +, -, *, / with float64 values.  Returns an
// error for anything beyond basic arithmetic.
func EvalFormula(expr string, vars map[string]float64) (float64, error) {
	if err := ValidateFormula(expr); err != nil {
		return 0, err
	}
	p := &formulaParser{input: expr, vars: vars, pos: 0}
	result := p.parseExpr()
	if p.err != nil {
		return 0, p.err
	}
	p.skipSpaces()
	if p.pos < len(p.input) {
		return 0, fmt.Errorf("unexpected character at position %d", p.pos)
	}
	return result, nil
}

// ── Recursive-descent parser for basic arithmetic ───────────────────────

type formulaParser struct {
	input string
	vars  map[string]float64
	pos   int
	err   error
}

func (p *formulaParser) skipSpaces() {
	for p.pos < len(p.input) && p.input[p.pos] == ' ' {
		p.pos++
	}
}

func (p *formulaParser) parseExpr() float64 {
	left := p.parseTerm()
	for p.err == nil {
		p.skipSpaces()
		if p.pos >= len(p.input) {
			break
		}
		op := p.input[p.pos]
		if op != '+' && op != '-' {
			break
		}
		p.pos++
		right := p.parseTerm()
		if op == '+' {
			left += right
		} else {
			left -= right
		}
	}
	return left
}

func (p *formulaParser) parseTerm() float64 {
	left := p.parseFactor()
	for p.err == nil {
		p.skipSpaces()
		if p.pos >= len(p.input) {
			break
		}
		op := p.input[p.pos]
		if op != '*' && op != '/' {
			break
		}
		p.pos++
		right := p.parseFactor()
		if op == '*' {
			left *= right
		} else {
			if right == 0 {
				p.err = fmt.Errorf("division by zero")
				return 0
			}
			left /= right
		}
	}
	return left
}

func (p *formulaParser) parseFactor() float64 {
	if p.err != nil {
		return 0
	}
	p.skipSpaces()
	if p.pos >= len(p.input) {
		p.err = fmt.Errorf("unexpected end of expression")
		return 0
	}

	// Parenthesised sub-expression
	if p.input[p.pos] == '(' {
		p.pos++
		val := p.parseExpr()
		p.skipSpaces()
		if p.pos < len(p.input) && p.input[p.pos] == ')' {
			p.pos++
		} else {
			p.err = fmt.Errorf("missing closing parenthesis")
		}
		return val
	}

	// Number literal
	if (p.input[p.pos] >= '0' && p.input[p.pos] <= '9') || p.input[p.pos] == '.' {
		return p.parseNumber()
	}

	// Variable reference
	if isIdentStart(p.input[p.pos]) {
		return p.parseVar()
	}

	p.err = fmt.Errorf("unexpected character %q at position %d", p.input[p.pos], p.pos)
	return 0
}

func (p *formulaParser) parseNumber() float64 {
	start := p.pos
	for p.pos < len(p.input) && ((p.input[p.pos] >= '0' && p.input[p.pos] <= '9') || p.input[p.pos] == '.') {
		p.pos++
	}
	var val float64
	_, err := fmt.Sscanf(p.input[start:p.pos], "%f", &val)
	if err != nil {
		p.err = fmt.Errorf("invalid number at position %d", start)
	}
	return val
}

func (p *formulaParser) parseVar() float64 {
	start := p.pos
	for p.pos < len(p.input) && isIdentChar(p.input[p.pos]) {
		p.pos++
	}
	name := p.input[start:p.pos]
	val, ok := p.vars[name]
	if !ok {
		p.err = fmt.Errorf("undefined variable %q", name)
	}
	return val
}

func isIdentStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

func isIdentChar(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}
