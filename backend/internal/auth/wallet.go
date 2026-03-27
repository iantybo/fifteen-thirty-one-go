package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"fifteen-thirty-one-go/backend/internal/config"
	"fifteen-thirty-one-go/backend/internal/models"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

const walletChallengeVersion = "v1"
const walletChallengeParts = 4 // v1|ts|addr|hmac

// GenerateChallenge produces a stateless, time-bound challenge string for the given wallet address.
func GenerateChallenge(cfg config.Config, walletAddress string) (string, error) {
	normalized, err := models.NormalizeWalletAddress(walletAddress)
	if err != nil {
		return "", err
	}
	ts := time.Now().UTC().UnixMilli()
	mac := signWalletChallenge([]byte(cfg.WalletChallengeSecret), ts, normalized)
	return fmt.Sprintf("%s|%d|%s|%s", walletChallengeVersion, ts, normalized, hex.EncodeToString(mac)), nil
}

// VerifyChallenge validates the challenge HMAC, timestamp freshness, and that it matches walletAddress.
func VerifyChallenge(cfg config.Config, challenge, walletAddress string) error {
	normalized, err := models.NormalizeWalletAddress(walletAddress)
	if err != nil {
		return err
	}
	parts := strings.Split(challenge, "|")
	if len(parts) != walletChallengeParts || parts[0] != walletChallengeVersion {
		return errors.New("invalid challenge format")
	}
	tsMs, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return errors.New("invalid challenge timestamp")
	}
	addr := parts[2]
	macHex := parts[3]
	if addr != normalized {
		return errors.New("challenge wallet mismatch")
	}
	expected := signWalletChallenge([]byte(cfg.WalletChallengeSecret), tsMs, addr)
	got, err := hex.DecodeString(macHex)
	if err != nil {
		return errors.New("invalid challenge signature encoding")
	}
	if !hmac.Equal(expected, got) {
		return errors.New("invalid challenge signature")
	}
	issued := time.UnixMilli(tsMs)
	now := time.Now().UTC()
	if issued.After(now.Add(1 * time.Minute)) {
		return errors.New("challenge not yet valid")
	}
	if now.Sub(issued) > cfg.WalletChallengeTTL {
		return errors.New("challenge expired")
	}
	return nil
}

func signWalletChallenge(secret []byte, tsMs int64, normalizedAddr string) []byte {
	h := hmac.New(sha256.New, secret)
	_, _ = fmt.Fprintf(h, "%d|%s", tsMs, normalizedAddr)
	return h.Sum(nil)
}

// RecoverAddress recovers the Ethereum address from an EIP-191 personal_sign over message.
func RecoverAddress(message, signatureHex string) (string, error) {
	sig, err := hexutil.Decode(signatureHex)
	if err != nil {
		return "", fmt.Errorf("decode signature: %w", err)
	}
	if len(sig) != crypto.SignatureLength {
		return "", fmt.Errorf("invalid signature length: %d", len(sig))
	}
	// personal_sign uses recovery id 27 or 28 in the last byte for Ethereum wallets.
	if sig[crypto.RecoveryIDOffset] >= 27 {
		sig[crypto.RecoveryIDOffset] -= 27
	}
	hash := accounts.TextHash([]byte(message))
	pub, err := crypto.Ecrecover(hash, sig)
	if err != nil {
		return "", fmt.Errorf("ecrecover: %w", err)
	}
	pubKey, err := crypto.UnmarshalPubkey(pub)
	if err != nil {
		return "", fmt.Errorf("unmarshal pubkey: %w", err)
	}
	addr := crypto.PubkeyToAddress(*pubKey)
	// Store and compare as lowercase hex (0x + 40 hex).
	return strings.ToLower(addr.Hex()), nil
}
