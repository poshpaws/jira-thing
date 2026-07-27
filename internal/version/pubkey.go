package version

// releaseSigningPubKey verifies the checksums.txt manifest published with each
// release (see .github/workflows/release.yml). The matching private key is
// stored as the RELEASE_SIGNING_KEY GitHub Actions secret.
// It is a var, not a const, so tests can substitute a throwaway test key.
var releaseSigningPubKey = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEmkvWe2/vniF1NV+fytQ+H5qYElku
PueBJiGZ8him59sFI6+AWZgZSBRq48pyz6VS+2Nw3b7eVGDqjKdMtLhk0Q==
-----END PUBLIC KEY-----
`
