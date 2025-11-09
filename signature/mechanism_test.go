package signature

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testGPGHomeDirectory = "./fixtures"
)

func TestNewGPGSigningMechanism(t *testing.T) {
	// A dumb test just for code coverage. We test more with newGPGSigningMechanismInDirectory().
	mech, err := NewGPGSigningMechanism()
	assert.NoError(t, err)
	mech.Close()
}

func TestNewGPGSigningMechanismInDirectory(t *testing.T) {
	// A dumb test just for code coverage.
	mech, err := newGPGSigningMechanismInDirectory(testGPGHomeDirectory)
	assert.NoError(t, err)
	mech.Close()
	// The various GPG failure cases are not obviously easy to reach.
}

func TestNewEphemeralGPGSigningMechanism(t *testing.T) {
	// Try validating a signature when the key is unknown.
	mech, keyIdentities, err := NewEphemeralGPGSigningMechanism([]byte{})
	require.NoError(t, err)
	defer mech.Close()
	assert.Empty(t, keyIdentities)
	signature, err := ioutil.ReadFile("./fixtures/invalid-blob.signature")
	require.NoError(t, err)
	content, signingFingerprint, err := mech.Verify(signature)
	require.Error(t, err)

	// Successful import
	keyBlob, err := ioutil.ReadFile("./fixtures/public-key.gpg")
	require.NoError(t, err)
	mech, keyIdentities, err = NewEphemeralGPGSigningMechanism(keyBlob)
	require.NoError(t, err)
	defer mech.Close()
	assert.Equal(t, []string{TestKeyFingerprint}, keyIdentities)
	// After import, the signature should validate.
	content, signingFingerprint, err = mech.Verify(signature)
	require.NoError(t, err)
	assert.Equal(t, []byte("This is not JSON\n"), content)
	assert.Equal(t, TestKeyFingerprint, signingFingerprint)

	// Two keys: just concatenate the valid input twice.
	mech, keyIdentities, err = NewEphemeralGPGSigningMechanism(bytes.Join([][]byte{keyBlob, keyBlob}, nil))
	require.NoError(t, err)
	defer mech.Close()
	assert.Equal(t, []string{TestKeyFingerprint, TestKeyFingerprint}, keyIdentities)

	// Invalid input: This is accepted anyway by GPG, just returns no keys.
	mech, keyIdentities, err = NewEphemeralGPGSigningMechanism([]byte("This is invalid"))
	require.NoError(t, err)
	defer mech.Close()
	assert.Equal(t, []string{}, keyIdentities)
	// The various GPG/GPGME failures cases are not obviously easy to reach.
}

func TestGPGSigningMechanismClose(t *testing.T) {
	// Closing a non-ephemeral mechanism does not remove anything in the directory.
	mech, err := newGPGSigningMechanismInDirectory(testGPGHomeDirectory)
	require.NoError(t, err)
	err = mech.Close()
	assert.NoError(t, err)
	_, err = os.Lstat(testGPGHomeDirectory)
	assert.NoError(t, err)
	_, err = os.Lstat(filepath.Join(testGPGHomeDirectory, "pubring.gpg"))
	assert.NoError(t, err)

	// Closing an ephemeral mechanism does remove the directory.
	mech, _, err = NewEphemeralGPGSigningMechanism([]byte{})
	require.NoError(t, err)
	gpgMech, ok := mech.(*gpgSigningMechanism)
	require.True(t, ok)
	dir := gpgMech.ephemeralDir
	assert.NotEmpty(t, dir)
	_, err = os.Lstat(dir)
	require.NoError(t, err)
	err = mech.Close()
	assert.NoError(t, err)
	_, err = os.Lstat(dir)
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestGPGSigningMechanismSign(t *testing.T) {
	mech, err := newGPGSigningMechanismInDirectory(testGPGHomeDirectory)
	require.NoError(t, err)
	defer mech.Close()

	// Successful signing
	content := []byte("content")
	signature, err := mech.Sign(content, TestKeyFingerprint)
	require.NoError(t, err)

	signedContent, signingFingerprint, err := mech.Verify(signature)
	require.NoError(t, err)
	assert.EqualValues(t, content, signedContent)
	assert.Equal(t, TestKeyFingerprint, signingFingerprint)

	// Error signing
	_, err = mech.Sign(content, "this fingerprint doesn't exist")
	assert.Error(t, err)
	// The various GPG/GPGME failures cases are not obviously easy to reach.
}

func assertSigningError(t *testing.T, content []byte, fingerprint string, err error) {
	assert.Error(t, err)
	assert.Nil(t, content)
	assert.Empty(t, fingerprint)
}

func TestGPGSigningMechanismVerify(t *testing.T) {
	mech, err := newGPGSigningMechanismInDirectory(testGPGHomeDirectory)
	require.NoError(t, err)
	defer mech.Close()

	// Successful verification
	signature, err := ioutil.ReadFile("./fixtures/invalid-blob.signature")
	require.NoError(t, err)
	content, signingFingerprint, err := mech.Verify(signature)
	require.NoError(t, err)
	assert.Equal(t, []byte("This is not JSON\n"), content)
	assert.Equal(t, TestKeyFingerprint, signingFingerprint)

	// For extra paranoia, test that we return nil data on error.

	// Completely invalid signature.
	content, signingFingerprint, err = mech.Verify([]byte{})
	assertSigningError(t, content, signingFingerprint, err)

	content, signingFingerprint, err = mech.Verify([]byte("invalid signature"))
	assertSigningError(t, content, signingFingerprint, err)

	// Literal packet, not a signature
	signature, err = ioutil.ReadFile("./fixtures/unsigned-literal.signature")
	require.NoError(t, err)
	content, signingFingerprint, err = mech.Verify(signature)
	assertSigningError(t, content, signingFingerprint, err)

	// Encrypted data, not a signature.
	signature, err = ioutil.ReadFile("./fixtures/unsigned-encrypted.signature")
	require.NoError(t, err)
	content, signingFingerprint, err = mech.Verify(signature)
	assertSigningError(t, content, signingFingerprint, err)

	// FIXME? Is there a way to create a multi-signature so that gpgme_op_verify returns multiple signatures?

	// Expired signature
	signature, err = ioutil.ReadFile("./fixtures/expired.signature")
	require.NoError(t, err)
	content, signingFingerprint, err = mech.Verify(signature)
	assertSigningError(t, content, signingFingerprint, err)

	// Corrupt signature
	signature, err = ioutil.ReadFile("./fixtures/corrupt.signature")
	require.NoError(t, err)
	content, signingFingerprint, err = mech.Verify(signature)
	assertSigningError(t, content, signingFingerprint, err)

	// Valid signature with an unknown key
	signature, err = ioutil.ReadFile("./fixtures/unknown-key.signature")
	require.NoError(t, err)
	content, signingFingerprint, err = mech.Verify(signature)
	assertSigningError(t, content, signingFingerprint, err)

	// The various GPG/GPGME failures cases are not obviously easy to reach.
}

func TestGPGSigningMechanismUntrustedSignatureContents(t *testing.T) {
	mech, _, err := NewEphemeralGPGSigningMechanism([]byte{})
	require.NoError(t, err)
	defer mech.Close()

	// A valid signature
	signature, err := ioutil.ReadFile("./fixtures/invalid-blob.signature")
	require.NoError(t, err)
	content, shortKeyID, err := mech.UntrustedSignatureContents(signature)
	require.NoError(t, err)
	assert.Equal(t, []byte("This is not JSON\n"), content)
	assert.Equal(t, TestKeyShortID, shortKeyID)

	// Completely invalid signature.
	_, _, err = mech.UntrustedSignatureContents([]byte{})
	assert.Error(t, err)

	_, _, err = mech.UntrustedSignatureContents([]byte("invalid signature"))
	assert.Error(t, err)

	// Literal packet, not a signature
	signature, err = ioutil.ReadFile("./fixtures/unsigned-literal.signature")
	require.NoError(t, err)
	content, shortKeyID, err = mech.UntrustedSignatureContents(signature)
	assert.Error(t, err)

	// Encrypted data, not a signature.
	signature, err = ioutil.ReadFile("./fixtures/unsigned-encrypted.signature")
	require.NoError(t, err)
	content, shortKeyID, err = mech.UntrustedSignatureContents(signature)
	assert.Error(t, err)

	// Expired signature
	signature, err = ioutil.ReadFile("./fixtures/expired.signature")
	require.NoError(t, err)
	content, shortKeyID, err = mech.UntrustedSignatureContents(signature)
	require.NoError(t, err)
	assert.Equal(t, []byte("This signature is expired.\n"), content)
	assert.Equal(t, TestKeyShortID, shortKeyID)

	// Corrupt signature
	signature, err = ioutil.ReadFile("./fixtures/corrupt.signature")
	require.NoError(t, err)
	content, shortKeyID, err = mech.UntrustedSignatureContents(signature)
	require.NoError(t, err)
	assert.Equal(t, []byte(`{"critical":{"identity":{"docker-reference":"testing/manifest"},"image":{"docker-manifest-digest":"sha256:20bf21ed457b390829cdbeec8795a7bea1626991fda603e0d01b4e7f60427e55"},"type":"atomic container signature"},"optional":{"creator":"atomic ","timestamp":1458239713}}`), content)
	assert.Equal(t, TestKeyShortID, shortKeyID)

	// Valid signature with an unknown key
	signature, err = ioutil.ReadFile("./fixtures/unknown-key.signature")
	require.NoError(t, err)
	content, shortKeyID, err = mech.UntrustedSignatureContents(signature)
	require.NoError(t, err)
	assert.Equal(t, []byte(`{"critical":{"identity":{"docker-reference":"testing/manifest"},"image":{"docker-manifest-digest":"sha256:20bf21ed457b390829cdbeec8795a7bea1626991fda603e0d01b4e7f60427e55"},"type":"atomic container signature"},"optional":{"creator":"atomic 0.1.13-dev","timestamp":1464633474}}`), content)
	assert.Equal(t, "E5476D1110D07803", shortKeyID)
}
