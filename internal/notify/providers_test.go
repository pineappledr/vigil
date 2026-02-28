package notify

import (
	"strings"
	"testing"
)

// ─── BuildShoutrrrURL Tests ─────────────────────────────────────────────

func TestBuildTelegramURL(t *testing.T) {
	fields := map[string]string{
		"bot_token": "123456:ABC-DEF",
		"chat_id":   "@mychannel",
	}
	u, err := BuildShoutrrrURL("telegram", fields)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(u, "telegram://123456:ABC-DEF@telegram?") {
		t.Errorf("unexpected URL: %s", u)
	}
	if !strings.Contains(u, "chats=%40mychannel") {
		t.Errorf("expected encoded chat_id in URL: %s", u)
	}
}

func TestBuildTelegramURL_WithOptions(t *testing.T) {
	fields := map[string]string{
		"bot_token": "tok",
		"chat_id":   "123",
		"silent":    "true",
		"protect":   "true",
		"thread_id": "42",
	}
	u, err := BuildShoutrrrURL("telegram", fields)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(u, "notification=no") {
		t.Errorf("expected notification=no: %s", u)
	}
	if !strings.Contains(u, "protect=yes") {
		t.Errorf("expected protect=yes: %s", u)
	}
	if !strings.Contains(u, "topic=42") {
		t.Errorf("expected topic=42: %s", u)
	}
}

func TestBuildTelegramURL_MissingFields(t *testing.T) {
	_, err := BuildShoutrrrURL("telegram", map[string]string{"bot_token": "tok"})
	if err == nil {
		t.Fatal("expected error for missing chat_id")
	}
}

func TestBuildDiscordURL(t *testing.T) {
	fields := map[string]string{
		"webhook_url": "https://discord.com/api/webhooks/123456/abcdef-token",
	}
	u, err := BuildShoutrrrURL("discord", fields)
	if err != nil {
		t.Fatal(err)
	}
	if u != "discord://abcdef-token@123456" {
		t.Errorf("unexpected URL: %s", u)
	}
}

func TestBuildDiscordURL_WithOptions(t *testing.T) {
	fields := map[string]string{
		"webhook_url": "https://discord.com/api/webhooks/111/tok",
		"username":    "Vigil Bot",
	}
	u, err := BuildShoutrrrURL("discord", fields)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(u, "username=Vigil+Bot") {
		t.Errorf("expected username param: %s", u)
	}
}

func TestBuildDiscordURL_TrailingSlash(t *testing.T) {
	fields := map[string]string{
		"webhook_url": "https://discord.com/api/webhooks/123/tok/",
	}
	u, err := BuildShoutrrrURL("discord", fields)
	if err != nil {
		t.Fatal(err)
	}
	if u != "discord://tok@123" {
		t.Errorf("unexpected URL: %s", u)
	}
}

func TestBuildSlackURL(t *testing.T) {
	fields := map[string]string{
		"webhook_url": "https://hooks.slack.com/services/TAAA/BBBB/CCCC",
	}
	u, err := BuildShoutrrrURL("slack", fields)
	if err != nil {
		t.Fatal(err)
	}
	if u != "slack://TAAA/BBBB/CCCC" {
		t.Errorf("unexpected URL: %s", u)
	}
}

func TestBuildSlackURL_WithOptions(t *testing.T) {
	fields := map[string]string{
		"webhook_url": "https://hooks.slack.com/services/T/B/C",
		"bot_name":    "Vigil",
		"channel":     "#alerts",
	}
	u, err := BuildShoutrrrURL("slack", fields)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(u, "botname=Vigil") {
		t.Errorf("expected botname: %s", u)
	}
	if !strings.Contains(u, "channel=%23alerts") {
		t.Errorf("expected channel: %s", u)
	}
}

func TestBuildEmailURL(t *testing.T) {
	fields := map[string]string{
		"host":     "smtp.gmail.com",
		"port":     "587",
		"username": "user@gmail.com",
		"password": "secret",
		"from":     "vigil@example.com",
		"to":       "admin@example.com",
		"security": "starttls",
	}
	u, err := BuildShoutrrrURL("email", fields)
	if err != nil {
		t.Fatal(err)
	}
	// url.PathEscape may or may not encode @ depending on Go version
	if !strings.Contains(u, "smtp://") || !strings.Contains(u, "gmail.com:secret@smtp.gmail.com:587/") {
		t.Errorf("unexpected URL: %s", u)
	}
	if !strings.Contains(u, "from=vigil%40example.com") {
		t.Errorf("expected from param: %s", u)
	}
	if !strings.Contains(u, "to=admin%40example.com") {
		t.Errorf("expected to param: %s", u)
	}
	if !strings.Contains(u, "useStartTLS=yes") {
		t.Errorf("expected useStartTLS=yes: %s", u)
	}
}

func TestBuildEmailURL_SSL(t *testing.T) {
	fields := map[string]string{
		"host": "mail.example.com", "port": "465",
		"from": "a@b.com", "to": "c@d.com",
		"security": "ssl",
	}
	u, err := BuildShoutrrrURL("email", fields)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(u, "encryption=ssl") {
		t.Errorf("expected encryption=ssl: %s", u)
	}
}

func TestBuildEmailURL_NoAuth(t *testing.T) {
	fields := map[string]string{
		"host": "localhost", "port": "25",
		"from": "a@b.com", "to": "c@d.com",
		"security": "none",
	}
	u, err := BuildShoutrrrURL("email", fields)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(u, "smtp://localhost:25/") {
		t.Errorf("expected no userinfo: %s", u)
	}
	if !strings.Contains(u, "useStartTLS=no") {
		t.Errorf("expected useStartTLS=no: %s", u)
	}
}

func TestBuildPushoverURL(t *testing.T) {
	fields := map[string]string{
		"user_key":  "ukey123",
		"api_token": "atok456",
	}
	u, err := BuildShoutrrrURL("pushover", fields)
	if err != nil {
		t.Fatal(err)
	}
	if u != "pushover://shoutrrr:atok456@ukey123/" {
		t.Errorf("unexpected URL: %s", u)
	}
}

func TestBuildPushoverURL_WithOptions(t *testing.T) {
	fields := map[string]string{
		"user_key":  "u",
		"api_token": "a",
		"devices":   "phone",
		"priority":  "1",
		"sound":     "siren",
		"title":     "Alert!",
	}
	u, err := BuildShoutrrrURL("pushover", fields)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(u, "devices=phone") {
		t.Errorf("expected devices: %s", u)
	}
	if !strings.Contains(u, "priority=1") {
		t.Errorf("expected priority: %s", u)
	}
	if !strings.Contains(u, "sound=siren") {
		t.Errorf("expected sound: %s", u)
	}
	if !strings.Contains(u, "title=Alert") {
		t.Errorf("expected title: %s", u)
	}
}

func TestBuildGotifyURL(t *testing.T) {
	fields := map[string]string{
		"server_url": "https://gotify.example.com",
		"app_token":  "tok123",
	}
	u, err := BuildShoutrrrURL("gotify", fields)
	if err != nil {
		t.Fatal(err)
	}
	if u != "gotify://gotify.example.com/tok123" {
		t.Errorf("unexpected URL: %s", u)
	}
}

func TestBuildGotifyURL_WithPriority(t *testing.T) {
	fields := map[string]string{
		"server_url": "http://localhost:8080",
		"app_token":  "t",
		"priority":   "8",
	}
	u, err := BuildShoutrrrURL("gotify", fields)
	if err != nil {
		t.Fatal(err)
	}
	if u != "gotify://localhost:8080/t?priority=8" {
		t.Errorf("unexpected URL: %s", u)
	}
}

func TestBuildGenericURL_HTTPS(t *testing.T) {
	fields := map[string]string{
		"webhook_url": "https://example.com/hook",
	}
	u, err := BuildShoutrrrURL("generic", fields)
	if err != nil {
		t.Fatal(err)
	}
	if u != "generic+https://example.com/hook" {
		t.Errorf("unexpected URL: %s", u)
	}
}

func TestBuildGenericURL_AlreadyPrefixed(t *testing.T) {
	fields := map[string]string{
		"webhook_url": "generic+https://example.com/hook",
	}
	u, err := BuildShoutrrrURL("generic", fields)
	if err != nil {
		t.Fatal(err)
	}
	if u != "generic+https://example.com/hook" {
		t.Errorf("unexpected URL: %s", u)
	}
}

func TestBuildGenericURL_BareDomain(t *testing.T) {
	fields := map[string]string{
		"webhook_url": "example.com/webhook",
	}
	u, err := BuildShoutrrrURL("generic", fields)
	if err != nil {
		t.Fatal(err)
	}
	if u != "generic+https://example.com/webhook" {
		t.Errorf("unexpected URL: %s", u)
	}
}

func TestBuildShoutrrrURL_UnknownProvider(t *testing.T) {
	_, err := BuildShoutrrrURL("unknown", map[string]string{})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

// ─── ValidateFields Tests ───────────────────────────────────────────────

func TestValidateFields_OK(t *testing.T) {
	err := ValidateFields("telegram", map[string]string{
		"bot_token": "tok", "chat_id": "123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateFields_MissingRequired(t *testing.T) {
	err := ValidateFields("telegram", map[string]string{
		"bot_token": "tok",
	})
	if err == nil {
		t.Fatal("expected error for missing chat_id")
	}
	if !strings.Contains(err.Error(), "Chat ID") {
		t.Errorf("expected error to mention Chat ID: %v", err)
	}
}

func TestValidateFields_UnknownProvider(t *testing.T) {
	err := ValidateFields("badprovider", map[string]string{})
	if err == nil {
		t.Fatal("expected error")
	}
}

// ─── MaskSecrets Tests ──────────────────────────────────────────────────

func TestMaskSecrets(t *testing.T) {
	fields := map[string]string{
		"bot_token": "super-secret",
		"chat_id":   "123",
	}
	masked := MaskSecrets("telegram", fields)

	if masked["bot_token"] != SecretMask {
		t.Errorf("expected bot_token masked, got %q", masked["bot_token"])
	}
	if masked["chat_id"] != "123" {
		t.Errorf("expected chat_id unchanged, got %q", masked["chat_id"])
	}
	// Original should be unmodified
	if fields["bot_token"] != "super-secret" {
		t.Error("original map was mutated")
	}
}

func TestMaskSecrets_EmptyNotMasked(t *testing.T) {
	fields := map[string]string{
		"bot_token": "",
		"chat_id":   "123",
	}
	masked := MaskSecrets("telegram", fields)
	if masked["bot_token"] != "" {
		t.Errorf("empty field should stay empty, got %q", masked["bot_token"])
	}
}

func TestMaskSecrets_UnknownProvider(t *testing.T) {
	fields := map[string]string{"key": "val"}
	masked := MaskSecrets("unknown", fields)
	if masked["key"] != "val" {
		t.Error("unknown provider should return fields unchanged")
	}
}

func TestMaskSecrets_Pushover(t *testing.T) {
	fields := map[string]string{
		"user_key":  "ukey",
		"api_token": "atok",
		"title":     "Alert",
	}
	masked := MaskSecrets("pushover", fields)
	if masked["user_key"] != SecretMask {
		t.Errorf("expected user_key masked: %q", masked["user_key"])
	}
	if masked["api_token"] != SecretMask {
		t.Errorf("expected api_token masked: %q", masked["api_token"])
	}
	if masked["title"] != "Alert" {
		t.Errorf("expected title unchanged: %q", masked["title"])
	}
}

// ─── GetProviderDefs Tests ──────────────────────────────────────────────

func TestGetProviderDefs(t *testing.T) {
	defs := GetProviderDefs()
	expected := []string{"telegram", "discord", "slack", "email", "pushover", "gotify", "generic"}
	for _, key := range expected {
		if _, ok := defs[key]; !ok {
			t.Errorf("missing provider: %s", key)
		}
	}
}

func TestGetProviderDef_Found(t *testing.T) {
	def, ok := GetProviderDef("discord")
	if !ok {
		t.Fatal("expected to find discord")
	}
	if def.Label != "Discord" {
		t.Errorf("unexpected label: %s", def.Label)
	}
}

func TestGetProviderDef_NotFound(t *testing.T) {
	_, ok := GetProviderDef("nonexistent")
	if ok {
		t.Fatal("expected not found")
	}
}
