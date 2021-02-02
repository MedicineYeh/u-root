// Copyright 2018 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stboot

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Signature contains the signature bytes and the
// corresponding certificate.
type Signature struct {
	Bytes []byte
	Cert  *x509.Certificate
}

// Write saves the signature and the certificate represented by s to files at
// a path named by dir. The filenames are composed of the first piece of the
// certificate's public key. The file extensions are '.signature' and '.cert'.
func (s *Signature) Write(dir string) error {
	stat, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !stat.IsDir() {
		return fmt.Errorf("not a directory: %s", dir)
	}

	id := fmt.Sprintf("%x", s.Cert.PublicKey)[2:18]
	sigName := fmt.Sprintf("%s.signature", id)
	sigPath := filepath.Join(dir, sigName)
	err = ioutil.WriteFile(sigPath, s.Bytes, os.ModePerm)
	if err != nil {
		return err
	}

	certName := fmt.Sprintf("%s.cert", id)
	certPath := filepath.Join(dir, certName)
	block := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: s.Cert.Raw,
	}
	var certBuf bytes.Buffer
	if err := pem.Encode(&certBuf, block); err != nil {
		return (err)
	}
	return ioutil.WriteFile(certPath, certBuf.Bytes(), os.ModePerm)
}

// Signer is used by OSPackage to hash, sign and varify the OSPackage.
type Signer interface {
	Sign(privKey string, data []byte) ([]byte, error)
	Verify(sig, hash []byte, cert *x509.Certificate) error
}

// DummySigner implements the Signer interface. It creates signatures
// that are always valid.
type DummySigner struct{}

// Sign returns a signature containing just 8 random bytes.
func (DummySigner) Sign(privKey string, data []byte) ([]byte, error) {
	sig := make([]byte, 8)
	_, err := rand.Read(sig)
	if err != nil {
		return nil, err
	}
	return sig, nil
}

// Verify will never return an error.
func (DummySigner) Verify(sig, hash []byte, cert *x509.Certificate) error {
	return nil
}

// Sha512PssSigner implements the Signer interface. It uses SHA512 hashes
// and PSS signatures along with x509 certificates.
type Sha512PssSigner struct{}

// Sign signes the provided data with the key named by privKey. The returned
// byte slice contains a PSS signature value.
func (Sha512PssSigner) Sign(privKey string, data []byte) ([]byte, error) {
	buf, err := ioutil.ReadFile(privKey)
	if err != nil {
		return nil, err
	}

	privPem, _ := pem.Decode(buf)
	key, err := x509.ParsePKCS1PrivateKey(privPem.Bytes)
	if err != nil {
		return nil, err
	}
	if key == nil {
		err = fmt.Errorf("key is empty")
		return nil, err
	}

	opts := &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash}

	sig, err := rsa.SignPSS(rand.Reader, key, crypto.SHA256, data, opts)
	if err != nil {
		return nil, err
	}
	if sig == nil {
		return nil, fmt.Errorf("signature is nil")
	}
	return sig, nil
}

// Verify checks if sig contains a valid signature of hash.
func (Sha512PssSigner) Verify(sig, hash []byte, cert *x509.Certificate) error {
	opts := &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash}
	err := rsa.VerifyPSS(cert.PublicKey.(*rsa.PublicKey), crypto.SHA512, hash, sig, opts)
	if err != nil {
		return fmt.Errorf("signature verification failed: %v", err)
	}
	return nil
}

// parseCertificate parses a x509 certificate from raw data.
func parseCertificate(raw []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(raw)
	return x509.ParseCertificate(block.Bytes)
}

// certPool returns a x509 certificate pool from PEM encoded data.
func certPool(pem []byte) (*x509.CertPool, error) {
	certPool := x509.NewCertPool()
	ok := certPool.AppendCertsFromPEM(pem)
	if !ok {
		return nil, errors.New("Failed to parse root certificate")
	}
	return certPool, nil
}

// ValidateCertificate validates cert against certPool. If cert is not signed
// by a certificate of certPool an error is returned.
func ValidateCertificate(cert *x509.Certificate, rootCertPEM []byte) error {
	certPool, err := certPool(rootCertPEM)
	if err != nil {
		return err
	}
	opts := x509.VerifyOptions{
		Roots: certPool,
	}
	_, err = cert.Verify(opts)
	return err
}
