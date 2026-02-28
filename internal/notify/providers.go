package notify

import (
	"fmt"
	"net/url"
	"strings"
)

// ─── Field & Provider Types ─────────────────────────────────────────────

// FieldType enumerates input types the frontend should render.
type FieldType string

const (
	FieldText     FieldType = "text"
	FieldPassword FieldType = "password"
	FieldNumber   FieldType = "number"
	FieldCheckbox FieldType = "checkbox"
	FieldSelect   FieldType = "select"
)

// SelectOption is a single choice in a select dropdown.
type SelectOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// ProviderField describes one configuration input for a provider.
type ProviderField struct {
	Key         string         `json:"key"`
	Label       string         `json:"label"`
	Type        FieldType      `json:"type"`
	Placeholder string         `json:"placeholder,omitempty"`
	HelpText    string         `json:"help_text,omitempty"`
	Required    bool           `json:"required"`
	Options     []SelectOption `json:"options,omitempty"`
	Default     string         `json:"default,omitempty"`
}

// ProviderDef describes a notification provider's form schema.
type ProviderDef struct {
	Type   string          `json:"type"`
	Label  string          `json:"label"`
	Fields []ProviderField `json:"fields"`
}

// ─── Provider Registry ──────────────────────────────────────────────────

var providerRegistry = map[string]ProviderDef{
	"telegram": {
		Type: "telegram", Label: "Telegram",
		Fields: []ProviderField{
			{Key: "bot_token", Label: "Bot Token", Type: FieldPassword, Required: true,
				Placeholder: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
				HelpText:    "Get a token from https://t.me/BotFather"},
			{Key: "chat_id", Label: "Chat ID", Type: FieldText, Required: true,
				Placeholder: "@channel or numeric ID",
				HelpText:    "Send a message to the bot and visit https://api.telegram.org/bot<TOKEN>/getUpdates"},
			{Key: "thread_id", Label: "Message Thread ID", Type: FieldText,
				HelpText: "Optional. For forum supergroups only"},
			{Key: "silent", Label: "Send Silently", Type: FieldCheckbox},
			{Key: "protect", Label: "Protect Forwarding/Saving", Type: FieldCheckbox},
		},
	},
	"discord": {
		Type: "discord", Label: "Discord",
		Fields: []ProviderField{
			{Key: "webhook_url", Label: "Discord Webhook URL", Type: FieldText, Required: true,
				Placeholder: "https://discord.com/api/webhooks/...",
				HelpText:    "Server Settings → Integrations → View Webhooks → New Webhook"},
			{Key: "username", Label: "Bot Display Name", Type: FieldText,
				Placeholder: "Vigil"},
			{Key: "avatar_url", Label: "Avatar URL", Type: FieldText},
		},
	},
	"slack": {
		Type: "slack", Label: "Slack",
		Fields: []ProviderField{
			{Key: "webhook_url", Label: "Webhook URL", Type: FieldText, Required: true,
				Placeholder: "https://hooks.slack.com/services/T.../B.../...",
				HelpText:    "More info: https://api.slack.com/messaging/webhooks"},
			{Key: "bot_name", Label: "Username", Type: FieldText, Placeholder: "Vigil"},
			{Key: "icon_emoji", Label: "Icon Emoji", Type: FieldText, Placeholder: ":bell:"},
			{Key: "channel", Label: "Channel Name", Type: FieldText, Placeholder: "#alerts",
				HelpText: "Leave blank to use the webhook's default channel"},
		},
	},
	"email": {
		Type: "email", Label: "Email (SMTP)",
		Fields: []ProviderField{
			{Key: "host", Label: "Hostname", Type: FieldText, Required: true,
				Placeholder: "smtp.gmail.com",
				HelpText:    "SMTP server hostname or IP"},
			{Key: "port", Label: "Port", Type: FieldNumber, Required: true,
				Default: "587", Placeholder: "587"},
			{Key: "security", Label: "Security", Type: FieldSelect, Default: "starttls",
				Options: []SelectOption{
					{Value: "starttls", Label: "STARTTLS (25, 587)"},
					{Value: "ssl", Label: "SSL/TLS (465)"},
					{Value: "none", Label: "None"},
				}},
			{Key: "username", Label: "Username", Type: FieldText,
				Placeholder: "user@example.com"},
			{Key: "password", Label: "Password", Type: FieldPassword},
			{Key: "from", Label: "From Email", Type: FieldText, Required: true,
				Placeholder: "\"Vigil\" <vigil@example.com>"},
			{Key: "to", Label: "To Email", Type: FieldText, Required: true,
				Placeholder: "admin@example.com",
				HelpText:    "Comma-separated for multiple recipients"},
			{Key: "subject", Label: "Custom Subject", Type: FieldText,
				Placeholder: "[Vigil] Alert",
				HelpText:    "Leave blank for default"},
		},
	},
	"pushover": {
		Type: "pushover", Label: "Pushover",
		Fields: []ProviderField{
			{Key: "user_key", Label: "User Key", Type: FieldPassword, Required: true,
				HelpText: "Your Pushover user key"},
			{Key: "api_token", Label: "Application Token", Type: FieldPassword, Required: true,
				HelpText: "Create an application at https://pushover.net/api"},
			{Key: "devices", Label: "Device", Type: FieldText,
				HelpText: "Leave blank to send to all devices"},
			{Key: "title", Label: "Message Title", Type: FieldText, Placeholder: "Vigil Alert"},
			{Key: "priority", Label: "Priority", Type: FieldSelect, Default: "0",
				Options: []SelectOption{
					{Value: "-2", Label: "Lowest"},
					{Value: "-1", Label: "Low"},
					{Value: "0", Label: "Normal"},
					{Value: "1", Label: "High"},
					{Value: "2", Label: "Emergency"},
				}},
			{Key: "sound", Label: "Notification Sound", Type: FieldSelect,
				Options: []SelectOption{
					{Value: "", Label: "Default"},
					{Value: "pushover", Label: "Pushover"},
					{Value: "bike", Label: "Bike"},
					{Value: "bugle", Label: "Bugle"},
					{Value: "cashregister", Label: "Cash Register"},
					{Value: "classical", Label: "Classical"},
					{Value: "cosmic", Label: "Cosmic"},
					{Value: "falling", Label: "Falling"},
					{Value: "gamelan", Label: "Gamelan"},
					{Value: "incoming", Label: "Incoming"},
					{Value: "intermission", Label: "Intermission"},
					{Value: "magic", Label: "Magic"},
					{Value: "mechanical", Label: "Mechanical"},
					{Value: "pianobar", Label: "Piano Bar"},
					{Value: "siren", Label: "Siren"},
					{Value: "spacealarm", Label: "Space Alarm"},
					{Value: "tugboat", Label: "Tug Boat"},
					{Value: "alien", Label: "Alien Alarm"},
					{Value: "climb", Label: "Climb"},
					{Value: "persistent", Label: "Persistent"},
					{Value: "echo", Label: "Echo"},
					{Value: "updown", Label: "Up Down"},
					{Value: "vibrate", Label: "Vibrate Only"},
					{Value: "none", Label: "None (silent)"},
				}},
		},
	},
	"gotify": {
		Type: "gotify", Label: "Gotify",
		Fields: []ProviderField{
			{Key: "app_token", Label: "Application Token", Type: FieldPassword, Required: true},
			{Key: "server_url", Label: "Server URL", Type: FieldText, Required: true,
				Placeholder: "https://gotify.example.com"},
			{Key: "priority", Label: "Priority", Type: FieldNumber,
				Default: "8", Placeholder: "0-10"},
		},
	},
	"signal": {
		Type: "signal", Label: "Signal",
		Fields: []ProviderField{
			{Key: "host", Label: "Signal CLI REST API Host", Type: FieldText, Required: true,
				Placeholder: "localhost:8080",
				HelpText:    "Hostname and port of your signal-cli-rest-api instance"},
			{Key: "number", Label: "Sender Number", Type: FieldText, Required: true,
				Placeholder: "+1234567890",
				HelpText:    "Phone number registered with signal-cli (include + prefix)"},
			{Key: "recipients", Label: "Recipients", Type: FieldText, Required: true,
				Placeholder: "+1234567890,+0987654321",
				HelpText:    "Comma-separated list of recipient phone numbers"},
		},
	},
	"generic": {
		Type: "generic", Label: "Generic Webhook",
		Fields: []ProviderField{
			{Key: "webhook_url", Label: "Webhook URL", Type: FieldText, Required: true,
				Placeholder: "https://example.com/api/webhook",
				HelpText:    "For all supported services and URL formats, see https://shoutrrr.nickfedor.com/v0.13.2/services/overview/"},
		},
	},
}

// GetProviderDefs returns all provider definitions for the frontend API.
func GetProviderDefs() map[string]ProviderDef {
	return providerRegistry
}

// GetProviderDef returns a single provider definition.
func GetProviderDef(serviceType string) (ProviderDef, bool) {
	def, ok := providerRegistry[serviceType]
	return def, ok
}

// ─── Validation ─────────────────────────────────────────────────────────

// ValidateFields checks that all required fields for a provider are present.
func ValidateFields(serviceType string, fields map[string]string) error {
	def, ok := providerRegistry[serviceType]
	if !ok {
		return fmt.Errorf("unknown provider: %s", serviceType)
	}
	for _, f := range def.Fields {
		if f.Required && strings.TrimSpace(fields[f.Key]) == "" {
			return fmt.Errorf("%s is required", f.Label)
		}
	}
	return nil
}

// MaskSecrets returns a copy of fields with password-type values replaced by a mask.
func MaskSecrets(serviceType string, fields map[string]string) map[string]string {
	def, ok := providerRegistry[serviceType]
	if !ok {
		return fields
	}
	masked := make(map[string]string, len(fields))
	for k, v := range fields {
		masked[k] = v
	}
	secretKeys := make(map[string]bool)
	for _, f := range def.Fields {
		if f.Type == FieldPassword {
			secretKeys[f.Key] = true
		}
	}
	for k := range masked {
		if secretKeys[k] && masked[k] != "" {
			masked[k] = "********"
		}
	}
	return masked
}

// IsSecretMask returns true if the value is the placeholder mask.
const SecretMask = "********"

// ─── URL Builders ───────────────────────────────────────────────────────

// BuildShoutrrrURL assembles a valid Shoutrrr URL from structured provider fields.
func BuildShoutrrrURL(serviceType string, fields map[string]string) (string, error) {
	switch serviceType {
	case "telegram":
		return buildTelegramURL(fields)
	case "discord":
		return buildDiscordURL(fields)
	case "slack":
		return buildSlackURL(fields)
	case "email":
		return buildEmailURL(fields)
	case "pushover":
		return buildPushoverURL(fields)
	case "gotify":
		return buildGotifyURL(fields)
	case "signal":
		return buildSignalURL(fields)
	case "generic":
		return buildGenericURL(fields)
	default:
		return "", fmt.Errorf("unknown provider: %s", serviceType)
	}
}

// telegram://botToken@telegram?chats=chatID[&notification=no][&preview=no][&parseMode=...]
func buildTelegramURL(f map[string]string) (string, error) {
	token := strings.TrimSpace(f["bot_token"])
	chatID := strings.TrimSpace(f["chat_id"])
	if token == "" || chatID == "" {
		return "", fmt.Errorf("Bot Token and Chat ID are required")
	}

	params := url.Values{}
	params.Set("chats", chatID)
	if f["thread_id"] != "" {
		params.Set("topic", strings.TrimSpace(f["thread_id"]))
	}
	if f["silent"] == "true" {
		params.Set("notification", "no")
	}
	if f["protect"] == "true" {
		params.Set("protect", "yes")
	}

	return fmt.Sprintf("telegram://%s@telegram?%s", token, params.Encode()), nil
}

// discord://token@webhookID[?username=...&avatarurl=...]
// Input: full webhook URL https://discord.com/api/webhooks/{id}/{token}
func buildDiscordURL(f map[string]string) (string, error) {
	webhookURL := strings.TrimSpace(f["webhook_url"])
	if webhookURL == "" {
		return "", fmt.Errorf("Discord Webhook URL is required")
	}

	// Parse: https://discord.com/api/webhooks/{id}/{token}
	trimmed := strings.TrimRight(webhookURL, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid Discord webhook URL format")
	}
	token := parts[len(parts)-1]
	id := parts[len(parts)-2]

	if token == "" || id == "" {
		return "", fmt.Errorf("could not extract webhook ID and token from URL")
	}

	u := fmt.Sprintf("discord://%s@%s", token, id)
	params := url.Values{}
	if f["username"] != "" {
		params.Set("username", f["username"])
	}
	if f["avatar_url"] != "" {
		params.Set("avatarurl", f["avatar_url"])
	}
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	return u, nil
}

// slack://token-a/token-b/token-c[?botname=...&icon=...&channel=...]
// Input: full webhook URL https://hooks.slack.com/services/T.../B.../...
func buildSlackURL(f map[string]string) (string, error) {
	webhookURL := strings.TrimSpace(f["webhook_url"])
	if webhookURL == "" {
		return "", fmt.Errorf("Webhook URL is required")
	}

	// Parse: https://hooks.slack.com/services/TAAA/BBBB/CCCC
	trimmed := strings.TrimRight(webhookURL, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid Slack webhook URL format")
	}
	tokenA := parts[len(parts)-3]
	tokenB := parts[len(parts)-2]
	tokenC := parts[len(parts)-1]

	u := fmt.Sprintf("slack://%s/%s/%s", tokenA, tokenB, tokenC)
	params := url.Values{}
	if f["bot_name"] != "" {
		params.Set("botname", f["bot_name"])
	}
	if f["icon_emoji"] != "" {
		params.Set("icon", f["icon_emoji"])
	}
	if f["channel"] != "" {
		params.Set("channel", f["channel"])
	}
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	return u, nil
}

// smtp://[user:pass@]host:port/?from=addr&to=addr[&subject=...][&useStartTLS=yes|no]
func buildEmailURL(f map[string]string) (string, error) {
	host := strings.TrimSpace(f["host"])
	port := strings.TrimSpace(f["port"])
	from := strings.TrimSpace(f["from"])
	to := strings.TrimSpace(f["to"])
	if host == "" || port == "" || from == "" || to == "" {
		return "", fmt.Errorf("Hostname, Port, From, and To are required")
	}

	userinfo := ""
	if f["username"] != "" {
		userinfo = url.PathEscape(f["username"])
		if f["password"] != "" {
			userinfo += ":" + url.PathEscape(f["password"])
		}
		userinfo += "@"
	}

	params := url.Values{}
	params.Set("from", from)
	params.Set("to", to)

	sec := f["security"]
	switch sec {
	case "none":
		params.Set("useStartTLS", "no")
	case "ssl":
		params.Set("encryption", "ssl")
	default:
		// starttls is the default
		params.Set("useStartTLS", "yes")
	}

	if f["subject"] != "" {
		params.Set("subject", f["subject"])
	}

	return fmt.Sprintf("smtp://%s%s:%s/?%s", userinfo, host, port, params.Encode()), nil
}

// pushover://shoutrrr:apiToken@userKey[/?devices=...&title=...&priority=...&sound=...]
func buildPushoverURL(f map[string]string) (string, error) {
	userKey := strings.TrimSpace(f["user_key"])
	apiToken := strings.TrimSpace(f["api_token"])
	if userKey == "" || apiToken == "" {
		return "", fmt.Errorf("User Key and Application Token are required")
	}

	u := fmt.Sprintf("pushover://shoutrrr:%s@%s/", apiToken, userKey)
	params := url.Values{}
	if f["devices"] != "" {
		params.Set("devices", f["devices"])
	}
	if f["title"] != "" {
		params.Set("title", f["title"])
	}
	if f["priority"] != "" && f["priority"] != "0" {
		params.Set("priority", f["priority"])
	}
	if f["sound"] != "" {
		params.Set("sound", f["sound"])
	}
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	return u, nil
}

// gotify://host[:port]/token[?priority=N]
func buildGotifyURL(f map[string]string) (string, error) {
	serverURL := strings.TrimSpace(f["server_url"])
	appToken := strings.TrimSpace(f["app_token"])
	if serverURL == "" || appToken == "" {
		return "", fmt.Errorf("Server URL and Application Token are required")
	}

	// Strip scheme — Shoutrrr uses gotify:// as the scheme
	host := serverURL
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimRight(host, "/")

	u := fmt.Sprintf("gotify://%s/%s", host, appToken)
	if f["priority"] != "" {
		u += "?priority=" + url.QueryEscape(f["priority"])
	}
	return u, nil
}

// signal://host:port/number[?recipients=...]
func buildSignalURL(f map[string]string) (string, error) {
	host := strings.TrimSpace(f["host"])
	number := strings.TrimSpace(f["number"])
	recipients := strings.TrimSpace(f["recipients"])
	if host == "" || number == "" || recipients == "" {
		return "", fmt.Errorf("Host, Sender Number, and Recipients are required")
	}

	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimRight(host, "/")

	params := url.Values{}
	params.Set("recipients", recipients)

	return fmt.Sprintf("signal://%s/%s?%s", host, url.PathEscape(number), params.Encode()), nil
}

// generic+https://example.com/path  or  generic://example.com/path
func buildGenericURL(f map[string]string) (string, error) {
	webhookURL := strings.TrimSpace(f["webhook_url"])
	if webhookURL == "" {
		return "", fmt.Errorf("Webhook URL is required")
	}

	// If user provided a raw URL, convert to generic+scheme format
	if strings.HasPrefix(webhookURL, "generic+") || strings.HasPrefix(webhookURL, "generic://") {
		return webhookURL, nil
	}
	if strings.HasPrefix(webhookURL, "https://") {
		return "generic+" + webhookURL, nil
	}
	if strings.HasPrefix(webhookURL, "http://") {
		return "generic+" + webhookURL, nil
	}
	return "generic+https://" + webhookURL, nil
}
