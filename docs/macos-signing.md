# macOS Code Signing & Notarisation

This guide explains how to set up Apple code signing and notarisation for
`jira-thing` release binaries. Once configured, macOS users can run the binary
without Gatekeeper warnings.

## Overview

Apple requires two things for unsigned binaries to be trusted:

1. **Code signing** — cryptographically signs the binary with a Developer ID
   Application certificate.
2. **Notarisation** — Apple scans the signed binary for malware and issues a
   "ticket" that Gatekeeper accepts.

The CI pipeline handles both steps automatically when the required secrets are
present. If secrets are missing, the pipeline skips signing and produces unsigned
binaries (existing behaviour).

---

## Prerequisites

- An [Apple Developer Program](https://developer.apple.com/programs/)
  membership (£79/year). A free account is **not** sufficient.
- A Mac with Xcode command-line tools installed (`xcode-select --install`).
- Access to the GitHub repository's Settings → Secrets.

---

## Step 1 — Create a Developer ID Application Certificate

1. Open **Keychain Access** on your Mac.
2. From the menu bar: **Keychain Access → Certificate Assistant → Request a
   Certificate from a Certificate Authority**.
3. Enter your email, select **Saved to disk**, and save the CSR file.
4. Go to https://developer.apple.com/account/resources/certificates/add
5. Select **Developer ID Application** and upload your CSR.
6. Download the resulting `.cer` file and double-click to install it into your
   login keychain.

Verify it's installed:

```bash
security find-identity -v -p codesigning
```

You should see something like:

```
1) ABCDEF1234... "Developer ID Application: Your Name (TEAMID)"
```

Note the **Team ID** (the alphanumeric string in parentheses).

---

## Step 2 — Export the Certificate as a .p12 File

1. Open **Keychain Access**.
2. In the **login** keychain, find your "Developer ID Application" certificate.
3. Expand it to reveal the private key underneath.
4. Select **both** the certificate and the private key.
5. Right-click → **Export 2 items…**
6. Save as `certificate.p12` and set a strong password when prompted.

---

## Step 3 — Base64-Encode the .p12

The CI pipeline needs the certificate as a base64 string (GitHub Secrets can't
store binary files directly).

```bash
base64 -i certificate.p12 -o certificate-base64.txt
```

The contents of `certificate-base64.txt` will be pasted into a GitHub Secret.

> **Security:** Delete `certificate.p12` and `certificate-base64.txt` from your
> machine once the secrets are configured. Never commit them to the repository.

---

## Step 4 — Generate an App-Specific Password

Apple's notarisation service requires an app-specific password (your normal
Apple ID password cannot be used in automation).

1. Go to https://appleid.apple.com/account/manage
2. Under **Sign-In and Security**, click **App-Specific Passwords**.
3. Click **Generate an app-specific password**.
4. Give it a label like `jira-thing-ci`.
5. Copy the generated password.

---

## Step 5 — Configure GitHub Repository Secrets

Go to the repository **Settings → Secrets and variables → Actions** and add:

| Secret name | Value |
|---|---|
| `APPLE_CERTIFICATE_BASE64` | Contents of `certificate-base64.txt` from Step 3 |
| `APPLE_CERTIFICATE_PASSWORD` | The password you set when exporting the `.p12` in Step 2 |
| `APPLE_ID` | Your Apple ID email address (the one linked to your developer account) |
| `APPLE_TEAM_ID` | Your 10-character Team ID from Step 1 |
| `APPLE_APP_PASSWORD` | The app-specific password from Step 4 |

---

## Step 6 — Verify

Push a new tag (e.g. `v0.9.3`) and watch the release workflow. The macOS
binaries will now be built on a macOS runner, signed, notarised, and uploaded.

You can verify signing locally after downloading:

```bash
codesign --verify --verbose jira-thing-darwin-arm64
# Should print: valid on disk
# satisfies its Designated Requirement

spctl --assess --verbose jira-thing-darwin-arm64
# Should print: accepted
# source=Notarized Developer ID
```

---

## How the CI Pipeline Works

The release workflow detects whether signing secrets are present:

- **Secrets present:** macOS binaries are built on `macos-latest`, signed with
  `codesign`, notarised with `xcrun notarytool`, and stapled with
  `xcrun stapler` (stapling embeds the notarisation ticket in the binary so it
  works offline).
- **Secrets absent:** macOS binaries are built on `ubuntu-latest` with
  cross-compilation (existing behaviour, unsigned).

This means contributors without access to the signing secrets can still build
and test the workflow — they just produce unsigned binaries.

---

## Troubleshooting

| Problem | Solution |
|---|---|
| `errSecInternalComponent` during codesign | The keychain is locked. The CI step unlocks it, but check the password secret is correct. |
| Notarisation returns "Invalid" | Ensure `--options runtime` (hardened runtime) is passed to `codesign`. Without it Apple rejects the binary. |
| `spctl` says "rejected" after notarisation | Run `xcrun stapler staple <binary>` — stapling may have been skipped. |
| Certificate expired | Certificates last 5 years. Regenerate from the Apple Developer portal and update the GitHub secret. |

---

## Revoking Access

If the certificate or secrets are compromised:

1. Revoke the certificate at https://developer.apple.com/account/resources/certificates/list
2. Revoke the app-specific password at https://appleid.apple.com/account/manage
3. Delete and re-create the GitHub secrets.
