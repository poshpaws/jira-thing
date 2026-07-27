package version

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func withStubbedRelease(t *testing.T, body string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	origClient := HTTPClient
	HTTPClient = srv.Client()
	HTTPClient.Transport = rewriteTransport{base: srv.URL}
	t.Cleanup(func() { HTTPClient = origClient })
}

// withTestSigningKey generates a throwaway ECDSA key pair, installs its public key as
// releaseSigningPubKey, and returns a sign function using the matching private key.
func withTestSigningKey(t *testing.T) (sign func(data []byte) []byte) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating test key: %v", err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshalling test public key: %v", err)
	}

	orig := releaseSigningPubKey
	releaseSigningPubKey = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}))
	t.Cleanup(func() { releaseSigningPubKey = orig })

	return func(data []byte) []byte {
		sum := sha256.Sum256(data)
		sig, err := ecdsa.SignASN1(rand.Reader, priv, sum[:])
		if err != nil {
			t.Fatalf("signing test data: %v", err)
		}
		return sig
	}
}

// assetURLs returns a downloadFn stub serving in-memory content keyed by URL.
func assetURLs(t *testing.T, content map[string][]byte) {
	t.Helper()
	orig := downloadFn
	downloadFn = func(ctx context.Context, url string) (io.ReadCloser, error) {
		data, ok := content[url]
		if !ok {
			return nil, fmt.Errorf("unexpected URL: %s", url)
		}
		return io.NopCloser(strings.NewReader(string(data))), nil
	}
	t.Cleanup(func() { downloadFn = orig })
}

func TestSelfUpdate_DevBuildRejected(t *testing.T) {
	_, err := SelfUpdate(context.Background(), "dev")
	if err == nil || !strings.Contains(err.Error(), "dev build") {
		t.Fatalf("expected dev build error, got: %v", err)
	}
}

func TestCheckUpgrade(t *testing.T) {
	tests := []struct {
		name       string
		current    string
		latest     string
		wantErr    string
		wantUpDate bool
	}{
		{"up to date", "v1.0.0", "v1.0.0", "", true},
		{"upgrade available", "v1.0.0", "v1.1.0", "", false},
		{"downgrade refused", "v1.5.0", "v1.0.0", "refusing to downgrade", false},
		{"invalid latest tag", "v1.0.0", "not-a-version", "not a valid version", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkUpgrade(tt.current, tt.latest)
			switch {
			case tt.wantUpDate:
				if !errors.Is(err, errAlreadyUpToDate) {
					t.Errorf("expected errAlreadyUpToDate, got: %v", err)
				}
			case tt.wantErr != "":
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got: %v", tt.wantErr, err)
				}
			default:
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestSelfUpdate_AlreadyUpToDate(t *testing.T) {
	withStubbedRelease(t, `{"tag_name":"v1.0.0","html_url":"https://example.com"}`)

	msg, err := SelfUpdate(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(msg, "up to date") {
		t.Errorf("expected up to date message, got: %s", msg)
	}
}

func TestSelfUpdate_Downgrade(t *testing.T) {
	withStubbedRelease(t, `{"tag_name":"v1.0.0","html_url":"https://example.com"}`)

	_, err := SelfUpdate(context.Background(), "v2.0.0")
	if err == nil || !strings.Contains(err.Error(), "refusing to downgrade") {
		t.Fatalf("expected downgrade error, got: %v", err)
	}
}

func TestSelfUpdate_NoMatchingAsset(t *testing.T) {
	withStubbedRelease(t, `{"tag_name":"v2.0.0","html_url":"https://example.com","assets":[{"name":"jira-thing-plan9-amd64","browser_download_url":"https://example.com/asset"}]}`)

	_, err := SelfUpdate(context.Background(), "v1.0.0")
	if err == nil || !strings.Contains(err.Error(), "no release asset found") {
		t.Fatalf("expected no-asset error, got: %v", err)
	}
}

func TestSelfUpdate_MissingChecksumsManifest(t *testing.T) {
	withStubbedRelease(t, `{"tag_name":"v2.0.0","html_url":"https://example.com","assets":[{"name":"jira-thing-`+assetSuffix()+`","browser_download_url":"https://example.com/asset"}]}`)

	_, err := SelfUpdate(context.Background(), "v1.0.0")
	if err == nil || !strings.Contains(err.Error(), "checksums.txt") {
		t.Fatalf("expected missing manifest error, got: %v", err)
	}
}

func TestSelfUpdate_BadManifestSignature(t *testing.T) {
	withTestSigningKey(t)
	name := "jira-thing-" + assetSuffix()
	manifestURL := "https://example.com/checksums.txt"
	sigURL := "https://example.com/checksums.txt.sig"
	assetURL := "https://example.com/asset"

	withStubbedRelease(t, fmt.Sprintf(`{"tag_name":"v2.0.0","html_url":"https://example.com","assets":[
		{"name":%q,"browser_download_url":%q},
		{"name":"checksums.txt","browser_download_url":%q},
		{"name":"checksums.txt.sig","browser_download_url":%q}
	]}`, name, assetURL, manifestURL, sigURL))

	assetURLs(t, map[string][]byte{
		manifestURL: []byte("deadbeef  " + name + "\n"),
		sigURL:      []byte("not a valid signature"),
	})

	_, err := SelfUpdate(context.Background(), "v1.0.0")
	if err == nil || !strings.Contains(err.Error(), "verifying release manifest signature") {
		t.Fatalf("expected signature verification error, got: %v", err)
	}
}

func TestSelfUpdate_Success(t *testing.T) {
	sign := withTestSigningKey(t)
	name := "jira-thing-" + assetSuffix()
	manifestURL := "https://example.com/checksums.txt"
	sigURL := "https://example.com/checksums.txt.sig"
	assetURL := "https://example.com/asset"
	binaryContents := "fake binary contents"

	withStubbedRelease(t, fmt.Sprintf(`{"tag_name":"v2.0.0","html_url":"https://example.com","assets":[
		{"name":%q,"browser_download_url":%q},
		{"name":"checksums.txt","browser_download_url":%q},
		{"name":"checksums.txt.sig","browser_download_url":%q}
	]}`, name, assetURL, manifestURL, sigURL))

	sum := sha256.Sum256([]byte(binaryContents))
	manifest := []byte(fmt.Sprintf("%x  %s\n", sum, name))
	sig := sign(manifest)

	assetURLs(t, map[string][]byte{
		manifestURL: manifest,
		sigURL:      sig,
		assetURL:    []byte(binaryContents),
	})

	origApply := applyFn
	defer func() { applyFn = origApply }()
	var applied string
	var appliedChecksum []byte
	applyFn = func(r io.Reader, checksum []byte) error {
		data, err := io.ReadAll(r)
		applied = string(data)
		appliedChecksum = checksum
		return err
	}

	msg, err := SelfUpdate(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if applied != binaryContents {
		t.Errorf("expected apply to receive downloaded bytes, got: %q", applied)
	}
	if fmt.Sprintf("%x", appliedChecksum) != fmt.Sprintf("%x", sum) {
		t.Errorf("expected checksum %x, got %x", sum, appliedChecksum)
	}
	if !strings.Contains(msg, "v1.0.0 → v2.0.0") {
		t.Errorf("expected version transition in message, got: %s", msg)
	}
}

func TestValidateDownloadURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr string
	}{
		{"github.com https", "https://github.com/foo/bar/releases/download/v1/asset", ""},
		{"objects.githubusercontent.com https", "https://objects.githubusercontent.com/asset", ""},
		{"http rejected", "http://github.com/foo", "non-HTTPS"},
		{"untrusted host rejected", "https://evil.example.com/asset", "untrusted host"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDownloadURL(tt.url)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}
