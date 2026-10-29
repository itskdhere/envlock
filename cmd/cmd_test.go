package cmd

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/itskdhere/envlock/pkg/crypto"
	"github.com/itskdhere/envlock/pkg/env"
	"github.com/itskdhere/envlock/pkg/vault"
)

// run execs or exits, so the test binary re-runs itself as envlock and as the command.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "print-leak" {
		fmt.Print(os.Getenv("LEAK"))
		os.Exit(0)
	}
	if os.Getenv("ENVLOCK_TEST_RUN") != "" {
		rootCmd.SetArgs([]string{"run", "--", os.Args[0], "print-leak"})
		os.Exit(Execute())
	}
	os.Exit(m.Run())
}

// inTempProject isolates cwd, config dir, flags, key env vars, and stdin.
func inTempProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("ENVLOCK_KEY", "")
	t.Setenv("ENVLOCK_KEY_PATH", "")
	config := t.TempDir()
	for _, v := range []string{"APPDATA", "XDG_CONFIG_HOME", "HOME"} {
		t.Setenv(v, config)
	}
	withStdin(t, "")
	flagEnv, flagFile, flagKeyFile = "", "", ""
	flagKeygenGlobal, flagKeygenOut, flagKeygenStdout, flagKeygenForce = false, "", false, false
	flagPullForce, flagRunNoOverride, flagPushForce, flagRecipientForce = false, false, false, false
	return dir
}

func withStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe failed: %v", err)
	}
	fmt.Fprint(w, input)
	w.Close()
	stdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = stdin; r.Close() })
}

func TestEndToEndFidelity(t *testing.T) {
	tempDir := inTempProject(t)

	// 1. Complex raw .env data preserving whitespace, comments, multiline, unicode, and quoting
	originalEnv := []byte(`# Core Configuration 🚀
APP_NAME="EnvLock Vault / Test"
PORT=8080

# Multi-line Certificate
PUBLIC_CERT="-----BEGIN CERTIFICATE-----
MIIDXTCCAkWgAwIBAgIJAL...
-----END CERTIFICATE-----"

# Quotes & Special Characters
DB_URI='postgresql://admin:p@$$w0rd!#%^&*()@localhost:5432/db?sslmode=require'
EMPTY_VAL=
TRAILING_WHITESPACE=value_with_spaces   
JSON_PAYLOAD={"key": "value", "active": true}

# Final trailing comment
`)

	envPath := filepath.Join(tempDir, ".env")
	lockPath := filepath.Join(tempDir, "env.lock")

	if err := env.WriteEnvFile(envPath, originalEnv); err != nil {
		t.Fatalf("failed to write initial .env: %v", err)
	}

	// 2. Run keygen
	if err := keygenCmd.RunE(keygenCmd, nil); err != nil {
		t.Fatalf("keygen failed: %v", err)
	}

	// Verify key files generated
	if _, err := os.Stat("envlock.key"); err != nil {
		t.Fatalf("envlock.key not found: %v", err)
	}
	if _, err := os.Stat("envlock.pub"); err != nil {
		t.Fatalf("envlock.pub not found: %v", err)
	}

	// 3. Run push
	if err := pushCmd.RunE(pushCmd, nil); err != nil {
		t.Fatalf("push failed: %v", err)
	}

	// Verify env.lock exists and validates against schema
	envelope, err := vault.ReadVault(lockPath)
	if err != nil {
		t.Fatalf("failed to read generated vault: %v", err)
	}
	if envelope.Version != 1 || envelope.Cipher != "AES-256-GCM" || len(envelope.Recipients) != 1 {
		t.Fatalf("unexpected envelope content: %+v", envelope)
	}

	// 4. Delete local .env
	if err := os.Remove(envPath); err != nil {
		t.Fatalf("failed to remove .env: %v", err)
	}

	// 5. Run pull
	if err := pullCmd.RunE(pullCmd, nil); err != nil {
		t.Fatalf("pull failed: %v", err)
	}

	// 6. Verify byte-for-byte exact fidelity
	decryptedBytes, err := env.ReadEnvFile(envPath)
	if err != nil {
		t.Fatalf("failed to read restored .env: %v", err)
	}

	if !bytes.Equal(originalEnv, decryptedBytes) {
		t.Fatalf("byte-for-byte fidelity failed!\nExpected:\n%s\nGot:\n%s", string(originalEnv), string(decryptedBytes))
	}
}

func TestMultiEnvironmentWorkflow(t *testing.T) {
	tempDir := inTempProject(t)

	// Generate key
	_ = keygenCmd.RunE(keygenCmd, nil)

	// Create .env.staging
	stagingEnv := []byte("ENVIRONMENT=staging\nDEBUG=true\n")
	stagingFile := filepath.Join(tempDir, ".env.staging")
	stagingLock := filepath.Join(tempDir, "env.staging.lock")
	_ = env.WriteEnvFile(stagingFile, stagingEnv)

	// Push staging
	flagEnv = "staging"
	if err := pushCmd.RunE(pushCmd, nil); err != nil {
		t.Fatalf("push staging failed: %v", err)
	}

	if _, err := os.Stat(stagingLock); err != nil {
		t.Fatalf("staging lockfile not created: %v", err)
	}

	// Remove staging .env and pull
	_ = os.Remove(stagingFile)
	if err := pullCmd.RunE(pullCmd, nil); err != nil {
		t.Fatalf("pull staging failed: %v", err)
	}

	restored, err := env.ReadEnvFile(stagingFile)
	if err != nil {
		t.Fatalf("failed to read restored staging env: %v", err)
	}

	if !bytes.Equal(stagingEnv, restored) {
		t.Fatalf("staging env mismatch")
	}
}

func TestPullOverwriteProtection(t *testing.T) {
	tempDir := inTempProject(t)

	_ = keygenCmd.RunE(keygenCmd, nil)

	initialContent := []byte("FOO=bar\n")
	envPath := filepath.Join(tempDir, ".env")
	_ = env.WriteEnvFile(envPath, initialContent)

	if err := pushCmd.RunE(pushCmd, nil); err != nil {
		t.Fatalf("push failed: %v", err)
	}

	// Modify local file
	modifiedContent := []byte("FOO=bar_modified\n")
	_ = env.WriteEnvFile(envPath, modifiedContent)

	// Pull with force = true should overwrite modified local file
	flagPullForce = true
	if err := pullCmd.RunE(pullCmd, nil); err != nil {
		t.Fatalf("pull with force failed: %v", err)
	}

	readBack, _ := env.ReadEnvFile(envPath)
	if !bytes.Equal(readBack, initialContent) {
		t.Fatalf("expected pull --force to overwrite with vault content")
	}
}

func TestRecipientListIsAuthenticated(t *testing.T) {
	tempDir := inTempProject(t)

	priv, _, _ := crypto.GenerateKeyPair()
	t.Setenv("ENVLOCK_KEY", crypto.FormatPrivateKey(priv))
	_ = env.WriteEnvFile(filepath.Join(tempDir, ".env"), []byte("FOO=bar\n"))
	if err := pushCmd.RunE(pushCmd, nil); err != nil {
		t.Fatalf("push failed: %v", err)
	}
	if err := pullCmd.RunE(pullCmd, nil); err != nil {
		t.Fatalf("pull of untouched vault failed: %v", err)
	}

	envelope, _ := vault.ReadVault("env.lock")
	_, attackerPub, _ := crypto.GenerateKeyPair()
	envelope.Recipients = append(envelope.Recipients, vault.RecipientEntry{PubKey: crypto.FormatPublicKey(attackerPub)})
	_ = vault.WriteVault("env.lock", envelope)

	if err := pullCmd.RunE(pullCmd, nil); err == nil {
		t.Fatal("pull must reject a vault whose recipient list was edited")
	}
	if err := pushCmd.RunE(pushCmd, nil); err == nil {
		t.Fatal("push must not re-encrypt for a hand-added recipient")
	}
}

func TestFirstPushEncryptsForOwnKey(t *testing.T) {
	tempDir := inTempProject(t)

	priv, _, _ := crypto.GenerateKeyPair()
	_, strayPub, _ := crypto.GenerateKeyPair()
	t.Setenv("ENVLOCK_KEY", crypto.FormatPrivateKey(priv))
	_ = os.WriteFile("envlock.pub", []byte(crypto.FormatPublicKey(strayPub)), 0644)
	_ = env.WriteEnvFile(filepath.Join(tempDir, ".env"), []byte("FOO=bar\n"))
	if err := pushCmd.RunE(pushCmd, nil); err != nil {
		t.Fatalf("push failed: %v", err)
	}
	if err := pullCmd.RunE(pullCmd, nil); err != nil {
		t.Fatalf("first push must encrypt for the private key in use, not envlock.pub: %v", err)
	}
}

func TestRecipientAddRemove(t *testing.T) {
	tempDir := inTempProject(t)

	owner, _, _ := crypto.GenerateKeyPair()
	mate, matePub, _ := crypto.GenerateKeyPair()
	ownerKey, mateKey := crypto.FormatPrivateKey(owner), crypto.FormatPrivateKey(mate)
	matePubStr := crypto.FormatPublicKey(matePub)
	plaintext := []byte("FOO=bar\n")

	t.Setenv("ENVLOCK_KEY", ownerKey)
	_ = env.WriteEnvFile(filepath.Join(tempDir, ".env"), plaintext)
	if err := pushCmd.RunE(pushCmd, nil); err != nil {
		t.Fatalf("push failed: %v", err)
	}

	if err := recipientAddCmd.RunE(recipientAddCmd, []string{"el_pub_nothex"}); err == nil {
		t.Fatal("add must reject a malformed public key")
	}
	if err := recipientAddCmd.RunE(recipientAddCmd, []string{matePubStr}); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if err := recipientAddCmd.RunE(recipientAddCmd, []string{matePubStr}); err == nil {
		t.Fatal("add must reject an existing recipient")
	}

	t.Setenv("ENVLOCK_KEY", mateKey)
	if got, err := decryptVault("env.lock"); err != nil || !bytes.Equal(got, plaintext) {
		t.Fatalf("added recipient must decrypt the vault: %q, %v", got, err)
	}
	before, _ := vault.ReadVault("env.lock")
	ownerEph, _ := crypto.ParsePublicKey(before.Recipients[0].EphemeralPubKey)
	ownerDEK, err := crypto.UnwrapDEK(before.Recipients[0].WrappedDEK, ownerEph, owner)
	if err != nil {
		t.Fatalf("owner's DEK must unwrap before removal: %v", err)
	}

	if err := recipientRemoveCmd.RunE(recipientRemoveCmd, []string{crypto.FormatPublicKey(owner.PublicKey())}); err != nil {
		t.Fatalf("any recipient must be able to remove another: %v", err)
	}
	if err := recipientRemoveCmd.RunE(recipientRemoveCmd, []string{matePubStr}); err == nil {
		t.Fatal("remove must refuse to drop the last recipient")
	}
	after, _ := vault.ReadVault("env.lock")
	iv, _ := hex.DecodeString(after.IV)
	ct, _ := hex.DecodeString(after.Ciphertext)
	if _, err := crypto.DecryptPayload(ct, iv, ownerDEK, after.PayloadAAD()); err == nil {
		t.Fatal("remove must re-encrypt under a fresh DEK the removed key never saw")
	}

	t.Setenv("ENVLOCK_KEY", ownerKey)
	if err := pullCmd.RunE(pullCmd, nil); err == nil {
		t.Fatal("removed recipient must not decrypt the vault")
	}
	if err := recipientAddCmd.RunE(recipientAddCmd, []string{crypto.FormatPublicKey(owner.PublicKey())}); err == nil {
		t.Fatal("a non-recipient must not be able to add keys")
	}
}

func TestRemoveDropsEveryCopyOfAKey(t *testing.T) {
	inTempProject(t)

	owner, _, _ := crypto.GenerateKeyPair()
	mal, malPub, _ := crypto.GenerateKeyPair()
	malPubStr := crypto.FormatPublicKey(malPub)
	malUpper := crypto.PubKeyPrefix + strings.ToUpper(strings.TrimPrefix(malPubStr, crypto.PubKeyPrefix))

	keys := []string{crypto.FormatPublicKey(owner.PublicKey()), malPubStr, malPubStr, malUpper}
	if err := sealVault("env.lock", []byte("FOO=bar\n"), keys); err != nil {
		t.Fatalf("seal failed: %v", err)
	}

	t.Setenv("ENVLOCK_KEY", crypto.FormatPrivateKey(owner))
	if err := recipientRemoveCmd.RunE(recipientRemoveCmd, []string{malPubStr}); err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	after, _ := vault.ReadVault("env.lock")
	for _, r := range after.Recipients {
		eph, _ := crypto.ParsePublicKey(r.EphemeralPubKey)
		if _, err := crypto.UnwrapDEK(r.WrappedDEK, eph, mal); err == nil {
			t.Fatalf("removed key still unwraps the DEK via %q", r.PubKey)
		}
	}

	rootCmd.SetArgs([]string{"recipient", "remvoe", malPubStr})
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("a mistyped recipient subcommand must fail")
	}
}

func TestPushFlagsARestoredRecipient(t *testing.T) {
	tempDir := inTempProject(t)

	owner, _, _ := crypto.GenerateKeyPair()
	_, matePub, _ := crypto.GenerateKeyPair()
	matePubStr := crypto.FormatPublicKey(matePub)
	t.Setenv("ENVLOCK_KEY", crypto.FormatPrivateKey(owner))
	_ = env.WriteEnvFile(filepath.Join(tempDir, ".env"), []byte("FOO=bar\n"))
	if err := pushCmd.RunE(pushCmd, nil); err != nil {
		t.Fatalf("push failed: %v", err)
	}
	if err := recipientAddCmd.RunE(recipientAddCmd, []string{matePubStr}); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	withMate, _ := os.ReadFile("env.lock")
	if err := recipientRemoveCmd.RunE(recipientRemoveCmd, []string{matePubStr}); err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	_ = os.WriteFile("env.lock", withMate, 0644)
	stderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	_, err := decryptVault("env.lock")
	os.Stderr = stderr
	w.Close()
	if warning, _ := io.ReadAll(r); err != nil || !bytes.Contains(warning, []byte(matePubStr)) {
		t.Fatalf("pull must warn about a removed key that came back: %v %q", err, warning)
	}
	withStdin(t, "n\n")
	if err := pushCmd.RunE(pushCmd, nil); err == nil {
		t.Fatal("push must not silently re-encrypt for a removed key that came back")
	}
	_, newPub, _ := crypto.GenerateKeyPair()
	if err := recipientAddCmd.RunE(recipientAddCmd, []string{crypto.FormatPublicKey(newPub)}); err == nil {
		t.Fatal("add must not silently keep a removed key that came back")
	}
	withStdin(t, "y\n")
	if err := pushCmd.RunE(pushCmd, nil); err != nil {
		t.Fatalf("push must continue once the key is confirmed: %v", err)
	}
	if err := pushCmd.RunE(pushCmd, nil); err != nil {
		t.Fatalf("a confirmed key must not be flagged again: %v", err)
	}
}

func TestRecipientCommandsFollowEnvFlag(t *testing.T) {
	inTempProject(t)

	owner, _, _ := crypto.GenerateKeyPair()
	_, matePub, _ := crypto.GenerateKeyPair()
	t.Setenv("ENVLOCK_KEY", crypto.FormatPrivateKey(owner))
	flagEnv = "prod"
	_ = env.WriteEnvFile(".env.prod", []byte("FOO=bar\n"))
	if err := pushCmd.RunE(pushCmd, nil); err != nil {
		t.Fatalf("push failed: %v", err)
	}
	if err := recipientAddCmd.RunE(recipientAddCmd, []string{crypto.FormatPublicKey(matePub)}); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if v, err := vault.ReadVault("env.prod.lock"); err != nil || len(v.Recipients) != 2 {
		t.Fatalf("add must re-encrypt env.prod.lock: %v", err)
	}
	if _, err := os.Stat("env.lock"); !os.IsNotExist(err) {
		t.Fatal("-e prod must not touch env.lock")
	}
}

func TestRemovingYourOwnKeyAsks(t *testing.T) {
	inTempProject(t)

	owner, _, _ := crypto.GenerateKeyPair()
	_, matePub, _ := crypto.GenerateKeyPair()
	ownerPubStr := crypto.FormatPublicKey(owner.PublicKey())
	if err := sealVault("env.lock", []byte("FOO=bar\n"), []string{ownerPubStr, crypto.FormatPublicKey(matePub)}); err != nil {
		t.Fatalf("seal failed: %v", err)
	}

	t.Setenv("ENVLOCK_KEY", crypto.FormatPrivateKey(owner))
	withStdin(t, "n\n")
	if err := recipientRemoveCmd.RunE(recipientRemoveCmd, []string{ownerPubStr}); err == nil {
		t.Fatal("removing your own key must ask first")
	}
	flagRecipientForce = true
	if err := recipientRemoveCmd.RunE(recipientRemoveCmd, []string{ownerPubStr}); err != nil {
		t.Fatalf("--force must skip the question: %v", err)
	}
	if _, err := decryptVault("env.lock"); err == nil {
		t.Fatal("your key must be gone after confirming")
	}
}

func TestExistingVaultIsNotClobbered(t *testing.T) {
	tempDir := inTempProject(t)

	priv, _, _ := crypto.GenerateKeyPair()
	t.Setenv("ENVLOCK_KEY", crypto.FormatPrivateKey(priv))
	_ = os.WriteFile("env.lock", []byte("not json"), 0644)

	if err := initCmd.RunE(initCmd, nil); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if _, err := os.Stat(".env"); !os.IsNotExist(err) {
		t.Fatal("init must not create a template .env when env.lock exists")
	}

	_ = env.WriteEnvFile(filepath.Join(tempDir, ".env"), []byte("FOO=bar\n"))
	if err := pushCmd.RunE(pushCmd, nil); err == nil {
		t.Fatal("push must not overwrite an unreadable env.lock")
	}
	if data, _ := os.ReadFile("env.lock"); string(data) != "not json" {
		t.Fatal("env.lock was modified")
	}
}

func TestNotGitIgnored(t *testing.T) {
	if _, err := osexec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	key := filepath.Join(dir, "envlock.key")

	if notGitIgnored(key) {
		t.Fatal("a key outside any git repository must not warn")
	}
	_ = osexec.Command("git", "-C", dir, "init", "-q").Run()
	if !notGitIgnored(key) {
		t.Fatal("an unignored key inside a git repository must warn")
	}
	_ = os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.key\n"), 0644)
	if notGitIgnored(key) {
		t.Fatal("an ignored key must not warn")
	}
}

func TestKeyResolutionWithEnvVar(t *testing.T) {
	priv, _, _ := crypto.GenerateKeyPair()
	privStr := crypto.FormatPrivateKey(priv)

	t.Setenv("ENVLOCK_KEY", privStr)

	resolved, err := ResolvePrivateKey("")
	if err != nil {
		t.Fatalf("ResolvePrivateKey with ENVLOCK_KEY failed: %v", err)
	}

	if resolved != privStr {
		t.Fatalf("resolved key mismatch: got %s, expected %s", resolved, privStr)
	}

	filePriv, _, _ := crypto.GenerateKeyPair()
	keyFile := filepath.Join(t.TempDir(), "flag.key")
	_ = os.WriteFile(keyFile, []byte(crypto.FormatPrivateKey(filePriv)), 0600)
	if resolved, _ := ResolvePrivateKey(keyFile); resolved != crypto.FormatPrivateKey(filePriv) {
		t.Fatal("--key-file must take precedence over ENVLOCK_KEY")
	}
}

func TestKeyFromStdinLeavesRestUnread(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe failed: %v", err)
	}
	stdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = stdin })

	priv, _, _ := crypto.GenerateKeyPair()
	fmt.Fprintf(w, "\xef\xbb\xbf%s\ny\n", crypto.FormatPrivateKey(priv))
	w.Close()

	if key, err := ResolvePrivateKey("-"); err != nil || key != crypto.FormatPrivateKey(priv) {
		t.Fatalf("ResolvePrivateKey(\"-\") = %q, %v", key, err)
	}
	if rest, _ := io.ReadAll(r); string(rest) != "y\n" {
		t.Fatalf("stdin after the key = %q, want %q", rest, "y\n")
	}
}

func TestRunExitCodesAndKeyStripping(t *testing.T) {
	tempDir := inTempProject(t)

	var exitErr *exitError
	if err := runCmd.RunE(runCmd, []string{"envlock-no-such-cmd"}); !errors.As(err, &exitErr) || exitErr.code != 125 {
		t.Fatalf("missing vault: expected exit 125, got %v", err)
	}

	priv, _, _ := crypto.GenerateKeyPair()
	t.Setenv("ENVLOCK_KEY", crypto.FormatPrivateKey(priv))
	_ = env.WriteEnvFile(filepath.Join(tempDir, ".env"), []byte("FOO=bar\nLEAK=${ENVLOCK_KEY}\n"))
	if err := pushCmd.RunE(pushCmd, nil); err != nil {
		t.Fatalf("push failed: %v", err)
	}

	if err := runCmd.RunE(runCmd, []string{"envlock-no-such-cmd"}); !errors.As(err, &exitErr) || exitErr.code != 127 {
		t.Fatalf("missing command: expected exit 127, got %v", err)
	}
	if os.Getenv("ENVLOCK_KEY") != "" {
		t.Fatal("ENVLOCK_KEY must not be passed on to the command")
	}

	child := osexec.Command(os.Args[0])
	child.Dir = tempDir
	child.Env = append(os.Environ(), "ENVLOCK_TEST_RUN=1", "ENVLOCK_KEY="+crypto.FormatPrivateKey(priv))
	out, err := child.Output()
	if err != nil {
		t.Fatalf("run subprocess failed: %v", err)
	}
	if string(out) != "${ENVLOCK_KEY}" {
		t.Fatalf("vault value ${ENVLOCK_KEY} must not expand to the key, got %q", out)
	}
}

func TestNewerVersion(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v0.5.0", "0.4.0", true},
		{"v0.10.0", "v0.9.1", true},
		{"v1.0.0", "0.99.99", true},
		{"v0.4.0", "0.4.0", false},
		{"v0.4.0", "0.5.0", false},
		{"v0.5.0", "v0.4.0-3-g8ea25f6", false},
		{"", "0.4.0", false},
		{"v0.5.0\x1b[2J", "0.4.0", false},
		{"v+1.0.0", "0.4.0", false},
	}
	for _, c := range cases {
		if got := newerVersion(c.latest, c.current); got != c.want {
			t.Errorf("newerVersion(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestCommandFlagMerging(t *testing.T) {
	commands := [][]string{
		{"--help"},
		{"keygen", "--help"},
		{"pull", "--help"},
		{"push", "--help"},
		{"init", "--help"},
		{"run", "--help"},
		{"recipient", "--help"},
		{"recipient", "add", "--help"},
		{"version"},
	}

	for _, args := range commands {
		rootCmd.SetArgs(args)
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("rootCmd.Execute(%v) failed: %v", args, err)
		}
	}
}
